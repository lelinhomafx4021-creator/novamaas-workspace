package controller

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service/videostudio"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type studioFixture struct {
	user      *model.User
	channel   *model.Channel
	project   *model.VideoProject
	upstream  *httptest.Server
	calls     *atomic.Int32
	lastBody  *atomic.Value
	lastPath  *atomic.Value
	statusFor func() int
}

var videoStudioTables = []any{
	&model.Task{}, &model.Channel{}, &model.User{}, &model.Token{}, &model.Log{},
	&model.BillingAccount{}, &model.BillingAccountEvent{}, &model.BillingEntry{}, &model.BillingHour{},
	&model.BillingOperation{}, &model.BillingStatement{}, &model.StorageObject{}, &model.MediaAsset{},
	&model.VideoProject{}, &model.VideoCreationRun{}, &model.TaskRequestBody{}, &model.Ability{},
	&model.UserSubscription{}, &model.SubscriptionPlan{}, &model.SubscriptionPreConsumeRecord{},
	&model.AssetWebhookEndpoint{}, &model.AssetWebhookDelivery{},
}

// setupVideoStudioSubmit runs on SQLite by default and on MySQL when
// VIDEO_STUDIO_MYSQL_DSN points at a disposable database.
func setupVideoStudioSubmit(t *testing.T) *studioFixture {
	t.Helper()
	initModelListColumnNames(t)
	var db *gorm.DB
	var err error
	if dsn := strings.TrimSpace(os.Getenv("VIDEO_STUDIO_MYSQL_DSN")); dsn != "" {
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, db.Migrator().DropTable(videoStudioTables...))
		previousDB, previousLogDB, previousDBType := model.DB, model.LOG_DB, common.MainDatabaseType()
		previousRedis, previousBatch, previousLog := common.RedisEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled
		model.DB, model.LOG_DB = db, db
		common.SetDatabaseTypes(common.DatabaseTypeMySQL, common.DatabaseTypeMySQL)
		common.RedisEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled = false, false, false
		t.Cleanup(func() {
			model.DB, model.LOG_DB = previousDB, previousLogDB
			common.SetDatabaseTypes(previousDBType, previousDBType)
			common.RedisEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled = previousRedis, previousBatch, previousLog
		})
	} else {
		db = setupVolcNativeControllerTest(t)
		// ":memory:" SQLite is per connection; async billing work must reuse it.
		sqlDB, dbErr := db.DB()
		require.NoError(t, dbErr)
		sqlDB.SetMaxOpenConns(1)
	}
	// initModelListColumnNames set backtick quoting, which MySQL shares.
	require.NoError(t, db.AutoMigrate(videoStudioTables...))
	savedPrices := ratio_setting.ModelPrice2JSONString()
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"MiniMax-H3":0.1}`))
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(savedPrices)) })

	fixture := &studioFixture{calls: &atomic.Int32{}, lastBody: &atomic.Value{}, lastPath: &atomic.Value{}, statusFor: func() int { return http.StatusOK }}
	fixture.upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fixture.calls.Add(1)
		fixture.lastPath.Store(r.URL.Path)
		body, _ := io.ReadAll(r.Body)
		fixture.lastBody.Store(string(body))
		w.Header().Set("Content-Type", "application/json")
		status := fixture.statusFor()
		w.WriteHeader(status)
		if status != http.StatusOK {
			_, _ = w.Write([]byte(`{"error":{"message":"upstream rejected"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"upstream-video-1","object":"video","status":"queued","model":"MiniMax-H3"}`))
	}))
	t.Cleanup(fixture.upstream.Close)

	baseURL := fixture.upstream.URL
	fixture.channel = &model.Channel{Type: constant.ChannelTypeOpenAI, Key: "upstream-key", BaseURL: &baseURL, Status: common.ChannelStatusEnabled}
	require.NoError(t, db.Create(fixture.channel).Error)
	require.NoError(t, db.Create(&model.Ability{Group: "default", Model: "MiniMax-H3", ChannelId: fixture.channel.Id, Enabled: true}).Error)
	var project *model.VideoProject
	fixture.user = &model.User{Username: "studio-user", Quota: 1_000_000, Group: "default", AffCode: "studio-aff"}
	require.NoError(t, db.Create(fixture.user).Error)
	project, err = videostudio.CreateProject(fixture.user.Id, videostudio.ProjectInput{})
	require.NoError(t, err)
	fixture.project = project
	return fixture
}

// submit runs the post-validation half of POST /pg/videos with the channel
// already selected, mirroring what Distribute leaves in the context.
func (fixture *studioFixture) submit(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/pg/videos", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("id", fixture.user.Id)
	common.SetContextKey(c, constant.ContextKeyUserId, fixture.user.Id)
	common.SetContextKey(c, constant.ContextKeyUserQuota, fixture.user.Quota)
	common.SetContextKey(c, constant.ContextKeyUserSetting, dto.UserSetting{BillingPreference: "wallet_only"})
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
	common.SetContextKey(c, constant.ContextKeyUserGroup, "default")

	ValidatePlaygroundVideo(c)
	if c.IsAborted() {
		return recorder
	}
	common.SetContextKey(c, constant.ContextKeyOriginalModel, gjson.Get(body, "model").String())
	common.SetContextKey(c, constant.ContextKeyChannelId, fixture.channel.Id)
	common.SetContextKey(c, constant.ContextKeyChannelType, fixture.channel.Type)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, fixture.upstream.URL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "upstream-key")
	c.Set("channel_type", fixture.channel.Type)
	c.Set("relay_mode", 0)
	PlaygroundVideo(c)
	return recorder
}

func studioBody(projectID string, clientRequestID string, prompt string) string {
	return `{"project_id":"` + projectID + `","node_id":"n1","client_request_id":"` + clientRequestID +
		`","model":"MiniMax-H3","prompt":"` + prompt + `","parameters":{"duration":5}}`
}

func TestPlaygroundVideoLinksRunToTaskAndReturnsStudioShape(t *testing.T) {
	fixture := setupVideoStudioSubmit(t)

	recorder := fixture.submit(t, studioBody(fixture.project.PublicID, "req-1", "city at night"))

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	response := recorder.Body.String()
	assert.True(t, gjson.Get(response, "success").Bool())
	taskID := gjson.Get(response, "data.task_id").String()
	require.True(t, strings.HasPrefix(taskID, "task_"))
	assert.Equal(t, "queued", gjson.Get(response, "data.status").String(), "a just-submitted task is queued, not unknown")
	assert.False(t, gjson.Get(response, "object").Exists(), "provider response must not leak into the studio response")
	assert.JSONEq(t, `{"model":"MiniMax-H3","prompt":"city at night","seconds":"5","metadata":{"resolution":"768P","ratio":"16:9"}}`, fixture.lastBody.Load().(string))

	task, exists, err := model.GetByTaskId(fixture.user.Id, taskID)
	require.NoError(t, err)
	require.True(t, exists, "the run's reserved task ID must be the persisted task ID")
	assert.Equal(t, "upstream-video-1", task.GetUpstreamTaskID())
	assert.Positive(t, task.Quota)
}

func TestPlaygroundSeedanceSubmitsNativeV3Request(t *testing.T) {
	fixture := setupVideoStudioSubmit(t)
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"MiniMax-H3":0.1,"doubao-seedance-2-0":0.1}`))
	baseURL := fixture.upstream.URL
	channel := &model.Channel{Type: constant.ChannelTypeVolcNative, Key: "upstream-key", BaseURL: &baseURL, Status: common.ChannelStatusEnabled}
	require.NoError(t, model.DB.Create(channel).Error)
	require.NoError(t, model.DB.Create(&model.Ability{Group: "default", Model: "doubao-seedance-2-0", ChannelId: channel.Id, Enabled: true}).Error)
	fixture.channel = channel
	body := `{"project_id":"` + fixture.project.PublicID + `","node_id":"n1","client_request_id":"seedance-v3",` +
		`"model":"doubao-seedance-2-0","prompt":"city at night","parameters":{"duration":5,"generate_audio":false,"seed":0}}`

	recorder := fixture.submit(t, body)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.Equal(t, "/api/v3/contents/generations/tasks", fixture.lastPath.Load())
	assert.JSONEq(t, `{"model":"doubao-seedance-2-0","content":[{"type":"text","text":"city at night"}],`+
		`"duration":5,"resolution":"720p","ratio":"16:9","generate_audio":false,"seed":0}`, fixture.lastBody.Load().(string))
	assert.Equal(t, int32(1), fixture.calls.Load())
	taskID := gjson.Get(recorder.Body.String(), "data.task_id").String()
	task, exists, err := model.GetByTaskId(fixture.user.Id, taskID)
	require.NoError(t, err)
	require.True(t, exists)
	assert.Equal(t, "upstream-video-1", task.GetUpstreamTaskID())
	assert.Positive(t, task.Quota)
}

func TestPlaygroundSeedanceDistributionExcludesCompatibleChannels(t *testing.T) {
	fixture := setupVideoStudioSubmit(t)
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"MiniMax-H3":0.1,"doubao-seedance-2-0":0.1}`))
	wasMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { common.MemoryCacheEnabled = wasMemoryCacheEnabled })
	baseURL := fixture.upstream.URL
	compatible := &model.Channel{Type: constant.ChannelTypeDoubaoVideo, Key: "upstream-key", BaseURL: &baseURL, Status: common.ChannelStatusEnabled}
	native := &model.Channel{Type: constant.ChannelTypeVolcNative, Key: "upstream-key", BaseURL: &baseURL, Status: common.ChannelStatusEnabled}
	require.NoError(t, model.DB.Create(compatible).Error)
	require.NoError(t, model.DB.Create(native).Error)
	for _, channel := range []*model.Channel{compatible, native} {
		require.NoError(t, model.DB.Create(&model.Ability{Group: "default", Model: "doubao-seedance-2-0", ChannelId: channel.Id, Enabled: true}).Error)
	}
	engine := gin.New()
	engine.POST("/pg/videos", func(c *gin.Context) {
		c.Set("id", fixture.user.Id)
		common.SetContextKey(c, constant.ContextKeyUsingGroup, fixture.user.Group)
		common.SetContextKey(c, constant.ContextKeyUserSetting, dto.UserSetting{BillingPreference: "wallet_only"})
		c.Next()
	}, ValidatePlaygroundVideo, middleware.Distribute(), PlaygroundVideo)
	body := `{"project_id":"` + fixture.project.PublicID + `","node_id":"n1","client_request_id":"seedance-distribution",` +
		`"model":"doubao-seedance-2-0","prompt":"city at night","parameters":{"duration":5}}`
	request := httptest.NewRequest(http.MethodPost, "/pg/videos", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.Equal(t, "/api/v3/contents/generations/tasks", fixture.lastPath.Load())
	taskID := gjson.Get(recorder.Body.String(), "data.task_id").String()
	task, exists, err := model.GetByTaskId(fixture.user.Id, taskID)
	require.NoError(t, err)
	require.True(t, exists)
	assert.Equal(t, native.Id, task.ChannelId)
}

