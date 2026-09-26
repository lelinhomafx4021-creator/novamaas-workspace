package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCostAccountingRestoresOriginalPriceBeforeApplyingCostDiscount(t *testing.T) {
	task := &model.Task{PrivateData: model.TaskPrivateData{BillingContext: &model.TaskBillingContext{
		GroupRatio: 0.86, CostDiscount: "0.8",
	}}}
	input := buildTaskCostAccountingInput(task, model.LogTypeConsume, 8600)
	require.NotNil(t, input)
	assert.Equal(t, "10000.000000", input.CostBasisQuota)
	assert.Equal(t, int64(8000), input.CostQuota)
}

func TestCostAccountingWithoutDiscountUsesSaleQuotaAsCost(t *testing.T) {
	consume := buildSaleCostAccountingInput(model.LogTypeConsume, 8600, 0.86, "")
	require.NotNil(t, consume)
	assert.Equal(t, "8600.000000", consume.CostBasisQuota)
	assert.Empty(t, consume.CostDiscount)
	assert.Equal(t, int64(8600), consume.CostQuota)

	refund := buildSaleCostAccountingInput(model.LogTypeRefund, 1720, 0.86, "")
	require.NotNil(t, refund)
	assert.Equal(t, "-1720.000000", refund.CostBasisQuota)
	assert.Empty(t, refund.CostDiscount)
	assert.Equal(t, int64(-1720), refund.CostQuota)
}

