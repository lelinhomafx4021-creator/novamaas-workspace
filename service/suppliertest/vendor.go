package suppliertest

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
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
	requireKVV        bool
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
	case VendorKimi:
		return vendorProfile{
			id:                vendor,
			cachePrefixRole:   "system",
			requireUsage:      true,
			requireCacheField: true,
			requireJSON:       true,
			requireTools:      true,
			requireKVV:        true,
		}
	case VendorGLM, VendorDeepSeek:
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

func thinkingRequired(vendor string) bool {
	switch vendor {
	case VendorGLM, VendorKimi, VendorDeepSeek:
		return true
	default:
		return false
	}
}

func applyThinking(req chatRequest, vendor string) chatRequest {
	req.Messages = []chatMessage{{Role: "user", Content: "What is 17 times 19? Think step by step."}}
	switch vendor {
	case VendorDeepSeek:
		// DeepSeek (如 DeepSeek-V3/V4, R1 等) 原生自带推理，严禁传 thinking 参数 (传了会报 400)
		req.Thinking = nil
		return req
	case VendorKimi:
		// Kimi 新一代模型 (如 K3, K2 等) 遵循 reasoning_effort 规范
		req.Thinking = nil
		req.ReasoningEffort = "low"
		return req
	case VendorGLM:
		// 智谱 GLM 思考协议
		req.Thinking = map[string]any{"type": "enabled"}
		return req
	default:
		req.Thinking = map[string]any{"type": "enabled"}
		return req
	}
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

const kimiKVVPrompt = "Please search for flights from Beijing to Shanghai on 2026-10-01 for 2 passengers in business class using the query_flight tool."

func kimiKVVTools() []map[string]any {
	return []map[string]any{
		{
			"type": "function",
			"function": map[string]any{
				"name":        "query_flight",
				"description": "Query available flights between cities with specific passenger count and seat class",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"origin": map[string]any{
							"type":        "string",
							"description": "Departure city name",
						},
						"destination": map[string]any{
							"type":        "string",
							"description": "Arrival city name",
						},
						"date": map[string]any{
							"type":        "string",
							"description": "Flight date in YYYY-MM-DD format",
						},
						"passengers": map[string]any{
							"type":        "integer",
							"description": "Number of passengers",
						},
						"seat_class": map[string]any{
							"type":        "string",
							"enum":        []string{"economy", "business", "first"},
							"description": "Cabin class",
						},
					},
					"required": []string{"origin", "destination", "date", "passengers"},
				},
			},
		},
	}
}

func validateKimiKVVResult(res StreamResult) (string, string) {
	if res.StatusCode != http.StatusOK || res.ErrorMessage != "" {
		return "fail", "KVV 认证请求失败：" + firstNonEmpty(res.ErrorMessage, fmt.Sprintf("HTTP %d", res.StatusCode))
	}
	if res.ToolName == "" {
		return "fail", "KVV 认证未通过：未触发工具调用（模型未调用 query_flight，返回了普通文本）"
	}
	if res.ToolName != "query_flight" {
		return "fail", fmt.Sprintf("KVV 认证未通过：触发了非预期工具 %s（期望 query_flight）", res.ToolName)
	}
	if strings.TrimSpace(res.ToolArgs) == "" {
		return "fail", "KVV 认证未通过：返回的工具参数为空"
	}

	var parsed map[string]any
	if err := common.UnmarshalJsonStr(res.ToolArgs, &parsed); err != nil {
		return "fail", "KVV 认证未通过：工具参数无法解析为合法 JSON：" + err.Error()
	}

	// Schema 必填字段校验
	for _, reqField := range []string{"origin", "destination", "date", "passengers"} {
		if _, ok := parsed[reqField]; !ok {
			return "fail", fmt.Sprintf("KVV Schema 校验失败：缺少必填字段 %q", reqField)
		}
	}

	// 类型约束校验：passengers 必须是数值类型
	passengersVal, ok := parsed["passengers"]
	if !ok {
		return "fail", "KVV Schema 校验失败：缺少 passengers"
	}
	passNum, isNum := passengersVal.(float64)
	if !isNum {
		return "fail", fmt.Sprintf("KVV Schema 校验失败：passengers 类型错误，期望 integer，实际为 %T", passengersVal)
	}
	if int(passNum) != 2 {
		return "fail", fmt.Sprintf("KVV Schema 校验失败：passengers 数值不匹配（期望 2，实际为 %v）", passNum)
	}

	// 枚举值校验：seat_class (如提供)
	if sc, ok := parsed["seat_class"].(string); ok && sc != "" {
		if sc != "business" && sc != "economy" && sc != "first" {
			return "fail", fmt.Sprintf("KVV Schema 校验失败：seat_class 非法枚举值 %q", sc)
		}
	}

	detailMsg := "KVV 认证通过：ToolCall 触发匹配 (query_flight)，Schema 参数 100% 校验合格 (origin, destination, date, passengers=2)"
	if res.FinishReason != "" {
		if res.FinishReason == "tool_calls" {
			detailMsg += "，finish_reason=tool_calls 合规"
		} else {
			detailMsg += fmt.Sprintf("，finish_reason=%s", res.FinishReason)
		}
	}
	if res.Reasoning != "" || res.ReasoningTokens > 0 {
		detailMsg += "，包含 Moonshot 流式思维链"
	}
	return "pass", detailMsg
}