func TestPlaygroundVideoRetryWithSameClientRequestDoesNotResubmit(t *testing.T) {
	fixture := setupVideoStudioSubmit(t)
	body := studioBody(fixture.project.PublicID, "req-1", "city at night")

	first := fixture.submit(t, body)
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())
	second := fixture.submit(t, body)
	require.Equal(t, http.StatusOK, second.Code, second.Body.String())

	assert.Equal(t, int32(1), fixture.calls.Load(), "a network retry must not create a second upstream task")
	assert.Equal(t, gjson.Get(first.Body.String(), "data.id").String(), gjson.Get(second.Body.String(), "data.id").String())

	conflict := fixture.submit(t, studioBody(fixture.project.PublicID, "req-1", "a different prompt"))
	assert.Equal(t, http.StatusConflict, conflict.Code)
	assert.Equal(t, int32(1), fixture.calls.Load())
}

func TestPlaygroundVideoRejectsForeignProjectBeforeUpstream(t *testing.T) {
	fixture := setupVideoStudioSubmit(t)
	other := &model.User{Username: "other-user", Quota: 1_000_000, Group: "default", AffCode: "other-aff"}
	require.NoError(t, model.DB.Create(other).Error)
	foreign, err := videostudio.CreateProject(other.Id, videostudio.ProjectInput{})
	require.NoError(t, err)

	recorder := fixture.submit(t, studioBody(foreign.PublicID, "req-1", "city at night"))

	assert.Equal(t, http.StatusNotFound, recorder.Code)
	assert.Zero(t, fixture.calls.Load())
}

