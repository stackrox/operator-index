package main

import (
	"os"
	"path/filepath"
	"testing"

	semver "github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadInputFile(t *testing.T) {
	tests := []struct {
		name          string
		filePath      string
		expectedError string
		validate      func(t *testing.T, config Configuration)
	}{
		{
			name:     "Valid input file",
			filePath: "testdata/valid_input.yaml",
			validate: func(t *testing.T, config Configuration) {
				assert.Equal(t, "4.0.0", config.OldestSupportedVersion.String())
				assert.Len(t, config.Images, 4)
				assert.Len(t, config.Versions, 4)

				assert.Equal(t, "example.com/image@sha256:6cdcf20771f9c46640b466f804190d00eaf2e59caee6d420436e78b283d177bf", config.Images[0].Image)
				assert.Equal(t, "3.62.0", config.Images[0].Version.String())

				assert.Equal(t, "3.62.0", config.Versions[0].String())
				assert.Equal(t, "4.0.0", config.Versions[1].String())
				assert.Equal(t, "4.1.0", config.Versions[2].String())
				assert.Equal(t, "4.2.0", config.Versions[3].String())
			},
		},
		{
			name:          "Invalid YAML format",
			filePath:      "testdata/invalid_yaml.yaml",
			expectedError: "failed to unmarshal YAML",
		},
		{
			name:          "Invalid oldest_supported_version",
			filePath:      "testdata/invalid_oldest_supported_version.yaml",
			expectedError: "invalid oldest_supported_version",
		},
		{
			name:          "Invalid image version",
			filePath:      "testdata/invalid_image_version.yaml",
			expectedError: "invalid version",
		},
		{
			name:          "Image reference without digest",
			filePath:      "testdata/image_without_digest.yaml",
			expectedError: "does not include a digest",
		},
		{
			name:          "Image reference is not a strict semantic version",
			filePath:      "testdata/not_strict_image_version.yaml",
			expectedError: "invalid semantic version",
		},
		{
			name:          "oldest_supported_version is not a strict semantic version",
			filePath:      "testdata/not_strict_oldest_supported_version.yaml",
			expectedError: "invalid semantic version",
		},
		{
			name:          "Non-existent file",
			filePath:      "testdata/non_existent.yaml",
			expectedError: "failed to read testdata/non_existent.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := readInputFile(tt.filePath)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, config)
				}
			}
		})
	}
}

