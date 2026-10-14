package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestChannelSupportsRequestPathVolcNativeOnlyAllowsNativeRoutes(t *testing.T) {
	channel := &model.Channel{Type: constant.ChannelTypeVolcNative}

	require.True(t, channelSupportsRequestPath(channel, "/api/v3/images/generations", "doubao-seedream-4-0"))
	require.True(t, channelSupportsRequestPath(channel, "/api/v3/contents/generations/tasks", "doubao-seedance-2-0-260128"))
	require.False(t, channelSupportsRequestPath(channel, "/v1/chat/completions", "doubao-seed-1-6"))
	require.False(t, channelSupportsRequestPath(channel, "/v1/images/generations", "doubao-seedream-4-0"))
}

func TestStudioSeedanceSelectsNativeChannelPath(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/pg/videos", strings.NewReader(`{"model":"doubao-seedance-2-0"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	require.Equal(t, "/v1/video/generations", DistributeRequestPath(c))
	common.SetContextKey(c, constant.ContextKeyVideoStudioRequestPath, constant.VolcNativeTaskSubmitPath)
	common.SetContextKey(c, constant.ContextKeyVideoStudioGroup, "video-group")
	path := DistributeRequestPath(c)
	require.Equal(t, "/api/v3/contents/generations/tasks", path)
	modelRequest, shouldSelect, err := getModelRequest(c)
	require.NoError(t, err)
	require.True(t, shouldSelect)
	require.Equal(t, "doubao-seedance-2-0", modelRequest.Model)
	require.Equal(t, "video-group", modelRequest.Group)
	require.Equal(t, "video-group", common.GetContextKeyString(c, constant.ContextKeyTokenGroup))
	require.True(t, channelSupportsRequestPath(&model.Channel{Type: constant.ChannelTypeVolcNative}, path, "doubao-seedance-2-0"))
	require.False(t, channelSupportsRequestPath(&model.Channel{Type: constant.ChannelTypeDoubaoVideo}, path, "doubao-seedance-2-0"))
}

func TestVolcNativeRouteRejectsExistingChannelsAndPrefixLookalikes(t *testing.T) {
	for _, channelType := range []int{constant.ChannelTypeDoubaoVideo, constant.ChannelTypeVolcEngine, constant.ChannelTypeOpenAI} {
		require.False(t, channelSupportsRequestPath(&model.Channel{Type: channelType}, "/api/v3/contents/generations/tasks", "seedance"))
	}
	require.False(t, channelSupportsRequestPath(&model.Channel{Type: constant.ChannelTypeVolcNative}, "/api/v3/images/generations-invalid", "seedream"))
}
