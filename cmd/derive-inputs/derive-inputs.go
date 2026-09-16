// derive-inputs derives the inputs for the operator-index upgrade test workflow.
// It merges two previously bash-based steps:
//  1. Determine ACS version(s) from bundles.yaml git diff.
//  2. Determine operator-index image from .tekton/ pipeline files.
//
// All outputs are written as KEY=VALUE lines to stdout so the caller can pipe
// them directly to $GITHUB_OUTPUT.
// Diagnostic messages go to stderr so they appear in GHA step logs.
//
// Required env vars (set from GitHub Actions context):
//
//	VERSION_STREAM_OVERRIDE       – skip bundles.yaml diff, use this version
//	OPERATOR_INDEX_IMAGE_OVERRIDE – skip image derivation, use this image
//	BASE_REF                      – base branch ref (used for pull_request diff base)
//	EVENT_NAME                    – github.event_name
//	SHA                           – commit SHA for building the image tag
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	semver "github.com/Masterminds/semver/v3"
)

func main() {
	if err := run(); err != nil {
		// ::error:: renders as a GHA step annotation in the UI
		logf("::error::%v", err)
		os.Exit(1)
	}
}

// run is the top-level coordinator. It reads env vars, picks a derive path,
// and delegates output writing to workflowInputs.write.
func run() error {
	versionStreamOverride := strings.TrimSpace(os.Getenv("VERSION_STREAM_OVERRIDE"))
	imageOverride := strings.TrimSpace(os.Getenv("OPERATOR_INDEX_IMAGE_OVERRIDE"))
	baseRef := strings.TrimSpace(os.Getenv("BASE_REF"))
	eventName := strings.TrimSpace(os.Getenv("EVENT_NAME"))
	sha := strings.TrimSpace(os.Getenv("SHA"))

	if imageOverride != "" {
		if versionStreamOverride == "" {
			return fmt.Errorf("OPERATOR_INDEX_IMAGE_OVERRIDE requires VERSION_STREAM_OVERRIDE to also be set")
		}
		inputs, err := inputsFromOverrides(versionStreamOverride, imageOverride)
		if err != nil {
			return err
		}
		return inputs.write()
	}

	inputs, err := inputsFromGit(versionStreamOverride, eventName, baseRef, sha)
	if err != nil {
		return err
	}
	if inputs == nil {
		logf("No new bundle versions in diff, skipping upgrade test")
		return nil
	}
	return inputs.write()
}

// inputsFromOverrides builds workflowInputs directly from the provided overrides,
// skipping git diff and .tekton/ derivation.
func inputsFromOverrides(versionStreamOverride, imageOverride string) (*workflowInputs, error) {
	logf("Both overrides provided, skipping derivation")
	v, err := semver.NewVersion(versionStreamOverride)
	if err != nil {
		return nil, fmt.Errorf("invalid VERSION_STREAM_OVERRIDE %q: %w", versionStreamOverride, err)
	}
	return &workflowInputs{
		versionStreams:     []*semver.Version{v},
		operatorIndexImage: imageOverride,
		ocpVersion:         "",
	}, nil
}

// inputsFromGit derives workflowInputs from the git diff and .tekton/ pipeline files.
// Returns nil, nil when the diff contains no new bundle versions (test should not run).
func inputsFromGit(versionStreamOverride, eventName, baseRef, sha string) (*workflowInputs, error) {
	streams, err := deriveVersionStreams(versionStreamOverride, eventName, baseRef)
	if err != nil {
		return nil, err
	}
	if len(streams) == 0 {
		return nil, nil
	}

	ocp, err := deriveOCPVersion()
	if err != nil {
		return nil, err
	}
	ocpTag := fmt.Sprintf("v%d-%d", ocp.Major(), ocp.Minor())
	tag := fmt.Sprintf("ocp-%s-%s-fast", ocpTag, sha)
	image := fmt.Sprintf("%s:%s", operatorIndexImage, tag)
	logf("Derived OCP version: %s\nImage: %s", ocpTag, image)

	return &workflowInputs{
		versionStreams:     streams,
		operatorIndexImage: image,
		ocpVersion:         fmt.Sprintf("%d.%d", ocp.Major(), ocp.Minor()),
	}, nil
}

// writeOutput writes a KEY=VALUE line in the GitHub Actions output format.
// The caller appends stdout to $GITHUB_OUTPUT to expose values to subsequent steps.
func writeOutput(key, value string) {
	fmt.Printf("%s=%s\n", key, value)
}

