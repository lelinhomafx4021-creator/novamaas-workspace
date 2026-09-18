package suppliertest

import (
	"fmt"
	"strings"
)

const (
	VendorGeneric  = "generic"
	VendorGLM      = "glm"
	VendorKimi     = "kimi"
	VendorDeepSeek = "deepseek"

	DefaultCacheWarmUser = "请用一个词回复：ping"
)

type vendorProfile struct {
	id                string
	cachePrefixRole   string
	requireUsage      bool
	requireCacheField bool
	requireJSON       bool
	requireTools      bool
}

func ResolveVendor(requested string) (string, error) {
	requested = strings.ToLower(strings.TrimSpace(requested))
	if requested == "" {
		return VendorGeneric, nil
	}
	switch requested {
	case VendorGeneric, VendorGLM, VendorKimi, VendorDeepSeek:
		return requested, nil
	default:
		return "", fmt.Errorf("unknown vendor %q", requested)
	}
}

func profileFor(vendor string) vendorProfile {
	switch vendor {
	case VendorGLM, VendorKimi, VendorDeepSeek:
		return vendorProfile{
			id:                vendor,
			cachePrefixRole:   "system",
			requireUsage:      true,
			requireCacheField: true,
			requireJSON:       true,
			requireTools:      true,
		}
	default:
		return vendorProfile{
			id:              VendorGeneric,
			cachePrefixRole: "system",
		}
	}
}

func vendorTitle(vendor string) string {
	switch vendor {
	case VendorGLM:
		return "GLM"
	case VendorKimi:
		return "Kimi"
	case VendorDeepSeek:
		return "DeepSeek"
	default:
		return "供应商"
	}
}

func thinkingRequired(vendor, model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	switch vendor {
	case VendorGLM:
		return strings.Contains(m, "glm-5") ||
			strings.HasPrefix(m, "glm-4.7") ||
			strings.HasPrefix(m, "glm-4.6") ||
			strings.HasPrefix(m, "glm-4.5")
	case VendorKimi:
		return strings.HasPrefix(m, "kimi-k3") ||
			strings.Contains(m, "kimi-k2.7") ||
			strings.Contains(m, "kimi-k2.6") ||
			strings.Contains(m, "k2-thinking")
	case VendorDeepSeek:
		return strings.Contains(m, "r1") || strings.Contains(m, "reasoner")
	default:
		return false
	}
}

func applyThinking(req chatRequest, vendor, model string) chatRequest {
	m := strings.ToLower(strings.TrimSpace(model))
	req.Messages = []chatMessage{{Role: "user", Content: "What is 17 times 19? Think step by step."}}
	if vendor == VendorKimi && strings.HasPrefix(m, "kimi-k3") {
		req.Thinking = nil
		req.ReasoningEffort = "low"
		return req
	}
	if vendor == VendorKimi && strings.Contains(m, "kimi-k2.7") {
		req.Thinking = nil
		return req
	}
	if vendor == VendorDeepSeek && (strings.Contains(m, "r1") || strings.Contains(m, "reasoner")) {
		req.Thinking = nil
		return req
	}
	req.Thinking = map[string]any{"type": "enabled"}
	return req
}

func cacheMessages(profile vendorProfile, prefix, followUp string, warm bool) []chatMessage {
	if profile.cachePrefixRole == "system" {
		user := followUp
		if warm {
			user = DefaultCacheWarmUser
		}
		return []chatMessage{
			{Role: "system", Content: prefix},
			{Role: "user", Content: user},
		}
	}
	if warm {
		return []chatMessage{{Role: "user", Content: prefix}}
	}
	return []chatMessage{
		{Role: "user", Content: prefix},
		{Role: "assistant", Content: "ok"},
		{Role: "user", Content: followUp},
	}
}

func checkStatus(required bool, skipMsg, failMsg string) (string, string) {
	if required {
		return "fail", failMsg
	}
	return "skip", skipMsg
}
