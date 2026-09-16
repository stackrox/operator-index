package main

import (
	"fmt"
	"regexp"

	semver "github.com/Masterminds/semver/v3"
)

const (
	operatorIndexImage = "quay.io/rhacs-eng/stackrox-operator-index"
	bundlesYAML        = "bundles.yaml"
	tektonDir          = ".tekton"
)

// ocpVersionFromFilename extracts the OCP major and minor from a .tekton
// pipeline filename like "operator-index-ocp-v4-22-build.yaml".
var ocpVersionFromFilename = regexp.MustCompile(`operator-index-ocp-v(\d+)-(\d+)-build\.yaml$`)

// workflowInputs holds all outputs written to $GITHUB_OUTPUT when the upgrade test should run.
type workflowInputs struct {
	versionStreams      []*semver.Version
	operatorIndexImage string
	ocpVersion         string // empty when not pinned; upgrade-test workflow treats "" as "use default"
}

// write emits all outputs as KEY=VALUE lines to stdout.
func (t *workflowInputs) write() error {
	streamStrs := make([]string, len(t.versionStreams))
	for i, v := range t.versionStreams {
		streamStrs[i] = fmt.Sprintf("%d.%d", v.Major(), v.Minor())
	}
	versionsJSON, err := toJSONArray(streamStrs)
	if err != nil {
		return fmt.Errorf("encode version-streams: %w", err)
	}
	writeOutput("version-streams", versionsJSON)
	writeOutput("operator-index-image", t.operatorIndexImage)
	writeOutput("ocp-version", t.ocpVersion)
	return nil
}
