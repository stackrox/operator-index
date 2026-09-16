package main

import (
	"fmt"
	"testing"

	semver "github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// versionStreamStrings formats a slice of versions as "MAJOR.MINOR" strings for comparison.
func versionStreamStrings(vs []*semver.Version) []string {
	if vs == nil {
		return nil
	}
	result := make([]string, len(vs))
	for i, v := range vs {
		result[i] = fmt.Sprintf("%d.%d", v.Major(), v.Minor())
	}
	return result
}

func TestParseNewVersionStreams(t *testing.T) {
	tests := []struct {
		name      string
		diff      string
		want      []string
		wantError bool
	}{
		{
			name: "single new version",
			diff: `
 images:
+  - image: registry.redhat.io/advanced-cluster-security/rhacs-operator-bundle@sha256:abc
+    version: 4.10.3
`,
			want: []string{"4.10"},
		},
		{
			name: "multiple new versions, same stream deduplicated",
			diff: `
+    version: 4.10.3
+    version: 4.10.4
+    version: 4.11.0
`,
			want: []string{"4.10", "4.11"},
		},
		{
			name: "oldest_supported_version change is ignored",
			diff: `
-oldest_supported_version: 4.8.0
+oldest_supported_version: 4.9.0
+    version: 4.11.0
`,
			want: []string{"4.11"},
		},
		{
			name: "no added version lines",
			diff: `
-    version: 4.9.0
`,
			want: nil,
		},
		{
			name: "empty diff",
			diff: "",
			want: nil,
		},
		{
			name:      "unparseable version is an error",
			diff:      "+    version: not-a-version\n",
			wantError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseNewVersionStreams(tt.diff)
			if tt.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, versionStreamStrings(got))
		})
	}
}

func TestToJSONArray(t *testing.T) {
	tests := []struct {
		items []string
		want  string
	}{
		{[]string{"4.10"}, `["4.10"]`},
		{[]string{"4.10", "4.11"}, `["4.10","4.11"]`},
		{[]string{}, `[]`},
	}
	for _, tt := range tests {
		got, err := toJSONArray(tt.items)
		require.NoError(t, err)
		assert.Equal(t, tt.want, got)
	}
}
