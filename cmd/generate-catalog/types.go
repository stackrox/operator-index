package main

import (
	"fmt"

	semver "github.com/Masterminds/semver/v3"
)

const (
	rhacsOperator         = "rhacs-operator"
	olmTemplateSchema     = "olm.template.basic"
	olmPackageSchema      = "olm.package"
	olmChannelSchema      = "olm.channel"
	olmDeprecationsSchema = "olm.deprecations"
	olmBundleSchema       = "olm.bundle"
)

// Describes format of the input file for catalog template generation.
// It contains:
// - OldestSupportedVersion - the oldest supported version of the operator. All versions < OldestSupportedVersion are marked as deprecated.
// - BrokenVersions - a list of versions which are broken and should be skipped in the catalog.
// - Images - a list of bundle images with their versions.
type Input struct {
	OldestSupportedVersion string             `yaml:"oldest_supported_version"`
	BrokenVersions         []string           `yaml:"broken_versions"`
	Images                 []InputBundleImage `yaml:"images"`
}

type InputBundleImage struct {
	Image   string `yaml:"image"`
	Version string `yaml:"version"`
}

// Describes domain logic configuration for the catalog template generation.
type Configuration struct {
	OldestSupportedVersion *semver.Version
	Images                 []BundleImage
	Versions               []*semver.Version
}

type BundleImage struct {
	Image   string
	Version *semver.Version
}

// Describes catalog template structure which is used to generate the catalog YAML file.
// See OLM catalog template documentation for more details: https://olm.operatorframework.io/docs/reference/catalog-templates/
type CatalogTemplate struct {
	Schema  string         `yaml:"schema"`
	Entries []CatalogEntry `yaml:"entries"`
}

type CatalogEntry interface {
	isCatalogEntry()
}

func (Package) isCatalogEntry()      {}
func (Channel) isCatalogEntry()      {}
func (Deprecations) isCatalogEntry() {}
func (BundleEntry) isCatalogEntry()  {}

type Package struct {
	Schema         string `yaml:"schema"`
	Name           string `yaml:"name"`
	DefaultChannel string `yaml:"defaultChannel"`
	Icon           Icon   `yaml:"icon"`
}

type Icon struct {
	Base64data string `yaml:"base64data"`
	MediaType  string `yaml:"mediatype"`
}

type GraphRoot struct {
	MainChannel  Channel // The main channel (e.g., "stable" or "latest") which contains all versions associated with this root (e.g., stable: 4.0.x, 4.1.x, etc.)
	Channels     []Channel
	FromVersion  *semver.Version // Inclusive lower bound of versions associated with this root.
	UntilVersion *semver.Version // Exclusive upper bound of versions associated with this root.
}

type Channel struct {
	Schema         string          `yaml:"schema"`
	Name           string          `yaml:"name"`
	Package        string          `yaml:"package"`
	Entries        []ChannelEntry  `yaml:"entries"`
	yStreamVersion *semver.Version `yaml:"-"`
}

type ChannelEntry struct {
	Name      string          `yaml:"name"`
	Replaces  string          `yaml:"replaces,omitempty"`
	SkipRange string          `yaml:"skipRange"`
	version   *semver.Version `yaml:"-"`
}

type Deprecations struct {
	Schema  string             `yaml:"schema"`
	Package string             `yaml:"package"`
	Entries []DeprecationEntry `yaml:"entries,omitempty"`
}

type DeprecationEntry struct {
	Reference DeprecationReference `yaml:"reference"`
	Message   string               `yaml:"message"`
}

type DeprecationReference struct {
	Schema string `yaml:"schema"`
	Name   string `yaml:"name"`
}

type BundleEntry struct {
	Schema string `yaml:"schema"`
	Image  string `yaml:"image"`
}

// Create base catalog template block.
// It has to contain objects with schema equal to: "olm.package", "olm.channel", "olm.deprecations" or "olm.bundle".
func newCatalogTemplate() CatalogTemplate {
	return CatalogTemplate{
		Schema: olmTemplateSchema,
	}
}

// newPackage creates a new "olm.package" object.
func newPackage(defaultChannel, iconBase64 string) Package {
	return Package{
		Schema:         olmPackageSchema,
		Name:           rhacsOperator,
		DefaultChannel: defaultChannel,
		Icon: Icon{
			Base64data: iconBase64,
			MediaType:  "image/png",
		},
	}
}

// addPackage adds an "olm.package" object to the base catalog.
func (c *CatalogTemplate) addPackage(pkg Package) {
	c.Entries = append(c.Entries, CatalogEntry(pkg))
}

