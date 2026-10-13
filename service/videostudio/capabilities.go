package videostudio

import (
	"slices"
	"strings"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

// Protocol names the request shape a model accepts on the shared video task
// relay. Each protocol maps to an existing task adaptor; no adaptor is added.
const (
	ProtocolSeedance = "seedance" // doubao-video adaptor, content[] with roles
	ProtocolWan3     = "wan3"     // ali adaptor, input.media + parameters
	ProtocolMiniMax  = "minimax"  // OpenAI-compatible /v1/videos adaptor
)

const (
	RoleFirstFrame     = "first_frame"
	RoleLastFrame      = "last_frame"
	RoleReferenceImage = "reference_image"
)

// Capability is the single description used both for the model picker and
// for server-side validation of a studio submission. Empty option lists mean
// the gateway does not constrain the value and the upstream decides; only
// limits already enforced by an adaptor are listed here.
type Capability struct {
	Model         string   `json:"model"`
	Protocol      string   `json:"protocol"`
	Roles         []string `json:"roles"`
	MaxReferences int      `json:"max_references"`
	MinDuration   int      `json:"min_duration"`
	MaxDuration   int      `json:"max_duration"`
	Resolutions   []string `json:"resolutions"`
	Ratios        []string `json:"ratios"`
	Audio         bool     `json:"audio"`
	Seed          bool     `json:"seed"`
	AssetLibrary  bool     `json:"asset_library"`
	Defaults      Defaults `json:"defaults"`
}

type Defaults struct {
	Duration   int    `json:"duration"`
	Resolution string `json:"resolution"`
	Ratio      string `json:"ratio"`
}

type capabilityProfile struct {
	matches    func(model string) bool
	capability Capability
}

var capabilityProfiles = []capabilityProfile{
	{
		matches: func(model string) bool { return strings.HasPrefix(model, "doubao-seedance") },
		capability: Capability{
			Protocol:      ProtocolSeedance,
			Roles:         []string{RoleFirstFrame, RoleLastFrame, RoleReferenceImage},
			MaxReferences: 9,
			MinDuration:   1,
			MaxDuration:   relaycommon.MaxTaskDurationSeconds,
			Resolutions:   []string{},
			Ratios:        []string{},
			Audio:         true,
			Seed:          true,
			AssetLibrary:  true,
			Defaults:      Defaults{Duration: 5, Resolution: "720p", Ratio: "16:9"},
		},
	},
	{
		// Mirrors the bounds enforced by convertToWan3Request.
		matches: func(model string) bool { return strings.HasPrefix(model, "wan3.0") },
		capability: Capability{
			Protocol:      ProtocolWan3,
			Roles:         []string{RoleFirstFrame, RoleLastFrame, RoleReferenceImage},
			MaxReferences: 9,
			MinDuration:   2,
			MaxDuration:   30,
			Resolutions:   []string{"480P", "720P", "1080P"},
			Ratios:        []string{"adaptive", "21:9", "16:9", "4:3", "1:1", "3:4", "9:16"},
			Audio:         true,
			Seed:          true,
			Defaults:      Defaults{Duration: 5, Resolution: "720P", Ratio: "16:9"},
		},
	},
	{
		// MiniMax-H3 is served through an OpenAI-compatible /v1/videos channel
		// (JSON: seconds + metadata.resolution/ratio). Image references need a
		// multipart upload upstream, so this version is text-to-video only.
		matches: func(model string) bool { return strings.HasPrefix(model, "MiniMax-H3") },
		capability: Capability{
			Protocol:    ProtocolMiniMax,
			Roles:       []string{},
			MinDuration: 1,
			MaxDuration: relaycommon.MaxTaskDurationSeconds,
			Resolutions: []string{},
			Ratios:      []string{},
			Defaults:    Defaults{Duration: 5, Resolution: "768P", Ratio: "16:9"},
		},
	},
}

// CapabilityFor returns the studio capability for a public model name.
func CapabilityFor(model string) (Capability, bool) {
	for _, profile := range capabilityProfiles {
		if profile.matches(model) {
			capability := profile.capability
			capability.Model = model
			return capability, true
		}
	}
	return Capability{}, false
}

// AvailableCapabilities intersects the profiles with the models the user's
// groups can actually reach.
func AvailableCapabilities(enabledModels []string) []Capability {
	capabilities := make([]Capability, 0)
	for _, model := range enabledModels {
		if capability, ok := CapabilityFor(model); ok {
			capabilities = append(capabilities, capability)
		}
	}
	slices.SortFunc(capabilities, func(a, b Capability) int { return strings.Compare(a.Model, b.Model) })
	return capabilities
}
