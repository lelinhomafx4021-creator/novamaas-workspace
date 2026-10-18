package controller

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/assetlibrary"
	storageService "github.com/QuantumNous/new-api/service/storage"
	"github.com/QuantumNous/new-api/service/suppliertest"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func GetSupplierVideoRun(c *gin.Context) {
	row, err := model.GetSupplierVideoRun(c.Request.Context(), c.Param("id"), c.GetInt("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Video test record not found"})
		return
	}
	var report suppliertest.VideoRunReport
	if err = common.UnmarshalJsonStr(string(row.Report), &report); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, report)
}

func DownloadSupplierVideoEvidence(c *gin.Context) {
	row, err := model.GetSupplierVideoRun(c.Request.Context(), c.Param("id"), c.GetInt("id"))
	if err != nil || row.EvidenceObjectID == "" {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Original evidence is unavailable"})
		return
	}
	raw, err := storageService.ReadSupplierVideoEvidence(c.Request.Context(), row.EvidenceObjectID, c.GetInt("id"))
	if err != nil {
		c.JSON(http.StatusGone, gin.H{"success": false, "message": "Original evidence is unavailable or expired"})
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.Header("Content-Disposition", `attachment; filename="`+row.RunID+`-evidence.json"`)
	c.Data(http.StatusOK, "application/json", raw)
}

func SupplierVerificationSettings(c *gin.Context) {
	owner := c.GetInt("id")
	var credential model.SupplierVerificationCredential
	err := model.DB.Where("owner_user_id = ?", owner).First(&credential).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		common.ApiError(c, err)
		return
	}
	if c.Request.Method == http.MethodPut {
		var input struct {
			AccessKeyID string `json:"access_key_id"`
			Secret      string `json:"secret_access_key"`
		}
		if err = common.DecodeJson(c.Request.Body, &input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid verification credentials"})
			return
		}
		input.AccessKeyID = strings.TrimSpace(input.AccessKeyID)
		input.Secret = strings.TrimSpace(input.Secret)
		if input.AccessKeyID == "" || input.Secret == "" || len(input.AccessKeyID) > 256 || len(input.Secret) > 512 {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Official access key and secret are required"})
			return
		}
		encrypted, encryptErr := assetlibrary.EncryptVerificationSecret(input.Secret)
		if encryptErr != nil {
			common.ApiError(c, encryptErr)
			return
		}
		credential = model.SupplierVerificationCredential{OwnerUserID: owner, AccessKeyID: input.AccessKeyID, EncryptedSecret: encrypted, UpdatedAt: time.Now().Unix()}
		err = model.DB.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "owner_user_id"}}, DoUpdates: clause.AssignmentColumns([]string{"access_key_id", "encrypted_secret", "updated_at"})}).Create(&credential).Error
		if err != nil {
			common.ApiError(c, err)
			return
		}
	}
	hint := ""
	if len(credential.AccessKeyID) > 4 {
		hint = "****" + credential.AccessKeyID[len(credential.AccessKeyID)-4:]
	}
	common.ApiSuccess(c, gin.H{"configured": credential.EncryptedSecret != "", "access_key_hint": hint})
}

func QuerySupplierVideoAssets(c *gin.Context) {
	var input suppliertest.VideoLibraryQuery
	if err := common.DecodeJson(c.Request.Body, &input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid asset query"})
		return
	}
	if input.Page < 1 {
		input.Page = 1
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Minute)
	defer cancel()
	exchange, err := suppliertest.QueryVideoLibrary(ctx, suppliertest.NewHTTPClient(service.GetHttpClient()), input)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if exchange.Status != "pass" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Material query failed for the generation token"})
		return
	}
	c.Data(http.StatusOK, "application/json", []byte(exchange.ResponseBody))
}

