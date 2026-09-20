package moonshot

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
)

type Protocol string

const (
	ProtocolChat      Protocol = "chat_completions"
	ProtocolResponses Protocol = "responses"
	ProtocolMessages  Protocol = "messages"
	ProtocolModels    Protocol = "models"
)

const (
	facadeContextKey       = "moonshot_facade"
	protocolContextKey     = "moonshot_protocol"
	originalPathContextKey = "moonshot_original_path"
)

// Middleware marks Moonshot-compatible routes and rewrites their public path to
// the canonical protocol path used by the existing relay pipeline.
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c == nil {
			return
		}
		if c.Request == nil || c.Request.URL == nil {
			c.Next()
			return
		}

		originalPath := c.Request.URL.Path
		canonicalPath, protocol, ok := canonicalRoute(originalPath)
		if !ok {
			c.Next()
			return
		}

		originalRawPath := c.Request.URL.RawPath
		c.Set(facadeContextKey, true)
		c.Set(protocolContextKey, string(protocol))
		c.Set(originalPathContextKey, originalPath)
		c.Request.URL.Path = canonicalPath
		c.Request.URL.RawPath = ""

		defer func() {
			c.Request.URL.Path = originalPath
			c.Request.URL.RawPath = originalRawPath
		}()
		c.Next()
	}
}

func canonicalRoute(path string) (string, Protocol, bool) {
	switch path {
	case "/moonshot/v1/chat/completions":
		return "/v1/chat/completions", ProtocolChat, true
	case "/moonshot/v1/responses":
		return "/v1/responses", ProtocolResponses, true
	case "/moonshot/anthropic/v1/messages":
		return "/v1/messages", ProtocolMessages, true
	case "/moonshot/v1/models":
		return "/v1/models", ProtocolModels, true
	default:
		return "", "", false
	}
}

func Enabled(c *gin.Context) bool {
	return c != nil && c.GetBool(facadeContextKey)
}

func ActiveProtocol(c *gin.Context) Protocol {
	if c == nil {
		return ""
	}
	return Protocol(c.GetString(protocolContextKey))
}

func OriginalPath(c *gin.Context) string {
	if c == nil {
		return ""
	}
	return c.GetString(originalPathContextKey)
}

func IsMoonshotModel(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	return strings.HasPrefix(model, "kimi-") || strings.HasPrefix(model, "moonshot-")
}

// NormalizeRequest removes supported facade-only markers before the request is
// sent to an OpenAI-compatible upstream and rejects Kimi-only behavior that the
// upstream cannot reproduce faithfully.
func NormalizeRequest(relayFormat types.RelayFormat, request dto.Request) error {
	if request == nil {
		return nil
	}
	modelName := ""
	switch req := request.(type) {
	case *dto.GeneralOpenAIRequest:
		modelName = req.Model
	case *dto.OpenAIResponsesRequest:
		modelName = req.Model
	case *dto.ClaudeRequest:
		modelName = req.Model
	}
	if !IsMoonshotModel(modelName) {
		return fmt.Errorf("model %q is not available through the Moonshot compatibility endpoint", modelName)
	}

	switch relayFormat {
	case types.RelayFormatOpenAI:
		req, ok := request.(*dto.GeneralOpenAIRequest)
		if !ok {
			return fmt.Errorf("expected OpenAI chat request, got %T", request)
		}
		if err := validateReasoningEffort(req.ReasoningEffort); err != nil {
			return err
		}
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(modelName)), "kimi-k3") {
			if req.Temperature != nil && *req.Temperature != 1.0 {
				return fmt.Errorf("temperature is fixed at 1.0 for kimi-k3")
			}
			if req.TopP != nil && *req.TopP != 0.95 {
				return fmt.Errorf("top_p is fixed at 0.95 for kimi-k3")
			}
			if req.N != nil && *req.N != 1 {
				return fmt.Errorf("n is fixed at 1 for kimi-k3")
			}
			if req.PresencePenalty != nil && *req.PresencePenalty != 0 {
				return fmt.Errorf("presence_penalty is fixed at 0 for kimi-k3")
			}
			if req.FrequencyPenalty != nil && *req.FrequencyPenalty != 0 {
				return fmt.Errorf("frequency_penalty is fixed at 0 for kimi-k3")
			}
		}
		if len(req.THINKING) > 0 && string(req.THINKING) != "null" {
			return fmt.Errorf("thinking is not supported by the configured OpenAI upstream; use reasoning_effort")
		}
		if req.ReasoningEffort == "max" {
			// OpenAI Chat Completions has no "max" tier. "high" is the
			// closest representable request value available.
			req.ReasoningEffort = "high"
		}
		for i := range req.Messages {
			message := &req.Messages[i]
			messageTools := strings.TrimSpace(string(message.Tools))
			if messageTools != "" && messageTools != "null" && messageTools != "[]" {
				return fmt.Errorf("messages[%d].tools dynamic loading cannot be reproduced by the configured OpenAI upstream", i)
			}
			message.Tools = nil
			if message.Partial != nil {
				if *message.Partial {
					return fmt.Errorf("messages[%d].partial=true cannot be reproduced by the configured OpenAI upstream", i)
				}
				message.Partial = nil
			}
			// Kimi preserved-thinking history is not accepted by the OpenAI
			// Chat Completions schema. Keep the assistant answer as context,
			// but do not forward an unsupported reasoning trace.
			message.ReasoningContent = nil
			message.Reasoning = nil
			for _, content := range message.ParseContent() {
				if content.Type == dto.ContentTypeVideoUrl || content.VideoUrl != nil {
					return fmt.Errorf("messages[%d] contains video input that cannot be reproduced by the configured OpenAI upstream", i)
				}
			}
		}
	case types.RelayFormatOpenAIResponses:
		req, ok := request.(*dto.OpenAIResponsesRequest)
		if !ok {
			return fmt.Errorf("expected OpenAI Responses request, got %T", request)
		}
		if req.Reasoning != nil {
			if err := validateReasoningEffort(req.Reasoning.Effort); err != nil {
				return err
			}
			if req.Reasoning.Effort == "max" {
				req.Reasoning.Effort = "high"
			}
		}
	case types.RelayFormatClaude:
		if _, ok := request.(*dto.ClaudeRequest); !ok {
			return fmt.Errorf("expected Anthropic Messages request, got %T", request)
		}
	}
	return nil
}