// logf writes a diagnostic progress message to stderr so it appears in the GHA step log.
// These are informational only; fatal errors go through fmt.Errorf.
func logf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

// toJSONArray encodes a string slice as a JSON array string.
func toJSONArray(items []string) (string, error) {
	b, err := json.Marshal(items)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// deriveVersionStreams returns deduplicated semver versions (one per unique MAJOR.MINOR)
// for the ACS versions to test: either the single override, or all new versions
// added in the diff.
func deriveVersionStreams(versionStreamOverride, eventName, baseRef string) ([]*semver.Version, error) {
	if versionStreamOverride != "" {
		v, err := semver.NewVersion(versionStreamOverride)
		if err != nil {
			return nil, fmt.Errorf("invalid VERSION_STREAM_OVERRIDE %q: %w", versionStreamOverride, err)
		}
		logf("Version override: %s (stream: %d.%d)", versionStreamOverride, v.Major(), v.Minor())
		return []*semver.Version{v}, nil
	}

	diffBase := "HEAD~1"
	if eventName == "pull_request" {
		diffBase = "origin/" + baseRef
	}
	diffOutput, err := gitDiff(diffBase)
	if err != nil {
		return nil, err
	}
	streams, err := parseNewVersionStreams(diffOutput)
	if err != nil {
		return nil, err
	}
	if len(streams) > 0 {
		names := make([]string, len(streams))
		for i, v := range streams {
			names[i] = fmt.Sprintf("%d.%d", v.Major(), v.Minor())
		}
		logf("New version streams to test: %s", strings.Join(names, ", "))
	}
	return streams, nil
}

// gitDiff runs "git diff <diffBase> -- bundles.yaml" and returns its stdout.
func gitDiff(diffBase string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("git", "diff", diffBase, "--", bundlesYAML)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git diff %s -- %s: %w\nstdout: %s\nstderr: %s",
			diffBase, bundlesYAML, err,
			strings.TrimSpace(stdout.String()),
			strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// parseNewVersionStreams parses git diff output and returns deduplicated semver versions
// (one per unique MAJOR.MINOR) for every bundle version line added to bundles.yaml.
//
// A line is a bundle version entry when it:
//   - starts with "+" (addition in the diff),
//   - is indented (ruling out top-level keys like "oldest_supported_version:"),
//   - has "version:" as the key,
//   - has a value that semver can parse.
func parseNewVersionStreams(diffOutput string) ([]*semver.Version, error) {
	const versionKey = "version:"
	seen := make(map[string]bool)
	var streams []*semver.Version
	for _, line := range strings.Split(diffOutput, "\n") {
		if !strings.HasPrefix(line, "+") {
			continue
		}
		// Strip the leading "+" and check for indentation. Top-level keys are
		// not indented, so trimming must remove at least one whitespace character.
		rest := line[1:]
		trimmed := strings.TrimLeft(rest, " \t")
		if trimmed == rest || !strings.HasPrefix(trimmed, versionKey) {
			continue
		}
		value := strings.TrimSpace(trimmed[len(versionKey):])
		v, err := semver.NewVersion(value)
		if err != nil {
			return nil, fmt.Errorf("unparseable version %q in %s diff: %w", value, bundlesYAML, err)
		}
		stream := fmt.Sprintf("%d.%d", v.Major(), v.Minor())
		if !seen[stream] {
			seen[stream] = true
			streams = append(streams, v)
		}
	}
	return streams, nil
}

// deriveOCPVersion reads .tekton/ to find the highest OCP version across all
// build pipelines, e.g. "v4-22" from "operator-index-ocp-v4-22-build.yaml".
func deriveOCPVersion() (*semver.Version, error) {
	pattern := filepath.Join(tektonDir, "operator-index-ocp-v*-build.yaml")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob %s: %w", pattern, err)
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no files matching %s found", pattern)
	}

	var highest *semver.Version
	for _, f := range matches {
		m := ocpVersionFromFilename.FindStringSubmatch(filepath.Base(f))
		if m == nil {
			continue
		}
		v, err := semver.NewVersion(fmt.Sprintf("%s.%s.0", m[1], m[2]))
		if err != nil {
			return nil, fmt.Errorf("parse OCP version from %s: %w", filepath.Base(f), err)
		}
		if highest == nil || v.GreaterThan(highest) {
			highest = v
		}
	}
	if highest == nil {
		return nil, fmt.Errorf("could not parse OCP version from any file in %s", tektonDir)
	}
	return highest, nil
}
