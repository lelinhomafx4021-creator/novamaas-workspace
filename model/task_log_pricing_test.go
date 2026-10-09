package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func seedLegacyTaskPricingLog(t *testing.T, modelName string, modelRatio, videoRatio float64) (Log, Task) {
	t.Helper()
	oldDB, oldLogs := DB, LOG_DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&Log{}, &Task{}, &BillingEntry{}, &BillingCorrectionRow{}, &Channel{}))
	DB, LOG_DB = db, db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	initCol()
	t.Cleanup(func() {
		DB, LOG_DB = oldDB, oldLogs
		assert.NoError(t, sqlDB.Close())
	})
	base, err := common.QuotaFromFloatStrict(modelRatio / 2 * common.QuotaPerUnit * .86)
	require.NoError(t, err)
	quota, err := common.QuotaFromFloatStrict(float64(base) * videoRatio)
	require.NoError(t, err)
	log := Log{UserId: 4, Type: LogTypeConsume, ModelName: modelName, CreatedAt: 100, ChannelId: 29, TokenId: 7,
		Group: "spe_sd_86", Quota: quota, Content: "操作 generate, 计算参数：video_input: 0.66", RequestId: "old-task-request",
		Other: common.MapToJsonStr(map[string]any{"is_task": true, "task_id": "legacy-video-task", "model_ratio": modelRatio, "model_price": -1, "group_ratio": .86})}
	task := Task{TaskID: "legacy-video-task", UserId: 4, ChannelId: 29, Group: "spe_sd_86", Status: TaskStatusSuccess, Quota: quota,
		PrivateData: TaskPrivateData{TokenId: 7, Key: "private-key", ResultURL: "https://private.example.com/video", BillingContext: &TaskBillingContext{OriginModelName: modelName, GroupRatio: .86,
			ModelRatio: modelRatio, ModelPrice: -1, OtherRatios: map[string]float64{"video_input": videoRatio}}}}
	require.NoError(t, DB.Create(&task).Error)
	require.NoError(t, LOG_DB.Create(&log).Error)
	return log, task
}

func TestLegacyTaskLogReturnsExactSavedPricingWithoutChangingEvidence(t *testing.T) {
	for _, tc := range []struct {
		name, model            string
		modelRatio, videoRatio float64
	}{
		{"seedance25_1080p", "doubao-seedance-2-5", 5, 1.1},
		{"seedance25_1080p_video", "doubao-seedance-2-5-260628", 5, 46.0 / 70},
		{"seedance20_1080p", "doubao-seedance-2-0", 46.0 / 14, 51.0 / 46},
		{"seedance20_4k", "doubao-seedance-2-0", 46.0 / 14, 26.0 / 46},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original, task := seedLegacyTaskPricingLog(t, tc.model, tc.modelRatio, tc.videoRatio)
			for _, reader := range []string{"owner", "admin", "token"} {
				var logs []*Log
				var total int64
				var err error
				if reader == "admin" {
					logs, total, err = GetAllLogs(LogTypeConsume, 0, 0, "", "", "", 0, 10, 0, "", "", "", false)
				} else if reader == "token" {
					logs, err = GetLogByTokenId(7)
					total = int64(len(logs))
				} else {
					logs, total, err = GetUserLogs(4, LogTypeConsume, 0, 0, "", "", 0, 10, "", "", "", false)
				}
				require.NoError(t, err)
				assert.EqualValues(t, 1, total)
				require.Len(t, logs, 1)
				metadata, err := common.StrToMap(logs[0].Other)
				require.NoError(t, err)
				ratios, ok := metadata["other_ratios"].(map[string]any)
				require.True(t, ok, "the returned log must carry its exact saved pricing multiplier")
				assert.Equal(t, tc.videoRatio, ratios["video_input"])
				assert.NotContains(t, logs[0].Other, "private-key")
				assert.NotContains(t, logs[0].Other, "private.example.com")
				assert.Equal(t, original.Quota, logs[0].Quota)
				assert.Equal(t, original.Content, logs[0].Content)
			}
			var stored Log
			require.NoError(t, LOG_DB.First(&stored, original.Id).Error)
			assert.Equal(t, original.Other, stored.Other)
			assert.Equal(t, original.Quota, stored.Quota)
			var storedTask Task
			require.NoError(t, DB.First(&storedTask, task.ID).Error)
			assert.Equal(t, task.PrivateData, storedTask.PrivateData)
			assert.Equal(t, task.Quota, storedTask.Quota)
		})
	}
}