// UploadSupplierVideoAsset uses the same token and public multipart endpoint as
// generation customers. The recorded preparation receipt can be bound to a run.
func UploadSupplierVideoAsset(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 33<<20)
	header, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "An image up to 30 MB is required"})
		return
	}
	if header.Size <= 0 || header.Size > 30<<20 {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"success": false, "message": "Image exceeds 30 MB"})
		return
	}
	endpoint, err := url.Parse(c.PostForm("base_url"))
	if err != nil || endpoint.Host == "" || endpoint.User != nil || (endpoint.Scheme != "http" && endpoint.Scheme != "https") {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid platform URL"})
		return
	}
	endpoint.Path = "/api/asset-library/assets"
	endpoint.RawQuery = ""
	endpoint.Fragment = ""
	client := suppliertest.NewHTTPClient(service.GetHttpClient())
	groupCtx, cancel := context.WithTimeout(c.Request.Context(), time.Minute)
	groupID, groupStages, err := suppliertest.EnsureVideoDefaultGroup(groupCtx, client, c.PostForm("base_url"), c.PostForm("api_key"))
	cancel()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Default material group could not be resolved", "stages": groupStages})
		return
	}
	file, err := header.Open()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	defer file.Close()
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	_ = writer.WriteField("group_id", groupID)
	_ = writer.WriteField("name", header.Filename)
	_ = writer.WriteField("type", "image")
	fileHeaders := make(textproto.MIMEHeader)
	fileHeaders.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": "file", "filename": header.Filename}))
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	fileHeaders.Set("Content-Type", contentType)
	part, err := writer.CreatePart(fileHeaders)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	digest := sha256.New()
	if _, err = io.Copy(io.MultiWriter(part, digest), file); err != nil {
		common.ApiError(c, err)
		return
	}
	_ = writer.Close()
	request, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, endpoint.String(), &buffer)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+strings.TrimPrefix(c.PostForm("api_key"), "Bearer "))
	requestHeaders := suppliertest.ReportRequestHeaders(request)
	started := time.Now()
	response, err := client.Do(request)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 4<<20+1))
	if err != nil || len(raw) > 4<<20 {
		common.ApiError(c, fmt.Errorf("upload response exceeds capture limit"))
		return
	}
	runID := "svr_" + common.GetRandomString(24)
	requestBody, _ := common.Marshal(map[string]any{"group_id": groupID, "name": header.Filename, "type": "image", "file": map[string]any{"name": header.Filename, "bytes": header.Size, "content_type": contentType, "sha256": hex.EncodeToString(digest.Sum(nil))}, "encoding": "multipart/form-data"})
	stage := suppliertest.StageEvidence{ID: "asset_upload_" + gjson.GetBytes(raw, "data.id").String(), Purpose: "Upload a material to the platform library", Method: "POST", Path: endpoint.Path, Endpoint: endpoint.String(), RequestHeaders: requestHeaders, RequestID: response.Header.Get("X-Request-Id"), RequestBody: string(requestBody), ResponseBody: string(raw), HTTPStatus: response.StatusCode, ElapsedMS: time.Since(started).Milliseconds(), Final: true, Status: "fail", Conclusion: "Material upload failed"}
	if response.StatusCode == 200 && gjson.GetBytes(raw, "success").Bool() {
		stage.Status = "pass"
		stage.Conclusion = "Material uploaded; readiness will be checked before generation"
	}
	report := suppliertest.VideoRunReport{ID: runID, Status: "prepared", Stages: []suppliertest.StageEvidence{}, StartedAt: time.Now().UnixMilli()}
	for _, groupStage := range groupStages {
		report.SetStage(groupStage)
	}
	report.SetStage(stage)
	reportJSON, _ := common.Marshal(report)
	row := model.SupplierVideoRun{RunID: runID, OwnerUserID: c.GetInt("id"), Report: model.SupplierVideoDocument(reportJSON)}
	if err = model.SaveSupplierVideoRun(c.Request.Context(), &row); err != nil {
		common.ApiError(c, err)
		return
	}
	if stage.Status != "pass" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Material upload failed", "preparation_run_id": runID, "stages": report.Stages})
		return
	}
	common.ApiSuccess(c, gin.H{"asset": gjson.GetBytes(raw, "data").Value(), "preparation_run_id": runID, "stages": report.Stages})
}

