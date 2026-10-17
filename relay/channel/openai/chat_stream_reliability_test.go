package openai

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChatStreamPartialFailureDoesNotSendSuccessfulTerminal(t *testing.T) {
	for _, format := range []types.RelayFormat{types.RelayFormatOpenAI, types.RelayFormatClaude, types.RelayFormatGemini} {
		for _, readError := range []bool{false, true} {
			name := string(format) + "/EOF"
			if readError {
				name = string(format) + "/broken HTTP body"
			}
			t.Run(name, func(t *testing.T) {
				c, w, info, resp := setupResponsesStreamReliabilityTest(t, io.NopCloser(strings.NewReader(
					"data: "+streamUsageTextChunk+"\n\ndata: "+streamUsageFinalChunk+"\n\ndata: "+streamUsageZeroChunk+"\n\n")))
				info.RelayMode = relayconstant.RelayModeChatCompletions
				info.RelayFormat = format
				info.ShouldIncludeUsage = true
				info.ClaudeConvertInfo = &relaycommon.ClaudeConvertInfo{}
				if readError {
					resp.Body = io.NopCloser(io.MultiReader(resp.Body, convertedStreamErrorReader{}))
				}

				usage, apiErr := OaiStreamHandler(c, info, resp)
				require.Nil(t, apiErr, "partial output must not enter retry/refund handling")
				require.NotNil(t, usage)
				assert.Equal(t, 5000, usage.PromptTokens)
				assert.Equal(t, 60, usage.CompletionTokens)
				assert.Equal(t, 4800, usage.PromptTokensDetails.CachedTokens)
				got := w.Body.String()
				assert.Contains(t, got, "hello world")
				assert.Contains(t, got, "The upstream stream ended before the response was complete.")
				assert.NotContains(t, got, "data: [DONE]")
				assert.NotContains(t, got, "event: message_stop\n")
				assert.NotContains(t, got, `"finishReason":"STOP"`)
				assert.True(t, info.StreamStatus.HasErrors())
			})
		}
	}
}

func TestChatStreamFailureBeforeOutputRemainsRetryable(t *testing.T) {
	for _, body := range []string{"", "data: " + streamUsageTextChunk + "\n\n"} {
		c, w, info, resp := setupResponsesStreamReliabilityTest(t, io.NopCloser(strings.NewReader(body)))
		info.RelayMode = relayconstant.RelayModeChatCompletions
		info.RelayFormat = types.RelayFormatOpenAI
		usage, apiErr := OaiStreamHandler(c, info, resp)
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
		assert.Nil(t, usage)
		assert.Empty(t, w.Body.String())
		assert.Empty(t, w.Header().Get("Content-Type"))
	}
}

func TestChatStreamExplicitTerminalsKeepTrailingUsage(t *testing.T) {
	for _, tail := range []string{
		"data: " + streamUsageEmptyChunk + "\n\ndata: " + streamUsageFinalChunk + "\n\n",
		"data: " + streamUsageFinalChunk + "\n\ndata: [DONE]\n\n",
	} {
		c, w, info, resp := setupResponsesStreamReliabilityTest(t, io.NopCloser(strings.NewReader("data: "+streamUsageTextChunk+"\n\n"+tail)))
		info.RelayMode = relayconstant.RelayModeChatCompletions
		info.RelayFormat = types.RelayFormatOpenAI
		info.ShouldIncludeUsage = true
		usage, apiErr := OaiStreamHandler(c, info, resp)
		require.Nil(t, apiErr)
		require.NotNil(t, usage)
		assert.Equal(t, 5060, usage.TotalTokens)
		assert.Equal(t, 4800, usage.PromptTokensDetails.CachedTokens)
		assert.Equal(t, 1, strings.Count(w.Body.String(), "data: [DONE]"))
		assert.NotContains(t, w.Body.String(), "upstream_stream_incomplete")
		assert.False(t, info.StreamStatus.HasErrors())
	}
}

