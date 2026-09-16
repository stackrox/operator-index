package main

import (
	"encoding/json"
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

// ocpTarget pairs an OCP version with the operator-index image built for it.
// JSON field names match the GHA workflow inputs so matrix.ocp-target.* works directly.
type ocpTarget struct {
	OCPVersion string `json:"ocp-version"`
	Image      string `json:"operator-index-image"`
}

func newOCPTarget(v *semver.Version, sha string) ocpTarget {
	tag := fmt.Sprintf("ocp-v%d-%d-%s-fast", v.Major(), v.Minor(), sha)
	return ocpTarget{
		OCPVersion: fmt.Sprintf("%d.%d", v.Major(), v.Minor()),
		Image:      fmt.Sprintf("%s:%s", operatorIndexImage, tag),
	}
}


// workflowInputs holds all outputs written to $GITHUB_OUTPUT when the upgrade test should run.
type workflowInputs struct {
	versionStreams []*semver.Version
	ocpTargets    []ocpTarget
}

func newWorkflowInputs(streams []*semver.Version, targets []ocpTarget) *workflowInputs {
	return &workflowInputs{versionStreams: streams, ocpTargets: targets}
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
	targetsJSON, err := json.Marshal(t.ocpTargets)
	if err != nil {
		return fmt.Errorf("encode ocp-targets: %w", err)
	}
	writeOutput("version-streams", versionsJSON)
	writeOutput("ocp-targets", string(targetsJSON))
	return nil
}
