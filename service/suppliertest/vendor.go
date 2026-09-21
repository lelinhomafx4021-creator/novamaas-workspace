package suppliertest

import (
	"fmt"
	"net/http"
	"regexp"
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

var kvvDateRegex = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

const (
	kimiKVVFlightPrompt   = "According to the Corporate Travel Governance Policy above, Assistant Director Zhang is scheduling an urgent technical architecture review in Shanghai with 1 external VIP architect. Please search for flights departing from Beijing to Shanghai on 2026-10-01 for 2 passengers in business class using the query_flight tool."
	kimiKVVNegativePrompt = "Based on European geography, what is the capital of France? Please answer in one word directly without using any corporate tools."
	kimiKVVHotelPrompt    = "In accordance with the travel plan for the same technical review schedule in Shanghai, please reserve lodging by booking a deluxe hotel in Shanghai for 3 nights using the book_hotel tool."

	// Legacy alias
	kimiKVVPrompt = kimiKVVFlightPrompt
)

const kimiKVVEnterpriseContext = `[Global Enterprise Travel & Operations Governance Manual v5.8]
Section 1: General Business Travel Principles & Scope
All travel booked on behalf of corporate staff, executives, technical fellows, and enterprise client delegations must comply with this Operations Manual. The Global Travel Operations Desk (GTOD) regulates bookings across major hubs: APAC Hubs (Beijing PEK/PKX, Shanghai SHA/PVG, Shenzhen SZX, Tokyo HND/NRT, Singapore SIN), EMEA Central Hubs (Frankfurt FRA, London LHR, Paris CDG), and Americas Regional Hubs (San Francisco SFO, New York JFK, Seattle SEA).

Section 2: Comprehensive Air Transportation Regulations & Class Entitlements
- Standard Booking Policy: All domestic flight journeys under 4 hours require Economy class reservation for standard staff grades (Level 1 through Level 5).
- Executive Class Exceptions: Business class seating is strictly authorized when:
  1) The traveler holds Senior Director or Partner status (Level 6+) with flight transit time exceeding 2 hours.
  2) Technical leads traveling for emergency site deployments, architectural escalation reviews, or high-stakes client technical pitches.
  3) Staff members accompanying external enterprise VIP guests or client C-level executives.
- Dual-Passenger Group Itinerary: When traveling in pairs where at least one passenger meets the executive threshold, both passengers are eligible for unified cabin class booking to facilitate in-flight briefing.
- Date Formatting Protocol: All departure and return schedules must be formatted in strict ISO-8601 YYYY-MM-DD convention.

Section 3: Corporate Accommodation, Lodging Standards, & Tier-City Limits
- Metropolitan Tier-1 Hubs: For tier-1 destinations including Beijing, Shanghai, Guangzhou, and Shenzhen, lodging allocations are contracted with preferred five-star hospitality partners.
- Room Type Allocation Directives:
  - Standard King/Twin: Standard room tier is default for solo travel under 3 nights.
  - Deluxe Room Tier: Authorized for multi-night stays (3 nights or greater) during quarterly engineering summits, partner architecture reviews, or client-facing project kickoff events.
  - Executive Suite Tier: Restricted to VP-grade officers or dedicated hospitality suites hosting client workshops.
- Nightly Rate Caps: Tier-1 hub cities allow a maximum of RMB 1,200 per night for standard tier, and up to RMB 2,000 per night for approved deluxe workshop stays.

Section 4: Ground Transportation & Corporate Car Rental Services
- Corporate Fleet Priority: For transportation between airport terminals, rail stations, and central office campuses, travelers must utilize designated corporate transit shuttles.
- Dedicated Vehicle Charter: When itineraries involve transit to remote data center facilities, high-tech industrial parks, or multi-client visits in suburban zones, employees may requisition dedicated vehicles through the rent_car integration.
- Vehicle Categories & Rental Duration:
  - Sedan: Standard for 1-2 passengers for urban city transit.
  - MPV/Van: Authorized for 3-6 passengers or when transporting sensitive hardware demo racks.
  - Professional Chauffeur Option: May be selected when consecutive client meetings prevent self-driving.

Section 5: Enterprise Expense Claim Submission & Accounting Audit Ledger
- Valid Expense Categories: Reimbursable line items comprise airfare, approved hotel lodging, licensed ground transit, and authorized business meals.
- Mandatory Cost Centers: All expense claims must be tagged with a certified internal department project code (e.g. TECH-ARCH-2026, INFRA-CORE-901, AI-PLATFORM-802).
- Invoice Verification: Original tax-compliant digital invoices (Fapiao) with matching company tax identification numbers must be recorded.

Section 6: Calendar Coordination, Employee Availability, & Meeting Scheduling
- Before dispatching cross-city travel requests, assistants and travel coordinators must verify employee calendar availability using query_calendar_conflict to prevent overlapping executive commitments.
- Minimum travel buffer: A minimum buffer of 2 hours post-arrival must be reserved prior to the first scheduled external technical engagement.`

func kimiKVVMessages(userPrompt string) []chatMessage {
	return []chatMessage{
		{
			Role:    "system",
			Content: kimiKVVEnterpriseContext,
		},
		{
			Role:    "user",
			Content: userPrompt,
		},
	}
}

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
							"description": "Number of passengers (integer only)",
						},
						"seat_class": map[string]any{
							"type":        "string",
							"enum":        []string{"economy", "business", "first"},
							"description": "Cabin class",
						},
					},
					"required": []string{"origin", "destination", "date", "passengers", "seat_class"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]any{
				"name":        "book_hotel",
				"description": "Book a hotel room in a specific city with stay duration and room type",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"city": map[string]any{
							"type":        "string",
							"description": "City where the hotel is located",
						},
						"nights": map[string]any{
							"type":        "integer",
							"description": "Number of nights to stay (integer only)",
						},
						"room_type": map[string]any{
							"type":        "string",
							"enum":        []string{"standard", "deluxe", "suite"},
							"description": "Room type",
						},
					},
					"required": []string{"city", "nights", "room_type"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]any{
				"name":        "rent_car",
				"description": "Requisition corporate car rental or executive chauffeur vehicle for business ground transit.",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"pickup_city": map[string]any{
							"type":        "string",
							"description": "City where the vehicle will be picked up",
						},
						"rental_days": map[string]any{
							"type":        "integer",
							"description": "Duration of vehicle rental in days (integer >= 1)",
						},
						"vehicle_category": map[string]any{
							"type":        "string",
							"enum":        []string{"sedan", "suv", "mpv", "luxury"},
							"description": "Vehicle model category",
						},
						"driver_required": map[string]any{
							"type":        "boolean",
							"description": "Whether a certified professional chauffeur is requested",
						},
					},
					"required": []string{"pickup_city", "rental_days", "vehicle_category"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]any{
				"name":        "submit_expense_claim",
				"description": "Submit travel expense reimbursement claim into the corporate fiscal audit ledger.",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"category": map[string]any{
							"type":        "string",
							"enum":        []string{"transport", "lodging", "meal", "incidentals"},
							"description": "Expense classification category",
						},
						"amount": map[string]any{
							"type":        "number",
							"description": "Total monetary amount in CNY",
						},
						"cost_center": map[string]any{
							"type":        "string",
							"description": "Corporate cost center billing code (e.g. TECH-ARCH-2026)",
						},
						"invoice_number": map[string]any{
							"type":        "string",
							"description": "Verified digital invoice tax registration code",
						},
					},
					"required": []string{"category", "amount", "cost_center"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]any{
				"name":        "query_calendar_conflict",
				"description": "Verify corporate executive schedule availability and detect calendar meeting conflicts prior to travel dispatch.",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"employee_id": map[string]any{
							"type":        "string",
							"description": "Corporate employee identification code (e.g. EMP-TECH-8821)",
						},
						"target_date": map[string]any{
							"type":        "string",
							"description": "Target date to evaluate in YYYY-MM-DD format",
						},
						"duration_hours": map[string]any{
							"type":        "integer",
							"description": "Expected travel and meeting duration in hours",
						},
					},
					"required": []string{"employee_id", "target_date"},
				},
			},
		},
	}
}

