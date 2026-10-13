package videostudio

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	storageService "github.com/QuantumNous/new-api/service/storage"
)

const (
	SourceStorage       = "storage"
	SourceSeedanceAsset = "seedance_asset"

	maxPromptRunes      = 4000
	maxClientRequestLen = 64
	maxNodeIDLen        = 64
)

type RequestError struct {
	StatusCode int
	Err        error
}

func (err *RequestError) Error() string       { return err.Err.Error() }
func (err *RequestError) Unwrap() error       { return err.Err }
func (err *RequestError) HTTPStatusCode() int { return err.StatusCode }

func badRequest(format string, args ...any) error {
	return &RequestError{StatusCode: http.StatusBadRequest, Err: fmt.Errorf(format, args...)}
}

type Reference struct {
	Source string `json:"source"`
	ID     string `json:"id"`
	Role   string `json:"role"`
}

// Parameters keeps optional scalars as pointers so an explicit false or 0
// is preserved instead of being replaced by a default.
type Parameters struct {
	Duration      *int    `json:"duration,omitempty"`
	Resolution    *string `json:"resolution,omitempty"`
	Ratio         *string `json:"ratio,omitempty"`
	GenerateAudio *bool   `json:"generate_audio,omitempty"`
	Seed          *int    `json:"seed,omitempty"`
}

// SubmitInput is the studio submission. It deliberately has no free-form
// metadata so clients cannot reach upstream fields the gateway does not check.
type SubmitInput struct {
	ProjectID       string `json:"project_id"`
	NodeID          string `json:"node_id"`
	ClientRequestID string `json:"client_request_id"`
	Model           string `json:"model"`
	// TokenID selects one of the user's API keys: its group and model
	// whitelist decide which models and channels the submission may use.
	TokenID    int         `json:"token_id,omitempty"`
	Group      string      `json:"group"`
	Prompt     string      `json:"prompt"`
	References []Reference `json:"references"`
	Parameters Parameters  `json:"parameters"`
}

// Validate normalizes the submission against the model capability and fills
// defaults. It does not touch storage or the database.
func (input *SubmitInput) Validate(capability Capability) error {
	input.Prompt = strings.TrimSpace(input.Prompt)
	if input.Prompt == "" || utf8.RuneCountInString(input.Prompt) > maxPromptRunes {
		return badRequest("prompt must contain 1 to %d characters", maxPromptRunes)
	}
	if input.ClientRequestID == "" || len(input.ClientRequestID) > maxClientRequestLen {
		return badRequest("client_request_id is required and must not exceed %d characters", maxClientRequestLen)
	}
	if len(input.NodeID) > maxNodeIDLen {
		return badRequest("node_id must not exceed %d characters", maxNodeIDLen)
	}
	if len(input.References) > capability.MaxReferences {
		return badRequest("%s accepts at most %d reference images", capability.Model, capability.MaxReferences)
	}
	seenRoles := map[string]bool{}
	for _, reference := range input.References {
		if reference.Source != SourceStorage && reference.Source != SourceSeedanceAsset {
			return badRequest("unsupported reference source %q", reference.Source)
		}
		if reference.Source == SourceSeedanceAsset && !capability.AssetLibrary {
			return badRequest("%s does not accept Seedance assets", capability.Model)
		}
		if !slices.Contains(capability.Roles, reference.Role) {
			return badRequest("%s does not support the %q reference role", capability.Model, reference.Role)
		}
		if reference.Role != RoleReferenceImage && seenRoles[reference.Role] {
			return badRequest("only one %s reference is allowed", reference.Role)
		}
		seenRoles[reference.Role] = true
	}
	if seenRoles[RoleReferenceImage] && (seenRoles[RoleFirstFrame] || seenRoles[RoleLastFrame]) {
		return badRequest("reference images cannot be combined with first or last frames")
	}
	if seenRoles[RoleLastFrame] && !seenRoles[RoleFirstFrame] {
		return badRequest("a last frame requires a first frame")
	}

	parameters := &input.Parameters
	if parameters.Duration == nil {
		parameters.Duration = common.GetPointer(capability.Defaults.Duration)
	}
	if *parameters.Duration < capability.MinDuration || *parameters.Duration > capability.MaxDuration {
		return badRequest("duration must be between %d and %d seconds", capability.MinDuration, capability.MaxDuration)
	}
	if parameters.Resolution == nil {
		parameters.Resolution = common.GetPointer(capability.Defaults.Resolution)
	}
	if !slices.Contains(capability.Resolutions, *parameters.Resolution) {
		return badRequest("unsupported resolution %q", *parameters.Resolution)
	}
	if parameters.Ratio == nil {
		parameters.Ratio = common.GetPointer(capability.Defaults.Ratio)
	}
	if !slices.Contains(capability.Ratios, *parameters.Ratio) {
		return badRequest("unsupported ratio %q", *parameters.Ratio)
	}
	if parameters.GenerateAudio != nil && !capability.Audio {
		return badRequest("%s does not support audio generation", capability.Model)
	}
	if parameters.Seed != nil && (!capability.Seed || *parameters.Seed < -1 || *parameters.Seed > 1<<31-1) {
		return badRequest("seed is not supported or out of range")
	}
	return nil
}

