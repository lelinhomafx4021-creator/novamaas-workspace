package model_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateMiniAppPlaygroundMedia(t *testing.T) {
	cases := []struct {
		name  string
		raw   string
		valid bool
	}{
		{name: "disabled by default", raw: `{}`, valid: true},
		{name: "configured modalities", raw: `{"image_input_models":["vision"],"voice_input_models":["chat"],"voice_output_models":["chat"],"transcription_model":"whisper-1","speech_model":"tts-1","speech_voice":"alloy"}`, valid: true},
		{name: "missing transcription model", raw: `{"voice_input_models":["chat"]}`, valid: false},
		{name: "missing speech voice", raw: `{"voice_output_models":["chat"],"speech_model":"tts-1"}`, valid: false},
		{name: "duplicate chat model", raw: `{"image_input_models":["vision","vision"]}`, valid: false},
		{name: "invalid voice", raw: `{"speech_voice":"../../audio"}`, valid: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateMiniAppPlaygroundMedia(tc.raw)
			if tc.valid {
				require.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