func TestHistoricalCostBackfillRecognizesRealtimeTaskSnapshotBySourceLog(t *testing.T) {
	truncate(t)
	log := &model.Log{
		UserId: 4, Type: model.LogTypeConsume, CreatedAt: 100, Quota: 1000,
		ChannelId: 6, RequestId: "request-task-existing", ModelName: "task-model",
		Other: common.MapToJsonStr(map[string]interface{}{"group_ratio": 1}),
	}
	require.NoError(t, model.LOG_DB.Create(log).Error)
	snapshot, err := model.BuildCostAccountingSnapshot(log, &model.CostAccountingInput{
		EventKey:       "task:existing:initial",
		CostBasisQuota: "1000",
		CostQuota:      1000,
		Source:         model.CostSnapshotSourceRealtime,
	})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(snapshot).Error)

	result, err := BackfillCostAccounting(CostAccountingBackfillInput{
		StartTimestamp: 1,
		EndTimestamp:   200,
		ChannelID:      6,
		CostDiscount:   "0.8",
		Limit:          100,
		Apply:          true,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Scanned)
	assert.Equal(t, 1, result.Existing)
	assert.Zero(t, result.Ready)
	assert.Zero(t, result.Applied)

	var count int64
	require.NoError(t, model.DB.Model(&model.CostAccountingSnapshot{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestTaskRefundRestoresOriginalPriceWithTheCapturedBillingDiscount(t *testing.T) {
	truncate(t)

	consume, err := model.BuildCostAccountingSnapshot(&model.Log{
		Id: 1, UserId: 9, Type: model.LogTypeConsume, CreatedAt: 100, Quota: 8600,
	}, buildTaskCostAccountingInput(&model.Task{PrivateData: model.TaskPrivateData{BillingContext: &model.TaskBillingContext{
		GroupRatio: 0.86, CostDiscount: "0.8",
	}}}, model.LogTypeConsume, 8600))
	require.NoError(t, err)
	task := &model.Task{
		TaskID: "task-accounting",
		PrivateData: model.TaskPrivateData{BillingContext: &model.TaskBillingContext{
			GroupRatio:   0.86,
			CostDiscount: "0.8",
		}},
	}
	refund, err := model.BuildCostAccountingSnapshot(&model.Log{
		Id: 2, UserId: 9, Type: model.LogTypeRefund, CreatedAt: 200, Quota: 1720,
	}, buildTaskCostAccountingInput(task, model.LogTypeRefund, 1720))
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(consume).Error)
	require.NoError(t, model.DB.Create(refund).Error)

	totals, err := model.SumCostAccounting(model.CostAccountingFilter{UserID: 9})
	require.NoError(t, err)
	assert.Equal(t, int64(6880), totals.RevenueQuota)
	assert.Equal(t, int64(6400), totals.CostQuota)
	assert.Equal(t, int64(480), totals.ProfitQuota)
}

func TestHistoricalCostBackfillSupportsPreviewApplyAndIdempotency(t *testing.T) {
	truncate(t)
	require.NoError(t, model.LOG_DB.Create(&model.Log{
		UserId: 4, Type: model.LogTypeConsume, CreatedAt: 100, Quota: 1000,
		ChannelId: 6, Other: common.MapToJsonStr(map[string]interface{}{"group_ratio": 2}),
	}).Error)
	require.NoError(t, model.LOG_DB.Create(&model.Log{
		UserId: 4, Type: model.LogTypeRefund, CreatedAt: 200, Quota: 200, ChannelId: 6,
		Other: common.MapToJsonStr(map[string]interface{}{"group_ratio": 2}),
	}).Error)

	input := CostAccountingBackfillInput{
		StartTimestamp: 1,
		EndTimestamp:   300,
		ChannelID:      6,
		CostDiscount:   "0.5",
		Limit:          100,
		BatchID:        "historical-1",
	}
	preview, err := BackfillCostAccounting(input)
	require.NoError(t, err)
	assert.Equal(t, 2, preview.Ready)
	assert.Equal(t, int64(200), preview.CostQuota)
	assert.Equal(t, int64(800), preview.RevenueQuota)
	assert.Zero(t, preview.Applied)

	input.Apply = true
	applied, err := BackfillCostAccounting(input)
	require.NoError(t, err)
	assert.Equal(t, 2, applied.Applied)

	repeated, err := BackfillCostAccounting(input)
	require.NoError(t, err)
	assert.Equal(t, 2, repeated.Existing)
	assert.Zero(t, repeated.Applied)

	totals, err := model.SumCostAccounting(model.CostAccountingFilter{ChannelID: 6})
	require.NoError(t, err)
	assert.Equal(t, int64(800), totals.RevenueQuota)
	assert.Equal(t, int64(200), totals.CostQuota)
	assert.Equal(t, int64(600), totals.ProfitQuota)

	repriceInput := CostAccountingRepriceInput{
		StartTimestamp: 1,
		EndTimestamp:   300,
		ChannelID:      6,
		CostDiscount:   "0.4",
		Limit:          100,
		BatchID:        "reprice-1",
		Reason:         "historical supplier discount correction",
		ActorID:        1,
	}
	repricePreview, err := RepriceCostAccounting(repriceInput)
	require.NoError(t, err)
	assert.Equal(t, 2, repricePreview.Changed)
	assert.Zero(t, repricePreview.Unchanged)
	assert.Equal(t, int64(200), repricePreview.CurrentCostQuota)
	assert.Equal(t, int64(160), repricePreview.NewCostQuota)
	assert.Equal(t, int64(-40), repricePreview.DeltaCostQuota)
	assert.Zero(t, repricePreview.Applied)

	repriceInput.Apply = true
	repriced, err := RepriceCostAccounting(repriceInput)
	require.NoError(t, err)
	assert.Equal(t, 2, repriced.Applied)
	totals, err = model.SumCostAccounting(model.CostAccountingFilter{ChannelID: 6})
	require.NoError(t, err)
	assert.Equal(t, int64(160), totals.CostQuota)
	assert.Equal(t, int64(640), totals.ProfitQuota)
	var repriceAdjustment model.CostAccountingAdjustment
	require.NoError(t, model.DB.Where("batch_id = ?", "reprice-1").First(&repriceAdjustment).Error)
	assert.Equal(t, "0.400000", repriceAdjustment.CostDiscount)

	breakEvenPreview, err := RepriceCostAccounting(CostAccountingRepriceInput{
		StartTimestamp: 1,
		EndTimestamp:   300,
		ChannelID:      6,
		CostDiscount:   "",
		Limit:          100,
		Reason:         "use zero-profit accounting",
		ActorID:        1,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(160), breakEvenPreview.CurrentCostQuota)
	assert.Equal(t, int64(800), breakEvenPreview.NewCostQuota)
	assert.Equal(t, int64(640), breakEvenPreview.DeltaCostQuota)
}

func TestHistoricalCostBackfillWithoutDiscountUsesTurnoverAsCost(t *testing.T) {
	truncate(t)
	require.NoError(t, model.LOG_DB.Create(&model.Log{
		UserId: 4, Type: model.LogTypeConsume, CreatedAt: 100, Quota: 860,
		ChannelId: 6, Other: common.MapToJsonStr(map[string]interface{}{"group_ratio": 0.86}),
	}).Error)
	require.NoError(t, model.LOG_DB.Create(&model.Log{
		UserId: 4, Type: model.LogTypeRefund, CreatedAt: 200, Quota: 172,
		ChannelId: 6, Other: common.MapToJsonStr(map[string]interface{}{"group_ratio": 0.86}),
	}).Error)

	preview, err := BackfillCostAccounting(CostAccountingBackfillInput{
		StartTimestamp: 1,
		EndTimestamp:   300,
		ChannelID:      6,
		CostDiscount:   "",
		Limit:          100,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, preview.Ready)
	assert.Equal(t, int64(688), preview.RevenueQuota)
	assert.Equal(t, int64(688), preview.CostQuota)
}

func TestHistoricalBackfillRestoresOriginalPriceForRefundedVideoTasks(t *testing.T) {
	truncate(t)
	records := []model.Log{
		{
			UserId: 7, Type: model.LogTypeConsume, CreatedAt: 100, Quota: 59147998,
			ChannelId: 29, ModelName: "doubao-seedance-2-5",
			Other: common.MapToJsonStr(map[string]interface{}{"group_ratio": 0.86}),
		},
		{
			UserId: 7, Type: model.LogTypeRefund, CreatedAt: 200, Quota: 12187490,
			ChannelId: 29, ModelName: "doubao-seedance-2-5",
			Other: common.MapToJsonStr(map[string]interface{}{"group_ratio": 0.86}),
		},
	}
	require.NoError(t, model.LOG_DB.Create(&records).Error)

	preview, err := BackfillCostAccounting(CostAccountingBackfillInput{
		StartTimestamp: 1,
		EndTimestamp:   300,
		ChannelID:      29,
		CostDiscount:   "0.82",
		Limit:          100,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(46960508), preview.RevenueQuota)
	assert.Equal(t, int64(44776298), preview.CostQuota)
	assert.Equal(t, int64(2184210), preview.RevenueQuota-preview.CostQuota)
}

func TestHistoricalRepriceMigratesTurnoverSnapshotsToOriginalPriceBasis(t *testing.T) {
	truncate(t)
	consumeLog := &model.Log{
		UserId: 7, Type: model.LogTypeConsume, CreatedAt: 100, Quota: 1000, ChannelId: 6,
		Other: common.MapToJsonStr(map[string]interface{}{"group_ratio": 2}),
	}
	refundLog := &model.Log{
		UserId: 7, Type: model.LogTypeRefund, CreatedAt: 200, Quota: 200, ChannelId: 6,
		Other: common.MapToJsonStr(map[string]interface{}{"group_ratio": 2}),
	}
	require.NoError(t, model.LOG_DB.Create(consumeLog).Error)
	require.NoError(t, model.LOG_DB.Create(refundLog).Error)
	consumeSnapshot, err := model.BuildCostAccountingSnapshot(consumeLog, &model.CostAccountingInput{
		CostBasisQuota: "1000",
		CostDiscount:   "0.5",
		CostQuota:      500,
		Source:         model.CostSnapshotSourceBackfill,
	})
	require.NoError(t, err)
	consumeSnapshot.SnapshotVersion = model.CostAccountingLegacyTurnoverBasisVersion
	require.NoError(t, model.DB.Create(consumeSnapshot).Error)
	refundSnapshot, err := model.BuildCostAccountingSnapshot(refundLog, &model.CostAccountingInput{
		CostBasisQuota: "-200",
		CostDiscount:   "0.5",
		CostQuota:      -100,
		Source:         model.CostSnapshotSourceBackfill,
	})
	require.NoError(t, err)
	refundSnapshot.SnapshotVersion = model.CostAccountingLegacyTurnoverBasisVersion
	require.NoError(t, model.DB.Create(refundSnapshot).Error)

	input := CostAccountingRepriceInput{
		StartTimestamp: 1,
		EndTimestamp:   300,
		ChannelID:      6,
		CostDiscount:   "0.5",
		Limit:          100,
		Reason:         "repair legacy refund cost",
		ActorID:        1,
	}
	preview, err := RepriceCostAccounting(input)
	require.NoError(t, err)
	assert.Equal(t, 2, preview.Changed)
	assert.Equal(t, int64(400), preview.CurrentCostQuota)
	assert.Equal(t, int64(200), preview.NewCostQuota)
	assert.Equal(t, int64(-200), preview.DeltaCostQuota)

	input.Apply = true
	applied, err := RepriceCostAccounting(input)
	require.NoError(t, err)
	assert.Equal(t, 2, applied.Applied)
	totals, err := model.SumCostAccounting(model.CostAccountingFilter{ChannelID: 6})
	require.NoError(t, err)
	assert.Equal(t, int64(800), totals.RevenueQuota)
	assert.Equal(t, int64(200), totals.CostQuota)
	assert.Equal(t, int64(600), totals.ProfitQuota)
}
