package suppliertest

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateKimiKVVFlightResult(t *testing.T) {
	t.Parallel()

	t.Run("pass on valid schema and tool trigger", func(t *testing.T) {
		res := StreamResult{
			StatusCode:   http.StatusOK,
			ToolName:     "query_flight",
			ToolArgs:     `{"origin":"Beijing","destination":"Shanghai","date":"2026-10-01","passengers":2,"seat_class":"business"}`,
			FinishReason: "tool_calls",
			Reasoning:    "Need to search flight",
		}
		status, msg := validateKimiKVVFlightResult(res)
		assert.Equal(t, "pass", status)
		assert.Contains(t, msg, "阶段1 正向复合 Schema 校验通过")
	})

	t.Run("fail when finish_reason is not tool_calls", func(t *testing.T) {
		res := StreamResult{
			StatusCode:   http.StatusOK,
			ToolName:     "query_flight",
			ToolArgs:     `{"origin":"Beijing","destination":"Shanghai","date":"2026-10-01","passengers":2,"seat_class":"business"}`,
			FinishReason: "stop",
		}
		status, msg := validateKimiKVVFlightResult(res)
		assert.Equal(t, "fail", status)
		assert.Contains(t, msg, "finish_reason 不合规")
	})

	t.Run("fail when date format is invalid", func(t *testing.T) {
		res := StreamResult{
			StatusCode:   http.StatusOK,
			ToolName:     "query_flight",
			ToolArgs:     `{"origin":"Beijing","destination":"Shanghai","date":"2026/10/01","passengers":2,"seat_class":"business"}`,
			FinishReason: "tool_calls",
		}
		status, msg := validateKimiKVVFlightResult(res)
		assert.Equal(t, "fail", status)
		assert.Contains(t, msg, "date 格式不合规")
	})

	t.Run("fail when no tool call was triggered", func(t *testing.T) {
		res := StreamResult{
			StatusCode: http.StatusOK,
			Content:    "Here are some flights from Beijing to Shanghai.",
		}
		status, msg := validateKimiKVVFlightResult(res)
		assert.Equal(t, "fail", status)
		assert.Contains(t, msg, "未触发工具调用")
	})

	t.Run("fail when wrong tool triggered", func(t *testing.T) {
		res := StreamResult{
			StatusCode:   http.StatusOK,
			ToolName:     "get_weather",
			ToolArgs:     `{"city":"Beijing"}`,
			FinishReason: "tool_calls",
		}
		status, msg := validateKimiKVVFlightResult(res)
		assert.Equal(t, "fail", status)
		assert.Contains(t, msg, "非预期工具")
	})

	t.Run("fail on invalid JSON arguments", func(t *testing.T) {
		res := StreamResult{
			StatusCode:   http.StatusOK,
			ToolName:     "query_flight",
			ToolArgs:     `{invalid json`,
			FinishReason: "tool_calls",
		}
		status, msg := validateKimiKVVFlightResult(res)
		assert.Equal(t, "fail", status)
		assert.Contains(t, msg, "无法解析为合法 JSON")
	})

	t.Run("fail on missing required fields", func(t *testing.T) {
		res := StreamResult{
			StatusCode:   http.StatusOK,
			ToolName:     "query_flight",
			ToolArgs:     `{"origin":"Beijing","destination":"Shanghai"}`,
			FinishReason: "tool_calls",
		}
		status, msg := validateKimiKVVFlightResult(res)
		assert.Equal(t, "fail", status)
		assert.Contains(t, msg, "缺少必填字段")
	})

	t.Run("fail on type violation (passengers is string instead of integer)", func(t *testing.T) {
		res := StreamResult{
			StatusCode:   http.StatusOK,
			ToolName:     "query_flight",
			ToolArgs:     `{"origin":"Beijing","destination":"Shanghai","date":"2026-10-01","passengers":"2","seat_class":"business"}`,
			FinishReason: "tool_calls",
		}
		status, msg := validateKimiKVVFlightResult(res)
		assert.Equal(t, "fail", status)
		assert.Contains(t, msg, "类型错误")
	})

	t.Run("fail on invalid enum value for seat_class", func(t *testing.T) {
		res := StreamResult{
			StatusCode:   http.StatusOK,
			ToolName:     "query_flight",
			ToolArgs:     `{"origin":"Beijing","destination":"Shanghai","date":"2026-10-01","passengers":2,"seat_class":"vip"}`,
			FinishReason: "tool_calls",
		}
		status, msg := validateKimiKVVFlightResult(res)
		assert.Equal(t, "fail", status)
		assert.Contains(t, msg, "seat_class 枚举值不合规")
	})
}

func TestValidateKimiKVVNegativeResult(t *testing.T) {
	t.Parallel()

	t.Run("pass on normal text without tool call", func(t *testing.T) {
		res := StreamResult{
			StatusCode:   http.StatusOK,
			Content:      "Paris",
			FinishReason: "stop",
		}
		status, msg := validateKimiKVVNegativeResult(res)
		assert.Equal(t, "pass", status)
		assert.Contains(t, msg, "负向对抗拒调通过")
	})

	t.Run("fail when tool call was hallucinated", func(t *testing.T) {
		res := StreamResult{
			StatusCode:   http.StatusOK,
			ToolName:     "query_flight",
			ToolArgs:     `{"origin":"Paris"}`,
			FinishReason: "tool_calls",
		}
		status, msg := validateKimiKVVNegativeResult(res)
		assert.Equal(t, "fail", status)
		assert.Contains(t, msg, "错误触发了工具调用")
	})

	t.Run("fail when content is empty", func(t *testing.T) {
		res := StreamResult{
			StatusCode:   http.StatusOK,
			FinishReason: "stop",
		}
		status, msg := validateKimiKVVNegativeResult(res)
		assert.Equal(t, "fail", status)
		assert.Contains(t, msg, "未输出任何文本回复")
	})
}

