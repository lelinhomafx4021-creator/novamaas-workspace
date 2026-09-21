package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	moonshotfacade "github.com/QuantumNous/new-api/relay/facade/moonshot"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKimiPassthroughDoesNotInventStreamingUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var shouldEmit bool
	engine := gin.New()
	engine.Use(moonshotfacade.Middleware())
	engine.POST("/*path", func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{
			MoonshotFacadeMode: dto.MoonshotFacadeModeKimiPassthrough,
		})
		common.SetContextKey(c, constant.ContextKeyOriginalModel, "kimi-k3")
		assert.False(t, allowsSyntheticUsage(c))
		shouldEmit = shouldEmitSyntheticStreamUsage(c, &relaycommon.RelayInfo{ShouldIncludeUsage: true}, false)
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodPost, "/moonshot/v1/chat/completions", nil)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)

	require.Equal(t, http.StatusNoContent, response.Code)
	assert.False(t, shouldEmit)
}

func TestKimiPassthroughPreservesNonStreamingResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	responseBody := `{"id":"chatcmpl-test","model":"kimi-k3-upstream","choices":[],"usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0},"kimi_extension":{"cache_write_tokens":9.0}}`
	engine := gin.New()
	engine.Use(moonshotfacade.Middleware())
	engine.POST("/*path", func(c *gin.Context) {
		setting := dto.ChannelSettings{
			MoonshotFacadeMode: dto.MoonshotFacadeModeKimiPassthrough,
			ForceFormat:        true,
		}
		common.SetContextKey(c, constant.ContextKeyChannelSetting, setting)
		common.SetContextKey(c, constant.ContextKeyOriginalModel, "kimi-k3")
		info := &relaycommon.RelayInfo{
			RelayFormat: types.RelayFormatOpenAI,
			ChannelMeta: &relaycommon.ChannelMeta{
				ChannelType:       constant.ChannelTypeOpenAI,
				ChannelSetting:    setting,
				UpstreamModelName: "kimi-k3-upstream",
			},
		}
		usage, apiErr := OpenaiHandler(c, info, &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(responseBody)),
		})
		require.Nil(t, apiErr)
		require.NotNil(t, usage)
	})

	request := httptest.NewRequest(http.MethodPost, "/moonshot/v1/chat/completions", nil)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, responseBody, response.Body.String())
}
