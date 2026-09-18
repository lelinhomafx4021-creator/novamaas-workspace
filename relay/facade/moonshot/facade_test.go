package moonshot

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newFacadeTestContext(protocol Protocol, model string) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	c.Set(facadeContextKey, true)
	c.Set(protocolContextKey, string(protocol))
	common.SetContextKey(c, constant.ContextKeyOriginalModel, model)
	return c
}

func TestMiddlewareRewritesMoonshotRoutesForRelayPipeline(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		publicPath    string
		canonicalPath string
		protocol      Protocol
	}{
		{publicPath: "/moonshot/v1/chat/completions", canonicalPath: "/v1/chat/completions", protocol: ProtocolChat},
		{publicPath: "/moonshot/v1/responses", canonicalPath: "/v1/responses", protocol: ProtocolResponses},
		{publicPath: "/moonshot/anthropic/v1/messages", canonicalPath: "/v1/messages", protocol: ProtocolMessages},
		{publicPath: "/moonshot/v1/models", canonicalPath: "/v1/models", protocol: ProtocolModels},
	}

	for _, tt := range tests {
		t.Run(tt.publicPath, func(t *testing.T) {
			engine := gin.New()
			engine.Use(Middleware())
			engine.Any("/*path", func(c *gin.Context) {
				assert.True(t, Enabled(c))
				assert.Equal(t, tt.protocol, ActiveProtocol(c))
				assert.Equal(t, tt.publicPath, OriginalPath(c))
				assert.Equal(t, tt.canonicalPath, c.Request.URL.Path)
				c.Status(http.StatusNoContent)
			})

			request := httptest.NewRequest(http.MethodPost, tt.publicPath, nil)
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)

			assert.Equal(t, http.StatusNoContent, response.Code)
			assert.Equal(t, tt.publicPath, request.URL.Path)
		})
	}
}

func TestNormalizeChatRequestRemovesFacadeOnlyFields(t *testing.T) {
	partial := false
	reasoning := "preserved reasoning"
	request := &dto.GeneralOpenAIRequest{
		Model:           "kimi-k3",
		ReasoningEffort: "max",
		Messages: []dto.Message{{
			Role:             "assistant",
			Content:          "answer",
			Partial:          &partial,
			ReasoningContent: &reasoning,
		}},
	}

	require.NoError(t, NormalizeRequest(types.RelayFormatOpenAI, request))
	assert.Equal(t, "high", request.ReasoningEffort)
	assert.Nil(t, request.Messages[0].Partial)
	assert.Nil(t, request.Messages[0].ReasoningContent)
}

func TestNormalizeChatRequestRejectsUnreproducibleKimiInput(t *testing.T) {
	trueValue := true
	wrongTemperature := 0.8
	tests := []struct {
		name    string
		request *dto.GeneralOpenAIRequest
		message string
	}{
		{
			name:    "non Moonshot model",
			request: &dto.GeneralOpenAIRequest{Model: "gpt-5"},
			message: "not available",
		},
		{
			name: "partial continuation",
			request: &dto.GeneralOpenAIRequest{Model: "kimi-k3", Messages: []dto.Message{{
				Role: "assistant", Content: "prefix", Partial: &trueValue,
			}}},
			message: "partial=true",
		},
		{
			name:    "invalid reasoning effort",
			request: &dto.GeneralOpenAIRequest{Model: "kimi-k3", ReasoningEffort: "medium"},
			message: "reasoning_effort",
		},
		{
			name:    "fixed temperature",
			request: &dto.GeneralOpenAIRequest{Model: "kimi-k3", Temperature: &wrongTemperature},
			message: "temperature is fixed",
		},
		{
			name: "video input",
			request: &dto.GeneralOpenAIRequest{Model: "kimi-k3", Messages: []dto.Message{{
				Role: "user",
				Content: []any{dto.MediaContent{
					Type: dto.ContentTypeVideoUrl,
					VideoUrl: &dto.MessageVideoUrl{
						Url: "data:video/mp4;base64,AAAA",
					},
				}},
			}}},
			message: "video input",
		},
		{
			name: "dynamic tool loading",
			request: &dto.GeneralOpenAIRequest{Model: "kimi-k3", Messages: []dto.Message{{
				Role: "system", Tools: []byte(`[{"type":"function"}]`),
			}}},
			message: "dynamic loading",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NormalizeRequest(types.RelayFormatOpenAI, tt.request)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.message)
		})
	}
}

func TestNormalizeResponsesRequestMapsKimiMaxEffort(t *testing.T) {
	request := &dto.OpenAIResponsesRequest{
		Model:     "kimi-k3",
		Reasoning: &dto.Reasoning{Effort: "max"},
	}

	require.NoError(t, NormalizeRequest(types.RelayFormatOpenAIResponses, request))
	assert.Equal(t, "high", request.Reasoning.Effort)
}

