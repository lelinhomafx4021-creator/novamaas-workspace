package controller

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/assetlibrary"
	"github.com/QuantumNous/new-api/service/suppliertest"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSupplierVideoRecordCannotBeReadByAnotherAdministrator(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	previous := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = previous })
	require.NoError(t, db.AutoMigrate(&model.SupplierVideoRun{}))
	row := &model.SupplierVideoRun{RunID: "svr_owned", OwnerUserID: 7, Report: `{"id":"svr_owned","model":"actual-model","stages":[]}`}
	require.NoError(t, model.SaveSupplierVideoRun(context.Background(), row))
	for _, owner := range []int{7, 8} {
		response := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(response)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/supplier-test/video/runs/svr_owned", nil)
		c.Set("id", owner)
		c.Params = gin.Params{{Key: "id", Value: "svr_owned"}}
		GetSupplierVideoRun(c)
		if owner == 7 {
			assert.Equal(t, 200, response.Code)
			assert.Contains(t, response.Body.String(), "actual-model")
		} else {
			assert.Equal(t, 404, response.Code)
			assert.NotContains(t, response.Body.String(), "actual-model")
		}
	}
}

func TestOfficialCredentialsAreEncryptedAndNeverReturned(t *testing.T) {
	t.Setenv("STORAGE_CREDENTIAL_ENCRYPTION_KEY", "supplier-test-fixture-master-key")
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	previous := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = previous })
	require.NoError(t, db.AutoMigrate(&model.SupplierVerificationCredential{}))
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Set("id", 7)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/supplier-test/video/verification-settings", strings.NewReader(`{"access_key_id":"official-test-key","secret_access_key":"private-test-secret"}`))
	SupplierVerificationSettings(c)
	require.Equal(t, 200, response.Code)
	assert.NotContains(t, response.Body.String(), "private-test-secret")
	var row model.SupplierVerificationCredential
	require.NoError(t, db.First(&row).Error)
	assert.NotContains(t, row.EncryptedSecret, "private-test-secret")
	secret, err := assetlibrary.DecryptVerificationSecret(row.EncryptedSecret)
	require.NoError(t, err)
	assert.Equal(t, "private-test-secret", secret)
}

func TestVideoMaterialUploadUsesDefaultGroupAndPreservesFileMetadata(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	previous := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = previous })
	require.NoError(t, db.AutoMigrate(&model.SupplierVideoRun{}))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		assert.Equal(t, "Bearer generation-key", request.Header.Get("Authorization"))
		if request.Method == http.MethodGet {
			assert.Equal(t, "/api/asset-library/groups", request.URL.Path)
			_, _ = io.WriteString(w, `{"success":true,"data":[{"id":"default-group","name":"default","status":"ready"}]}`)
			return
		}
		require.NoError(t, request.ParseMultipartForm(1024))
		assert.Equal(t, "default-group", request.FormValue("group_id"))
		assert.Equal(t, "photo.heic", request.FormValue("name"))
		file, header, err := request.FormFile("file")
		require.NoError(t, err)
		defer file.Close()
		assert.Equal(t, "image/heic", header.Header.Get("Content-Type"))
		raw, err := io.ReadAll(file)
		require.NoError(t, err)
		assert.Equal(t, "fixture", string(raw))
		_, _ = io.WriteString(w, `{"success":true,"data":{"id":"asset-uploaded","name":"photo.heic","status":"processing"}}`)
	}))
	defer server.Close()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("base_url", server.URL))
	require.NoError(t, writer.WriteField("api_key", "generation-key"))
	headers := textproto.MIMEHeader{"Content-Disposition": {`form-data; name="file"; filename="photo.heic"`}, "Content-Type": {"image/heic"}}
	part, err := writer.CreatePart(headers)
	require.NoError(t, err)
	_, err = io.WriteString(part, "fixture")
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Set("id", 7)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/supplier-test/video/assets/upload", &body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	UploadSupplierVideoAsset(c)
	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "asset-uploaded")
	var run model.SupplierVideoRun
	require.NoError(t, db.First(&run).Error)
	assert.Contains(t, string(run.Report), "sha256")
	assert.Contains(t, string(run.Report), "request_headers")
	assert.Contains(t, string(run.Report), "Authorization: [REDACTED]")
	assert.NotContains(t, string(run.Report), "generation-key")
}