func TestPlaygroundVideoUpstreamFailureMarksRunWithoutTask(t *testing.T) {
	fixture := setupVideoStudioSubmit(t)
	fixture.statusFor = func() int { return http.StatusBadRequest }

	recorder := fixture.submit(t, studioBody(fixture.project.PublicID, "req-1", "city at night"))

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	runs, err := videostudio.ListRuns(fixture.user.Id, fixture.project, 1, 20)
	require.NoError(t, err)
	require.Len(t, runs.Items, 1)
	assert.Equal(t, "failed", runs.Items[0].Status)
	assert.NotEmpty(t, runs.Items[0].FailReason)
	_, exists, err := model.GetByTaskId(fixture.user.Id, runs.Items[0].TaskID)
	require.NoError(t, err)
	assert.False(t, exists, "a rejected submission must not persist a task")

	retried := fixture.submit(t, studioBody(fixture.project.PublicID, "req-1", "city at night"))
	assert.Equal(t, http.StatusOK, retried.Code)
	assert.Equal(t, "failed", gjson.Get(retried.Body.String(), "data.status").String(), "the failed run is replayed, not resubmitted")
	assert.Equal(t, int32(1), fixture.calls.Load())
}

// The shared submit path must keep the provider-shaped response for the
// public video API; only the studio skips it.
func TestRelayTaskKeepsProviderResponseContract(t *testing.T) {
	fixture := setupVideoStudioSubmit(t)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(`{"model":"MiniMax-H3","prompt":"city at night","seconds":5}`))
	c.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(c, constant.ContextKeyUserId, fixture.user.Id)
	common.SetContextKey(c, constant.ContextKeyUserQuota, fixture.user.Quota)
	common.SetContextKey(c, constant.ContextKeyUserSetting, dto.UserSetting{BillingPreference: "wallet_only"})
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
	common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "MiniMax-H3")
	common.SetContextKey(c, constant.ContextKeyChannelId, fixture.channel.Id)
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, fixture.upstream.URL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "upstream-key")
	c.Set("channel_type", constant.ChannelTypeOpenAI)
	c.Set("token_unlimited_quota", true)

	RelayTask(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	response := recorder.Body.String()
	assert.Equal(t, "video", gjson.Get(response, "object").String())
	assert.True(t, strings.HasPrefix(gjson.Get(response, "id").String(), "task_"), "the public task ID replaces the upstream ID")
	assert.False(t, gjson.Get(response, "success").Exists())
}