func TestTransformChatUsesKimiResponseShape(t *testing.T) {
	c := newFacadeTestContext(ProtocolChat, "kimi-k3")
	input := []byte(`{
		"id":"chatcmpl-upstream","object":"chat.completion","created":123,"model":"gpt-5",
		"system_fingerprint":"fp_upstream",
		"choices":[{"index":0,"message":{"role":"assistant","content":"answer","reasoning":"thought","refusal":null},"finish_reason":"stop","logprobs":{"content":[]}}],
		"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15,"prompt_tokens_details":{"cached_tokens":4,"cache_write_tokens":2},"completion_tokens_details":{"reasoning_tokens":2},"billing_usage":{"amount":1}}
	}`)

	output, err := TransformJSON(c, input)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, common.Unmarshal(output, &payload))
	assert.Equal(t, "kimi-k3", payload["model"])
	assert.NotContains(t, payload, "system_fingerprint")

	usage, ok := payload["usage"].(map[string]any)
	require.True(t, ok)
	assert.Len(t, usage, 5)
	assert.EqualValues(t, 4, usage["cached_tokens"])
	assert.NotContains(t, usage, "billing_usage")
	promptDetails, ok := usage["prompt_tokens_details"].(map[string]any)
	require.True(t, ok)
	assert.EqualValues(t, 4, promptDetails["cached_tokens"])
	assert.EqualValues(t, 2, promptDetails["cache_write_tokens"])

	choices, ok := payload["choices"].([]any)
	require.True(t, ok)
	require.Len(t, choices, 1)
	choice, ok := choices[0].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, choice, "logprobs")
	message, ok := choice["message"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "thought", message["reasoning_content"])
	assert.NotContains(t, message, "reasoning")
	assert.NotContains(t, message, "refusal")
}

func TestTransformChatBuildsDetailsWithoutInventingCacheWrites(t *testing.T) {
	c := newFacadeTestContext(ProtocolChat, "kimi-k3")
	input := []byte(`{
		"id":"chatcmpl-upstream","object":"chat.completion","created":123,"model":"gpt-5",
		"choices":[],
		"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15,"cached_tokens":0}
	}`)

	output, err := TransformJSON(c, input)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, common.Unmarshal(output, &payload))
	usage, ok := payload["usage"].(map[string]any)
	require.True(t, ok)
	promptDetails, ok := usage["prompt_tokens_details"].(map[string]any)
	require.True(t, ok)
	assert.EqualValues(t, 0, promptDetails["cached_tokens"])
	assert.NotContains(t, promptDetails, "cache_write_tokens")
}

func TestTransformResponsesNormalizesNestedStreamResponse(t *testing.T) {
	c := newFacadeTestContext(ProtocolResponses, "kimi-k3")
	input := `{
		"type":"response.completed","sequence_number":4,
		"response":{"id":"resp_upstream","object":"response","model":"gpt-5","output":[],
		"usage":{"input_tokens":20,"input_tokens_details":{"cached_tokens":6,"cache_write_tokens":2},"output_tokens":8,"output_tokens_details":{"reasoning_tokens":3},"total_tokens":28,"billing_usage":{"amount":1}}}
	}`

	output, err := TransformStreamData(c, input)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, common.Unmarshal([]byte(output), &payload))
	response, ok := payload["response"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "kimi-k3", response["model"])
	usage, ok := response["usage"].(map[string]any)
	require.True(t, ok)
	assert.Len(t, usage, 5)
	assert.NotContains(t, usage, "billing_usage")
	inputDetails, ok := usage["input_tokens_details"].(map[string]any)
	require.True(t, ok)
	assert.EqualValues(t, 6, inputDetails["cached_tokens"])
	assert.EqualValues(t, 2, inputDetails["cache_write_tokens"])
}

func TestTransformMessagesUsesKimiCompatibleUsage(t *testing.T) {
	c := newFacadeTestContext(ProtocolMessages, "kimi-k3")
	input := `{
		"type":"message_start",
		"message":{"id":"msg_upstream","type":"message","role":"assistant","model":"gpt-5","content":[],
		"usage":{"input_tokens":12,"output_tokens":4,"cache_read_input_tokens":3,"cache_creation_input_tokens":1,"claude_cache_creation_5_m_tokens":1,"claude_cache_creation_1_h_tokens":0,"output_tokens_details":{"thinking_tokens":2},"billing_usage":{"amount":1}}}
	}`

	output, err := TransformStreamData(c, input)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, common.Unmarshal([]byte(output), &payload))
	message, ok := payload["message"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "kimi-k3", message["model"])
	usage, ok := message["usage"].(map[string]any)
	require.True(t, ok)
	assert.Len(t, usage, 6)
	assert.NotContains(t, usage, "billing_usage")
	assert.NotContains(t, usage, "claude_cache_creation_5_m_tokens")
	assert.NotContains(t, usage, "claude_cache_creation_1_h_tokens")
	cacheCreation, ok := usage["cache_creation"].(map[string]any)
	require.True(t, ok)
	assert.EqualValues(t, 1, cacheCreation["ephemeral_5m_input_tokens"])
	assert.EqualValues(t, 0, cacheCreation["ephemeral_1h_input_tokens"])
	outputDetails, ok := usage["output_tokens_details"].(map[string]any)
	require.True(t, ok)
	assert.EqualValues(t, 2, outputDetails["thinking_tokens"])
}