// Hash identifies the submission content for idempotency checks. Group is
// derived from the key when one is chosen, so it is not part of the identity.
func (input *SubmitInput) Hash() (string, error) {
	identity := *input
	if identity.TokenID != 0 {
		identity.Group = ""
	}
	data, err := common.Marshal(&identity)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

type resolvedReference struct {
	Role string
	URL  string
}

// ResolveReferences checks ownership and turns references into URLs the
// upstream can fetch. Seedance assets stay as asset:// handles; the doubao
// adaptor maps them to the selected channel's replica.
func ResolveReferences(ctx context.Context, ownerUserID int, references []Reference) ([]resolvedReference, error) {
	resolved := make([]resolvedReference, 0, len(references))
	for _, reference := range references {
		if reference.Source == SourceSeedanceAsset {
			asset, err := model.FindMediaAsset(reference.ID, ownerUserID, false)
			if err != nil {
				return nil, &RequestError{StatusCode: http.StatusNotFound, Err: fmt.Errorf("asset %s does not exist", reference.ID)}
			}
			if asset.AssetType != model.AssetTypeImage {
				return nil, badRequest("asset %s is not an image", reference.ID)
			}
			resolved = append(resolved, resolvedReference{Role: reference.Role, URL: "asset://" + asset.PublicID})
			continue
		}
		url, _, err := storageService.PresignVideoStudioReference(ctx, ownerUserID, reference.ID, false)
		if err != nil {
			return nil, &RequestError{StatusCode: http.StatusNotFound, Err: fmt.Errorf("reference %s does not exist or has expired; upload it again", reference.ID)}
		}
		resolved = append(resolved, resolvedReference{Role: reference.Role, URL: url})
	}
	return resolved, nil
}

// BuildTaskRequest converts a validated submission into the JSON body the
// existing /v1/video/generations task adaptor for the protocol accepts.
func BuildTaskRequest(capability Capability, input *SubmitInput, references []resolvedReference) (map[string]any, error) {
	parameters := input.Parameters
	body := map[string]any{"model": input.Model, "prompt": input.Prompt}
	if input.Group != "" {
		body["group"] = input.Group
	}
	switch capability.Protocol {
	case ProtocolSeedance:
		content := []map[string]any{{"type": "text", "text": input.Prompt}}
		for _, reference := range references {
			content = append(content, map[string]any{
				"type": "image_url", "image_url": map[string]any{"url": reference.URL}, "role": reference.Role,
			})
		}
		metadata := map[string]any{
			"content":    content,
			"duration":   *parameters.Duration,
			"resolution": *parameters.Resolution,
			"ratio":      *parameters.Ratio,
		}
		if parameters.GenerateAudio != nil {
			metadata["generate_audio"] = *parameters.GenerateAudio
		}
		if parameters.Seed != nil {
			metadata["seed"] = *parameters.Seed
		}
		body["metadata"] = metadata
	case ProtocolWan3:
		// Official DashScope shape; the ali adaptor merges input/parameters.
		input := map[string]any{"prompt": input.Prompt}
		if len(references) > 0 {
			media := make([]map[string]any, 0, len(references))
			for _, reference := range references {
				media = append(media, map[string]any{"type": reference.Role, "url": reference.URL})
			}
			input["media"] = media
		}
		wanParameters := map[string]any{
			"duration":   *parameters.Duration,
			"resolution": *parameters.Resolution,
			"ratio":      *parameters.Ratio,
		}
		if parameters.GenerateAudio != nil {
			wanParameters["audio"] = *parameters.GenerateAudio
		}
		if parameters.Seed != nil {
			wanParameters["seed"] = *parameters.Seed
		}
		body["input"] = input
		body["parameters"] = wanParameters
	case ProtocolOpenAIVideo:
		if len(references) > 0 {
			return nil, badRequest("%s does not accept reference images in the studio yet", capability.Model)
		}
		body["seconds"] = fmt.Sprint(*parameters.Duration)
		body["metadata"] = map[string]any{"resolution": *parameters.Resolution, "ratio": *parameters.Ratio}
	default:
		return nil, errors.New("unsupported video studio protocol")
	}
	return body, nil
}

// PreparedSubmission is a validated submission ready to send: ownership was
// checked, references resolved, and the provider task body built.
type PreparedSubmission struct {
	Project     *model.VideoProject
	Input       *SubmitInput
	RequestHash string
	Snapshot    string
	TaskBody    map[string]any
}

// PrepareSubmission validates a submission and builds its task body. It does
// not write: nothing is recorded until a channel has been selected.
// modelAvailable reports whether the user's group can reach input.Model.
func PrepareSubmission(ctx context.Context, ownerUserID int, input *SubmitInput, modelAvailable bool) (*PreparedSubmission, error) {
	// Hash the client's own submission so an identical retry matches even
	// though Validate fills in defaults below.
	hash, err := input.Hash()
	if err != nil {
		return nil, err
	}
	capability, ok := CapabilityFor(input.Model)
	if !ok {
		return nil, badRequest("model is not available in the video studio")
	}
	if !modelAvailable {
		return nil, &RequestError{StatusCode: http.StatusForbidden, Err: errors.New("model is not available for this group")}
	}
	if err = input.Validate(capability); err != nil {
		return nil, err
	}
	project, err := model.FindVideoProject(ownerUserID, input.ProjectID)
	if err != nil {
		return nil, err
	}
	references, err := ResolveReferences(ctx, ownerUserID, input.References)
	if err != nil {
		return nil, err
	}
	taskBody, err := BuildTaskRequest(capability, input, references)
	if err != nil {
		return nil, err
	}
	snapshot, err := common.Marshal(input)
	if err != nil {
		return nil, err
	}
	return &PreparedSubmission{Project: project, Input: input, RequestHash: hash, Snapshot: string(snapshot), TaskBody: taskBody}, nil
}