func TestVideoStudioProjectSaveUsesRevisionAndFitsTextColumns(t *testing.T) {
	fixture := setupVideoStudioSubmit(t)
	userID := fixture.user.Id
	draft := `{"version":1,"nodes":{"gen_1":{"prompt":"` + strings.Repeat("夜", 18000) + `"}}}`

	saved, err := videostudio.SaveProject(userID, fixture.project.PublicID, videostudio.ProjectInput{Revision: fixture.project.Revision, Draft: &draft})
	require.NoError(t, err, "a draft at the size limit must fit a MySQL TEXT column")
	assert.Equal(t, fixture.project.Revision+1, saved.Revision)
	loaded, err := model.FindVideoProject(userID, fixture.project.PublicID)
	require.NoError(t, err)
	assert.Equal(t, draft, loaded.Draft)
	assert.Equal(t, "{}", loaded.Canvas, "saving the draft alone keeps the canvas layout")

	_, err = videostudio.SaveProject(userID, fixture.project.PublicID, videostudio.ProjectInput{Revision: fixture.project.Revision, Draft: &draft})
	var requestErr *videostudio.RequestError
	require.ErrorAs(t, err, &requestErr)
	assert.Equal(t, http.StatusConflict, requestErr.StatusCode, "a stale revision must not overwrite")

	oversized := `{"version":1,"blob":"` + strings.Repeat("x", 61*1024) + `"}`
	_, err = videostudio.SaveProject(userID, fixture.project.PublicID, videostudio.ProjectInput{Revision: saved.Revision, Draft: &oversized})
	require.ErrorAs(t, err, &requestErr)
	assert.Equal(t, http.StatusRequestEntityTooLarge, requestErr.StatusCode)
}

