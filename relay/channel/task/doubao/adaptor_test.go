package doubao

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relaykitdto "github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedance20GenericRequestResolutionAndVideoBilling(t *testing.T) {
	tiers := []struct {
		resolution string
		noVideo    float64
		withVideo  float64
	}{
		{resolution: "480p", noVideo: 46, withVideo: 28},
		{resolution: "720p", noVideo: 46, withVideo: 28},
		{resolution: "1080p", noVideo: 51, withVideo: 31},
		{resolution: "4k", noVideo: 26, withVideo: 16},
	}
	for _, modelName := range []string{"doubao-seedance-2-0", "doubao-seedance-2-0-260128"} {
		for _, tier := range tiers {
			for _, hasVideo := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/video=%t", modelName, tier.resolution, hasVideo), func(t *testing.T) {
					metadata := map[string]interface{}{"resolution": tier.resolution}
					wantPrice := tier.noVideo
					if hasVideo {
						metadata["content"] = []interface{}{map[string]interface{}{"type": "video_url", "video_url": map[string]interface{}{"url": "https://example.com/input.mp4"}}}
						wantPrice = tier.withVideo
					}
					data, err := common.Marshal(relaycommon.TaskSubmitReq{Model: modelName, Prompt: "animate", Metadata: metadata})
					require.NoError(t, err)
					ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
					ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", strings.NewReader(string(data)))
					ctx.Request.Header.Set("Content-Type", "application/json")
					info := &relaycommon.RelayInfo{OriginModelName: modelName, ChannelMeta: &relaycommon.ChannelMeta{}, TaskRelayInfo: &relaycommon.TaskRelayInfo{}}
					adaptor := &TaskAdaptor{}
					require.Nil(t, adaptor.ValidateRequestAndSetAction(ctx, info))
					ratios := adaptor.EstimateBilling(ctx, info)
					if wantPrice == 46 {
						assert.Empty(t, ratios, "base-tier requests must not add a second price multiplier")
					} else {
						require.Len(t, ratios, 1)
						assert.InDelta(t, wantPrice/46, ratios["video_input"], 1e-12)
					}

					body, err := adaptor.BuildRequestBody(ctx, info)
					require.NoError(t, err)
					upstream, err := io.ReadAll(body)
					require.NoError(t, err)
					var payload requestPayload
					require.NoError(t, common.Unmarshal(upstream, &payload))
					assert.Equal(t, modelName, payload.Model)
					assert.Equal(t, tier.resolution, payload.Resolution, "generic metadata must reach the provider's top-level resolution")
					videoCount := 0
					for _, item := range payload.Content {
						if item.Type == "video_url" {
							videoCount++
						}
					}
					if hasVideo {
						assert.Equal(t, 1, videoCount)
					} else {
						assert.Zero(t, videoCount)
					}
				})
			}
		}
	}
}

func TestSeedance25TaskValidationAndBilling(t *testing.T) {
	tests := []struct {
		name       string
		resolution interface{}
		hasVideo   bool
		wantRatio  float64
		invalid    bool
	}{
		{name: "default", wantRatio: 1},
		{name: "480p", resolution: "480p", wantRatio: 1},
		{name: "720p video input", resolution: "720p", hasVideo: true, wantRatio: 42.0 / 70},
		{name: "1080p", resolution: "1080p", wantRatio: 77.0 / 70},
		{name: "1080p video input", resolution: "1080p", hasVideo: true, wantRatio: 46.0 / 70},
		{name: "unsupported 4k", resolution: "4k", invalid: true},
		{name: "unknown resolution", resolution: "1440p", invalid: true},
		{name: "wrong resolution type", resolution: 1080, invalid: true},
	}
	for _, model := range []string{"doubao-seedance-2-5", "doubao-seedance-2-5-260628"} {
		for _, tt := range tests {
			t.Run(model+"/"+tt.name, func(t *testing.T) {
				metadata := map[string]interface{}{}
				if tt.resolution != nil {
					metadata["resolution"] = tt.resolution
				}
				if tt.hasVideo {
					metadata["content"] = []interface{}{map[string]interface{}{"type": "video_url", "video_url": map[string]interface{}{"url": "https://example.com/input.mp4"}}}
				}
				body, err := common.Marshal(relaycommon.TaskSubmitReq{Model: model, Prompt: "animate", Metadata: metadata})
				require.NoError(t, err)
				ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
				ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", strings.NewReader(string(body)))
				ctx.Request.Header.Set("Content-Type", "application/json")
				info := &relaycommon.RelayInfo{OriginModelName: model, TaskRelayInfo: &relaycommon.TaskRelayInfo{}}
				adaptor := &TaskAdaptor{}
				taskErr := adaptor.ValidateRequestAndSetAction(ctx, info)
				if tt.invalid {
					require.NotNil(t, taskErr)
					assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
					return
				}
				require.Nil(t, taskErr)
				ratio := adaptor.EstimateBilling(ctx, info)["video_input"]
				if ratio == 0 {
					ratio = 1
				}
				assert.InDelta(t, tt.wantRatio, ratio, 1e-12)
			})
		}
	}
}

