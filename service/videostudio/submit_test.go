package videostudio

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/model_setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validInput(model string) SubmitInput {
	return SubmitInput{ProjectID: "vp-1", NodeID: "node-1", ClientRequestID: "req-1", Model: model, Prompt: "city at night"}
}

func TestSubmitValidationEnforcesCapability(t *testing.T) {
	tests := []struct {
		name    string
		model   string
		mutate  func(input *SubmitInput)
		wantErr string
	}{
		{name: "defaults fill missing parameters", model: "wan3.0-video"},
		{
			name:  "wan3 duration upper bound comes from the adaptor contract",
			model: "wan3.0-video", mutate: func(input *SubmitInput) { input.Parameters.Duration = common.GetPointer(31) },
			wantErr: "duration must be between 2 and 30 seconds",
		},
		{
			name:  "billing duration bound applies to unconstrained models",
			model: "MiniMax-H3", mutate: func(input *SubmitInput) { input.Parameters.Duration = common.GetPointer(1 << 30) },
			wantErr: "duration must be between",
		},
		{
			name:  "last frame needs a first frame",
			model: "doubao-seedance-2-0",
			mutate: func(input *SubmitInput) {
				input.References = []Reference{{Source: SourceStorage, ID: "ref_1", Role: RoleLastFrame}}
			},
			wantErr: "a last frame requires a first frame",
		},
		{
			name:  "reference images do not mix with frames",
			model: "doubao-seedance-2-0",
			mutate: func(input *SubmitInput) {
				input.References = []Reference{
					{Source: SourceStorage, ID: "ref_1", Role: RoleFirstFrame},
					{Source: SourceStorage, ID: "ref_2", Role: RoleReferenceImage},
				}
			},
			wantErr: "cannot be combined",
		},
		{
			name:  "seedance assets are Seedance only",
			model: "wan3.0-video",
			mutate: func(input *SubmitInput) {
				input.References = []Reference{{Source: SourceSeedanceAsset, ID: "asset-1", Role: RoleFirstFrame}}
			},
			wantErr: "does not accept Seedance assets",
		},
		{
			name:  "minimax H3 is text only in the studio",
			model: "MiniMax-H3",
			mutate: func(input *SubmitInput) {
				input.References = []Reference{{Source: SourceStorage, ID: "ref_1", Role: RoleFirstFrame}}
			},
			wantErr: "at most 0 reference images",
		},
		{
			name:    "client request id is required for idempotency",
			model:   "wan3.0-video",
			mutate:  func(input *SubmitInput) { input.ClientRequestID = "" },
			wantErr: "client_request_id is required",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capability, ok := CapabilityFor(tt.model)
			require.True(t, ok)
			input := validInput(tt.model)
			if tt.mutate != nil {
				tt.mutate(&input)
			}
			err := input.Validate(capability)
			if tt.wantErr == "" {
				require.NoError(t, err)
				assert.Equal(t, capability.Defaults.Duration, *input.Parameters.Duration)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestSubmitValidationKeepsExplicitFalseAndZero(t *testing.T) {
	capability, _ := CapabilityFor("doubao-seedance-2-0")
	input := validInput("doubao-seedance-2-0")
	input.Parameters.GenerateAudio = common.GetPointer(false)
	input.Parameters.Seed = common.GetPointer(0)
	require.NoError(t, input.Validate(capability))

	body, err := BuildTaskRequest(capability, &input, nil)
	require.NoError(t, err)
	metadata := body["metadata"].(map[string]any)
	assert.Equal(t, false, metadata["generate_audio"])
	assert.Equal(t, 0, metadata["seed"])
}

func TestBuildTaskRequestMatchesProviderContracts(t *testing.T) {
	references := []resolvedReference{
		{Role: RoleFirstFrame, URL: "https://oss.test/first.png"},
		{Role: RoleLastFrame, URL: "asset://asset-1"},
	}

	seedance, _ := CapabilityFor("doubao-seedance-2-0")
	input := validInput("doubao-seedance-2-0")
	require.NoError(t, input.Validate(seedance))
	body, err := BuildTaskRequest(seedance, &input, references)
	require.NoError(t, err)
	data, err := common.Marshal(body)
	require.NoError(t, err)
	assert.JSONEq(t, `{"model":"doubao-seedance-2-0","prompt":"city at night","metadata":{
		"content":[{"type":"text","text":"city at night"},
			{"type":"image_url","image_url":{"url":"https://oss.test/first.png"},"role":"first_frame"},
			{"type":"image_url","image_url":{"url":"asset://asset-1"},"role":"last_frame"}],
		"duration":5,"resolution":"720p","ratio":"16:9"}}`, string(data))

	wan, _ := CapabilityFor("wan3.0-video")
	input = validInput("wan3.0-video")
	require.NoError(t, input.Validate(wan))
	body, err = BuildTaskRequest(wan, &input, references[:1])
	require.NoError(t, err)
	data, err = common.Marshal(body)
	require.NoError(t, err)
	assert.JSONEq(t, `{"model":"wan3.0-video","prompt":"city at night",
		"input":{"prompt":"city at night","media":[{"type":"first_frame","url":"https://oss.test/first.png"}]},
		"parameters":{"duration":5,"resolution":"720P","ratio":"16:9"}}`, string(data))

	minimax, _ := CapabilityFor("MiniMax-H3")
	input = validInput("MiniMax-H3")
	require.NoError(t, input.Validate(minimax))
	body, err = BuildTaskRequest(minimax, &input, nil)
	require.NoError(t, err)
	data, err = common.Marshal(body)
	require.NoError(t, err)
	assert.JSONEq(t, `{"model":"MiniMax-H3","prompt":"city at night","seconds":"5","metadata":{"resolution":"768P","ratio":"16:9"}}`, string(data))
}

func TestCapabilityMatchesByNameKeyword(t *testing.T) {
	tests := map[string]string{
		"doubao-seedance-2-0":  ProtocolSeedance,
		"Seedance-Pro":         ProtocolSeedance,
		"wan3.0-video":         ProtocolWan3,
		"alibaba/wan3.0-t2v":   ProtocolWan3,
		"MiniMax-H3":           ProtocolOpenAIVideo,
		"vendor-h3-video-fast": ProtocolOpenAIVideo,
	}
	for name, protocol := range tests {
		capability, ok := CapabilityFor(name)
		require.True(t, ok, name)
		assert.Equal(t, protocol, capability.Protocol, name)
		assert.Equal(t, name, capability.Model)
	}
	_, ok := CapabilityFor("gpt-4o")
	assert.False(t, ok)
}

func TestCapabilityRulesAreConfigurable(t *testing.T) {
	common.OptionMapRWMutex.Lock()
	previousMap := common.OptionMap
	common.OptionMap = map[string]string{}
	common.OptionMap[model_setting.VideoStudioModelRulesOptionKey] = `[{"keyword":" Kling ","protocol":"openai_video"}]`
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousMap
		common.OptionMapRWMutex.Unlock()
	})

	capability, ok := CapabilityFor("vendor-KLING-v2")
	require.True(t, ok)
	assert.Equal(t, ProtocolOpenAIVideo, capability.Protocol)
	_, ok = CapabilityFor("doubao-seedance-2-0")
	assert.False(t, ok, "saved rules replace the defaults")

	require.Error(t, model_setting.ValidateVideoStudioModelRules(`[{"keyword":"x","protocol":"unknown"}]`))
	require.Error(t, model_setting.ValidateVideoStudioModelRules(`[{"keyword":"a","protocol":"wan3"},{"keyword":"A","protocol":"wan3"}]`))
}