func validateReasoningEffort(effort string) error {
	if effort == "" || effort == "low" || effort == "high" || effort == "max" {
		return nil
	}
	return fmt.Errorf("reasoning_effort must be one of low, high, or max")
}

func TransformJSON(c *gin.Context, data []byte) ([]byte, error) {
	if !Enabled(c) || len(data) == 0 {
		return data, nil
	}

	switch ActiveProtocol(c) {
	case ProtocolChat:
		return transformChat(c, data, false)
	case ProtocolResponses:
		return transformResponses(c, data)
	case ProtocolMessages:
		return transformMessages(c, data)
	default:
		return data, nil
	}
}

func TransformStreamData(c *gin.Context, data string) (string, error) {
	if !Enabled(c) || data == "" || data == "[DONE]" {
		return data, nil
	}

	var (
		transformed []byte
		err         error
	)
	switch ActiveProtocol(c) {
	case ProtocolChat:
		transformed, err = transformChat(c, common.StringToByteSlice(data), true)
	case ProtocolResponses:
		transformed, err = transformResponses(c, common.StringToByteSlice(data))
	case ProtocolMessages:
		transformed, err = transformMessages(c, common.StringToByteSlice(data))
	default:
		return data, nil
	}
	if err != nil {
		return "", err
	}
	return string(transformed), nil
}

type sourceEnvelope struct {
	ID      string          `json:"id"`
	Object  string          `json:"object"`
	Created int64           `json:"created"`
	Model   string          `json:"model"`
	Choices json.RawMessage `json:"choices"`
	Usage   json.RawMessage `json:"usage"`
	Error   json.RawMessage `json:"error"`
}

type sourceUsage struct {
	PromptTokens          int                       `json:"prompt_tokens"`
	CompletionTokens      int                       `json:"completion_tokens"`
	TotalTokens           int                       `json:"total_tokens"`
	CachedTokens          *int                      `json:"cached_tokens"`
	PromptTokensDetails   *sourceInputTokenDetails  `json:"prompt_tokens_details"`
	InputTokens           int                       `json:"input_tokens"`
	OutputTokens          int                       `json:"output_tokens"`
	InputTokensDetails    *sourceInputTokenDetails  `json:"input_tokens_details"`
	CompletionDetails     *sourceOutputTokenDetails `json:"completion_tokens_details"`
	OutputTokensDetails   *sourceOutputTokenDetails `json:"output_tokens_details"`
	CacheReadInputTokens  int                       `json:"cache_read_input_tokens"`
	CacheWriteInputTokens int                       `json:"cache_creation_input_tokens"`
}