func TestDoubaoVideoBuildRequestBodyUsesCachedBase64Staging(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", nil)
	ctx.Set("task_request", relaycommon.TaskSubmitReq{
		Model:  "seedance-public",
		Prompt: "animate the reference",
		Metadata: map[string]interface{}{
			"content": []interface{}{
				map[string]interface{}{
					"type": "video_url",
					"video_url": map[string]interface{}{
						"url": "data:video/mp4;base64,AAAA",
					},
				},
			},
		},
	})
	staged := []byte(`{"model":"seedance-public","content":[{"type":"video_url","video_url":{"url":"https://storage.example.com/reference.mp4"}},{"type":"text","text":"animate the reference"}]}`)
	ctx.Set("doubao_video_base64_staging:relay_media_temp", stagedRequestBody{body: staged, convertedCount: 1})
	info := &relaycommon.RelayInfo{
		UserId:          19,
		RequestId:       "request-doubao-video",
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelOtherSettings: relaykitdto.ChannelOtherSettings{Base64Staging: &relaykitdto.Base64StagingSettings{Enabled: true}}},
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{PublicTaskID: "task_doubao_video"},
		OriginModelName: "seedance-public",
	}

	body, err := (&TaskAdaptor{}).BuildRequestBody(ctx, info)
	require.NoError(t, err)
	got, err := io.ReadAll(body)
	require.NoError(t, err)

	assert.JSONEq(t, string(staged), string(got))
	assert.True(t, common.GetContextKeyBool(ctx, constant.ContextKeyTemporaryMediaConverted))
	assert.Equal(t, 1, common.GetContextKeyInt(ctx, constant.ContextKeyTemporaryMediaConvertedCount))
	assert.JSONEq(t, string(staged), common.GetContextKeyString(ctx, constant.ContextKeyVideoTaskUpstreamRequestBody))
}

func TestDoubaoVideoBuildRequestBodyAppliesModelMappingAfterBase64Staging(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", nil)
	ctx.Set("task_request", relaycommon.TaskSubmitReq{Model: "seedance-public", Prompt: "hello"})
	ctx.Set("doubao_video_base64_staging:relay_media_temp", stagedRequestBody{
		body:           []byte(`{"model":"seedance-public","content":[{"type":"text","text":"hello"}]}`),
		convertedCount: 1,
	})
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			IsModelMapped:     true,
			UpstreamModelName: "doubao-seedance-2-0-260128",
			ChannelOtherSettings: relaykitdto.ChannelOtherSettings{
				Base64Staging: &relaykitdto.Base64StagingSettings{Enabled: true, StoragePolicy: "relay_media_temp"},
			},
		},
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{PublicTaskID: "task_doubao_video"},
		OriginModelName: "seedance-public",
	}

	body, err := (&TaskAdaptor{}).BuildRequestBody(ctx, info)
	require.NoError(t, err)
	got, err := io.ReadAll(body)
	require.NoError(t, err)

	assert.JSONEq(t, `{"model":"doubao-seedance-2-0-260128","content":[{"type":"text","text":"hello"}]}`, string(got))
	assert.JSONEq(t, string(got), common.GetContextKeyString(ctx, constant.ContextKeyVideoTaskUpstreamRequestBody))
}
