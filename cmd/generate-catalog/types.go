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

// Input describes format of the input file for catalog template generation.
// It contains:
// - OldestSupportedVersion - the oldest supported version of the operator. All versions < OldestSupportedVersion are marked as deprecated.
// - Images - a list of bundle images with their versions.
type Input struct {
	OldestSupportedVersion string             `yaml:"oldest_supported_version"`
	Images                 []InputBundleImage `yaml:"images"`
}

type InputBundleImage struct {
	Image   string `yaml:"image"`
	Version string `yaml:"version"`
}

// Configuration describes domain logic configuration for the catalog template generation.
type Configuration struct {
	OldestSupportedVersion *semver.Version
	Images                 []BundleImage
	Versions               []*semver.Version
}

type BundleImage struct {
	Image   string
	Version *semver.Version
}

// CatalogTemplate describes catalog template structure which is used to generate the catalog YAML file.
// It has to contain entries with schema equal to: "olm.package", "olm.channel", "olm.deprecations" or "olm.bundle".
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

// addChannels adds a slice of "olm.channel" objects to the base catalog.
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

// newChannel creates a new "olm.channel" object.
// It will be represented in YAML like this:
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

func makeYStreamVersion(v *semver.Version) *semver.Version {
	return semver.New(v.Major(), v.Minor(), 0, "", "")
}

// newChannelEntry creates an object to be added to Channel entries list.
// Channel entries effectively form the upgrade graph within the channel telling OLM from which versions it's allowed to upgrade to a particular one.
// It will be represented in YAML like this:
// |  - name: rhacs-operator.v<version>
// |    replaces: rhacs-operator.v<previousEntryVersion>
// |    skipRange: '>= <previousYStreamVersion> < <version>'
func newChannelEntry(version *semver.Version) ChannelEntry {
	return ChannelEntry{
		Name:    generateBundleName(version),
		version: version,
	}
}

func (e *ChannelEntry) setReplaces(previousEntryVersion *semver.Version) {
	e.Replaces = generateBundleName(previousEntryVersion)
}

func (e *ChannelEntry) clearReplaces() {
	e.Replaces = ""
}

func (e *ChannelEntry) setSkipRange(skipRangeFrom, skipRangeTo *semver.Version) {
	e.SkipRange = fmt.Sprintf(">= %s < %s", skipRangeFrom, skipRangeTo)
}

// newDeprecations creates a new "olm.deprecations" object which should be added to the catalog base.
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

// newChannelDeprecationEntry creates a new channel DeprecationEntry reference object which should be added to Deprecation reference list.
// It will be represented in YAML like this:
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

// newBundleDeprecationEntry creates a new bundle DeprecationEntry reference object which should be added to Deprecation reference list.
// It will be represented in YAML like this:
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

// newBundleEntry creates a new "olm.bundle" object which should be added to the catalog base.
// It will be represented in YAML like this:
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

// channelLineage is a helper struct for the generation time.
// It groups channels together. There's a main one, it's the most complete including all versions in this lineage, and there are Y-Stream channels that are subsets of the main one.
type channelLineage struct {
	MainChannel     Channel // The main channel (e.g., "stable" or "latest") which contains all versions associated with this channel lineage (e.g., stable: 4.0.x, 4.1.x, etc.)
	YStreamChannels []Channel
	FromVersion     *semver.Version // Inclusive lower bound of versions associated with this channel lineage.
	UntilVersion    *semver.Version // Exclusive upper bound of versions associated with this channel lineage.
}

func newChannelLineage(name string, from, until *semver.Version) channelLineage {
	mainChannel := Channel{
		Schema:  olmChannelSchema,
		Name:    name,
		Package: rhacsOperator,
	}
	return channelLineage{
		MainChannel:  mainChannel,
		FromVersion:  from,
		UntilVersion: until,
	}
}