// A submission rejected before channel selection must leave nothing behind,
// otherwise the canvas would show a run stuck in "submitting".
func TestPlaygroundVideoRejectedBeforeChannelLeavesNoRun(t *testing.T) {
	fixture := setupVideoStudioSubmit(t)
	body := `{"project_id":"` + fixture.project.PublicID + `","node_id":"n1","client_request_id":"req-1","model":"MiniMax-H3","prompt":"x","parameters":{"duration":5,"resolution":"4k"}}`

	recorder := fixture.submit(t, body)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	runs, err := videostudio.ListRuns(fixture.user.Id, fixture.project, 1, 20)
	require.NoError(t, err)
	assert.Empty(t, runs.Items)
	assert.Zero(t, fixture.calls.Load())
}

func TestVideoStudioKeyScopesModelsAndOwnership(t *testing.T) {
	fixture := setupVideoStudioSubmit(t)
	other := &model.User{Username: "key-other", Quota: 1, Group: "default", AffCode: "key-other-aff"}
	require.NoError(t, model.DB.Create(other).Error)
	own := &model.Token{UserId: fixture.user.Id, Name: "own", Key: "own-key", Status: common.TokenStatusEnabled, ExpiredTime: -1, Group: "vip", ModelLimitsEnabled: true, ModelLimits: "MiniMax-H3"}
	disabled := &model.Token{UserId: fixture.user.Id, Name: "off", Key: "off-key", Status: common.TokenStatusDisabled, ExpiredTime: -1}
	foreign := &model.Token{UserId: other.Id, Name: "foreign", Key: "foreign-key", Status: common.TokenStatusEnabled, ExpiredTime: -1}
	require.NoError(t, model.DB.Create([]*model.Token{own, disabled, foreign}).Error)

	key, err := videoStudioKey(fixture.user.Id, own.Id)
	require.NoError(t, err)
	assert.Equal(t, "vip", key.Group, "the key decides the group")
	assert.Equal(t, map[string]bool{"MiniMax-H3": true}, availableForKey(map[string]bool{"MiniMax-H3": true, "wan3.0-video": true}, key), "the key whitelist narrows the group's models")

	passthrough, err := videoStudioKey(fixture.user.Id, 0)
	require.NoError(t, err)
	assert.Nil(t, passthrough)
	assert.Len(t, availableForKey(map[string]bool{"a": true, "b": true}, nil), 2)

	_, err = videoStudioKey(fixture.user.Id, foreign.Id)
	var requestErr *videostudio.RequestError
	require.ErrorAs(t, err, &requestErr)
	assert.Equal(t, http.StatusNotFound, requestErr.StatusCode, "another user's key must look nonexistent")
	_, err = videoStudioKey(fixture.user.Id, disabled.Id)
	require.ErrorAs(t, err, &requestErr)
	assert.Equal(t, http.StatusForbidden, requestErr.StatusCode)
}

func TestPlaygroundVideoUsesSelectedKeyForQuotaAndModelLimits(t *testing.T) {
	fixture := setupVideoStudioSubmit(t)
	key := &model.Token{
		UserId: fixture.user.Id, Name: "video", Key: "video-key", Status: common.TokenStatusEnabled,
		ExpiredTime: -1, Group: "default", RemainQuota: 1234, ModelLimitsEnabled: true, ModelLimits: "MiniMax-H3",
	}
	require.NoError(t, model.DB.Create(key).Error)
	body := `{"project_id":"` + fixture.project.PublicID + `","node_id":"n1","client_request_id":"key-request","model":"MiniMax-H3","token_id":` + strconv.Itoa(key.Id) + `,"prompt":"city at night"}`
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/pg/videos", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("id", fixture.user.Id)

	ValidatePlaygroundVideo(c)

	require.False(t, c.IsAborted(), recorder.Body.String())
	assert.Equal(t, key.Id, c.GetInt("token_id"))
	assert.Equal(t, key.RemainQuota, c.GetInt("token_quota"))
	assert.True(t, c.GetBool("token_model_limit_enabled"))
	limits, exists := c.Get("token_model_limit")
	require.True(t, exists)
	assert.Equal(t, map[string]bool{"MiniMax-H3": true}, limits)
	common.CleanupBodyStorage(c)
}