func TestManualVideoQueryRecordsEvidenceAndDistinguishesTerminalResults(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	previous := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = previous })
	require.NoError(t, db.AutoMigrate(&model.SupplierVideoRun{}, &model.StoragePolicy{}))
	for _, fixture := range []struct {
		name, body, status, stageID, stageStatus string
		httpStatus                               int
		final                                    bool
		videoURL                                 string
		truncated                                bool
	}{
		{"completed", `{"id":"manual-task","status":"succeeded","content":{"video_url":"https://cdn.example/video.mp4"}}`, "completed", "video_result", "pass", 200, true, "https://cdn.example/video.mp4", false},
		{"missing video", `{"id":"manual-task","status":"succeeded"}`, "failed", "video_result", "fail", 200, true, "", false},
		{"generation failed", `{"id":"manual-task","status":"failed","error":{"message":"generation rejected"}}`, "failed", "video_result", "fail", 200, true, "", false},
		{"HTTP failure", `{"id":"manual-task","status":"succeeded","content":{"video_url":"https://cdn.example/wrong.mp4"}}`, "failed", "video_result", "fail", 404, false, "", false},
		{"another task", `{"id":"another-task","status":"succeeded","content":{"video_url":"https://cdn.example/wrong.mp4"}}`, "failed", "video_result", "fail", 200, false, "", false},
		{"queued", `{"id":"manual-task","status":"queued"}`, "incomplete", "video_queued", "running", 200, false, "", false},
		{"running", `{"id":"manual-task","status":"running"}`, "incomplete", "video_running", "running", 200, false, "", false},
		{"incomplete response", `{}`, "failed", "video_result", "fail", 200, false, "", true},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				assert.Equal(t, "Bearer manual-secret-key", request.Header.Get("Authorization"))
				w.Header().Set("X-Request-Id", "manual-request-id")
				if fixture.truncated {
					w.Header().Set("Content-Length", "999")
				}
				w.WriteHeader(fixture.httpStatus)
				_, err := io.WriteString(w, fixture.body)
				require.NoError(t, err)
			}))
			defer server.Close()
			response := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(response)
			c.Set("id", 7)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/supplier-test/video/query", strings.NewReader(`{"base_url":"`+server.URL+`","api_key":"manual-secret-key","task_id":"manual-task"}`))
			QuerySupplierTestVideoTask(c)
			require.Equal(t, http.StatusOK, response.Code)
			var result struct {
				Report suppliertest.VideoRunReport `json:"video_report"`
			}
			require.NoError(t, common.Unmarshal(response.Body.Bytes(), &result))
			assert.Equal(t, "manual-task", result.Report.TaskID)
			assert.Equal(t, fixture.status, result.Report.Status)
			assert.Equal(t, fixture.videoURL, result.Report.VideoURL)
			assert.Equal(t, 1, result.Report.VideoPolling.Count)
			var actual *suppliertest.StageEvidence
			for i := range result.Report.Stages {
				if result.Report.Stages[i].ID == fixture.stageID {
					actual = &result.Report.Stages[i]
				}
			}
			require.NotNil(t, actual)
			assert.Equal(t, fixture.stageStatus, actual.Status)
			assert.Equal(t, fixture.final, actual.Final)
			expectedBody := fixture.body
			if fixture.truncated {
				expectedBody = ""
				assert.Equal(t, "Response could not be captured completely", actual.Conclusion)
			}
			assert.Equal(t, expectedBody, actual.ResponseBody)
			assert.Contains(t, actual.RequestHeaders, "Authorization: [REDACTED]")
			assert.Equal(t, "manual-request-id", actual.RequestID)
			assert.NotContains(t, response.Body.String(), "manual-secret-key")
			stored, err := model.GetSupplierVideoRun(context.Background(), result.Report.ID, 7)
			require.NoError(t, err)
			assert.Contains(t, string(stored.Report), "request_headers")
			assert.NotContains(t, string(stored.Report), "manual-secret-key")
		})
	}
}