func runRecordedSupplierVideo(c *gin.Context, ctx context.Context, req suppliertest.RunRequest, emit suppliertest.Emitter) error {
	owner := c.GetInt("id")
	runID := "svr_" + common.GetRandomString(24)
	row := &model.SupplierVideoRun{RunID: runID, OwnerUserID: owner, Report: "{}"}
	var previous *suppliertest.VideoRunReport
	if req.Video.ResumeRunID != "" {
		var err error
		row, err = model.GetSupplierVideoRun(ctx, req.Video.ResumeRunID, owner)
		if err != nil {
			return fmt.Errorf("video test record not found")
		}
		previous = &suppliertest.VideoRunReport{}
		if err = common.UnmarshalJsonStr(string(row.Report), previous); err != nil {
			return err
		}
		if previous.Status == "running" {
			return fmt.Errorf("this video test is still running")
		}
		if previous.VideoURL == "" {
			return fmt.Errorf("this record has no video artifact to verify")
		}
		req.BaseURL = previous.BaseURL
		req.Model = previous.Model
		req.Video.PlatformMode = previous.PlatformMode
		if row.EvidenceObjectID != "" {
			raw, readErr := storageService.ReadSupplierVideoEvidence(ctx, row.EvidenceObjectID, owner)
			if readErr != nil {
				return fmt.Errorf("original evidence is unavailable or expired; cannot safely supplement this record")
			}
			if err = common.Unmarshal(raw, &previous.RawEvidence); err != nil {
				return fmt.Errorf("original evidence is unreadable; cannot safely supplement this record")
			}
			if stage, ok := previous.RawEvidence["video_result"]; ok {
				if previous.OriginalVideoURL == "" {
					_, previous.OriginalVideoURL, _ = suppliertest.ParseVideoTaskState([]byte(stage.ResponseBody))
				}
			}
		} else {
			previous.RawEvidence = map[string]suppliertest.StageEvidence{}
			for _, stage := range previous.Stages {
				previous.RawEvidence[stage.ID] = stage
			}
			previous.EvidenceError = "Earlier original evidence is unavailable; its attachment entries contain redacted snapshots"
		}
		runID = row.RunID
		req.Video.PreparationRunIDs = nil
	}
	for _, preparationID := range req.Video.PreparationRunIDs {
		prep, err := model.GetSupplierVideoRun(ctx, preparationID, owner)
		if err != nil {
			return fmt.Errorf("material preparation evidence not found")
		}
		var prepared suppliertest.VideoRunReport
		if err = common.UnmarshalJsonStr(string(prep.Report), &prepared); err != nil {
			return err
		}
		if prepared.Status != "prepared" {
			return fmt.Errorf("invalid material preparation record")
		}
		used := false
		for _, stage := range prepared.Stages {
			assetID := gjson.Get(stage.ResponseBody, "data.id").String()
			for _, content := range gjson.Get(req.Video.RawPayload, "content").Array() {
				if assetID != "" && content.Get("image_url.url").String() == "asset://"+assetID {
					used = true
					break
				}
			}
		}
		if used {
			req.PreparedStages = append(req.PreparedStages, prepared.Stages...)
		}
	}
	if req.Video.VerifyOfficial {
		var credential model.SupplierVerificationCredential
		if err := model.DB.Where("owner_user_id = ?", owner).First(&credential).Error; err == nil {
			secret, err := assetlibrary.DecryptVerificationSecret(credential.EncryptedSecret)
			if err != nil {
				return err
			}
			req.VerificationCredentials = &suppliertest.OfficialCredentials{AccessKeyID: credential.AccessKeyID, Secret: secret}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	}
	if previous != nil {
		copyReport := *previous
		copyReport.Status = "running"
		encoded, _ := common.Marshal(copyReport)
		if err := model.ClaimSupplierVideoVerification(ctx, row, string(encoded)); err != nil {
			return fmt.Errorf("this video test was updated by another request")
		}
	} else if err := model.SaveSupplierVideoRun(ctx, row); err != nil {
		return err
	}
	var saveErr error
	sentStages := map[string]suppliertest.StageEvidence{}
	publish := func(event suppliertest.Event) {
		if event.VideoReport != nil {
			if req.Video.PlatformMode && event.VideoReport.TaskID != "" && event.VideoReport.ExpectedModel == "" {
				target, _ := url.Parse(req.BaseURL)
				if strings.EqualFold(target.Host, c.Request.Host) {
					if snapshots, found, readErr := storageService.ReadArchivedRequestSnapshots(ctx, event.VideoReport.TaskID, ""); readErr == nil && found {
						event.VideoReport.ExpectedModel = gjson.GetBytes(snapshots.Upstream, "model").String()
					}
				}
			}

			encoded, err := common.Marshal(event.VideoReport)
			if err == nil {
				row.Report = model.SupplierVideoDocument(encoded)
				saveCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				err = model.SaveSupplierVideoRun(saveCtx, row)
				cancel()
			}
			if err != nil {
				saveErr = err
			}
			// Stream each changed stage once. The complete snapshot remains available
			// through the run endpoint, without repeating large bodies on SSE updates.
			delta := *event.VideoReport
			delta.Stages = []suppliertest.StageEvidence{}
			for _, stage := range event.VideoReport.Stages {
				if sentStages[stage.ID] != stage {
					delta.Stages = append(delta.Stages, stage)
					sentStages[stage.ID] = stage
				}
			}
			event.VideoReport = &delta
			event.VideoReportDelta = true
		}
		emit(event)
	}
	report := suppliertest.RunVideoReport(ctx, suppliertest.NewHTTPClient(service.GetHttpClient()), service.GetSSRFProtectedHTTPClient(), req, runID, previous, publish)
	oldEvidenceObjectID := row.EvidenceObjectID
	if len(report.RawEvidence) > 0 {
		raw, err := common.Marshal(report.RawEvidence)
		if err == nil {
			storeCtx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			object, storeErr := storageService.PutSupplierVideoEvidence(storeCtx, owner, runID, raw)
			cancel()
			if storeErr == nil {
				row.EvidenceObjectID = object.ObjectID
				report.EvidenceAvailable = true
			} else {
				report.EvidenceError = "Original evidence storage is unavailable; check the request snapshot storage policy"
			}
		}
	}
	publish(suppliertest.Event{Type: "evidence", Module: suppliertest.ModuleVideo, VideoReport: report})
	if saveErr != nil {
		return fmt.Errorf("failed to save video test evidence")
	}
	if oldEvidenceObjectID != "" && oldEvidenceObjectID != row.EvidenceObjectID {
		var old model.StorageObject
		if model.DB.Where("object_id = ? AND owner_user_id = ?", oldEvidenceObjectID, owner).First(&old).Error == nil {
			_ = model.MarkStorageObjectForDeletion(old.ID, time.Now().Unix(), "superseded supplier test evidence")
		}
	}
	emit(suppliertest.Event{Type: "done"})
	return nil
}

func recordSupplierVideoQuery(c *gin.Context, baseURL, taskID string, stage suppliertest.StageEvidence) (*suppliertest.VideoRunReport, error) {
	report := &suppliertest.VideoRunReport{ID: "svr_" + common.GetRandomString(24), BaseURL: baseURL, Model: gjson.Get(stage.ResponseBody, "model").String(), TaskID: taskID, Status: "incomplete", StartedAt: time.Now().UnixMilli(), Assets: []suppliertest.VideoAssetEvidence{}, Stages: []suppliertest.StageEvidence{}}
	report.SetStage(suppliertest.StageEvidence{ID: "video_submit", Purpose: "Submit the video task", Status: "skip", Conclusion: "Original submission evidence is unavailable for this task query"})
	stage = report.EvaluateVideoQuery(stage)
	if stage.Final {
		report.Status = "completed"
		report.CompletedAt = time.Now().UnixMilli()
	}
	if stage.Status == "fail" {
		report.Status = "failed"
	}
	report.VideoPolling = suppliertest.VideoPollingSummary{Count: 1, ElapsedMS: stage.ElapsedMS}
	report.SetStage(stage)
	report.MarkUnexecutedStages(false)
	encoded, err := common.Marshal(report)
	if err != nil {
		return nil, err
	}
	row := &model.SupplierVideoRun{RunID: report.ID, OwnerUserID: c.GetInt("id"), Report: model.SupplierVideoDocument(encoded)}
	if err = model.SaveSupplierVideoRun(c.Request.Context(), row); err != nil {
		return nil, err
	}
	rawEvidence, err := common.Marshal(report.RawEvidence)
	if err != nil {
		return nil, err
	}
	object, storeErr := storageService.PutSupplierVideoEvidence(c.Request.Context(), row.OwnerUserID, report.ID, rawEvidence)
	if storeErr == nil {
		row.EvidenceObjectID = object.ObjectID
		report.EvidenceAvailable = true
	} else {
		report.EvidenceError = "Original evidence storage is unavailable; check the request snapshot storage policy"
	}
	encoded, err = common.Marshal(report)
	if err != nil {
		return nil, err
	}
	row.Report = model.SupplierVideoDocument(encoded)
	return report, model.SaveSupplierVideoRun(c.Request.Context(), row)
}
