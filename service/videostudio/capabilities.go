package videostudio

import (
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/pkg/seedancepricing"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/model_setting"
)

// Protocol names the body shape sent to /v1/video/generations. The route is
// the same for every model; the selected channel's adaptor reads this shape.
const (
	ProtocolSeedance    = model_setting.VideoStudioProtocolSeedance    // doubao-video adaptor: content[] with roles
	ProtocolWan3        = model_setting.VideoStudioProtocolWan3        // ali adaptor: input.media + parameters
	ProtocolOpenAIVideo = model_setting.VideoStudioProtocolOpenAIVideo // OpenAI-compatible adaptor: seconds + metadata
)

// Common choices for providers whose exact limits are not encoded in an
// adaptor. The upstream still rejects combinations it does not support.
var (
	commonResolutions = []string{"480p", "720p", "1080p"}
	commonRatios      = []string{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9"}
)

const (
	RoleFirstFrame     = "first_frame"
	RoleLastFrame      = "last_frame"
	RoleReferenceImage = "reference_image"
)

// Capability is the single description used both for the model picker and
// for server-side validation of a studio submission.
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

// capabilityProfiles holds the limits of each protocol. Which protocol a
// model uses is decided by the administrator's keyword rules, not here.
var capabilityProfiles = map[string]Capability{
	ProtocolSeedance: {
		Protocol:      ProtocolSeedance,
		Roles:         []string{RoleFirstFrame, RoleLastFrame, RoleReferenceImage},
		MaxReferences: 9,
		MinDuration:   1,
		MaxDuration:   relaycommon.MaxTaskDurationSeconds,
		Resolutions:   commonResolutions,
		Ratios:        append([]string{"adaptive"}, commonRatios...),
		Audio:         true,
		Seed:          true,
		AssetLibrary:  true,
		Defaults:      Defaults{Duration: 5, Resolution: "720p", Ratio: "16:9"},
	},
	// The ali adaptor also accepts 1080P, but the studio offers only the
	// resolutions the service supports. It converts this shape for wan3.0 models.
	ProtocolWan3: {
		Protocol:      ProtocolWan3,
		Roles:         []string{RoleFirstFrame, RoleLastFrame, RoleReferenceImage},
		MaxReferences: 9,
		MinDuration:   2,
		MaxDuration:   30,
		Resolutions:   []string{"480P", "720P"},
		Ratios:        []string{"adaptive", "21:9", "16:9", "4:3", "1:1", "3:4", "9:16"},
		Audio:         true,
		Seed:          true,
		Defaults:      Defaults{Duration: 5, Resolution: "720P", Ratio: "16:9"},
	},
	// Served by an OpenAI-compatible channel. Reference images need a
	// multipart upload upstream, so it is text-to-video here.
	ProtocolOpenAIVideo: {
		Protocol:    ProtocolOpenAIVideo,
		Roles:       []string{},
		MinDuration: 1,
		MaxDuration: relaycommon.MaxTaskDurationSeconds,
		Resolutions: []string{"768P", "1080P"},
		Ratios:      commonRatios,
		Defaults:    Defaults{Duration: 5, Resolution: "768P", Ratio: "16:9"},
	},
}

// CapabilityFor returns the studio capability for a public model name: the
// first administrator rule whose keyword the name contains selects the protocol.
func CapabilityFor(model string) (Capability, bool) {
	return capabilityForRules(model, model_setting.GetVideoStudioModelRules())
}

func capabilityForRules(model string, rules []model_setting.VideoStudioModelRule) (Capability, bool) {
	name := strings.ToLower(model)
	for _, rule := range rules {
		if !strings.Contains(name, rule.Keyword) {
			continue
		}
		capability := capabilityProfiles[rule.Protocol]
		capability.Model = model
		if resolutions, priced := seedancepricing.OutputResolutions(model); priced && rule.Protocol == ProtocolSeedance {
			// Seedance output resolution differs per model (2.0 fast: 720p only).
			capability.Resolutions = resolutions
		}
		return capability, true
	}
	return Capability{}, false
}

// SplitModels sorts the models a key can reach into studio video models and
// the remaining models, which the canvas offers for prompt optimization.
func SplitModels(enabledModels []string) (video []Capability, text []string) {
	rules := model_setting.GetVideoStudioModelRules()
	video = make([]Capability, 0)
	text = make([]string, 0)
	for _, model := range enabledModels {
		if capability, ok := capabilityForRules(model, rules); ok {
			video = append(video, capability)
		} else {
			text = append(text, model)
		}
	}
	slices.SortFunc(video, func(a, b Capability) int { return strings.Compare(a.Model, b.Model) })
	slices.Sort(text)
	return video, text
}