func TestPlaygroundVideoChargesSelectedKeyAndRejectsInsufficientKeyQuota(t *testing.T) {
	fixture := setupVideoStudioSubmit(t)
	key := &model.Token{
		UserId: fixture.user.Id, Name: "video", Key: "video-quota-key", Status: common.TokenStatusEnabled,
		ExpiredTime: -1, Group: "default", RemainQuota: 1_000_000,
	}
	require.NoError(t, model.DB.Create(key).Error)
	body := func(requestID string, tokenID int) string {
		return `{"project_id":"` + fixture.project.PublicID + `","node_id":"n1","client_request_id":"` + requestID +
			`","model":"MiniMax-H3","token_id":` + strconv.Itoa(tokenID) + `,"prompt":"city at night","parameters":{"duration":5}}`
	}

	success := fixture.submit(t, body("charged-key", key.Id))
	require.Equal(t, http.StatusOK, success.Code, success.Body.String())
	charged, err := model.GetTokenByIds(key.Id, fixture.user.Id)
	require.NoError(t, err)
	assert.Less(t, charged.RemainQuota, 1_000_000)
	assert.Equal(t, int32(1), fixture.calls.Load())

	limited := &model.Token{
		UserId: fixture.user.Id, Name: "limited", Key: "limited-video-key", Status: common.TokenStatusEnabled,
		ExpiredTime: -1, Group: "default", RemainQuota: 1,
	}
	require.NoError(t, model.DB.Create(limited).Error)
	denied := fixture.submit(t, body("insufficient-key", limited.Id))
	assert.Equal(t, http.StatusForbidden, denied.Code)
	assert.Equal(t, int32(1), fixture.calls.Load(), "insufficient key quota must stop before the upstream call")
}

func TestPlaygroundChatChecksSelectedTextKeyBeforeDistribution(t *testing.T) {
	fixture := setupVideoStudioSubmit(t)
	require.NoError(t, model.DB.Create(&model.Ability{Group: "default", Model: "gpt-4o-mini", ChannelId: fixture.channel.Id, Enabled: true}).Error)
	key := &model.Token{
		UserId: fixture.user.Id, Name: "text", Key: "text-key", Status: common.TokenStatusEnabled,
		ExpiredTime: -1, Group: "default", RemainQuota: 4321, ModelLimitsEnabled: true, ModelLimits: "gpt-4o-mini",
	}
	require.NoError(t, model.DB.Create(key).Error)
	validate := func(group string, modelName string) (*gin.Context, *httptest.ResponseRecorder) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		body, err := common.Marshal(gin.H{
			"model": modelName, "group": group, "token_id": key.Id,
			"messages": []any{gin.H{"role": "user", "content": "rewrite this prompt"}},
		})
		require.NoError(t, err)
		c.Request = httptest.NewRequest(http.MethodPost, "/pg/chat/completions", strings.NewReader(string(body)))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("id", fixture.user.Id)
		ValidatePlaygroundChat(c)
		common.CleanupBodyStorage(c)
		return c, recorder
	}

	allowed, response := validate("default", "gpt-4o-mini")
	require.False(t, allowed.IsAborted(), response.Body.String())
	assert.True(t, allowed.GetBool("playground_key_selected"))
	assert.Equal(t, key.Id, allowed.GetInt("token_id"))
	assert.Equal(t, key.RemainQuota, allowed.GetInt("token_quota"))
	_, response = validate("other", "gpt-4o-mini")
	assert.Equal(t, http.StatusForbidden, response.Code)
	_, response = validate("default", "MiniMax-H3")
	assert.Equal(t, http.StatusForbidden, response.Code)
}