func TestValidateImageReference(t *testing.T) {
	tests := []struct {
		name          string
		image         string
		expectedError string
	}{
		{
			name:  "Valid image reference with digest",
			image: "registry.example.com/repo/image@sha256:7fd7595e6a61352088f9a3a345be03a6c0b9caa0bbc5ddd8c61ba1d38b2c3b8e",
		},
		{
			name:  "Valid image reference with complex path",
			image: "quay.io/rhacs-eng/operator-bundle@sha256:abc123def456abc123def456abc123def456abc123def456abc123def456abcd",
		},
		{
			name:          "Empty image reference",
			image:         "",
			expectedError: "repository name must have at least one component",
		},
		{
			name:          "Image reference without digest",
			image:         "example.com/image:v1.0.0",
			expectedError: "does not include a digest",
		},
		{
			name:          "Image reference without digest or tag",
			image:         "example.com/image",
			expectedError: "does not include a digest",
		},
		{
			name:          "Image reference with unsupported digest algorithm",
			image:         "example.com/image@md5:9241e37fcf7f3f88c5e944bd46b0a268",
			expectedError: "unsupported digest algorithm",
		},
		{
			name:          "Image reference with invalid sha256 digest",
			image:         "example.com/image@sha256:invaliddigest",
			expectedError: "invalid reference format",
		},
		{
			name:          "Image reference with not sha256 digest algorithm",
			image:         "example.com/image@sha384:fdbd8e75a67f29f701a4e040385e2e23986303ea10239211af907fcbb83578b3e417cb71ce646efd0819dd8c088de1bd",
			expectedError: "digest algorithm is not sha256",
		},
		{
			name:          "Image reference without registry",
			image:         "bare-image-name-without-registry@sha256:6cdcf20771f9c46640b466f804190d00eaf2e59caee6d420436e78b283d177bf",
			expectedError: "needs the registry to be explicitly defined",
		},
		{
			name:          "Image reference with both tag and digest",
			image:         "registry.example.com/repo/image:v1.0.0@sha256:7fd7595e6a61352088f9a3a345be03a6c0b9caa0bbc5ddd8c61ba1d38b2c3b8e",
			expectedError: "should not contain a tag",
		},
		{
			name:          "Invalid characters in image reference",
			image:         "example.com/image@sha256:ZZZZ",
			expectedError: "invalid reference format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateImageReference(tt.image)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateImageReferences(t *testing.T) {
	tests := []struct {
		name          string
		images        []BundleImage
		expectedError string
	}{
		{
			name: "All valid images",
			images: []BundleImage{
				{
					Image:   "registry.example.com/repo/image@sha256:7fd7595e6a61352088f9a3a345be03a6c0b9caa0bbc5ddd8c61ba1d38b2c3b8e",
					Version: semver.MustParse("1.0.0"),
				},
				{
					Image:   "registry.example.com/repo/image@sha256:6cdcf20771f9c46640b466f804190d00eaf2e59caee6d420436e78b283d177bf",
					Version: semver.MustParse("1.0.1"),
				},
			},
		},
		{
			name: "One invalid image",
			images: []BundleImage{
				{
					Image:   "registry.example.com/repo/image@sha256:7fd7595e6a61352088f9a3a345be03a6c0b9caa0bbc5ddd8c61ba1d38b2c3b8e",
					Version: semver.MustParse("1.0.0"),
				},
				{
					Image:   "example.com/image:v1.0.0",
					Version: semver.MustParse("1.0.1"),
				},
			},
			expectedError: "does not include a digest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateImageReferences(tt.images)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetAllVersions(t *testing.T) {
	images := []BundleImage{
		{Image: "img1", Version: semver.MustParse("1.0.0")},
		{Image: "img2", Version: semver.MustParse("2.0.0")},
		{Image: "img3", Version: semver.MustParse("3.0.0")},
	}

	versions := getAllVersions(images)

	assert.Len(t, versions, 3)
	assert.Equal(t, "1.0.0", versions[0].String())
	assert.Equal(t, "2.0.0", versions[1].String())
	assert.Equal(t, "3.0.0", versions[2].String())
}

func TestValidateVersionsAreSorted(t *testing.T) {
	tests := []struct {
		name          string
		versions      []*semver.Version
		expectedError string
	}{
		{
			name: "Sorted versions",
			versions: []*semver.Version{
				semver.MustParse("1.0.0"),
				semver.MustParse("1.0.1"),
				semver.MustParse("1.1.0"),
				semver.MustParse("2.0.0"),
			},
		},
		{
			name: "Single version",
			versions: []*semver.Version{
				semver.MustParse("1.0.0"),
			},
		},
		{
			name:     "Empty versions",
			versions: []*semver.Version{},
		},
		{
			name: "Unsorted versions",
			versions: []*semver.Version{
				semver.MustParse("2.0.0"),
				semver.MustParse("1.0.0"),
			},
			expectedError: "versions are not sorted in ascending order",
		},
		{
			name: "Duplicate versions",
			versions: []*semver.Version{
				semver.MustParse("1.0.0"),
				semver.MustParse("1.0.0"),
			},
			expectedError: "versions are not sorted in ascending order",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateVersionsAreSorted(tt.versions)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHasGapInVersions(t *testing.T) {
	tests := []struct {
		name          string
		versions      []*semver.Version
		expectedError string
	}{
		{
			name: "No gaps in patch versions",
			versions: []*semver.Version{
				semver.MustParse("1.0.0"),
				semver.MustParse("1.0.1"),
				semver.MustParse("1.0.2"),
			},
		},
		{
			name: "No gaps in minor versions",
			versions: []*semver.Version{
				semver.MustParse("1.0.0"),
				semver.MustParse("1.1.0"),
				semver.MustParse("1.2.0"),
			},
		},
		{
			name: "No gaps in major versions",
			versions: []*semver.Version{
				semver.MustParse("1.0.0"),
				semver.MustParse("2.0.0"),
				semver.MustParse("3.0.0"),
			},
		},
		{
			name: "Mixed version increments without gaps",
			versions: []*semver.Version{
				semver.MustParse("1.0.0"),
				semver.MustParse("1.0.1"),
				semver.MustParse("1.1.0"),
				semver.MustParse("2.0.0"),
			},
		},
		{
			name: "Gap in patch versions",
			versions: []*semver.Version{
				semver.MustParse("1.0.0"),
				semver.MustParse("1.0.2"),
			},
			expectedError: "unexpected version sequence",
		},
		{
			name: "Gap in minor versions",
			versions: []*semver.Version{
				semver.MustParse("1.0.0"),
				semver.MustParse("1.2.0"),
			},
			expectedError: "unexpected version sequence",
		},
		{
			name: "Gap in major versions",
			versions: []*semver.Version{
				semver.MustParse("1.0.0"),
				semver.MustParse("3.0.0"),
			},
			expectedError: "unexpected version sequence",
		},
		{
			name: "Single version has no gaps",
			versions: []*semver.Version{
				semver.MustParse("1.0.0"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := hasGapInVersions(tt.versions)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGenerateChannels(t *testing.T) {
	tests := []struct {
		name             string
		versions         []*semver.Version
		expectedChannels int
		validate         func(t *testing.T, channels []Channel)
	}{
		{
			name: "Single Y-stream versions",
			versions: []*semver.Version{
				semver.MustParse("3.62.0"),
				semver.MustParse("3.62.1"),
				semver.MustParse("3.62.2"),
			},
			expectedChannels: 1,
			validate: func(t *testing.T, channels []Channel) {
				assert.Equal(t, "rhacs-3.62", channels[0].Name)
			},
		},
		{
			name: "Multiple Y-stream versions",
			versions: []*semver.Version{
				semver.MustParse("3.62.0"),
				semver.MustParse("3.62.1"),
				semver.MustParse("4.0.0"),
				semver.MustParse("4.0.1"),
				semver.MustParse("4.1.0"),
			},
			expectedChannels: 3,
			validate: func(t *testing.T, channels []Channel) {
				assert.Equal(t, "rhacs-3.62", channels[0].Name)
				assert.Equal(t, "rhacs-4.0", channels[1].Name)
				assert.Equal(t, "rhacs-4.1", channels[2].Name)
			},
		},
		{
			name:             "No versions",
			versions:         []*semver.Version{},
			expectedChannels: 0,
		},
		{
			name: "Major version jump",
			versions: []*semver.Version{
				semver.MustParse("3.62.0"),
				semver.MustParse("4.0.0"),
				semver.MustParse("5.0.0"),
			},
			expectedChannels: 3,
			validate: func(t *testing.T, channels []Channel) {
				assert.Equal(t, "rhacs-3.62", channels[0].Name)
				assert.Equal(t, "rhacs-4.0", channels[1].Name)
				assert.Equal(t, "rhacs-5.0", channels[2].Name)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channels := generateChannels(tt.versions)

			assert.Len(t, channels, tt.expectedChannels)
			if tt.validate != nil {
				tt.validate(t, channels)
			}
		})
	}
}

func TestGenerateChannelEntries(t *testing.T) {
	tests := []struct {
		name             string
		versions         []*semver.Version
		startingVersions []*semver.Version
		expectedEntries  int
		validate         func(t *testing.T, entries []ChannelEntry)
	}{
		{
			name: "Simple version sequence with first version at the start",
			versions: []*semver.Version{
				semver.MustParse("3.62.0"),
				semver.MustParse("3.62.1"),
			},
			startingVersions: []*semver.Version{
				semver.MustParse("3.62.0"),
			},
			expectedEntries: 2,
			validate: func(t *testing.T, entries []ChannelEntry) {
				// First entry should not have replaces
				assert.Equal(t, "rhacs-operator.v3.62.0", entries[0].Name)
				assert.Empty(t, entries[0].Replaces)
				assert.Equal(t, ">= 3.61.0 < 3.62.0", entries[0].SkipRange)

				assert.Equal(t, "rhacs-operator.v3.62.1", entries[1].Name)
				assert.Equal(t, "rhacs-operator.v3.62.0", entries[1].Replaces)
				assert.Equal(t, ">= 3.61.0 < 3.62.1", entries[1].SkipRange)
			},
		},
		{
			name: "Version sequence across Y-streams",
			versions: []*semver.Version{
				semver.MustParse("4.0.0"),
				semver.MustParse("4.0.1"),
				semver.MustParse("4.0.2"),
				semver.MustParse("4.1.0"),
			},
			startingVersions: []*semver.Version{
				semver.MustParse("4.0.0"),
			},
			expectedEntries: 4,
			validate: func(t *testing.T, entries []ChannelEntry) {
				assert.Equal(t, "rhacs-operator.v4.0.0", entries[0].Name)
				assert.Empty(t, entries[0].Replaces)
				assert.Equal(t, ">= 3.61.0 < 4.0.0", entries[0].SkipRange)

				assert.Equal(t, "rhacs-operator.v4.0.1", entries[1].Name)
				assert.Equal(t, "rhacs-operator.v4.0.0", entries[1].Replaces)
				assert.Equal(t, ">= 3.61.0 < 4.0.1", entries[1].SkipRange)

				assert.Equal(t, "rhacs-operator.v4.1.0", entries[3].Name)
				assert.Equal(t, "rhacs-operator.v4.0.2", entries[3].Replaces)
				assert.Equal(t, ">= 4.0.0 < 4.1.0", entries[3].SkipRange)
			},
		},
		{
			name: "Major version transition - 4.0.0 is starting version",
			versions: []*semver.Version{
				semver.MustParse("3.62.0"),
				semver.MustParse("3.62.1"),
				semver.MustParse("4.0.0"),
			},
			startingVersions: []*semver.Version{
				semver.MustParse("3.62.0"),
				semver.MustParse("4.0.0"),
			},
			expectedEntries: 3,
			validate: func(t *testing.T, entries []ChannelEntry) {
				assert.Equal(t, "rhacs-operator.v3.62.0", entries[0].Name)
				assert.Empty(t, entries[0].Replaces)

				assert.Equal(t, "rhacs-operator.v3.62.1", entries[1].Name)
				assert.Equal(t, "rhacs-operator.v3.62.0", entries[1].Replaces)

				assert.Equal(t, "rhacs-operator.v4.0.0", entries[2].Name)
				assert.Empty(t, entries[2].Replaces)
				assert.Equal(t, ">= 3.62.0 < 4.0.0", entries[2].SkipRange)
			},
		},
		{
			name: "SkipRange changes at Y-stream boundaries",
			versions: []*semver.Version{
				semver.MustParse("4.0.0"),
				semver.MustParse("4.0.1"),
				semver.MustParse("4.0.2"),
				semver.MustParse("4.1.0"),
				semver.MustParse("4.1.1"),
			},
			startingVersions: []*semver.Version{
				semver.MustParse("4.0.0"),
			},
			expectedEntries: 5,
			validate: func(t *testing.T, entries []ChannelEntry) {
				assert.Equal(t, ">= 4.0.0 < 4.1.0", entries[3].SkipRange)
				assert.Equal(t, ">= 4.0.0 < 4.1.1", entries[4].SkipRange)
			},
		},
		{
			name: "Multiple starting versions",
			versions: []*semver.Version{
				semver.MustParse("3.62.0"),
				semver.MustParse("4.0.0"),
				semver.MustParse("4.1.0"),
			},
			startingVersions: []*semver.Version{
				semver.MustParse("3.62.0"),
				semver.MustParse("4.0.0"),
			},
			expectedEntries: 3,
			validate: func(t *testing.T, entries []ChannelEntry) {
				// Both 3.62.0 and 4.0.0 are starting versions, should not have replaces
				assert.Empty(t, entries[0].Replaces) // 3.62.0
				assert.Empty(t, entries[1].Replaces) // 4.0.0

				// 4.1.0 should have replaces
				assert.Equal(t, "rhacs-operator.v4.0.0", entries[2].Replaces) // 4.1.0
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries := generateChannelEntries(tt.versions, tt.startingVersions)

			assert.Len(t, entries, tt.expectedEntries)
			if tt.validate != nil {
				tt.validate(t, entries)
			}
		})
	}
}

func TestGenerateDeprecations(t *testing.T) {
	tests := []struct {
		name                   string
		versions               []*semver.Version
		channels               []Channel
		oldestSupportedVersion *semver.Version
		validate               func(t *testing.T, deprecations Deprecations)
	}{
		{
			name: "Deprecate old channel and old bundle",
			versions: []*semver.Version{
				semver.MustParse("3.62.0"),
				semver.MustParse("4.0.0"),
				semver.MustParse("4.1.0"),
			},
			channels: []Channel{
				{Name: "rhacs-3.62", yStreamVersion: semver.MustParse("3.62.0")},
				{Name: "rhacs-4.0", yStreamVersion: semver.MustParse("4.0.0")},
			},
			oldestSupportedVersion: semver.MustParse("4.0.0"),
			validate: func(t *testing.T, deprecations Deprecations) {
				// Should have latest channel deprecation + old channel + old bundle
				assert.GreaterOrEqual(t, len(deprecations.Entries), 3)

				// Check that 3.62.0 is deprecated as a bundle
				hasOldBundleDeprecation := false
				for _, entry := range deprecations.Entries {
					if entry.Reference.Schema == olmBundleSchema && entry.Reference.Name == "rhacs-operator.v3.62.0" {
						hasOldBundleDeprecation = true
						assert.Contains(t, entry.Message, "no longer supported")
					}
				}
				assert.True(t, hasOldBundleDeprecation, "Should deprecate old bundle 3.62.0")
			},
		},
		{
			name: "Latest channel is always deprecated",
			versions: []*semver.Version{
				semver.MustParse("4.0.0"),
			},
			channels: []Channel{
				{Name: "latest"},
			},
			oldestSupportedVersion: semver.MustParse("4.0.0"),
			validate: func(t *testing.T, deprecations Deprecations) {
				hasLatestDeprecation := false
				for _, entry := range deprecations.Entries {
					if entry.Reference.Schema == olmChannelSchema && entry.Reference.Name == "latest" {
						hasLatestDeprecation = true
					}
				}
				assert.True(t, hasLatestDeprecation, "Latest channel should be deprecated")
			},
		},
		{
			name: "No deprecations when all versions are supported",
			versions: []*semver.Version{
				semver.MustParse("4.0.0"),
				semver.MustParse("4.1.0"),
			},
			channels: []Channel{
				{Name: "rhacs-4.0", yStreamVersion: semver.MustParse("4.0.0")},
				{Name: "rhacs-4.1", yStreamVersion: semver.MustParse("4.1.0")},
			},
			oldestSupportedVersion: semver.MustParse("4.0.0"),
			validate: func(t *testing.T, deprecations Deprecations) {
				assert.Equal(t, 1, len(deprecations.Entries))
				assert.Equal(t, olmChannelSchema, deprecations.Entries[0].Reference.Schema)
				assert.Equal(t, "latest", deprecations.Entries[0].Reference.Name)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deprecations := generateDeprecations(tt.versions, tt.channels, tt.oldestSupportedVersion)

			assert.Equal(t, olmDeprecationsSchema, deprecations.Schema)
			assert.Equal(t, rhacsOperator, deprecations.Package)
			if tt.validate != nil {
				tt.validate(t, deprecations)
			}
		})
	}
}

func TestGenerateBundles(t *testing.T) {
	images := []BundleImage{
		{Image: "registry.io/bundle1@sha256:abc123", Version: semver.MustParse("1.0.0")},
		{Image: "registry.io/bundle2@sha256:def456", Version: semver.MustParse("2.0.0")},
	}

	bundles := generateBundles(images)

	assert.Len(t, bundles, 2)
	assert.Equal(t, olmBundleSchema, bundles[0].Schema)
	assert.Equal(t, "registry.io/bundle1@sha256:abc123", bundles[0].Image)
	assert.Equal(t, "registry.io/bundle2@sha256:def456", bundles[1].Image)
}

func TestPopulateChannels(t *testing.T) {
	latestTree := newChannelTree("latest", semver.MustParse("3.62.0"), semver.MustParse("4.0.0"))
	stableTree := newChannelTree("stable", semver.MustParse("4.0.0"), semver.MustParse("9999.0.0"))
	trees := []*ChannelTree{&latestTree, &stableTree}

	channels := []Channel{
		{Name: "rhacs-3.62", yStreamVersion: semver.MustParse("3.62.0")},
		{Name: "rhacs-4.0", yStreamVersion: semver.MustParse("4.0.0")},
		{Name: "rhacs-4.1", yStreamVersion: semver.MustParse("4.1.0")},
	}

	populateChannels(trees, channels)

	assert.Len(t, latestTree.YStreamChannels, 1)
	assert.Equal(t, "rhacs-3.62", latestTree.YStreamChannels[0].Name)

	assert.Len(t, stableTree.YStreamChannels, 2)
	assert.Equal(t, "rhacs-4.0", stableTree.YStreamChannels[0].Name)
	assert.Equal(t, "rhacs-4.1", stableTree.YStreamChannels[1].Name)
}

func TestPopulateChannelEntries(t *testing.T) {
	latestTree := newChannelTree("latest", semver.MustParse("3.62.0"), semver.MustParse("4.0.0"))
	latestTree.YStreamChannels = []Channel{
		{Name: "rhacs-3.62", yStreamVersion: semver.MustParse("3.62.0")},
	}

	stableTree := newChannelTree("stable", semver.MustParse("4.0.0"), semver.MustParse("9999.0.0"))
	stableTree.YStreamChannels = []Channel{
		{Name: "rhacs-4.0", yStreamVersion: semver.MustParse("4.0.0")},
		{Name: "rhacs-4.1", yStreamVersion: semver.MustParse("4.1.0")},
		{Name: "rhacs-5.0", yStreamVersion: semver.MustParse("5.0.0")},
	}

	entries := []ChannelEntry{
		{Name: "rhacs-operator.v3.62.0", version: semver.MustParse("3.62.0")},
		{Name: "rhacs-operator.v3.62.1", version: semver.MustParse("3.62.1")},
		{Name: "rhacs-operator.v4.0.0", version: semver.MustParse("4.0.0")},
		{Name: "rhacs-operator.v4.0.1", version: semver.MustParse("4.0.1")},
		{Name: "rhacs-operator.v4.1.0", version: semver.MustParse("4.1.0")},
		{Name: "rhacs-operator.v5.0.0", version: semver.MustParse("5.0.0")},
		{Name: "rhacs-operator.v5.0.1", version: semver.MustParse("5.0.1")},
	}

	trees := []*ChannelTree{&latestTree, &stableTree}
	populateChannelEntries(trees, entries)

	assert.Len(t, latestTree.YStreamChannels, 1)
	assert.Equal(t, "rhacs-3.62", latestTree.YStreamChannels[0].Name)

	assert.Len(t, latestTree.YStreamChannels[0].Entries, 2)
	assert.Equal(t, "rhacs-operator.v3.62.0", latestTree.YStreamChannels[0].Entries[0].Name)
	assert.Equal(t, "rhacs-operator.v3.62.1", latestTree.YStreamChannels[0].Entries[1].Name)

	assert.Len(t, latestTree.MainChannel.Entries, 2)
	assert.Equal(t, "rhacs-operator.v3.62.0", latestTree.MainChannel.Entries[0].Name)
	assert.Equal(t, "rhacs-operator.v3.62.1", latestTree.MainChannel.Entries[1].Name)

	assert.Len(t, stableTree.YStreamChannels, 3)

	assert.Equal(t, "rhacs-4.0", stableTree.YStreamChannels[0].Name)
	assert.Len(t, stableTree.YStreamChannels[0].Entries, 2)
	assert.Equal(t, "rhacs-operator.v4.0.0", stableTree.YStreamChannels[0].Entries[0].Name)
	assert.Equal(t, "rhacs-operator.v4.0.1", stableTree.YStreamChannels[0].Entries[1].Name)

	assert.Equal(t, "rhacs-4.1", stableTree.YStreamChannels[1].Name)
	assert.Len(t, stableTree.YStreamChannels[1].Entries, 3)
	assert.Equal(t, "rhacs-operator.v4.0.0", stableTree.YStreamChannels[1].Entries[0].Name)
	assert.Equal(t, "rhacs-operator.v4.0.1", stableTree.YStreamChannels[1].Entries[1].Name)
	assert.Equal(t, "rhacs-operator.v4.1.0", stableTree.YStreamChannels[1].Entries[2].Name)

	assert.Equal(t, "rhacs-5.0", stableTree.YStreamChannels[2].Name)
	assert.Len(t, stableTree.YStreamChannels[2].Entries, 5)
	assert.Equal(t, "rhacs-operator.v4.0.0", stableTree.YStreamChannels[2].Entries[0].Name)
	assert.Equal(t, "rhacs-operator.v4.0.1", stableTree.YStreamChannels[2].Entries[1].Name)
	assert.Equal(t, "rhacs-operator.v4.1.0", stableTree.YStreamChannels[2].Entries[2].Name)
	assert.Equal(t, "rhacs-operator.v5.0.0", stableTree.YStreamChannels[2].Entries[3].Name)
	assert.Equal(t, "rhacs-operator.v5.0.1", stableTree.YStreamChannels[2].Entries[4].Name)

	assert.Len(t, stableTree.MainChannel.Entries, 5)
	assert.Equal(t, "rhacs-operator.v4.0.0", stableTree.MainChannel.Entries[0].Name)
	assert.Equal(t, "rhacs-operator.v4.0.1", stableTree.MainChannel.Entries[1].Name)
	assert.Equal(t, "rhacs-operator.v4.1.0", stableTree.MainChannel.Entries[2].Name)
	assert.Equal(t, "rhacs-operator.v5.0.0", stableTree.MainChannel.Entries[3].Name)
	assert.Equal(t, "rhacs-operator.v5.0.1", stableTree.MainChannel.Entries[4].Name)
}

func TestFlattenChannelsFromTrees(t *testing.T) {
	latestTree := newChannelTree("latest", semver.MustParse("3.62.0"), semver.MustParse("4.0.0"))
	latestTree.YStreamChannels = []Channel{
		{Name: "rhacs-3.62"},
	}

	stableTree := newChannelTree("stable", semver.MustParse("4.0.0"), semver.MustParse("9999.0.0"))
	stableTree.YStreamChannels = []Channel{
		{Name: "rhacs-4.0"},
		{Name: "rhacs-4.1"},
	}

	trees := []*ChannelTree{&latestTree, &stableTree}
	channels := flattenChannelsFromTrees(trees)

	// Should have 4 channels total: rhacs-3.62, latest, rhacs-4.0, rhacs-4.1, stable
	assert.Len(t, channels, 5)
}

func TestChannelShouldHaveEntry(t *testing.T) {
	tests := []struct {
		name     string
		channel  Channel
		entry    ChannelEntry
		expected bool
	}{
		{
			name:     "Entry belongs to channel",
			channel:  Channel{yStreamVersion: semver.MustParse("4.0.0")},
			entry:    ChannelEntry{version: semver.MustParse("4.0.1")},
			expected: true,
		},
		{
			name:     "Entry is exact Y-stream version",
			channel:  Channel{yStreamVersion: semver.MustParse("4.0.0")},
			entry:    ChannelEntry{version: semver.MustParse("4.0.0")},
			expected: true,
		},
		{
			name:     "Entry is from newer Y-stream",
			channel:  Channel{yStreamVersion: semver.MustParse("4.0.0")},
			entry:    ChannelEntry{version: semver.MustParse("4.1.0")},
			expected: false,
		},
		{
			name:     "Entry minor equals channel minor",
			channel:  Channel{yStreamVersion: semver.MustParse("4.1.0")},
			entry:    ChannelEntry{version: semver.MustParse("4.1.5")},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := channelShouldHaveEntry(tt.channel, tt.entry)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWriteToFile(t *testing.T) {
	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "test_output.yaml")

	ct := newCatalogTemplate()
	pkg := Package{
		Schema:         olmPackageSchema,
		Name:           rhacsOperator,
		DefaultChannel: "stable",
		Icon:           Icon{Base64data: "dGVzdA==", MediaType: "image/png"},
	}
	ct.addPackage(pkg)

	err := writeToFile(outputPath, ct)
	require.NoError(t, err)

	_, err = os.Stat(outputPath)
	assert.NoError(t, err)

	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	assert.Contains(t, string(content), "DO NOT EDIT")
	assert.Contains(t, string(content), olmTemplateSchema)
	assert.Contains(t, string(content), rhacsOperator)
}
