package suppliertest

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/tidwall/gjson"
)

type VideoLibraryQuery struct {
	BaseURL  string `json:"base_url"`
	APIKey   string `json:"api_key"`
	Search   string `json:"search"`
	GroupID  string `json:"group_id"`
	Page     int    `json:"page"`
	Resource string `json:"resource"`
	AssetID  string `json:"asset_id"`
}

// EnsureVideoDefaultGroup reuses the platform's ordinary default group. The
// test workflow does not create a separate namespace for each test or task.
func EnsureVideoDefaultGroup(ctx context.Context, client *http.Client, baseURL, apiKey string) (string, []StageEvidence, error) {
	stage, err := QueryVideoLibrary(ctx, client, VideoLibraryQuery{BaseURL: baseURL, APIKey: apiKey, Resource: "groups"})
	stage.ID = "asset_default_group"
	stage.Purpose = "Resolve the default material group"
	if err != nil || stage.Status != "pass" {
		stage.Conclusion = "Default material group could not be resolved"
		return "", []StageEvidence{stage}, fmt.Errorf("default material group could not be resolved")
	}
	stage.Status = "pass"
	stage.Final = true
	stage.Conclusion = "Reused the default material group"
	for _, group := range gjson.Get(stage.ResponseBody, "data").Array() {
		name := strings.TrimSpace(group.Get("name").String())
		if (strings.EqualFold(name, "default") || name == "默认分组" || strings.EqualFold(name, "default group")) && group.Get("status").String() == "ready" {
			return group.Get("id").String(), []StageEvidence{stage}, nil
		}
	}
	stage.Conclusion = "No default material group exists; create it before upload"
	stages := []StageEvidence{stage}
	body, _ := common.Marshal(map[string]string{"name": "default"})
	endpoint, _ := url.Parse(baseURL)
	endpoint.Path = "/api/asset-library/groups"
	endpoint.RawQuery = ""
	endpoint.Fragment = ""
	created := StageEvidence{ID: "asset_default_group_create", Purpose: "Create the default material group", Method: "POST", Path: endpoint.Path, RequestBody: string(body), Status: "fail", Conclusion: "Default material group creation failed"}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return "", append(stages, created), err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+strings.TrimPrefix(strings.TrimSpace(apiKey), "Bearer "))
	started := time.Now()
	response, err := client.Do(request)
	created.ElapsedMS = time.Since(started).Milliseconds()
	if err != nil {
		return "", append(stages, created), err
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 4<<20+1))
	created.HTTPStatus = response.StatusCode
	if err != nil || len(raw) > 4<<20 {
		return "", append(stages, created), fmt.Errorf("default group response could not be captured completely")
	}
	created.ResponseBody = string(raw)
	created.Endpoint = request.URL.String()
	created.RequestID = firstNonEmpty(response.Header.Get("X-Request-Id"), response.Header.Get("X-Tt-Logid"))
	created.ElapsedMS = time.Since(started).Milliseconds()
	id := gjson.GetBytes(raw, "data.id").String()
	if response.StatusCode == 200 && gjson.GetBytes(raw, "success").Bool() && id != "" {
		created.Status = "pass"
		created.Final = true
		created.Conclusion = "Default material group created"
		return id, append(stages, created), nil
	}
	return "", append(stages, created), fmt.Errorf("default material group creation failed")
}

