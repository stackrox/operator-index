package main

import (
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
				tt.validate(t, config)
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

func TestGetAllVersions(t *testing.T) {
	images := []BundleImage{
		{Image: "img1", Version: v("1.0.0")},
		{Image: "img2", Version: v("2.0.0")},
		{Image: "img3", Version: v("3.0.0")},
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
				v("1.0.0"),
				v("1.0.1"),
				v("1.1.0"),
				v("2.0.0"),
				v("3.0.0-pre.1"),
				v("3.0.0"),
			},
		},
		{
			name: "Single version",
			versions: []*semver.Version{
				v("1.0.0"),
			},
		},
		{
			name:     "Empty versions",
			versions: []*semver.Version{},
		},
		{
			name: "Unsorted versions",
			versions: []*semver.Version{
				v("2.0.0"),
				v("1.0.0"),
			},
			expectedError: "versions are not sorted in ascending order",
		},
		{
			name: "Duplicate versions",
			versions: []*semver.Version{
				v("1.0.0"),
				v("1.0.0"),
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
				v("1.0.0"),
				v("1.0.1"),
				v("1.0.2"),
			},
		},
		{
			name: "No gaps in minor versions",
			versions: []*semver.Version{
				v("1.0.0"),
				v("1.1.0"),
				v("1.2.0"),
			},
		},
		{
			name: "No gaps in major versions",
			versions: []*semver.Version{
				v("1.0.0"),
				v("2.0.0"),
				v("3.0.0"),
			},
		},
		{
			name: "Mixed version increments without gaps",
			versions: []*semver.Version{
				v("1.0.0"),
				v("1.0.1"),
				v("1.1.0"),
				v("2.0.0"),
			},
		},
		{
			name: "Gap in patch versions",
			versions: []*semver.Version{
				v("1.0.0"),
				v("1.0.2"),
			},
			expectedError: "unexpected version sequence",
		},
		{
			name: "Gap in minor versions",
			versions: []*semver.Version{
				v("1.0.0"),
				v("1.2.0"),
			},
			expectedError: "unexpected version sequence",
		},
		{
			name: "Gap in major versions",
			versions: []*semver.Version{
				v("1.0.0"),
				v("3.0.0"),
			},
			expectedError: "unexpected version sequence",
		},
		{
			name: "Single version has no gaps",
			versions: []*semver.Version{
				v("1.0.0"),
			},
		},
		{
			name:     "Empty versions slice has no gaps",
			versions: []*semver.Version{},
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

func TestGenerateEmptyChannels(t *testing.T) {
	tests := []struct {
		name             string
		versions         []*semver.Version
		expectedChannels []string
	}{
		{
			name: "Single Y-stream versions",
			versions: []*semver.Version{
				v("3.62.0"),
				v("3.62.1"),
				v("3.62.2"),
			},
			expectedChannels: []string{"rhacs-3.62"},
		},
		{
			name: "Multiple Y-stream versions",
			versions: []*semver.Version{
				v("3.62.0"),
				v("3.62.1"),
				v("4.0.0"),
				v("4.0.1"),
				v("4.1.0"),
			},
			expectedChannels: []string{"rhacs-3.62", "rhacs-4.0", "rhacs-4.1"},
		},
		{
			name:             "No versions",
			versions:         []*semver.Version{},
			expectedChannels: []string{},
		},
		{
			name: "Major version jump",
			versions: []*semver.Version{
				v("3.62.0"),
				v("4.0.0"),
				v("5.0.0"),
			},
			expectedChannels: []string{"rhacs-3.62", "rhacs-4.0", "rhacs-5.0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channels := generateEmptyChannels(tt.versions)
			assert.Equal(t, tt.expectedChannels, collectChannelNames(channels))
		})
	}
}

func TestGenerateChannelEntries(t *testing.T) {
	versions := []*semver.Version{
		v("3.62.0"),
		v("3.62.1"),
		v("4.0.0"),
		v("4.0.1"),
		v("4.0.2"),
		v("4.1.0"),
		v("4.1.1"),
	}
	entries := generateChannelEntries(versions)

	assert.Len(t, entries, len(versions))
	// Clearing `Replaces` for the starting version is done in a different function, so we expect it to be set here.
	assertChannelEntry(t, entries[0], "rhacs-operator.v3.62.0", "rhacs-operator.v3.61.0", ">= 3.61.0 < 3.62.0")
	assertChannelEntry(t, entries[1], "rhacs-operator.v3.62.1", "rhacs-operator.v3.62.0", ">= 3.61.0 < 3.62.1")
	assertChannelEntry(t, entries[2], "rhacs-operator.v4.0.0", "rhacs-operator.v3.62.1", ">= 3.62.0 < 4.0.0")
	assertChannelEntry(t, entries[3], "rhacs-operator.v4.0.1", "rhacs-operator.v4.0.0", ">= 3.62.0 < 4.0.1")
	assertChannelEntry(t, entries[4], "rhacs-operator.v4.0.2", "rhacs-operator.v4.0.1", ">= 3.62.0 < 4.0.2")
	assertChannelEntry(t, entries[5], "rhacs-operator.v4.1.0", "rhacs-operator.v4.0.2", ">= 4.0.0 < 4.1.0")
	assertChannelEntry(t, entries[6], "rhacs-operator.v4.1.1", "rhacs-operator.v4.1.0", ">= 4.0.0 < 4.1.1")
}

func TestGenerateDeprecations(t *testing.T) {
	versions := []*semver.Version{
		v("3.62.0"),
		v("3.62.1"),
		v("4.0.0"),
		v("4.1.0"),
	}
	channels := []Channel{
		channel("3.62.0"),
		{Name: "latest"},
		channel("4.0.0"),
		channel("4.1.0"),
		{Name: "stable"},
	}
	oldestSupportedVersion := v("4.0.0")

	deprecations := generateDeprecations(versions, channels, oldestSupportedVersion)

	assert.Equal(t, olmDeprecationsSchema, deprecations.Schema)
	assert.Equal(t, rhacsOperator, deprecations.Package)

	assert.Len(t, deprecations.Entries, 4)
	assertContainsDeprecationEntry(t, deprecations.Entries, olmChannelSchema, "latest", latestChannelDeprecationMessage)
	assertContainsDeprecationEntry(t, deprecations.Entries, olmChannelSchema, "rhacs-3.62", channelDeprecationMessage)
	assertContainsDeprecationEntry(t, deprecations.Entries, olmBundleSchema, "rhacs-operator.v3.62.0", bundleDeprecationMessage)
	assertContainsDeprecationEntry(t, deprecations.Entries, olmBundleSchema, "rhacs-operator.v3.62.1", bundleDeprecationMessage)
}

func TestGenerateBundles(t *testing.T) {
	images := []BundleImage{
		{Image: "registry.io/bundle1@sha256:abc123", Version: v("1.0.0")},
		{Image: "registry.io/bundle2@sha256:def456", Version: v("2.0.0")},
	}

	bundles := generateBundles(images)

	assert.Len(t, bundles, len(images))
	assert.Equal(t, olmBundleSchema, bundles[0].Schema)
	assert.Equal(t, olmBundleSchema, bundles[1].Schema)
	assert.Equal(t, "registry.io/bundle1@sha256:abc123", bundles[0].Image)
	assert.Equal(t, "registry.io/bundle2@sha256:def456", bundles[1].Image)
}

func TestAssignChannels(t *testing.T) {
	latestLineage := newChannelLineage("latest", v("3.62.0"), v("4.0.0"))
	stableLineage := newChannelLineage("stable", v("4.0.0"), v("9999.0.0"))
	lineages := []channelLineage{latestLineage, stableLineage}

	channels := []Channel{
		channel("3.62.0"),
		channel("4.0.0"),
		channel("4.1.0"),
	}

	assignChannels(lineages, channels)

	assert.Len(t, lineages[0].YStreamChannels, 1)
	assert.Equal(t, "latest", lineages[0].MainChannel.Name)
	assert.Equal(t, "rhacs-3.62", lineages[0].YStreamChannels[0].Name)

	assert.Len(t, lineages[1].YStreamChannels, 2)
	assert.Equal(t, "stable", lineages[1].MainChannel.Name)
	assert.Equal(t, "rhacs-4.0", lineages[1].YStreamChannels[0].Name)
	assert.Equal(t, "rhacs-4.1", lineages[1].YStreamChannels[1].Name)
}

func TestAssignChannelEntries(t *testing.T) {
	latestLineage := newChannelLineage("latest", v("3.62.0"), v("4.0.0"))
	latestLineage.YStreamChannels = []Channel{
		channel("3.62.0"),
	}

	stableLineage := newChannelLineage("stable", v("4.0.0"), v("9999.0.0"))
	stableLineage.YStreamChannels = []Channel{
		channel("4.0.0"),
		channel("4.1.0"),
		channel("5.0.0"),
	}

	entries := []ChannelEntry{
		// "latest" Entries
		{version: v("3.62.0")},
		{version: v("3.62.1")},
		// "stable" Entries
		{version: v("4.0.0")},
		{version: v("4.0.1")},
		{version: v("4.1.0")},
		{version: v("5.0.0")},
		{version: v("5.0.1")},
	}

	lineages := []channelLineage{latestLineage, stableLineage}
	assignChannelEntries(lineages, entries)

	latestLineage = lineages[0]
	stableLineage = lineages[1]

	// Verify contents of "latest" lineage
	assert.Len(t, latestLineage.YStreamChannels, 1)

	assert.Equal(t, "rhacs-3.62", latestLineage.YStreamChannels[0].Name)
	assertChannelContents(t, latestLineage.YStreamChannels[0], "3.62.0", "3.62.1")

	assert.Equal(t, latestChannelName, latestLineage.MainChannel.Name)
	assertChannelContents(t, latestLineage.MainChannel, "3.62.0", "3.62.1")

	// Verify contents of "stable" lineage
	assert.Len(t, stableLineage.YStreamChannels, 3)

	assert.Equal(t, "rhacs-4.0", stableLineage.YStreamChannels[0].Name)
	assertChannelContents(t, stableLineage.YStreamChannels[0], "4.0.0", "4.0.1")

	assert.Equal(t, "rhacs-4.1", stableLineage.YStreamChannels[1].Name)
	assertChannelContents(t, stableLineage.YStreamChannels[1], "4.0.0", "4.0.1", "4.1.0")

	assert.Equal(t, "rhacs-5.0", stableLineage.YStreamChannels[2].Name)
	assertChannelContents(t, stableLineage.YStreamChannels[2], "4.0.0", "4.0.1", "4.1.0", "5.0.0", "5.0.1")

	assert.Equal(t, stableChannelName, stableLineage.MainChannel.Name)
	assertChannelContents(t, stableLineage.MainChannel, "4.0.0", "4.0.1", "4.1.0", "5.0.0", "5.0.1")
}

func TestFlattenChannels(t *testing.T) {
	latestLineage := newChannelLineage("latest", v("3.62.0"), v("4.0.0"))
	latestLineage.YStreamChannels = []Channel{
		{Name: "rhacs-3.62"},
	}

	stableLineage := newChannelLineage("stable", v("4.0.0"), v("9999.0.0"))
	stableLineage.YStreamChannels = []Channel{
		channel("4.0"),
		channel("4.1"),
	}

	lineages := []channelLineage{latestLineage, stableLineage}
	channels := flattenChannels(lineages)

	expectedChannels := []string{"rhacs-3.62", "latest", "rhacs-4.0", "rhacs-4.1", "stable"}
	assert.Len(t, channels, len(expectedChannels))
	assert.Equal(t, expectedChannels, collectChannelNames(channels))
}

func TestClearReplacesForStartingEntries(t *testing.T) {
	channels := []Channel{
		{
			Name:           "rhacs-3.62",
			yStreamVersion: v("3.62.0"),
			Entries: []ChannelEntry{
				{Name: "rhacs-operator.v3.62.0", Replaces: "rhacs-operator.v3.61.0", version: v("3.62.0")},
			},
		},
		{
			Name:           "rhacs-4.0",
			yStreamVersion: v("4.0.0"),
			Entries: []ChannelEntry{
				{Name: "rhacs-operator.v4.0.0", Replaces: "rhacs-operator.v3.62.1", version: v("4.0.0")},
				{Name: "rhacs-operator.v4.0.1", Replaces: "rhacs-operator.v4.0.0", version: v("4.0.1")},
				{Name: "rhacs-operator.v4.0.2", Replaces: "rhacs-operator.v4.0.1", version: v("4.0.2")},
			},
		},
		{
			Name:           "rhacs-4.1",
			yStreamVersion: v("4.1.0"),
			Entries:        []ChannelEntry{},
		},
	}

	adjustStartingEntries(channels)

	assert.Empty(t, channels[0].Entries[0].Replaces)

	assert.Empty(t, channels[1].Entries[0].Replaces)
	assert.Equal(t, "rhacs-operator.v4.0.0", channels[1].Entries[1].Replaces)
	assert.Equal(t, "rhacs-operator.v4.0.1", channels[1].Entries[2].Replaces)

	assert.Len(t, channels[2].Entries, 0)
}

func TestChannelShouldHaveEntry(t *testing.T) {
	tests := []struct {
		name           string
		channelVersion string
		entryVersion   string
		expectedToHave bool
	}{
		{
			name:           "Entry belongs to channel",
			channelVersion: "4.0.0",
			entryVersion:   "4.0.1",
			expectedToHave: true,
		},
		{
			name:           "Entry is exact Y-stream version",
			channelVersion: "4.0.0",
			entryVersion:   "4.0.0",
			expectedToHave: true,
		},
		{
			name:           "Entry is from newer Y-stream",
			channelVersion: "4.0.0",
			entryVersion:   "4.1.0",
			expectedToHave: false,
		},
		{
			name:           "Entry is from older Major version",
			channelVersion: "5.0.0",
			entryVersion:   "4.0.0",
			expectedToHave: true,
		},
		{
			name:           "Entry is from older Y-stream",
			channelVersion: "4.1.0",
			entryVersion:   "4.0.6",
			expectedToHave: true,
		},
		{
			name:           "Entry minor equals channel minor",
			channelVersion: "4.1.0",
			entryVersion:   "4.1.5",
			expectedToHave: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel := channel(tt.channelVersion)
			entry := ChannelEntry{version: v(tt.entryVersion)}

			result := channelShouldHaveEntry(channel, entry)

			assert.Equal(t, tt.expectedToHave, result)
		})
	}
}

// v is a shorthand for semver.MustParse
func v(version string) *semver.Version {
	return semver.MustParse(version)
}

func channel(yStreamVersion string) Channel {
	return newChannel(v(yStreamVersion))
}

func collectChannelNames(channels []Channel) []string {
	names := make([]string, len(channels))
	for i, ch := range channels {
		names[i] = ch.Name
	}
	return names
}

func assertChannelEntry(t *testing.T, entry ChannelEntry, name, replaces, skipRange string) {
	t.Helper()
	assert.Equal(t, name, entry.Name)
	assert.Equal(t, replaces, entry.Replaces)
	assert.Equal(t, skipRange, entry.SkipRange)
}

func assertChannelContents(t *testing.T, channel Channel, expectedVersions ...string) {
	t.Helper()
	assert.Len(t, channel.Entries, len(expectedVersions))
	actualVersions := make([]string, len(channel.Entries))
	for i, entry := range channel.Entries {
		actualVersions[i] = entry.version.String()
	}
	assert.Equal(t, expectedVersions, actualVersions)
}

func assertContainsDeprecationEntry(t *testing.T, entries []DeprecationEntry, schema, name, message string) {
	t.Helper()
	found := false
	for _, entry := range entries {
		if entry.Reference.Name == name {
			assert.Equal(t, schema, entry.Reference.Schema)
			assert.Equal(t, message, entry.Message)
			found = true
			break
		}
	}
	assert.True(t, found, "expected to find deprecation entry with name=%s", name)
}
