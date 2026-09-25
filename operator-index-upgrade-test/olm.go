package upgradetest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	semver "github.com/Masterminds/semver/v3"
	yaml "github.com/goccy/go-yaml"
)

type packageManifestList struct {
	Items []struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Status struct {
			Channels []struct {
				Name       string `json:"name"`
				CurrentCSV string `json:"currentCSV"`
			} `json:"channels"`
		} `json:"status"`
	} `json:"items"`
}

func ocRun(args ...string) error {
	cmd := exec.Command("oc", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func ocOutput(args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("oc", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("%w\nstdout: %s\nstderr: %s", err,
			strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
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
func ReadOldestSupportedVersion() (*semver.Version, error) {
	data, err := os.ReadFile("../bundles.yaml")
	if err != nil {
		return nil, fmt.Errorf("reading bundles.yaml: %w", err)
	}
	var b bundlesYAML
	if err := yaml.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("parsing bundles.yaml: %w", err)
	}
	v, err := semver.NewVersion(b.OldestSupportedVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid oldest_supported_version %q: %w", b.OldestSupportedVersion, err)
	}
	return v, nil
}

// ParseVersionStream parses a MAJOR.MINOR version stream string (e.g. "4.10") into a semver.Version.
func ParseVersionStream(ver string) (*semver.Version, error) {
	if strings.Count(ver, ".") != 1 {
		return nil, fmt.Errorf("version stream must be MAJOR.MINOR (e.g. 4.10), got: %q", ver)
	}
	v, err := semver.NewVersion(ver)
	if err != nil {
		return nil, fmt.Errorf("invalid version stream %q: %w", ver, err)
	}
	return v, nil
}

// GetLatestOfficialStream returns the highest available version stream across all rhacs-MAJOR.MINOR
// channels in the official redhat-operators catalog, retrying until timeout because
// packagemanifests can lag behind the catalog's READY state.
func GetLatestOfficialStream(timeout time.Duration) (*semver.Version, error) {
	deadline := time.Now().Add(timeout)
	fmt.Printf("  Looking for latest rhacs-*.* channel in redhat-operators (timeout: %v)...\n", timeout)
	for time.Now().Before(deadline) {
		out, err := exec.Command("oc", "get", "packagemanifest",
			"-n", "openshift-marketplace",
			"-l", "catalog=redhat-operators",
			"-o", "json",
		).Output()
		if err != nil {
			fmt.Printf("  packagemanifest query failed: %v, retrying in 10s...\n", err)
			time.Sleep(10 * time.Second)
			continue
		}
		var list packageManifestList
		if err := json.Unmarshal(out, &list); err != nil {
			fmt.Printf("  failed to parse packagemanifest JSON: %v, retrying in 10s...\n", err)
			time.Sleep(10 * time.Second)
			continue
		}
		var latest *semver.Version
		for _, item := range list.Items {
			if item.Metadata.Name != "rhacs-operator" {
				continue
			}
			for _, ch := range item.Status.Channels {
				if !strings.HasPrefix(ch.Name, "rhacs-") {
					continue
				}
				s, err := ParseVersionStream(strings.TrimPrefix(ch.Name, "rhacs-"))
				if err != nil {
					continue
				}
				if latest == nil || s.GreaterThan(latest) {
					latest = s
				}
			}
		}
		if latest != nil {
			return latest, nil
		}
		fmt.Println("  rhacs-*.* channels not yet in packagemanifest, retrying in 10s...")
		time.Sleep(10 * time.Second)
	}
	return nil, fmt.Errorf("no rhacs-*.* channels found in redhat-operators after %v", timeout)
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
	for time.Now().Before(deadline) {
		out, err := exec.Command("oc", "get", "packagemanifest",
			"-n", "openshift-marketplace",
			"-l", "catalog="+catalogLabel,
			"-o", "json",
		).Output()
		if err == nil {
			var list packageManifestList
			if err := json.Unmarshal(out, &list); err == nil {
				for _, item := range list.Items {
					if item.Metadata.Name != "rhacs-operator" {
						continue
					}
					for _, ch := range item.Status.Channels {
						if ch.Name == channel && ch.CurrentCSV != "" {
							fmt.Printf("  Target CSV: %s\n", ch.CurrentCSV)
							return ch.CurrentCSV, nil
						}
					}
				}
			}
		}
		fmt.Println("  packagemanifest not ready yet, retrying in 10s...")
		time.Sleep(10 * time.Second)
	}
	return "", fmt.Errorf("could not resolve currentCSV for channel %s from %s after %v",
		channel, catalogLabel, timeout)
}

// InstallFromOfficial installs the ACS Operator from redhat-operators at the given stream's channel.
func InstallFromOfficial(stream *semver.Version) (targetCSV string, err error) {
	channel := fmt.Sprintf("rhacs-%d.%d", stream.Major(), stream.Minor())
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

// UpgradeToLatestOfficial upgrades to the given GA stream via redhat-operators.
func UpgradeToLatestOfficial(target *semver.Version) (targetCSV string, err error) {
	if err := EnableDefaultSources(); err != nil {
		return "", err
	}
	if err := WaitForCatalog("redhat-operators", "openshift-marketplace", 3*time.Minute); err != nil {
		return "", err
	}
	channel := fmt.Sprintf("rhacs-%d.%d", target.Major(), target.Minor())
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
	var errs []error
	if err := ocRun("delete", "subscription", "rhacs-operator",
		"-n", "openshift-operators", "--ignore-not-found"); err != nil {
		errs = append(errs, fmt.Errorf("delete subscription: %w", err))
	}
	out, err := ocOutput("get", "csv", "-n", "openshift-operators", "--no-headers")
	if err != nil {
		errs = append(errs, fmt.Errorf("list CSVs: %w", err))
	}
	for _, line := range strings.Split(out, "\n") {
		if fields := strings.Fields(line); len(fields) > 0 && strings.HasPrefix(fields[0], "rhacs-operator.") {
			if err := ocRun("delete", "csv", fields[0],
				"-n", "openshift-operators", "--ignore-not-found"); err != nil {
				errs = append(errs, fmt.Errorf("delete csv %s: %w", fields[0], err))
			}
		}
	}
	if err := ocRun("delete", "catalogsource", "my-operator-catalog",
		"-n", "openshift-marketplace", "--ignore-not-found"); err != nil {
		errs = append(errs, fmt.Errorf("delete catalogsource: %w", err))
	}
	if err := EnableDefaultSources(); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}