type sourceInputTokenDetails struct {
	CachedTokens         *int `json:"cached_tokens"`
	CachedCreationTokens *int `json:"cached_creation_tokens"`
	CacheWriteTokens     *int `json:"cache_write_tokens"`
}

type sourceOutputTokenDetails struct {
	ReasoningTokens *int `json:"reasoning_tokens"`
	ThinkingTokens  *int `json:"thinking_tokens"`
}

type cacheTokenDetails struct {
	CachedTokens     *int `json:"cached_tokens,omitempty"`
	CacheWriteTokens *int `json:"cache_write_tokens,omitempty"`
}

type chatUsage struct {
	PromptTokens        int                `json:"prompt_tokens"`
	CompletionTokens    int                `json:"completion_tokens"`
	TotalTokens         int                `json:"total_tokens"`
	CachedTokens        *int               `json:"cached_tokens,omitempty"`
	PromptTokensDetails *cacheTokenDetails `json:"prompt_tokens_details,omitempty"`
}

type chatMessage struct {
	Role             string          `json:"role"`
	Content          any             `json:"content"`
	ToolCalls        json.RawMessage `json:"tool_calls,omitempty"`
	ReasoningContent *string         `json:"reasoning_content,omitempty"`
}

type chatChoice struct {
	Index        int             `json:"index"`
	Message      chatMessage     `json:"message"`
	FinishReason string          `json:"finish_reason"`
	Logprobs     json.RawMessage `json:"logprobs,omitempty"`
}

type chatResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []chatChoice `json:"choices"`
	Usage   *chatUsage   `json:"usage,omitempty"`
}

type chatDelta struct {
	Role             string                 `json:"role,omitempty"`
	Content          *string                `json:"content,omitempty"`
	ToolCalls        []dto.ToolCallResponse `json:"tool_calls,omitempty"`
	ReasoningContent *string                `json:"reasoning_content,omitempty"`
}

type chatStreamChoice struct {
	Index        int       `json:"index"`
	Delta        chatDelta `json:"delta"`
	FinishReason *string   `json:"finish_reason"`
	Logprobs     *any      `json:"logprobs,omitempty"`
}

type sourceChatChoice struct {
	Index        int             `json:"index"`
	Message      dto.Message     `json:"message"`
	FinishReason string          `json:"finish_reason"`
	Logprobs     json.RawMessage `json:"logprobs"`
}

type chatStreamResponse struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Created int64              `json:"created"`
	Model   string             `json:"model"`
	Choices []chatStreamChoice `json:"choices"`
	Usage   *chatUsage         `json:"usage,omitempty"`
}

func transformChat(c *gin.Context, data []byte, stream bool) ([]byte, error) {
	var source sourceEnvelope
	if err := common.Unmarshal(data, &source); err != nil {
		return nil, err
	}
	if len(source.Error) > 0 && string(source.Error) != "null" {
		return data, nil
	}

	model := publicModel(c, source.Model)
	usage, err := chatUsageFromRaw(source.Usage)
	if err != nil {
		return nil, err
	}
	if stream {
		var choices []dto.ChatCompletionsStreamResponseChoice
		if len(source.Choices) > 0 {
			if err := common.Unmarshal(source.Choices, &choices); err != nil {
				return nil, err
			}
		}
		outChoices := make([]chatStreamChoice, 0, len(choices))
		for _, choice := range choices {
			reasoning := choice.Delta.ReasoningContent
			if reasoning == nil {
				reasoning = choice.Delta.Reasoning
			}
			outChoices = append(outChoices, chatStreamChoice{
				Index: choice.Index, Logprobs: choice.Logprobs,
				Delta: chatDelta{
					Role:             choice.Delta.Role,
					Content:          choice.Delta.Content,
					ToolCalls:        choice.Delta.ToolCalls,
					ReasoningContent: reasoning,
				},
				FinishReason: choice.FinishReason,
			})
		}
		return common.Marshal(chatStreamResponse{
			ID: source.ID, Object: source.Object, Created: source.Created,
			Model: model, Choices: outChoices, Usage: usage,
		})
	}

	var choices []sourceChatChoice
	if len(source.Choices) > 0 {
		if err := common.Unmarshal(source.Choices, &choices); err != nil {
			return nil, err
		}
	}
	outChoices := make([]chatChoice, 0, len(choices))
	for _, choice := range choices {
		reasoning := choice.Message.ReasoningContent
		if reasoning == nil {
			reasoning = choice.Message.Reasoning
		}
		outChoices = append(outChoices, chatChoice{
			Index: choice.Index, Logprobs: choice.Logprobs,
			Message: chatMessage{
				Role:             choice.Message.Role,
				Content:          choice.Message.Content,
				ToolCalls:        choice.Message.ToolCalls,
				ReasoningContent: reasoning,
			},
			FinishReason: choice.FinishReason,
		})
	}
	return common.Marshal(chatResponse{
		ID: source.ID, Object: source.Object, Created: source.Created,
		Model: model, Choices: outChoices, Usage: usage,
	})
}

