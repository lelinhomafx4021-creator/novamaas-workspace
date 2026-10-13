package model_setting

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const VideoStudioModelRulesOptionKey = "video_studio.model_rules"

// Request body shapes the video studio can send to /v1/video/generations.
const (
	VideoStudioProtocolSeedance    = "seedance"
	VideoStudioProtocolWan3        = "wan3"
	VideoStudioProtocolOpenAIVideo = "openai_video"
)

// VideoStudioModelRule maps a model name keyword to a request protocol.
type VideoStudioModelRule struct {
	Keyword  string `json:"keyword"`
	Protocol string `json:"protocol"`
}

// DefaultVideoStudioModelRules is used until an administrator saves rules.
const DefaultVideoStudioModelRules = `[{"keyword":"seedance","protocol":"seedance"},{"keyword":"wan","protocol":"wan3"},{"keyword":"h3","protocol":"openai_video"}]`

func ParseVideoStudioModelRules(raw string) ([]VideoStudioModelRule, error) {
	var rules []VideoStudioModelRule
	if err := common.UnmarshalJsonStr(raw, &rules); err != nil {
		return nil, err
	}
	if len(rules) > 50 {
		return nil, errors.New("too many video studio model rules")
	}
	seen := make(map[string]struct{}, len(rules))
	for i := range rules {
		rules[i].Keyword = strings.ToLower(strings.TrimSpace(rules[i].Keyword))
		keyword := rules[i].Keyword
		if keyword == "" || len(keyword) > 64 {
			return nil, errors.New("video studio model keyword must contain 1 to 64 characters")
		}
		if _, exists := seen[keyword]; exists {
			return nil, errors.New("duplicate video studio model keyword")
		}
		seen[keyword] = struct{}{}
		switch rules[i].Protocol {
		case VideoStudioProtocolSeedance, VideoStudioProtocolWan3, VideoStudioProtocolOpenAIVideo:
		default:
			return nil, errors.New("unsupported video studio protocol")
		}
	}
	return rules, nil
}

func ValidateVideoStudioModelRules(raw string) error {
	_, err := ParseVideoStudioModelRules(raw)
	return err
}

// GetVideoStudioModelRules returns the saved rules in order; the first rule
// whose keyword the model name contains wins.
func GetVideoStudioModelRules() []VideoStudioModelRule {
	common.OptionMapRWMutex.RLock()
	raw := common.Interface2String(common.OptionMap[VideoStudioModelRulesOptionKey])
	common.OptionMapRWMutex.RUnlock()
	if rules, err := ParseVideoStudioModelRules(raw); raw != "" && err == nil {
		return rules
	}
	rules, _ := ParseVideoStudioModelRules(DefaultVideoStudioModelRules)
	return rules
}
