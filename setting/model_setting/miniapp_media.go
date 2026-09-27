package model_setting

import (
	"errors"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const MiniAppPlaygroundMediaOptionKey = "miniapp.playground_media"

type MiniAppPlaygroundMedia struct {
	ImageInputModels   []string `json:"image_input_models"`
	VoiceInputModels   []string `json:"voice_input_models"`
	VoiceOutputModels  []string `json:"voice_output_models"`
	TranscriptionModel string   `json:"transcription_model"`
	SpeechModel        string   `json:"speech_model"`
	SpeechVoice        string   `json:"speech_voice"`
}

var speechVoicePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,32}$`)

func ValidateMiniAppPlaygroundMedia(raw string) error {
	var settings MiniAppPlaygroundMedia
	if err := common.UnmarshalJsonStr(raw, &settings); err != nil {
		return err
	}
	for _, models := range [][]string{settings.ImageInputModels, settings.VoiceInputModels, settings.VoiceOutputModels} {
		if len(models) > 200 {
			return errors.New("too many mini program capability models")
		}
		seen := make(map[string]struct{}, len(models))
		for _, name := range models {
			if strings.TrimSpace(name) != name || name == "" || len(name) > 128 {
				return errors.New("invalid mini program capability model")
			}
			if _, exists := seen[name]; exists {
				return errors.New("duplicate mini program capability model")
			}
			seen[name] = struct{}{}
		}
	}
	if len(settings.TranscriptionModel) > 128 || len(settings.SpeechModel) > 128 ||
		strings.TrimSpace(settings.TranscriptionModel) != settings.TranscriptionModel ||
		strings.TrimSpace(settings.SpeechModel) != settings.SpeechModel {
		return errors.New("invalid mini program audio model")
	}
	if settings.SpeechVoice != "" && !speechVoicePattern.MatchString(settings.SpeechVoice) {
		return errors.New("invalid mini program speech voice")
	}
	if len(settings.VoiceInputModels) > 0 && settings.TranscriptionModel == "" {
		return errors.New("transcription model is required for voice input")
	}
	if len(settings.VoiceOutputModels) > 0 && (settings.SpeechModel == "" || settings.SpeechVoice == "") {
		return errors.New("speech model and voice are required for voice output")
	}
	return nil
}

func GetMiniAppPlaygroundMedia() MiniAppPlaygroundMedia {
	common.OptionMapRWMutex.RLock()
	raw := common.Interface2String(common.OptionMap[MiniAppPlaygroundMediaOptionKey])
	common.OptionMapRWMutex.RUnlock()
	var settings MiniAppPlaygroundMedia
	if raw != "" && ValidateMiniAppPlaygroundMedia(raw) == nil {
		_ = common.UnmarshalJsonStr(raw, &settings)
	}
	return settings
}

func HasModel(models []string, name string) bool {
	for _, model := range models {
		if model == name {
			return true
		}
	}
	return false
}