func chatUsageFromRaw(raw json.RawMessage) (*chatUsage, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var source sourceUsage
	if err := common.Unmarshal(raw, &source); err != nil {
		return nil, err
	}
	cachedTokens := source.CachedTokens
	if cachedTokens == nil && source.PromptTokensDetails != nil {
		cachedTokens = source.PromptTokensDetails.CachedTokens
	}
	var promptTokensDetails *cacheTokenDetails
	if source.PromptTokensDetails != nil {
		cacheWriteTokens := source.PromptTokensDetails.CacheWriteTokens
		if cacheWriteTokens == nil {
			cacheWriteTokens = source.PromptTokensDetails.CachedCreationTokens
		}
		if source.PromptTokensDetails.CachedTokens != nil || cacheWriteTokens != nil {
			promptTokensDetails = &cacheTokenDetails{
				CachedTokens:     source.PromptTokensDetails.CachedTokens,
				CacheWriteTokens: cacheWriteTokens,
			}
		}
	}
	if promptTokensDetails == nil && cachedTokens != nil {
		promptTokensDetails = &cacheTokenDetails{CachedTokens: cachedTokens}
	}
	return &chatUsage{
		PromptTokens: source.PromptTokens, CompletionTokens: source.CompletionTokens,
		TotalTokens: source.TotalTokens, CachedTokens: cachedTokens,
		PromptTokensDetails: promptTokensDetails,
	}, nil
}

type responsesInputTokenDetails struct {
	CachedTokens     *int `json:"cached_tokens,omitempty"`
	CacheWriteTokens *int `json:"cache_write_tokens,omitempty"`
}

type responsesOutputTokenDetails struct {
	ReasoningTokens *int `json:"reasoning_tokens,omitempty"`
}

type responsesUsage struct {
	InputTokens         int                          `json:"input_tokens"`
	InputTokensDetails  *responsesInputTokenDetails  `json:"input_tokens_details,omitempty"`
	OutputTokens        int                          `json:"output_tokens"`
	OutputTokensDetails *responsesOutputTokenDetails `json:"output_tokens_details,omitempty"`
	TotalTokens         int                          `json:"total_tokens"`
}

func transformResponses(c *gin.Context, data []byte) ([]byte, error) {
	var payload map[string]json.RawMessage
	if err := common.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	if _, isError := payload["error"]; isError && payload["object"] == nil && payload["response"] == nil {
		return data, nil
	}
	if rawModel, ok := payload["model"]; ok {
		var model string
		if err := common.Unmarshal(rawModel, &model); err == nil {
			payload["model"], _ = common.Marshal(publicModel(c, model))
		}
	}
	if rawUsage, ok := payload["usage"]; ok && string(rawUsage) != "null" {
		usage, err := responsesUsageFromRaw(rawUsage)
		if err != nil {
			return nil, err
		}
		payload["usage"], err = common.Marshal(usage)
		if err != nil {
			return nil, err
		}
	}
	if rawResponse, ok := payload["response"]; ok && len(rawResponse) > 0 && string(rawResponse) != "null" {
		response, err := transformResponses(c, rawResponse)
		if err != nil {
			return nil, err
		}
		payload["response"] = response
	}
	return common.Marshal(payload)
}