// addChannels adds a list of "olm.channel" objects to the base catalog.
func (c *CatalogTemplate) addChannels(channels []Channel) {
	for _, channel := range channels {
		c.Entries = append(c.Entries, CatalogEntry(channel))
	}
}

// addDeprecations adds an "olm.deprecations" object to the base catalog.
func (c *CatalogTemplate) addDeprecations(deprecations Deprecations) {
	c.Entries = append(c.Entries, CatalogEntry(deprecations))
}

// addBundles adds a list of "olm.bundle" objects to the base catalog.
func (c *CatalogTemplate) addBundles(bundles []BundleEntry) {
	for _, bundle := range bundles {
		c.Entries = append(c.Entries, CatalogEntry(bundle))
	}
}

func newRootChannel(name string, from, until *semver.Version) GraphRoot {
	mainChannel := Channel{
		Schema:  olmChannelSchema,
		Name:    name,
		Package: rhacsOperator,
	}
	return GraphRoot{
		MainChannel:  mainChannel,
		FromVersion:  from,
		UntilVersion: until,
	}
}

// Create a new "olm.channel" object.
// it will be represented in YAML like this:
// |  - schema: olm.channel
// |    name: rhacs-3.64
// |    package: rhacs-operator
// |    entries:
// |      - <ChannelEntry>
func newChannel(version *semver.Version) Channel {
	return Channel{
		Schema:         olmChannelSchema,
		Name:           fmt.Sprintf("rhacs-%d.%d", version.Major(), version.Minor()),
		Package:        rhacsOperator,
		yStreamVersion: makeYStreamVersion(version),
	}
}

// newChannelEntry creates an object to be added to Channel entries list.
// Channel entries effectively form the upgrade graph within the channel telling OLM from which versions it's allowed to upgrade to a particular one.
// it will be represented in YAML like this:
// |  - name: rhacs-operator.v<version>
// |    replaces: rhacs-operator.v<previousEntryVersion>
// |    skipRange: '>= <previousYStreamVersion> < <version>'
func newChannelEntry(version *semver.Version) ChannelEntry {
	return ChannelEntry{
		Name:    generateBundleName(version),
		version: version,
	}
}

func (e *ChannelEntry) addReplaces(version, previousEntryVersion *semver.Version) {
	e.Replaces = generateBundleName(previousEntryVersion)
}

func (e *ChannelEntry) addSkipRange(skipRangeFrom, skipRangeTo *semver.Version) {
	e.SkipRange = fmt.Sprintf(">= %s < %s", skipRangeFrom, skipRangeTo)
}

// Create a new "olm.deprecations" object which should be added to the catalog base.
// It will be represented in YAML like this:
// |  - schema: olm.deprecations
// |    package: rhacs-operator
// |    entries:
// |      - <DeprecationEntry>
func newDeprecations(entries []DeprecationEntry) Deprecations {
	return Deprecations{
		Schema:  olmDeprecationsSchema,
		Package: rhacsOperator,
		Entries: entries,
	}
}

// Create a new channel DeprecationEntry reference object which should be added to Deprecation reference list.
// it will be represented in YAML like this:
// |  - reference:
// |    schema: olm.channel
// |    name: <name>
// |    message: |
// |      <message>
func newChannelDeprecationEntry(name string, message string) DeprecationEntry {
	return DeprecationEntry{
		Reference: DeprecationReference{
			Schema: olmChannelSchema,
			Name:   name,
		},
		Message: message,
	}
}

// Create a new bundle DeprecationEntry reference object which should be added to Deprecation reference list.
// it will be represented in YAML like this:
// |  - reference:
// |    schema: olm.bundle
// |    name: rhacs-operator.v<version>
// |    message: |
// |      <message>
func newBundleDeprecationEntry(version *semver.Version, message string) DeprecationEntry {
	return DeprecationEntry{
		Reference: DeprecationReference{
			Schema: olmBundleSchema,
			Name:   generateBundleName(version),
		},
		Message: message,
	}
}

// Create a new "olm.bundle" object which should be added to the catalog base.
// it will be represented in YAML like this:
// |  - image: <bundle_image_reference>
// |    schema: olm.bundle
func newBundleEntry(image string) BundleEntry {
	return BundleEntry{
		Schema: olmBundleSchema,
		Image:  image,
	}
}

func generateBundleName(version *semver.Version) string {
	return fmt.Sprintf("%s.v%s", rhacsOperator, version)
}

func makeYStreamVersion(v *semver.Version) *semver.Version {
	return semver.New(v.Major(), v.Minor(), 0, "", "")
}