func TestLegacyTaskLogDoesNotGuessMissingOrAmbiguousPricing(t *testing.T) {
	for _, scenario := range []string{"foreign_owner", "duplicate_task", "wrong_model", "wrong_channel", "wrong_token", "wrong_group", "wrong_group_ratio", "wrong_model_ratio", "missing_context", "missing_saved_ratio", "missing_task_id", "explicit_empty_map", "existing_flat_ratio"} {
		t.Run(scenario, func(t *testing.T) {
			original, task := seedLegacyTaskPricingLog(t, "doubao-seedance-2-5", 5, 46.0/70)
			metadata, err := common.StrToMap(original.Other)
			require.NoError(t, err)
			switch scenario {
			case "foreign_owner":
				task.UserId = 8
			case "duplicate_task":
				duplicate := task
				duplicate.ID = 0
				require.NoError(t, DB.Create(&duplicate).Error)
			case "wrong_model":
				task.PrivateData.BillingContext.OriginModelName = "another-model"
			case "wrong_channel":
				task.ChannelId = 31
			case "wrong_token":
				task.PrivateData.TokenId = 9
			case "wrong_group":
				task.Group = "another-group"
			case "wrong_group_ratio":
				task.PrivateData.BillingContext.GroupRatio = .77
			case "wrong_model_ratio":
				task.PrivateData.BillingContext.ModelRatio = 6
			case "missing_context":
				task.PrivateData.BillingContext = nil
			case "missing_saved_ratio":
				task.PrivateData.BillingContext.OtherRatios = nil
			case "missing_task_id":
				delete(metadata, "task_id")
			case "explicit_empty_map":
				metadata["other_ratios"] = map[string]any{}
			case "existing_flat_ratio":
				metadata["video_input"] = 1.1
			}
			require.NoError(t, DB.Save(&task).Error)
			original.Other = common.MapToJsonStr(metadata)
			require.NoError(t, LOG_DB.Save(&original).Error)
			logs, _, err := GetUserLogs(4, LogTypeConsume, 0, 0, "", "", 0, 10, "", "", "", false)
			require.NoError(t, err)
			require.Len(t, logs, 1)
			assert.JSONEq(t, original.Other, logs[0].Other)
			assert.Equal(t, original.Quota, logs[0].Quota)
		})
	}
}

func TestLegacyTaskLogEffectivePricingOverridesSavedSubmissionMultiplier(t *testing.T) {
	original, _ := seedLegacyTaskPricingLog(t, "doubao-seedance-2-5", 5, 1.1)
	source := BillingEntry{EventKey: "legacy-task-source", UserID: 4, Sequence: 1, PostedAt: original.CreatedAt, Kind: "usage", ModelName: original.ModelName,
		Quota: int64(original.Quota), TokenID: original.TokenId, RequestID: original.RequestId, SourceLogID: original.Id}
	require.NoError(t, DB.Create(&source).Error)
	require.NoError(t, DB.Create(&BillingCorrectionRow{BatchID: "current-tier", SourceEntryID: source.ID, TaskID: "legacy-video-task",
		TargetPricing: `{"pricing_mode":"tokens","model_price":-1,"model_ratio":5,"other_ratios":{}}`}).Error)
	require.NoError(t, DB.Create(&BillingEntry{EventKey: "legacy-task-correction", UserID: 4, Sequence: 2, PostedAt: 200, Kind: "rate_correction", ModelName: original.ModelName,
		Quota: 0, TokenID: original.TokenId, SourceEntryID: source.ID, CorrectionID: "current-tier", BillingGroup: original.Group, BillingRate: "0.86"}).Error)
	logs, _, err := GetUserLogs(4, LogTypeConsume, 0, 0, "", "", 0, 10, "", "", "", false)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	metadata, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{}, metadata["other_ratios"], "effective empty pricing must override the saved submission's 1.1 multiplier")
	assert.Equal(t, original.Quota, logs[0].Quota)
	var stored Log
	require.NoError(t, LOG_DB.First(&stored, original.Id).Error)
	assert.Equal(t, original.Other, stored.Other)
}