func responsesUsageFromRaw(raw json.RawMessage) (*responsesUsage, error) {
	var source sourceUsage
	if err := common.Unmarshal(raw, &source); err != nil {
		return nil, err
	}
	inputTokens := source.InputTokens
	if inputTokens == 0 {
		inputTokens = source.PromptTokens
	}
	outputTokens := source.OutputTokens
	if outputTokens == 0 {
		outputTokens = source.CompletionTokens
	}
	totalTokens := source.TotalTokens
	if totalTokens == 0 {
		totalTokens = inputTokens + outputTokens
	}

	inputDetails := source.InputTokensDetails
	if inputDetails == nil {
		inputDetails = source.PromptTokensDetails
	}
	var responseInputDetails *responsesInputTokenDetails
	if inputDetails != nil {
		cacheWriteTokens := inputDetails.CacheWriteTokens
		if cacheWriteTokens == nil {
			cacheWriteTokens = inputDetails.CachedCreationTokens
		}
		if inputDetails.CachedTokens != nil || cacheWriteTokens != nil {
			responseInputDetails = &responsesInputTokenDetails{
				CachedTokens: inputDetails.CachedTokens, CacheWriteTokens: cacheWriteTokens,
			}
		}
	}
	outputDetails := source.OutputTokensDetails
	if outputDetails == nil {
		outputDetails = source.CompletionDetails
	}
	var responseOutputDetails *responsesOutputTokenDetails
	if outputDetails != nil {
		reasoningTokens := outputDetails.ReasoningTokens
		if reasoningTokens == nil {
			reasoningTokens = outputDetails.ThinkingTokens
		}
		if reasoningTokens != nil {
			responseOutputDetails = &responsesOutputTokenDetails{ReasoningTokens: reasoningTokens}
		}
	}
	return &responsesUsage{
		InputTokens: inputTokens, InputTokensDetails: responseInputDetails,
		OutputTokens: outputTokens, OutputTokensDetails: responseOutputDetails,
		TotalTokens: totalTokens,
	}, nil
}

func transformMessages(c *gin.Context, data []byte) ([]byte, error) {
	var payload map[string]json.RawMessage
	if err := common.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	if rawModel, ok := payload["model"]; ok {
		var model string
		if err := common.Unmarshal(rawModel, &model); err == nil {
			payload["model"], _ = common.Marshal(publicModel(c, model))
		}
	}
	if rawUsage, ok := payload["usage"]; ok && string(rawUsage) != "null" {
		var usage map[string]json.RawMessage
		if err := common.Unmarshal(rawUsage, &usage); err != nil {
			return nil, err
		}
		delete(usage, "billing_usage")
		delete(usage, "usage_semantic")
		delete(usage, "usage_source")
		if _, exists := usage["cache_creation"]; !exists {
			var (
				cache5m    int
				cache1h    int
				hasCache5m bool
				hasCache1h bool
			)
			if rawCache5m, ok := usage["claude_cache_creation_5_m_tokens"]; ok {
				if err := common.Unmarshal(rawCache5m, &cache5m); err != nil {
					return nil, err
				}
				hasCache5m = true
			}
			if rawCache1h, ok := usage["claude_cache_creation_1_h_tokens"]; ok {
				if err := common.Unmarshal(rawCache1h, &cache1h); err != nil {
					return nil, err
				}
				hasCache1h = true
			}
			if hasCache5m || hasCache1h {
				cacheCreation := make(map[string]int, 2)
				if hasCache5m {
					cacheCreation["ephemeral_5m_input_tokens"] = cache5m
				}
				if hasCache1h {
					cacheCreation["ephemeral_1h_input_tokens"] = cache1h
				}
				usage["cache_creation"], _ = common.Marshal(cacheCreation)
			}
		}
		delete(usage, "claude_cache_creation_5_m_tokens")
		delete(usage, "claude_cache_creation_1_h_tokens")
		if rawOutputDetails, ok := usage["output_tokens_details"]; ok && string(rawOutputDetails) != "null" {
			var source sourceOutputTokenDetails
			if err := common.Unmarshal(rawOutputDetails, &source); err != nil {
				return nil, err
			}
			thinkingTokens := source.ThinkingTokens
			if thinkingTokens == nil {
				thinkingTokens = source.ReasoningTokens
			}
			if thinkingTokens == nil {
				delete(usage, "output_tokens_details")
			} else {
				usage["output_tokens_details"], _ = common.Marshal(struct {
					ThinkingTokens *int `json:"thinking_tokens"`
				}{ThinkingTokens: thinkingTokens})
			}
		}
		var err error
		payload["usage"], err = common.Marshal(usage)
		if err != nil {
			return nil, err
		}
	}
	if rawMessage, ok := payload["message"]; ok && len(rawMessage) > 0 && string(rawMessage) != "null" {
		message, err := transformMessages(c, rawMessage)
		if err != nil {
			return nil, err
		}
		payload["message"] = message
	}
	return common.Marshal(payload)
}

func publicModel(c *gin.Context, fallback string) string {
	model := common.GetContextKeyString(c, constant.ContextKeyOriginalModel)
	if model == "" {
		return fallback
	}
	return model
}