func TestChatStreamOneFinishedChoiceDoesNotCompleteAnotherChoice(t *testing.T) {
	for _, missingChoice := range []bool{false, true} {
		t.Run(map[bool]string{false: "unfinished second choice", true: "missing second choice"}[missingChoice], func(t *testing.T) {
			body := "data: " + streamUsageTextChunk + "\n\ndata: " + streamUsageEmptyChunk + "\n\n"
			if !missingChoice {
				body += "data: {\"choices\":[{\"index\":1,\"delta\":{\"content\":\"partial\"}}]}\n\n"
			}
			c, w, info, resp := setupResponsesStreamReliabilityTest(t, io.NopCloser(strings.NewReader(body)))
			info.RelayMode = relayconstant.RelayModeChatCompletions
			info.RelayFormat = types.RelayFormatOpenAI
			n := 2
			info.Request = &dto.GeneralOpenAIRequest{N: &n}
			_, apiErr := OaiStreamHandler(c, info, resp)
			require.Nil(t, apiErr)
			assert.Contains(t, w.Body.String(), "upstream_stream_incomplete")
			assert.NotContains(t, w.Body.String(), "data: [DONE]")
		})
	}
}

func TestChatStreamAllRequestedChoicesFinishBeforeEOF(t *testing.T) {
	body := "data: " + streamUsageTextChunk + "\n\n" +
		"data: {\"choices\":[{\"index\":1,\"delta\":{\"content\":\"another answer\"}}]}\n\n" +
		"data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"},{\"index\":1,\"delta\":{},\"finish_reason\":\"length\"}]}\n\n" +
		"data: " + streamUsageFinalChunk + "\n\n"
	c, w, info, resp := setupResponsesStreamReliabilityTest(t, io.NopCloser(strings.NewReader(body)))
	info.RelayMode = relayconstant.RelayModeChatCompletions
	info.RelayFormat = types.RelayFormatOpenAI
	n := 2
	info.Request = &dto.GeneralOpenAIRequest{N: &n}
	usage, apiErr := OaiStreamHandler(c, info, resp)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.Equal(t, 5060, usage.TotalTokens)
	assert.Contains(t, w.Body.String(), "another answer")
	assert.Equal(t, 1, strings.Count(w.Body.String(), "data: [DONE]"))
	assert.False(t, info.StreamStatus.HasErrors())
}

func TestLegacyCompletionStreamPreservesExplicitFinish(t *testing.T) {
	body := "data: {\"id\":\"cmpl-test\",\"choices\":[{\"index\":0,\"text\":\"hello\",\"finish_reason\":null}]}\n\n" +
		"data: {\"id\":\"cmpl-test\",\"choices\":[{\"index\":0,\"text\":\"\",\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":11,\"completion_tokens\":7,\"total_tokens\":18}}\n\n"
	c, w, info, resp := setupResponsesStreamReliabilityTest(t, io.NopCloser(strings.NewReader(body)))
	c.Request.URL.Path = "/v1/completions"
	info.RelayMode = relayconstant.RelayModeCompletions
	info.RelayFormat = types.RelayFormatOpenAI
	usage, apiErr := OaiStreamHandler(c, info, resp)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.Equal(t, 18, usage.TotalTokens)
	assert.Contains(t, w.Body.String(), `"text":"hello"`)
	assert.Contains(t, w.Body.String(), "data: [DONE]")
	assert.False(t, info.StreamStatus.HasErrors())
}

func TestChatStreamTimeoutAfterPartialToolArgumentsReportsError(t *testing.T) {
	reader, writer := io.Pipe()
	t.Cleanup(func() { _ = reader.Close(); _ = writer.Close() })
	c, w, info, resp := setupResponsesStreamReliabilityTest(t, reader)
	constant.StreamingTimeout = 1
	info.RelayMode = relayconstant.RelayModeChatCompletions
	info.RelayFormat = types.RelayFormatOpenAI
	info.ShouldIncludeUsage = true
	done := make(chan struct{})
	var apiErr *types.NewAPIError
	go func() {
		_, apiErr = OaiStreamHandler(c, info, resp)
		close(done)
	}()
	_, err := io.WriteString(writer, "data: "+streamUsageTextChunk+"\n\ndata: {\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"name\":\"lookup\",\"arguments\":\"{\"}}]}}]}\n\n")
	require.NoError(t, err)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = writer.Close()
		<-done
		t.Fatal("stream did not stop at its idle timeout")
	}
	require.Nil(t, apiErr)
	assert.Contains(t, w.Body.String(), "upstream_stream_timeout")
	assert.NotContains(t, w.Body.String(), `"usage":`)
	assert.NotContains(t, w.Body.String(), "data: [DONE]")
	assert.Equal(t, relaycommon.StreamEndReasonTimeout, info.StreamStatus.EndReason)
}