func TestValidateKimiKVVHotelResult(t *testing.T) {
	t.Parallel()

	t.Run("pass on valid hotel booking", func(t *testing.T) {
		res := StreamResult{
			StatusCode:   http.StatusOK,
			ToolName:     "book_hotel",
			ToolArgs:     `{"city":"Shanghai","nights":3,"room_type":"deluxe"}`,
			FinishReason: "tool_calls",
		}
		status, msg := validateKimiKVVHotelResult(res)
		assert.Equal(t, "pass", status)
		assert.Contains(t, msg, "多工具歧义路由校验通过")
	})

	t.Run("fail when flight tool was called instead of hotel", func(t *testing.T) {
		res := StreamResult{
			StatusCode:   http.StatusOK,
			ToolName:     "query_flight",
			ToolArgs:     `{"origin":"Shanghai"}`,
			FinishReason: "tool_calls",
		}
		status, msg := validateKimiKVVHotelResult(res)
		assert.Equal(t, "fail", status)
		assert.Contains(t, msg, "路由歧义错误")
	})

	t.Run("fail when nights is not integer", func(t *testing.T) {
		res := StreamResult{
			StatusCode:   http.StatusOK,
			ToolName:     "book_hotel",
			ToolArgs:     `{"city":"Shanghai","nights":"3","room_type":"deluxe"}`,
			FinishReason: "tool_calls",
		}
		status, msg := validateKimiKVVHotelResult(res)
		assert.Equal(t, "fail", status)
		assert.Contains(t, msg, "nights 类型错误")
	})
}

func TestKimiKVVCheckAgainstFakeUpstream(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		str := string(body)

		w.Header().Set("Content-Type", "text/event-stream")
		if strings.Contains(str, "capital of France") {
			_, _ = io.WriteString(w, `data: {"id":"chatcmpl-kvv-2","choices":[{"index":0,"delta":{"content":"Paris"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":2}}`+"\n\n")
			_, _ = io.WriteString(w, "data: [DONE]\n\n")
			return
		}
		if strings.Contains(str, "book_hotel") && strings.Contains(str, "deluxe hotel") {
			_, _ = io.WriteString(w, `data: {"id":"chatcmpl-kvv-3","choices":[{"index":0,"delta":{"tool_calls":[{"id":"call_2","type":"function","function":{"name":"book_hotel","arguments":"{\"city\":\"Shanghai\",\"nights\":3,\"room_type\":\"deluxe\"}"}}]}}],"usage":{"prompt_tokens":10,"completion_tokens":15}}`+"\n\n")
			_, _ = io.WriteString(w, `data: {"id":"chatcmpl-kvv-3","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`+"\n\n")
			_, _ = io.WriteString(w, "data: [DONE]\n\n")
			return
		}
		if strings.Contains(str, "query_flight") && strings.Contains(str, "Beijing") {
			_, _ = io.WriteString(w, `data: {"id":"chatcmpl-kvv-1","choices":[{"index":0,"delta":{"tool_calls":[{"id":"call_1","type":"function","function":{"name":"query_flight","arguments":"{\"origin\":\"Beijing\",\"destination\":\"Shanghai\",\"date\":\"2026-10-01\",\"passengers\":2,\"seat_class\":\"business\"}"}}],"reasoning_content":"searching flight"}}],"usage":{"prompt_tokens":10,"completion_tokens":20}}`+"\n\n")
			_, _ = io.WriteString(w, `data: {"id":"chatcmpl-kvv-1","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`+"\n\n")
			_, _ = io.WriteString(w, "data: [DONE]\n\n")
			return
		}

		_, _ = io.WriteString(w, `data: {"id":"chatcmpl-normal","choices":[{"index":0,"delta":{"content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":5}}`+"\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer upstream.Close()

	var events []Event
	err := Run(context.Background(), upstream.Client(), RunRequest{
		BaseURL: upstream.URL,
		APIKey:  "test-key",
		Model:   "kimi-k2-chat",
		Vendor:  VendorKimi,
		Modules: []string{ModuleBasic},
		Basic: BasicConfig{
			Checks: []string{CheckKimiKVV},
		},
	}, func(e Event) {
		events = append(events, e)
	})
	require.NoError(t, err)

	var kvvCheck *Event
	for i := range events {
		if events[i].CheckID == CheckKimiKVV {
			kvvCheck = &events[i]
		}
	}
	require.NotNil(t, kvvCheck)
	assert.Equal(t, "pass", kvvCheck.Status)
	assert.Contains(t, kvvCheck.Message, "KVV 严苛认证全部通过")
	assert.Contains(t, kvvCheck.Message, "正向复合 Schema 100% 合规")
	assert.Contains(t, kvvCheck.Message, "负向拒调工具 0 幻觉")
	assert.Contains(t, kvvCheck.Message, "多工具歧义路由精准命中 book_hotel")
}
