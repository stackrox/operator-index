package upgradetest

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	yaml "github.com/goccy/go-yaml"
)

func ocRun(args ...string) error {
	cmd := exec.Command("oc", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func ocOutput(args ...string) (string, error) {
	var stderr bytes.Buffer
	cmd := exec.Command("oc", args...)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil && stderr.Len() > 0 {
		return strings.TrimSpace(string(out)), fmt.Errorf("%w\nstderr: %s", err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(string(out)), err
}

func ocApply(manifest string) error {
	cmd := exec.Command("oc", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(manifest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

type bundlesYAML struct {
	OldestSupportedVersion string `yaml:"oldest_supported_version"`
}

// ReadOldestSupportedVersion parses oldest_supported_version from bundles.yaml.
func ReadOldestSupportedVersion() (major, minor int, err error) {
	data, err := os.ReadFile("../bundles.yaml")
	if err != nil {
		return 0, 0, fmt.Errorf("reading bundles.yaml: %w", err)
	}
	var b bundlesYAML
	if err := yaml.Unmarshal(data, &b); err != nil {
		return 0, 0, fmt.Errorf("parsing bundles.yaml: %w", err)
	}
	return ParseACSVersion(b.OldestSupportedVersion)
}

// ParseACSVersion parses "4.10" or "4.10.3" or "4.10.0-rc.1" into (major=4, minor=10).
func ParseACSVersion(ver string) (major, minor int, err error) {
	parts := strings.SplitN(ver, ".", 3)
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("ACS_VERSION must be MAJOR.MINOR (e.g. 4.10), got: %q", ver)
	}
	maj, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid major in %q: %w", ver, err)
	}
	min, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid minor in %q: %w", ver, err)
	}
	return maj, min, nil
}

// GetLatestOfficialMinor returns the highest available minor for the given major
// from the official redhat-operators catalog, retrying until timeout because
// packagemanifests can lag behind the catalog's READY state.
func GetLatestOfficialMinor(major int, timeout time.Duration) (int, error) {
	deadline := time.Now().Add(timeout)
	prefix := fmt.Sprintf("rhacs-%d.", major)
	fmt.Printf("  Looking for latest rhacs-%d.* channel in redhat-operators (timeout: %v)...\n", major, timeout)
	for time.Now().Before(deadline) {
		out, err := exec.Command("oc", "get", "packagemanifest",
			"-n", "openshift-marketplace",
			"-l", "catalog=redhat-operators",
			"-o", `jsonpath={range .items[?(@.metadata.name=="rhacs-operator")].status.channels[*]}{.name}{"\n"}{end}`,
		).Output()
		if err != nil {
			fmt.Printf("  packagemanifest query failed: %v, retrying in 10s...\n", err)
			time.Sleep(10 * time.Second)
			continue
		}
		maxMinor := -1
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if !strings.HasPrefix(line, prefix) {
				continue
			}
			n, err := strconv.Atoi(strings.TrimPrefix(line, prefix))
			if err != nil {
				continue
			}
			if n > maxMinor {
				maxMinor = n
			}
		}
		if maxMinor >= 0 {
			return maxMinor, nil
		}
		fmt.Printf("  rhacs-%d.* channels not yet in packagemanifest, retrying in 10s...\n", major)
		time.Sleep(10 * time.Second)
	}
	return 0, fmt.Errorf("no rhacs-%d.* channels found in redhat-operators after %v", major, timeout)
}

// DisableDefaultSources disables all default OperatorHub catalog sources.
func DisableDefaultSources() error {
	fmt.Println("  Disabling default OperatorHub sources...")
	return ocRun("patch", "OperatorHub", "cluster",
		"--type", "json",
		"-p", `[{"op":"add","path":"/spec/disableAllDefaultSources","value":true}]`)
}

// EnableDefaultSources re-enables all default OperatorHub catalog sources.
func EnableDefaultSources() error {
	fmt.Println("  Enabling default OperatorHub sources...")
	return ocRun("patch", "OperatorHub", "cluster",
		"--type", "json",
		"-p", `[{"op":"add","path":"/spec/disableAllDefaultSources","value":false}]`)
}

// ApplyCustomCatalog creates or updates the custom CatalogSource.
func ApplyCustomCatalog(indexImage string) error {
	fmt.Printf("  Applying custom CatalogSource (image: %s)...\n", indexImage)
	return ocApply(fmt.Sprintf(`apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: my-operator-catalog
  namespace: openshift-marketplace
spec:
  sourceType: grpc
  image: %s
  displayName: My Operator Catalog
  publisher: Custom`, indexImage))
}

// WaitForCatalog polls until the named CatalogSource reaches READY state.
func WaitForCatalog(name, ns string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	fmt.Printf("  Waiting for CatalogSource/%s to be READY (timeout: %v)...\n", name, timeout)
	for time.Now().Before(deadline) {
		state, _ := ocOutput("get", "catalogsource", name, "-n", ns,
			"-o", "jsonpath={.status.connectionState.lastObservedState}")
		if state == "READY" {
			fmt.Printf("  ✅ CatalogSource/%s is READY\n", name)
			return nil
		}
		fmt.Printf("  state=%s, retrying in 10s...\n", state)
		time.Sleep(10 * time.Second)
	}
	return fmt.Errorf("CatalogSource/%s not READY within %v", name, timeout)
}

// ApplySubscription creates or updates the rhacs-operator Subscription.
func ApplySubscription(channel, source, sourceNS string) error {
	fmt.Printf("  Applying Subscription (channel=%s, source=%s)...\n", channel, source)
	return ocApply(fmt.Sprintf(`apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: rhacs-operator
  namespace: openshift-operators
spec:
  channel: %s
  installPlanApproval: Automatic
  name: rhacs-operator
  source: %s
  sourceNamespace: %s`, channel, source, sourceNS))
}

// ResolveTargetCSV returns the currentCSV for a channel from the specified catalog,
// retrying until timeout because packagemanifests can lag behind catalog READY.
func ResolveTargetCSV(catalogLabel, channel string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	fmt.Printf("  Resolving currentCSV for channel %s from %s...\n", channel, catalogLabel)
	jsonpath := fmt.Sprintf(
		`{range .items[?(@.metadata.name=="rhacs-operator")].status.channels[?(@.name=="%s")]}{.currentCSV}{end}`,
		channel)
	for time.Now().Before(deadline) {
		csv, _ := ocOutput("get", "packagemanifest",
			"-n", "openshift-marketplace",
			"-l", "catalog="+catalogLabel,
			"-o", "jsonpath="+jsonpath)
		if csv != "" {
			fmt.Printf("  Target CSV: %s\n", csv)
			return csv, nil
		}
		fmt.Println("  packagemanifest not ready yet, retrying in 10s...")
		time.Sleep(10 * time.Second)
	}
	return "", fmt.Errorf("could not resolve currentCSV for channel %s from %s after %v",
		channel, catalogLabel, timeout)
}

// InstallFromOfficial installs the ACS Operator from redhat-operators at channel rhacs-MAJOR.MINOR.
func InstallFromOfficial(major, minor int) (targetCSV string, err error) {
	channel := fmt.Sprintf("rhacs-%d.%d", major, minor)
	fmt.Printf("  Installing ACS Operator from redhat-operators, channel %s...\n", channel)
	if err := WaitForCatalog("redhat-operators", "openshift-marketplace", 2*time.Minute); err != nil {
		return "", err
	}
	csv, err := ResolveTargetCSV("redhat-operators", channel, 2*time.Minute)
	if err != nil {
		return "", err
	}
	if err := ApplySubscription(channel, "redhat-operators", "openshift-marketplace"); err != nil {
		return "", err
	}
	return csv, nil
}

// applyCustomCatalogAndSubscribe is the shared implementation for InstallFromCustom and UpgradeViaCustom.
func applyCustomCatalogAndSubscribe(indexImage, channel string) (string, error) {
	if err := DisableDefaultSources(); err != nil {
		return "", err
	}
	if err := ApplyCustomCatalog(indexImage); err != nil {
		return "", err
	}
	if err := WaitForCatalog("my-operator-catalog", "openshift-marketplace", 3*time.Minute); err != nil {
		return "", err
	}
	csv, err := ResolveTargetCSV("my-operator-catalog", channel, time.Minute)
	if err != nil {
		return "", err
	}
	if err := ApplySubscription(channel, "my-operator-catalog", "openshift-marketplace"); err != nil {
		return "", err
	}
	return csv, nil
}

// InstallFromCustom installs from the custom index on the given channel, disabling default sources.
func InstallFromCustom(indexImage, channel string) (targetCSV string, err error) {
	fmt.Printf("  Installing ACS Operator from custom index, channel %s...\n", channel)
	return applyCustomCatalogAndSubscribe(indexImage, channel)
}

// UpgradeViaCustom upgrades by refreshing the custom CatalogSource and updating the subscription.
func UpgradeViaCustom(indexImage, channel string) (targetCSV string, err error) {
	fmt.Printf("  Upgrading via custom catalog (channel: %s)...\n", channel)
	csv, err := applyCustomCatalogAndSubscribe(indexImage, channel)
	if err != nil {
		return "", err
	}
	fmt.Printf("  ✅ Subscription updated — OLM will upgrade to %s\n", csv)
	return csv, nil
}

// UpgradeToLatestOfficial upgrades to the latest GA via redhat-operators.
func UpgradeToLatestOfficial(major int) (targetCSV string, err error) {
	if err := EnableDefaultSources(); err != nil {
		return "", err
	}
	if err := WaitForCatalog("redhat-operators", "openshift-marketplace", 3*time.Minute); err != nil {
		return "", err
	}
	latestMinor, err := GetLatestOfficialMinor(major, 3*time.Minute)
	if err != nil {
		return "", err
	}
	channel := fmt.Sprintf("rhacs-%d.%d", major, latestMinor)
	fmt.Printf("  Upgrading to latest GA channel: %s...\n", channel)
	csv, err := ResolveTargetCSV("redhat-operators", channel, 3*time.Minute)
	if err != nil {
		return "", err
	}
	if err := ApplySubscription(channel, "redhat-operators", "openshift-marketplace"); err != nil {
		return "", err
	}
	fmt.Printf("  ✅ Subscription updated to %s, target: %s\n", channel, csv)
	return csv, nil
}

// WaitForCSV polls until the target CSV reaches Succeeded phase.
// It correctly handles multi-hop OLM upgrade graphs (e.g. 4.8→4.9→4.10).
func WaitForCSV(targetCSV string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	fmt.Printf("  Waiting for %s to reach Succeeded (timeout: %v)...\n", targetCSV, timeout)
	for time.Now().Before(deadline) {
		phase, _ := ocOutput("get", "csv", targetCSV,
			"-n", "openshift-operators",
			"-o", "jsonpath={.status.phase}")
		if phase == "Succeeded" {
			fmt.Printf("  ✅ %s is Succeeded\n", targetCSV)
			return nil
		}
		if phase == "Failed" {
			return fmt.Errorf("%s reached Failed phase — OLM will not recover without a resource change", targetCSV)
		}
		// Show all in-progress rhacs CSVs so logs show the hop chain.
		progress, _ := ocOutput("get", "csv", "-n", "openshift-operators", "--no-headers")
		fmt.Printf("  phase=%s, csvs=%s, waiting 15s...\n", phase,
			strings.ReplaceAll(progress, "\n", " | "))
		time.Sleep(15 * time.Second)
	}
	return fmt.Errorf("%s did not reach Succeeded within %v", targetCSV, timeout)
}

// ResetOperator removes the operator subscription, CSVs, and custom catalog between tests.
func ResetOperator() error {
	fmt.Println("  Resetting operator state...")
	_ = ocRun("delete", "subscription", "rhacs-operator",
		"-n", "openshift-operators", "--ignore-not-found")
	out, _ := ocOutput("get", "csv", "-n", "openshift-operators", "--no-headers")
	for _, line := range strings.Split(out, "\n") {
		if fields := strings.Fields(line); len(fields) > 0 && strings.HasPrefix(fields[0], "rhacs-operator.") {
			_ = ocRun("delete", "csv", fields[0],
				"-n", "openshift-operators", "--ignore-not-found")
		}
	}
	_ = ocRun("delete", "catalogsource", "my-operator-catalog",
		"-n", "openshift-marketplace", "--ignore-not-found")
	return EnableDefaultSources()
}