func validateKimiKVVFlightResult(res StreamResult) (string, string) {
	if res.StatusCode != http.StatusOK || res.ErrorMessage != "" {
		return "fail", "KVV [阶段1 Schema] 请求失败：" + firstNonEmpty(res.ErrorMessage, fmt.Sprintf("HTTP %d", res.StatusCode))
	}
	if res.ToolName == "" {
		return "fail", "KVV [阶段1 Schema] 校验未通过：未触发工具调用（模型未调用 query_flight，返回了普通文本）"
	}
	if res.ToolName != "query_flight" {
		return "fail", fmt.Sprintf("KVV [阶段1 Schema] 校验未通过：触发了非预期工具 %s（期望 query_flight）", res.ToolName)
	}
	if res.FinishReason != "" && res.FinishReason != "tool_calls" {
		return "fail", fmt.Sprintf("KVV [阶段1 Schema] 校验未通过：finish_reason 不合规（期望 tool_calls，实际为 %s）", res.FinishReason)
	}
	if strings.TrimSpace(res.ToolArgs) == "" {
		return "fail", "KVV [阶段1 Schema] 校验未通过：返回的工具参数为空"
	}

	var parsed map[string]any
	if err := common.UnmarshalJsonStr(res.ToolArgs, &parsed); err != nil {
		return "fail", "KVV [阶段1 Schema] 校验未通过：工具参数无法解析为合法 JSON：" + err.Error()
	}

	for _, reqField := range []string{"origin", "destination", "date", "passengers", "seat_class"} {
		if _, ok := parsed[reqField]; !ok {
			return "fail", fmt.Sprintf("KVV [阶段1 Schema] 校验失败：缺少必填字段 %q", reqField)
		}
	}

	// 日期格式正则强校验 YYYY-MM-DD
	dateStr, _ := parsed["date"].(string)
	if !kvvDateRegex.MatchString(dateStr) {
		return "fail", fmt.Sprintf("KVV [阶段1 Schema] 校验失败：date 格式不合规（期望 YYYY-MM-DD，实际为 %q）", dateStr)
	}

	// 类型约束校验：passengers 必须是数值类型且必须为 2
	passengersVal, ok := parsed["passengers"]
	if !ok {
		return "fail", "KVV [阶段1 Schema] 校验失败：缺少 passengers"
	}
	passNum, isNum := passengersVal.(float64)
	if !isNum {
		return "fail", fmt.Sprintf("KVV [阶段1 Schema] 校验失败：passengers 类型错误，期望 integer，实际为 %T", passengersVal)
	}
	if int(passNum) != 2 {
		return "fail", fmt.Sprintf("KVV [阶段1 Schema] 校验失败：passengers 数值不匹配（期望 2，实际为 %v）", passNum)
	}

	// 枚举值校验：seat_class 必须为 business
	sc, _ := parsed["seat_class"].(string)
	if strings.ToLower(strings.TrimSpace(sc)) != "business" {
		return "fail", fmt.Sprintf("KVV [阶段1 Schema] 校验失败：seat_class 枚举值不合规（期望 business，实际为 %q）", sc)
	}

	return "pass", "阶段1 正向复合 Schema 校验通过"
}

