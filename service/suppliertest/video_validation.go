package suppliertest

import (
	"fmt"
	"math"
	"strings"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/tidwall/gjson"
)

func ValidateVideoDraft(req RunRequest) error {
	if req.Video.ResumeRunID != "" {
		return nil
	}
	if len(req.Video.Checks) > 0 {
		submit := false
		for _, check := range req.Video.Checks {
			if check == CheckVideoSubmit {
				submit = true
			}
		}
		if !submit {
			return nil
		}
	}
	raw := req.Video.RawPayload
	if raw == "" {
		return nil
	}
	if len(raw) > 64<<20 {
		return fmt.Errorf("Video request exceeds 64 MiB")
	}
	if !gjson.Valid(raw) || !gjson.Parse(raw).IsObject() {
		return nil // Preserve the runner's existing failed-check contract for malformed native JSON.
	}
	duration := gjson.Get(raw, "duration")
	if duration.Exists() && (duration.Type != gjson.Number || math.IsNaN(duration.Num) || math.IsInf(duration.Num, 0) || duration.Num < -1 || duration.Num > relaycommon.MaxTaskDurationSeconds || math.Trunc(duration.Num) != duration.Num) {
		return fmt.Errorf("Video duration is outside the supported limit")
	}
	counts := map[string]int{}
	for _, item := range gjson.Get(raw, "content").Array() {
		if item.Get("type").String() != "image_url" {
			continue
		}
		source := item.Get("image_url.url").String()
		if source == "" {
			return fmt.Errorf("Each material requires a URL or image data")
		}
		role := item.Get("role").String()
		if role == "" {
			role = "first_frame"
		}
		if role != "first_frame" && role != "last_frame" && role != "reference_image" {
			return fmt.Errorf("Unsupported image role")
		}
		counts[role]++
	}
	maximum := 30
	model := gjson.Get(raw, "model").String()
	if model == "" {
		model = req.Model
	}
	if strings.Contains(model, "seedance-2-0") {
		maximum = 9
	}
	if counts["reference_image"] > maximum || counts["first_frame"] > 1 || counts["last_frame"] > 1 {
		return fmt.Errorf("Material count exceeds the model or scene limit")
	}
	if counts["reference_image"] > 0 && counts["first_frame"]+counts["last_frame"] > 0 {
		return fmt.Errorf("Reference images cannot be combined with first or last frames")
	}
	if counts["last_frame"] > 0 && counts["first_frame"] != 1 {
		return fmt.Errorf("An end frame requires a first frame")
	}
	switch req.Video.Scenario {
	case "text":
		if counts["reference_image"]+counts["first_frame"]+counts["last_frame"] != 0 {
			return fmt.Errorf("Text-to-video cannot include images")
		}
	case "first_frame":
		if counts["first_frame"] != 1 || counts["reference_image"]+counts["last_frame"] != 0 {
			return fmt.Errorf("The first-frame scene requires one first frame")
		}
	case "first_last_frame":
		if counts["first_frame"] != 1 || counts["last_frame"] != 1 || counts["reference_image"] != 0 {
			return fmt.Errorf("The first-and-last-frame scene requires both frames")
		}
	case "reference":
		if counts["reference_image"] < 1 || counts["first_frame"]+counts["last_frame"] != 0 {
			return fmt.Errorf("The reference scene requires reference images")
		}
	}
	return nil
}