// QueryVideoLibrary uses the generation token, never the administrator's global
// asset visibility. The gateway still performs final channel replica checks.
func QueryVideoLibrary(ctx context.Context, client *http.Client, input VideoLibraryQuery) (StageEvidence, error) {
	stage := StageEvidence{Method: "GET", Status: "fail", Conclusion: "Material query failed"}
	endpoint, err := url.Parse(input.BaseURL)
	if err != nil || endpoint.Host == "" || endpoint.User != nil || (endpoint.Scheme != "http" && endpoint.Scheme != "https") {
		return stage, fmt.Errorf("invalid platform URL")
	}
	endpoint.Path = "/api/asset-library/assets"
	endpoint.RawQuery = ""
	endpoint.Fragment = ""
	query := endpoint.Query()
	if input.Page < 1 {
		input.Page = 1
	}
	query.Set("p", fmt.Sprint(input.Page))
	query.Set("page_size", "40")
	if input.Search != "" {
		query.Set("search", input.Search)
	}
	if input.GroupID != "" {
		query.Set("group_id", input.GroupID)
	}
	switch input.Resource {
	case "", "assets":
	case "groups":
		endpoint.Path = "/api/asset-library/groups"
		query = url.Values{}
	case "preview":
		if input.AssetID == "" || strings.ContainsAny(input.AssetID, "/?#") || strings.Contains(input.AssetID, "..") {
			return stage, fmt.Errorf("invalid material ID")
		}
		endpoint.Path += "/" + input.AssetID + "/preview"
		query = url.Values{"variant": {"original"}}
	default:
		return stage, fmt.Errorf("unsupported material resource")
	}
	endpoint.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return stage, err
	}
	request.Header.Set("Authorization", "Bearer "+strings.TrimPrefix(strings.TrimSpace(input.APIKey), "Bearer "))
	stage.Path = endpoint.RequestURI()
	stage.Endpoint = endpoint.String()
	started := time.Now()
	response, err := client.Do(request)
	if err != nil {
		return stage, err
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 4<<20+1))
	stage.HTTPStatus = response.StatusCode
	stage.ElapsedMS = time.Since(started).Milliseconds()
	stage.RequestID = firstNonEmpty(response.Header.Get("X-Request-Id"), response.Header.Get("X-Tt-Logid"))
	if len(raw) > 4<<20 {
		return stage, fmt.Errorf("asset response exceeds capture limit")
	}
	stage.ResponseBody = string(raw)
	if err == nil && response.StatusCode == 200 && gjson.GetBytes(raw, "success").Bool() {
		stage.Status = "pass"
	}
	return stage, err
}

func prepareVideoAssets(ctx context.Context, client *http.Client, req RunRequest, report *VideoRunReport, publish func(*VideoRunReport)) error {
	if req.Video.ResumeRunID != "" {
		return nil
	}
	content := gjson.Get(req.Video.RawPayload, "content")
	for index, item := range content.Array() {
		if item.Get("type").String() != "image_url" {
			continue
		}
		source := item.Get("image_url.url").String()
		asset := VideoAssetEvidence{Index: len(report.Assets), Role: item.Get("role").String(), URL: ReportURL(source), Source: "url"}
		if asset.Role == "" {
			asset.Role = "first_frame"
		}
		if strings.HasPrefix(source, "data:") {
			asset.Source = "base64"
		}
		if !strings.HasPrefix(source, "asset://") {
			report.Assets = append(report.Assets, asset)
			continue
		}
		asset.Source = "asset"
		asset.AssetID = strings.TrimPrefix(source, "asset://")
		asset.Reused = true
		for _, prepared := range req.PreparedStages {
			if gjson.Get(prepared.ResponseBody, "data.id").String() == asset.AssetID {
				asset.Reused = false
			}
		}
		if !req.Video.PlatformMode {
			report.Assets = append(report.Assets, asset)
			continue
		}
		deadline, cancel := context.WithTimeout(ctx, 5*time.Minute)
		stage := StageEvidence{ID: fmt.Sprintf("asset_%d", index), Purpose: "Check material ownership and readiness", Method: "GET", Status: "fail"}
		for {
			query, err := QueryVideoLibrary(deadline, client, VideoLibraryQuery{BaseURL: req.BaseURL, APIKey: req.APIKey, Search: asset.AssetID, Page: 1})
			query.ID = stage.ID
			query.Purpose = stage.Purpose
			stage = query
			if err != nil {
				stage.Conclusion = err.Error()
				break
			}
			if stage.Status != "pass" {
				break
			}
			stage.Status = "fail"
			raw := []byte(stage.ResponseBody)
			found := false
			for _, value := range gjson.GetBytes(raw, "data.items").Array() {
				if value.Get("id").String() == asset.AssetID {
					found = true
					asset.Status = value.Get("status").String()
					asset.Name = value.Get("name").String()
					asset.Size = value.Get("size").Int()
					asset.SHA256 = value.Get("sha256").String()
					break
				}
			}
			if !found {
				stage.Conclusion = "Material does not exist or is not accessible to the generation token"
				break
			}
			if asset.Status == "ready" {
				stage.Status = "pass"
				stage.Final = true
				stage.Conclusion = "Material is ready; the generation gateway will validate the selected channel replica"
				break
			}
			if asset.Status != "processing" {
				stage.Conclusion = "Material is unavailable"
				stage.Final = true
				break
			}
			timer := time.NewTimer(5 * time.Second)
			select {
			case <-deadline.Done():
				timer.Stop()
				stage.Conclusion = "Material readiness query timed out"
			case <-timer.C:
				continue
			}
			break
		}
		cancel()
		report.Assets = append(report.Assets, asset)
		report.SetStage(stage)
		publish(report)
		if stage.Status != "pass" {
			return fmt.Errorf("%s", stage.Conclusion)
		}
	}
	return nil
}
