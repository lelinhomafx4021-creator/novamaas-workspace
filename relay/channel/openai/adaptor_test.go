package openai

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertOpenAIRequestPreservesMappedKimiThinkingModel(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{Model: "kimi-k2-thinking"}
	info := &relaycommon.RelayInfo{
		OriginModelName: "public-kimi-thinking",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenRouter,
			UpstreamModelName: "kimi-k2-thinking",
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, request)

	require.NoError(t, err)
	require.Same(t, request, converted)
	assert.Equal(t, "kimi-k2-thinking", info.UpstreamModelName)
	assert.Equal(t, "kimi-k2-thinking", request.Model)
	assert.Empty(t, request.Reasoning)
}