func TestChatStreamClientCancelDoesNotAppendTerminal(t *testing.T) {
	for _, cancelOnFlush := range []bool{false, true} {
		c, w, info, resp := setupResponsesStreamReliabilityTest(t, io.NopCloser(strings.NewReader("data: "+streamUsageTextChunk+"\n\ndata: "+streamUsageFinalChunk+"\n\ndata: [DONE]\n\n")))
		info.RelayMode = relayconstant.RelayModeChatCompletions
		info.RelayFormat = types.RelayFormatOpenAI
		ctx, cancel := context.WithCancel(c.Request.Context())
		t.Cleanup(cancel)
		c.Request = c.Request.WithContext(ctx)
		if cancelOnFlush {
			c.Writer = &responsesCancelOnFlushWriter{ResponseWriter: c.Writer, cancel: cancel}
		} else {
			c.Writer = &responsesCancelWriter{ResponseWriter: c.Writer, cancel: cancel}
		}
		usage, apiErr := OaiStreamHandler(c, info, resp)
		require.Nil(t, apiErr)
		require.NotNil(t, usage)
		assert.NotContains(t, w.Body.String(), "data: [DONE]")
		assert.NotContains(t, w.Body.String(), "upstream_stream_incomplete")
		assert.Equal(t, relaycommon.StreamEndReasonClientGone, info.StreamStatus.EndReason)
	}
}

func TestChatStreamClientWriteFailureDoesNotRetryOrAppend(t *testing.T) {
	c, w, info, resp := setupResponsesStreamReliabilityTest(t, io.NopCloser(strings.NewReader("data: "+streamUsageTextChunk+"\n\ndata: "+streamUsageFinalChunk+"\n\ndata: [DONE]\n\n")))
	info.RelayMode = relayconstant.RelayModeChatCompletions
	info.RelayFormat = types.RelayFormatOpenAI
	info.SetEstimatePromptTokens(11)
	writeErr := errors.New("client socket disconnected")
	writer := &responsesDisconnectedWriter{ResponseWriter: c.Writer, err: writeErr}
	c.Writer = writer
	usage, apiErr := OaiStreamHandler(c, info, resp)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.Equal(t, 11, usage.PromptTokens)
	assert.Positive(t, usage.CompletionTokens)
	assert.Empty(t, w.Body.String())
	assert.Equal(t, 1, writer.attempts)
	assert.True(t, info.StreamStatus.HasErrors())
	assert.ErrorIs(t, info.StreamStatus.EndError, writeErr)
}

func TestChatStreamRejectsErrorAndMalformedFrames(t *testing.T) {
	for _, tail := range []string{`{"error":{"type":"server_error","message":"private upstream details"}}`, `{not valid JSON}`} {
		c, w, info, resp := setupResponsesStreamReliabilityTest(t, io.NopCloser(strings.NewReader("data: "+streamUsageTextChunk+"\n\ndata: "+tail+"\n\ndata: [DONE]\n\n")))
		info.RelayMode = relayconstant.RelayModeChatCompletions
		info.RelayFormat = types.RelayFormatOpenAI
		_, apiErr := OaiStreamHandler(c, info, resp)
		require.Nil(t, apiErr)
		assert.Contains(t, w.Body.String(), "hello world")
		assert.Contains(t, w.Body.String(), "upstream_stream_incomplete")
		assert.NotContains(t, w.Body.String(), "private upstream details")
		assert.NotContains(t, w.Body.String(), "data: [DONE]")
	}
}
