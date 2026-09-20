package suppliertest

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateKimiKVVResult(t *testing.T) {
	t.Parallel()

	t.Run("pass on valid schema and tool trigger", func(t *testing.T) {
		res := StreamResult{
			StatusCode: http.StatusOK,
			ToolName:   "query_flight",
			ToolArgs:   `{"origin":"Beijing","destination":"Shanghai","date":"2026-10-01","passengers":2,"seat_class":"business"}`,
			Reasoning:  "Need to search flight",
		}
		status, msg := validateKimiKVVResult(res)
		assert.Equal(t, "pass", status)
		assert.Contains(t, msg, "KVV 认证通过")
		assert.Contains(t, msg, "Moonshot 流式思维链")
	})

	t.Run("fail when no tool call was triggered", func(t *testing.T) {
		res := StreamResult{
			StatusCode: http.StatusOK,
			Content:    "Here are some flights from Beijing to Shanghai.",
		}
		status, msg := validateKimiKVVResult(res)
		assert.Equal(t, "fail", status)
		assert.Contains(t, msg, "未触发工具调用")
	})

	t.Run("fail when wrong tool triggered", func(t *testing.T) {
		res := StreamResult{
			StatusCode: http.StatusOK,
			ToolName:   "get_weather",
			ToolArgs:   `{"city":"Beijing"}`,
		}
		status, msg := validateKimiKVVResult(res)
		assert.Equal(t, "fail", status)
		assert.Contains(t, msg, "非预期工具")
	})

	t.Run("fail on invalid JSON arguments", func(t *testing.T) {
		res := StreamResult{
			StatusCode: http.StatusOK,
			ToolName:   "query_flight",
			ToolArgs:   `{invalid json`,
		}
		status, msg := validateKimiKVVResult(res)
		assert.Equal(t, "fail", status)
		assert.Contains(t, msg, "无法解析为合法 JSON")
	})

	t.Run("fail on missing required fields", func(t *testing.T) {
		res := StreamResult{
			StatusCode: http.StatusOK,
			ToolName:   "query_flight",
			ToolArgs:   `{"origin":"Beijing","destination":"Shanghai"}`,
		}
		status, msg := validateKimiKVVResult(res)
		assert.Equal(t, "fail", status)
		assert.Contains(t, msg, "缺少必填字段")
	})

	t.Run("fail on type violation (passengers is string instead of integer)", func(t *testing.T) {
		res := StreamResult{
			StatusCode: http.StatusOK,
			ToolName:   "query_flight",
			ToolArgs:   `{"origin":"Beijing","destination":"Shanghai","date":"2026-10-01","passengers":"2"}`,
		}
		status, msg := validateKimiKVVResult(res)
		assert.Equal(t, "fail", status)
		assert.Contains(t, msg, "类型错误")
	})

	t.Run("fail on invalid enum value for seat_class", func(t *testing.T) {
		res := StreamResult{
			StatusCode: http.StatusOK,
			ToolName:   "query_flight",
			ToolArgs:   `{"origin":"Beijing","destination":"Shanghai","date":"2026-10-01","passengers":2,"seat_class":"vip"}`,
		}
		status, msg := validateKimiKVVResult(res)
		assert.Equal(t, "fail", status)
		assert.Contains(t, msg, "非法枚举值")
	})
}

func TestKimiKVVCheckAgainstFakeUpstream(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		str := string(body)

		w.Header().Set("Content-Type", "text/event-stream")
		if assert.Contains(t, str, "query_flight") {
			_, _ = io.WriteString(w, `data: {"id":"chatcmpl-kvv","choices":[{"index":0,"delta":{"tool_calls":[{"id":"call_1","type":"function","function":{"name":"query_flight","arguments":"{\"origin\":\"Beijing\",\"destination\":\"Shanghai\",\"date\":\"2026-10-01\",\"passengers\":2}"}}],"reasoning_content":"searching"}}],"usage":{"prompt_tokens":10,"completion_tokens":20}}`+"\n\n")
			_, _ = io.WriteString(w, `data: {"id":"chatcmpl-kvv","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`+"\n\n")
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
	assert.Contains(t, kvvCheck.Message, "KVV 认证通过")
}
