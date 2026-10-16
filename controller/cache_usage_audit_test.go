package controller

import (
	"encoding/csv"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupCacheUsageAuditTest(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	savedDB, savedLogDB := model.DB, model.LOG_DB
	model.DB, model.LOG_DB = db, db
	t.Cleanup(func() { model.DB, model.LOG_DB = savedDB, savedLogDB })
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Log{}, &model.Token{},
		&model.BillingCorrection{}, &model.BillingCorrectionRow{}, &model.BillingEntry{}))
	setupBillingPermissions(t, db)
	require.NoError(t, db.Create(&model.Log{UserId: 4, TokenId: 3, TokenName: "prod", ModelName: "gpt-5", RequestId: "req-cache-1",
		CreatedAt: 1_760_000_000, Type: model.LogTypeConsume, PromptTokens: 4921, CompletionTokens: 86, Quota: 5043,
		Other: `{"request_path":"/v1/messages","request_conversion":["Claude Messages","OpenAI Compatible"],"cache_tokens":4825,"model_ratio":10,"completion_ratio":5,"group_ratio":0.5,"cache_ratio":0.1}`}).Error)
}

func cacheUsageAuditRequest(role int, target string, body string) (*httptest.ResponseRecorder, *gin.Context) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("role", role)
	ctx.Set("id", 1)
	ctx.Request = httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	return recorder, ctx
}

const cacheUsageAuditBody = `{"user_id":4,"start_at":1759939200,"end_at":1760025600}`

func TestCacheUsageAuditQueryRequiresFinancialAccounting(t *testing.T) {
	setupCacheUsageAuditTest(t)
	recorder, ctx := cacheUsageAuditRequest(common.RoleCommonUser, "/api/billing/admin/cache-usage-audit", cacheUsageAuditBody)
	QueryCacheUsageAudit(ctx)
	assert.Equal(t, http.StatusForbidden, recorder.Code)

	recorder, ctx = cacheUsageAuditRequest(common.RoleRootUser, "/api/billing/admin/cache-usage-audit", cacheUsageAuditBody)
	QueryCacheUsageAudit(ctx)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Total struct {
				DuplicateTokens int64 `json:"duplicate_tokens"`
			} `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	assert.Equal(t, int64(4825), response.Data.Total.DuplicateTokens)

	recorder, ctx = cacheUsageAuditRequest(common.RoleRootUser, "/api/billing/admin/cache-usage-audit", `{"start_at":1,"end_at":99999999}`)
	QueryCacheUsageAudit(ctx)
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestCacheUsageAuditExportsCSVAndJSON(t *testing.T) {
	setupCacheUsageAuditTest(t)
	recorder, ctx := cacheUsageAuditRequest(common.RoleRootUser, "/api/billing/admin/cache-usage-audit/export?format=csv", cacheUsageAuditBody)
	ExportCacheUsageAudit(ctx)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.Contains(t, recorder.Header().Get("Content-Disposition"), "cache-usage-audit-20251009-20251009.csv")
	body := recorder.Body.String()
	require.True(t, strings.HasPrefix(body, "\xef\xbb\xbf"))
	reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(body, "\xef\xbb\xbf")))
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	require.NoError(t, err)
	require.Len(t, records, 3)
	assert.Equal(t, []string{CacheUsageAuditCompensationNote}, records[0])
	records = records[1:]
	assert.Equal(t, len(cacheUsageAuditCSVHeader), len(records[1]))
	column := func(name string) string {
		for i, header := range records[0] {
			if header == name {
				return records[1][i]
			}
		}
		t.Fatalf("missing column %s", name)
		return ""
	}
	assert.Equal(t, "req-cache-1", column("请求ID"))
	assert.Equal(t, "2025-10-09 16:53:20", column("时间"))
	assert.Equal(t, "4921", column("旧返回input_tokens"))
	assert.Equal(t, "96", column("正确input_tokens"))
	assert.Equal(t, "4825", column("多算token(缓存当输入)"))
	assert.Equal(t, "affected", column("核对状态"))
	assert.Equal(t, "messages", column("协议"))
	assert.Equal(t, "0.096500", column("应补-按官价"))
	assert.Equal(t, "0.048250", column("应补-按分组价"))
	assert.Equal(t, "", column("应补-按下游价"))

	recorder, ctx = cacheUsageAuditRequest(common.RoleRootUser, "/api/billing/admin/cache-usage-audit/export?format=json", cacheUsageAuditBody)
	ExportCacheUsageAudit(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)
	var exported struct {
		Rows []struct {
			RequestID       string `json:"request_id"`
			DuplicateTokens int64  `json:"duplicate_tokens"`
		} `json:"rows"`
		Summary struct {
			Total struct {
				Requests int64 `json:"requests"`
			} `json:"total"`
			Rows []any `json:"rows"`
		} `json:"summary"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &exported))
	require.Len(t, exported.Rows, 1)
	assert.Equal(t, int64(4825), exported.Rows[0].DuplicateTokens)
	assert.Equal(t, int64(1), exported.Summary.Total.Requests)
	assert.Nil(t, exported.Summary.Rows)

	recorder, ctx = cacheUsageAuditRequest(common.RoleRootUser, "/api/billing/admin/cache-usage-audit/export?format=xlsx", cacheUsageAuditBody)
	ExportCacheUsageAudit(ctx)
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}
