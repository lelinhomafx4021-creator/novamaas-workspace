package controller

import (
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/gin-gonic/gin"
)

const (
	maxPlaygroundImageBytes  = 1 << 20
	maxPlaygroundImages      = 4
	maxPlaygroundAudioBytes  = 10 << 20
	maxPlaygroundSpeechRunes = 2000
)

func playgroundAvailableModels(c *gin.Context, group string) (map[string]bool, bool) {
	user, err := model.GetUserCache(c.GetInt("id"))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "user unavailable"})
		return nil, false
	}
	if group == "" {
		group = user.Group
	}
	var groups []string
	if group == "auto" {
		if !service.GroupInUserUsableGroups(user.Group, "auto") {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "group unavailable"})
			return nil, false
		}
		groups = service.GetUserAutoGroup(user.Group)
	} else {
		if !service.GroupInUserUsableGroups(user.Group, group) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "group unavailable"})
			return nil, false
		}
		groups = []string{group}
	}
	available := make(map[string]bool)
	for _, name := range service.GetGroupsEnabledModels(groups) {
		available[name] = true
	}
	return available, true
}

func GetPlaygroundMedia(c *gin.Context) {
	available, ok := playgroundAvailableModels(c, c.Query("group"))
	if !ok {
		return
	}
	settings := model_setting.GetMiniAppPlaygroundMedia()
	filter := func(models []string) []string {
		result := make([]string, 0, len(models))
		for _, name := range models {
			if available[name] {
				result = append(result, name)
			}
		}
		return result
	}
	settings.ImageInputModels = filter(settings.ImageInputModels)
	settings.VoiceInputModels = filter(settings.VoiceInputModels)
	settings.VoiceOutputModels = filter(settings.VoiceOutputModels)
	if !available[settings.TranscriptionModel] {
		settings.TranscriptionModel = ""
		settings.VoiceInputModels = []string{}
	}
	if !available[settings.SpeechModel] {
		settings.SpeechModel = ""
		settings.VoiceOutputModels = []string{}
	}
	common.ApiSuccess(c, settings)
}

func ValidatePlaygroundChat(c *gin.Context) {
	var request struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content any    `json:"content"`
		} `json:"messages"`
	}
	if err := common.UnmarshalBodyReusable(c, &request); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid chat request"})
		return
	}
	settings := model_setting.GetMiniAppPlaygroundMedia()
	imageCount := 0
	for _, message := range request.Messages {
		parts, ok := message.Content.([]any)
		if !ok {
			continue
		}
		for _, raw := range parts {
			part, ok := raw.(map[string]any)
			if !ok {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid media part"})
				return
			}
			kind, _ := part["type"].(string)
			if kind == "text" {
				if _, ok := part["text"].(string); !ok {
					c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid text part"})
					return
				}
				continue
			}
			if kind != "image_url" || message.Role != "user" || !model_setting.HasModel(settings.ImageInputModels, request.Model) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "image input unavailable for this model"})
				return
			}
			image, ok := part["image_url"].(map[string]any)
			url, _ := image["url"].(string)
			prefix, payload, found := strings.Cut(url, ",")
			if !ok || !found || len(payload) > base64.StdEncoding.EncodedLen(maxPlaygroundImageBytes) ||
				(prefix != "data:image/jpeg;base64" && prefix != "data:image/png;base64" && prefix != "data:image/webp;base64") {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid image attachment"})
				return
			}
			decoded, err := base64.StdEncoding.DecodeString(payload)
			if err != nil || len(decoded) == 0 || len(decoded) > maxPlaygroundImageBytes {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid image attachment"})
				return
			}
			imageCount++
			if imageCount > maxPlaygroundImages {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "too many image attachments"})
				return
			}
		}
	}
}

func ValidatePlaygroundAudio(c *gin.Context) {
	var request struct {
		Model     string `json:"model"`
		Group     string `json:"group"`
		ChatModel string `json:"chat_model"`
		Input     string `json:"input"`
		Voice     string `json:"voice"`
	}
	if err := common.UnmarshalBodyReusable(c, &request); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid audio request"})
		return
	}
	available, ok := playgroundAvailableModels(c, request.Group)
	if !ok {
		return
	}
	settings := model_setting.GetMiniAppPlaygroundMedia()
	if !available[request.Model] || !available[request.ChatModel] {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "audio model unavailable"})
		return
	}
	if strings.HasSuffix(c.Request.URL.Path, "/speech") {
		if request.Model != settings.SpeechModel || request.Voice != settings.SpeechVoice ||
			!model_setting.HasModel(settings.VoiceOutputModels, request.ChatModel) ||
			len([]rune(request.Input)) == 0 || len([]rune(request.Input)) > maxPlaygroundSpeechRunes {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "speech output unavailable or too long"})
		}
		return
	}
	if request.Model != settings.TranscriptionModel ||
		!model_setting.HasModel(settings.VoiceInputModels, request.ChatModel) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "voice input unavailable"})
		return
	}
	form, err := common.ParseMultipartFormReusable(c)
	if err != nil || len(form.File["file"]) != 1 || form.File["file"][0].Size == 0 ||
		form.File["file"][0].Size > maxPlaygroundAudioBytes {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid audio file"})
		return
	}
	c.Next()
	_ = form.RemoveAll()
}

func PlaygroundAudio(c *gin.Context) {
	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatOpenAIAudio, nil, nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := setupPlaygroundToken(c, relayInfo.UsingGroup); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user unavailable"})
		return
	}
	Relay(c, types.RelayFormatOpenAIAudio)
}