func TestPlaygroundChatUsesSelectedTextKeyForPromptOptimization(t *testing.T) {
	fixture := setupVideoStudioSubmit(t)
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"MiniMax-H3":0.1,"gpt-4o-mini":0.1}`))
	require.NoError(t, model.DB.Create(&model.Ability{Group: "default", Model: "gpt-4o-mini", ChannelId: fixture.channel.Id, Enabled: true}).Error)
	key := &model.Token{
		UserId: fixture.user.Id, Name: "text", Key: "prompt-text-key", Status: common.TokenStatusEnabled,
		ExpiredTime: -1, Group: "default", RemainQuota: 1_000_000, ModelLimitsEnabled: true, ModelLimits: "gpt-4o-mini",
	}
	require.NoError(t, model.DB.Create(key).Error)
	var upstreamBody atomic.Value
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		upstreamBody.Store(string(body))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-1","object":"chat.completion","created":1,"model":"gpt-4o-mini","choices":[{"index":0,"message":{"role":"assistant","content":"improved prompt"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}}`))
	}))
	t.Cleanup(upstream.Close)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := `{"model":"gpt-4o-mini","group":"default","token_id":` + strconv.Itoa(key.Id) + `,"stream":false,"messages":[{"role":"user","content":"rewrite this prompt"}]}`
	c.Request = httptest.NewRequest(http.MethodPost, "/pg/chat/completions", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("id", fixture.user.Id)
	common.SetContextKey(c, constant.ContextKeyUserId, fixture.user.Id)
	common.SetContextKey(c, constant.ContextKeyUserQuota, fixture.user.Quota)
	common.SetContextKey(c, constant.ContextKeyUserSetting, dto.UserSetting{BillingPreference: "wallet_only"})
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
	common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
	ValidatePlaygroundChat(c)
	require.False(t, c.IsAborted(), recorder.Body.String())
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "gpt-4o-mini")
	common.SetContextKey(c, constant.ContextKeyChannelId, fixture.channel.Id)
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, upstream.URL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "upstream-key")
	c.Set("channel_type", constant.ChannelTypeOpenAI)
	c.Set("relay_mode", 0)
	Playground(c)
	common.CleanupBodyStorage(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.Contains(t, recorder.Body.String(), "improved prompt")
	assert.NotContains(t, upstreamBody.Load().(string), "token_id")
	charged, err := model.GetTokenByIds(key.Id, fixture.user.Id)
	require.NoError(t, err)
	assert.Less(t, charged.RemainQuota, 1_000_000)
}

func TestListVideoStudioModelsSplitsVideoAndTextByKey(t *testing.T) {
	fixture := setupVideoStudioSubmit(t)
	require.NoError(t, model.DB.Create(&model.Ability{Group: "default", Model: "gpt-4o-mini", ChannelId: fixture.channel.Id, Enabled: true}).Error)
	require.NoError(t, model.DB.Create(&model.Ability{Group: "default", Model: "gpt-4o", ChannelId: fixture.channel.Id, Enabled: true}).Error)
	textKey := &model.Token{UserId: fixture.user.Id, Name: "text", Key: "text-key", Status: common.TokenStatusEnabled, ExpiredTime: -1, Group: "default", ModelLimitsEnabled: true, ModelLimits: "gpt-4o-mini,MiniMax-H3"}
	require.NoError(t, model.DB.Create(textKey).Error)

	list := func(query string) []gjson.Result {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/video-studio/models?"+query, nil)
		c.Set("id", fixture.user.Id)
		ListVideoStudioModels(c)
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		return gjson.Get(recorder.Body.String(), "data").Array()
	}

	text := list("kind=text&token_id=" + strconv.Itoa(textKey.Id))
	require.Len(t, text, 1, "the whitelist keeps one text model and the video model never shows as text")
	assert.Equal(t, "gpt-4o-mini", text[0].String())

	video := list("token_id=" + strconv.Itoa(textKey.Id))
	require.Len(t, video, 1)
	assert.Equal(t, "MiniMax-H3", video[0].Get("model").String())
}