func validateKimiKVVNegativeResult(res StreamResult) (string, string) {
	if res.StatusCode != http.StatusOK || res.ErrorMessage != "" {
		return "fail", "KVV [阶段2 负向对抗] 请求失败：" + firstNonEmpty(res.ErrorMessage, fmt.Sprintf("HTTP %d", res.StatusCode))
	}
	if res.ToolName != "" || strings.TrimSpace(res.ToolArgs) != "" {
		return "fail", fmt.Sprintf("KVV [阶段2 负向对抗] 校验未通过：普通问答错误触发了工具调用 (%s)，模型存在强行调工具的幻觉", res.ToolName)
	}
	if res.FinishReason != "" && res.FinishReason != "stop" {
		return "fail", fmt.Sprintf("KVV [阶段2 负向对抗] 校验未通过：finish_reason 异常（期望 stop，实际为 %s）", res.FinishReason)
	}
	if strings.TrimSpace(res.Content) == "" {
		return "fail", "KVV [阶段2 负向对抗] 校验未通过：模型既未调工具也未输出任何文本回复"
	}
	return "pass", "阶段2 负向对抗拒调通过"
}

func validateKimiKVVHotelResult(res StreamResult) (string, string) {
	if res.StatusCode != http.StatusOK || res.ErrorMessage != "" {
		return "fail", "KVV [阶段3 多工具路由] 请求失败：" + firstNonEmpty(res.ErrorMessage, fmt.Sprintf("HTTP %d", res.StatusCode))
	}
	if res.ToolName == "" {
		return "fail", "KVV [阶段3 多工具路由] 校验未通过：未触发工具调用（模型未调用 book_hotel）"
	}
	if res.ToolName != "book_hotel" {
		return "fail", fmt.Sprintf("KVV [阶段3 多工具路由] 校验未通过：路由歧义错误，期望调用 book_hotel，实际调用了 %s", res.ToolName)
	}
	if res.FinishReason != "" && res.FinishReason != "tool_calls" {
		return "fail", fmt.Sprintf("KVV [阶段3 多工具路由] 校验未通过：finish_reason 不合规（期望 tool_calls，实际为 %s）", res.FinishReason)
	}
	if strings.TrimSpace(res.ToolArgs) == "" {
		return "fail", "KVV [阶段3 多工具路由] 校验未通过：返回的工具参数为空"
	}

	var parsed map[string]any
	if err := common.UnmarshalJsonStr(res.ToolArgs, &parsed); err != nil {
		return "fail", "KVV [阶段3 多工具路由] 校验未通过：工具参数无法解析为合法 JSON：" + err.Error()
	}

	for _, reqField := range []string{"city", "nights", "room_type"} {
		if _, ok := parsed[reqField]; !ok {
			return "fail", fmt.Sprintf("KVV [阶段3 多工具路由] 校验失败：缺少必填字段 %q", reqField)
		}
	}

	// 城市匹配：Shanghai 或 上海
	cityStr, _ := parsed["city"].(string)
	cityLower := strings.ToLower(cityStr)
	if !strings.Contains(cityLower, "shanghai") && !strings.Contains(cityStr, "上海") {
		return "fail", fmt.Sprintf("KVV [阶段3 多工具路由] 校验失败：city 未正确识别上海（实际为 %q）", cityStr)
	}

	// nights: 必须是整型数值且等于 3
	nightsVal, ok := parsed["nights"]
	if !ok {
		return "fail", "KVV [阶段3 多工具路由] 校验失败：缺少 nights"
	}
	nightsNum, isNum := nightsVal.(float64)
	if !isNum {
		return "fail", fmt.Sprintf("KVV [阶段3 多工具路由] 校验失败：nights 类型错误，期望 integer，实际为 %T", nightsVal)
	}
	if int(nightsNum) != 3 {
		return "fail", fmt.Sprintf("KVV [阶段3 多工具路由] 校验失败：nights 数值不匹配（期望 3，实际为 %v）", nightsNum)
	}

	// room_type: 必须为 deluxe
	rt, _ := parsed["room_type"].(string)
	if strings.ToLower(strings.TrimSpace(rt)) != "deluxe" {
		return "fail", fmt.Sprintf("KVV [阶段3 多工具路由] 校验失败：room_type 枚举值不合规（期望 deluxe，实际为 %q）", rt)
	}

	return "pass", "阶段3 多工具歧义路由校验通过"
}

func validateKimiKVVResult(res StreamResult) (string, string) {
	status, msg := validateKimiKVVFlightResult(res)
	if status != "pass" {
		return status, msg
	}
	detailMsg := "KVV 认证通过：ToolCall 触发匹配 (query_flight)，Schema 参数 100% 校验合格 (origin, destination, date, passengers=2, seat_class=business)，finish_reason=tool_calls 合规"
	if res.Reasoning != "" || res.ReasoningTokens > 0 {
		detailMsg += "，包含 Moonshot 流式思维链"
	}
	return "pass", detailMsg
}
