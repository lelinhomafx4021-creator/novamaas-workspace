package model

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedBillingCorrection(t *testing.T, sourceTime ...int64) BillingCorrectionInput {
	t.Helper()
	truncateTables(t)
	posted := common.GetTimestamp() - 60
	if len(sourceTime) > 0 {
		posted = sourceTime[0]
	}
	user := User{Id: 901, Username: "correction_customer", Group: "customer_group", Quota: 1000000, UsedQuota: 424280, Status: common.UserStatusEnabled}
	require.NoError(t, DB.Create(&user).Error)
	require.NoError(t, DB.Create(&BillingAccount{UserID: 901, AccountingStartAt: posted - 7140, Sequence: 3}).Error)
	require.NoError(t, DB.Create(&Token{Id: 903, UserId: 901, Key: "never-export-this-secret", RemainQuota: 1000000, UsedQuota: 424280}).Error)
	require.NoError(t, DB.Create(&Channel{Id: 904, UsedQuota: 424280}).Error)
	bc := &TaskBillingContext{OriginModelName: "wan-prime", ModelPrice: 0.257142, GroupRatio: 0.66, OtherRatios: map[string]float64{"seconds": 5}}
	for _, task := range []Task{
		{TaskID: "success-task", UserId: 901, Group: "wan", ChannelId: 904, Quota: 424280, Status: TaskStatusSuccess, PrivateData: TaskPrivateData{TokenId: 903, BillingContext: bc}},
		{TaskID: "failed-task", UserId: 901, Group: "wan", ChannelId: 904, Quota: 0, Status: TaskStatusFailure, PrivateData: TaskPrivateData{TokenId: 903, BillingContext: bc}},
	} {
		require.NoError(t, DB.Create(&task).Error)
	}
	for i, id := range []string{"success-task", "failed-task"} {
		metadata, err := common.Marshal(map[string]any{"task_id": id, "group_ratio": 0.66})
		require.NoError(t, err)
		log := Log{UserId: 901, Type: LogTypeConsume, CreatedAt: posted, ModelName: "wan-prime", Quota: 424280, Group: "wan", TokenId: 903, ChannelId: 904, Other: string(metadata)}
		require.NoError(t, DB.Create(&log).Error)
		require.NoError(t, DB.Create(&BillingEntry{EventKey: id, UserID: 901, Sequence: int64(i + 1), PostedAt: posted, Kind: "usage", ModelName: "wan-prime", Quota: 424280, TokenID: 903, SourceLogID: log.Id}).Error)
	}
	metadata, err := common.Marshal(map[string]any{"task_id": "failed-task", "group_ratio": 0.66})
	require.NoError(t, err)
	require.NoError(t, DB.Create(&Log{UserId: 901, Type: LogTypeRefund, CreatedAt: posted + 1, ModelName: "wan-prime", Quota: 424280, Group: "wan", TokenId: 903, ChannelId: 904, Other: string(metadata)}).Error)
	require.NoError(t, DB.Create(&BillingEntry{EventKey: "refund", UserID: 901, Sequence: 3, PostedAt: posted + 1, Kind: "task_adjustment", ModelName: "wan-prime", Quota: -424280, TokenID: 903, RequestID: "failed-task"}).Error)
	return BillingCorrectionInput{UserID: 901, StartAt: posted - 60, EndAt: posted + 59, Models: []string{"wan-prime"}, TargetGroup: "wan-prime", Reason: "Correct the wrong billing group"}
}

func TestBillingCorrectionReplaysDiscountRoundingAndPreservesRefunds(t *testing.T) {
	input := seedBillingCorrection(t)
	batch, err := PreviewBillingCorrection(input, 1, "0.77", "customer_group")
	require.NoError(t, err)
	require.True(t, batch.CanApply)
	require.Len(t, batch.Rows, 3)
	assert.Equal(t, int64(494995), batch.Rows[0].CorrectedQuota, "truncate base fee first, then apply duration; scaling the discounted quota gives an incorrect answer")
	assert.Equal(t, int64(141430), batch.ChargeDelta)
	assert.Equal(t, int64(70715), batch.RefundDelta)
	assert.Equal(t, int64(70715), batch.NetDelta)
	quota, err := GetUserQuota(901, true)
	require.NoError(t, err)
	assert.Equal(t, 1000000, quota, "preview must not move money")
	_, err = ApplyBillingCorrection(batch.ID, batch.SHA256, "0.77", "customer_group", 1, false, "")
	require.NoError(t, err)
	_, err = ApplyBillingCorrection(batch.ID, batch.SHA256, "0.77", "customer_group", 1, false, "")
	require.NoError(t, err, "retry must not charge twice")
	quota, err = GetUserQuota(901, true)
	require.NoError(t, err)
	assert.Equal(t, 929285, quota)
	var user User
	require.NoError(t, DB.First(&user, 901).Error)
	assert.Equal(t, 494995, user.UsedQuota)
	var token Token
	require.NoError(t, DB.First(&token, 903).Error)
	assert.Equal(t, 494995, token.UsedQuota)
	assert.Equal(t, 929285, token.RemainQuota)
	var channel Channel
	require.NoError(t, DB.First(&channel, 904).Error)
	assert.Equal(t, int64(494995), channel.UsedQuota)
	var task Task
	require.NoError(t, DB.First(&task, "task_id = ?", "success-task").Error)
	assert.Equal(t, 424280, task.Quota, "historical task evidence stays intact")
	assert.Equal(t, 0.66, task.PrivateData.BillingContext.GroupRatio)
	var corrections []BillingEntry
	require.NoError(t, DB.Where("kind = ?", "rate_correction").Find(&corrections).Error)
	require.Len(t, corrections, 3)
	var total int64
	for _, entry := range corrections {
		total += entry.Quota
		assert.Equal(t, "0.77", entry.BillingRate)
		assert.Positive(t, entry.SourceEntryID)
	}
	assert.Equal(t, batch.NetDelta, total)
	var costs []CostAccountingSnapshot
	require.NoError(t, DB.Where("batch_id = ?", batch.ID).Find(&costs).Error)
	require.Len(t, costs, 3)
	var revenue int64
	for _, cost := range costs {
		revenue += cost.RevenueQuota
		assert.Zero(t, cost.CostQuota)
	}
	assert.Equal(t, batch.NetDelta, revenue)
	hours, err := GetBillingHours(901, 0, common.GetTimestamp()+3600)
	require.NoError(t, err)
	require.Len(t, hours, 1)
	assert.Equal(t, batch.ChargeDelta, hours[0].Charge)
	assert.Equal(t, batch.RefundDelta, hours[0].Refund)
	_, err = ApplyBillingCorrection(batch.ID, batch.SHA256, "", "", 1, true, "Undo this adjustment")
	require.NoError(t, err)
	_, err = ApplyBillingCorrection(batch.ID, batch.SHA256, "", "", 1, true, "Undo this adjustment")
	require.NoError(t, err)
	quota, err = GetUserQuota(901, true)
	require.NoError(t, err)
	assert.Equal(t, 1000000, quota)
	require.NoError(t, DB.First(&token, 903).Error)
	assert.Equal(t, 424280, token.UsedQuota)
	var count int64
	require.NoError(t, DB.Model(&BillingEntry{}).Where("kind = ?", "rate_correction").Count(&count).Error)
	assert.Equal(t, int64(6), count, "undo appends opposite entries instead of deleting history")
}

func TestBillingCorrectionRejectsChangedEvidenceAndOverlappingBatches(t *testing.T) {
	input := seedBillingCorrection(t)
	first, err := PreviewBillingCorrection(input, 1, "0.77", "customer_group")
	require.NoError(t, err)
	second, err := PreviewBillingCorrection(input, 1, "0.77", "customer_group")
	require.NoError(t, err)
	_, err = ApplyBillingCorrection(first.ID, "wrong", "0.77", "customer_group", 1, false, "")
	assert.ErrorIs(t, err, ErrBillingConflict)
	_, err = ApplyBillingCorrection(first.ID, first.SHA256, "0.88", "customer_group", 1, false, "")
	assert.ErrorIs(t, err, ErrBillingConflict)
	_, err = ApplyBillingCorrection(first.ID, first.SHA256, "0.77", "customer_group", 2, false, "")
	assert.ErrorIs(t, err, ErrBillingConflict)
	_, err = ApplyBillingCorrection(first.ID, first.SHA256, "0.77", "customer_group", 1, false, "")
	require.NoError(t, err)
	_, err = ApplyBillingCorrection(second.ID, second.SHA256, "0.77", "customer_group", 1, false, "")
	assert.ErrorIs(t, err, ErrBillingConflict)
	blocked, err := PreviewBillingCorrection(input, 1, "0.77", "customer_group")
	require.NoError(t, err)
	assert.False(t, blocked.CanApply)
	assert.Zero(t, blocked.NetDelta, "a repeated target uses the effective amount and cannot double charge")
	assert.Equal(t, int64(494995), blocked.Rows[0].EffectiveQuota)
}

func TestBillingCorrectionFailsClosedForPartialRangeAndUnverifiablePrices(t *testing.T) {
	for _, mode := range []string{"partial_refund_range", "changed_price", "unfinished", "subscription", "missing_logs"} {
		t.Run(mode, func(t *testing.T) {
			input := seedBillingCorrection(t)
			switch mode {
			case "partial_refund_range":
				input.EndAt = common.GetTimestamp() - 59
			case "changed_price":
				var task Task
				require.NoError(t, DB.First(&task, "task_id = ?", "success-task").Error)
				task.PrivateData.BillingContext.ModelPrice = 1
				require.NoError(t, DB.Model(&task).Update("private_data", task.PrivateData).Error)
			case "unfinished":
				require.NoError(t, DB.Model(&Task{}).Where("task_id = ?", "success-task").Update("status", TaskStatusInProgress).Error)
			case "subscription":
				var task Task
				require.NoError(t, DB.First(&task, "task_id = ?", "success-task").Error)
				task.PrivateData.BillingSource = "subscription"
				require.NoError(t, DB.Model(&task).Update("private_data", task.PrivateData).Error)
			case "missing_logs":
				require.NoError(t, DB.Where("user_id = ?", 901).Delete(&Log{}).Error)
			}
			batch, err := PreviewBillingCorrection(input, 1, "0.77", "customer_group")
			require.NoError(t, err)
			assert.False(t, batch.CanApply)
			_, err = ApplyBillingCorrection(batch.ID, batch.SHA256, "0.77", "customer_group", 1, false, "")
			assert.ErrorIs(t, err, ErrBillingCorrectionBlocked)
		})
	}
}

func TestBillingCorrectionRollsBackOnInsufficientWalletOrTokenQuota(t *testing.T) {
	for _, tc := range []struct{ mode, counter string }{
		{BillingCorrectionGroupRate, "wallet"}, {BillingCorrectionGroupRate, "token"},
		{BillingCorrectionModelPricing, "wallet"}, {BillingCorrectionModelPricing, "token"},
	} {
		t.Run(tc.mode+"_"+tc.counter, func(t *testing.T) {
			input := seedBillingCorrection(t)
			input.Mode = tc.mode
			rate := "0.77"
			if tc.mode == BillingCorrectionModelPricing {
				price := .3
				configureBillingCorrectionPricing(t, "wan-prime", &price, nil)
				input.TargetGroup, rate = "", ""
			}
			batch, err := PreviewBillingCorrection(input, 1, rate, "customer_group")
			require.NoError(t, err)
			if tc.counter == "wallet" {
				require.NoError(t, DB.Model(&User{}).Where("id = ?", 901).Update("quota", 0).Error)
			} else {
				require.NoError(t, DB.Model(&Token{}).Where("id = ?", 903).Update("remain_quota", 0).Error)
			}
			_, err = ApplyBillingCorrection(batch.ID, batch.SHA256, rate, "customer_group", 1, false, "")
			assert.ErrorIs(t, err, ErrBillingInsufficientQuota)
			var count int64
			require.NoError(t, DB.Model(&BillingEntry{}).Where("kind = ?", "rate_correction").Count(&count).Error)
			assert.Zero(t, count)
			require.NoError(t, DB.Model(&CostAccountingSnapshot{}).Count(&count).Error)
			assert.Zero(t, count)
			stored, err := GetBillingCorrection(batch.ID)
			require.NoError(t, err)
			assert.Equal(t, "preview", stored.Status)
		})
	}
}

func TestBillingCorrectionAuditIsAdminOnlyIncludingExplicitFilters(t *testing.T) {
	input := seedBillingCorrection(t)
	batch, err := PreviewBillingCorrection(input, 1, "0.77", "customer_group")
	require.NoError(t, err)
	_, err = ApplyBillingCorrection(batch.ID, batch.SHA256, "0.77", "customer_group", 1, false, "")
	require.NoError(t, err)
	require.NoError(t, PublishBillingCorrectionAudit(batch.ID))
	require.NoError(t, PublishBillingCorrectionAudit(batch.ID))
	for _, logType := range []int{LogTypeUnknown, LogTypeBillingCorrection} {
		logs, total, err := GetUserLogs(901, logType, 0, 0, "", "", 0, 100, "", "", "", false)
		require.NoError(t, err)
		if logType == LogTypeUnknown {
			assert.Equal(t, int64(3), total)
		} else {
			assert.Zero(t, total)
		}
		for _, log := range logs {
			assert.NotEqual(t, LogTypeBillingCorrection, log.Type)
		}
	}
	logs, total, err := GetAllLogs(LogTypeBillingCorrection, 0, 0, "", "", "", 0, 100, 0, "", "", "", false)
	require.NoError(t, err)
	require.Equal(t, int64(3), total)
	require.Len(t, logs, 3)
	assert.Contains(t, logs[0].Content, batch.ID)
	assert.NotContains(t, logs[0].Other, "never-export-this-secret")
	self, _, err := GetUserLogs(901, LogTypeUnknown, 0, 0, "", "", 0, 100, "", logs[0].RequestId, "", false)
	require.NoError(t, err)
	assert.Empty(t, self)
	require.NoError(t, DB.Model(&Log{}).Where("type = ?", LogTypeBillingCorrection).Update("token_id", 903).Error)
	byToken, err := GetLogByTokenId(903)
	require.NoError(t, err)
	for _, log := range byToken {
		assert.NotEqual(t, LogTypeBillingCorrection, log.Type)
	}
}

func TestBillingCorrectionStatisticsArePrivateAndCostDoesNotIncrease(t *testing.T) {
	input := seedBillingCorrection(t)
	batch, err := PreviewBillingCorrection(input, 1, "0.77", "customer_group")
	require.NoError(t, err)
	_, err = ApplyBillingCorrection(batch.ID, batch.SHA256, "0.77", "customer_group", 1, false, "")
	require.NoError(t, err)
	require.NoError(t, PublishBillingCorrectionAudit(batch.ID))
	customer, err := SumLogStatistics(CostAccountingFilter{UserID: 901})
	require.NoError(t, err)
	assert.Equal(t, int64(494995), customer.RevenueQuota)
	assert.Equal(t, int64(3), customer.Records)
	adminFilter := CostAccountingFilter{UserID: 901, IncludeBillingCorrections: true}
	admin, err := SumLogStatistics(adminFilter)
	require.NoError(t, err)
	assert.Equal(t, int64(989990), admin.Quota)
	assert.Equal(t, int64(494995), admin.RefundQuota)
	assert.Equal(t, int64(494995), admin.RevenueQuota)
	assert.Equal(t, int64(2), admin.Requests, "corrections do not create upstream requests")
	totals, err := SumCostAccounting(adminFilter)
	require.NoError(t, err)
	totals = ReconcileCostAccountingTotals(admin, totals)
	assert.Equal(t, int64(424280), totals.CostQuota)
	assert.Equal(t, int64(70715), totals.ProfitQuota)
	assert.True(t, totals.AccountingComplete)
	adminFilter.LogType = LogTypeBillingCorrection
	correctionStats, err := SumLogStatistics(adminFilter)
	require.NoError(t, err)
	totals, err = SumCostAccounting(adminFilter)
	require.NoError(t, err)
	assert.Equal(t, correctionStats.RevenueQuota, totals.RevenueQuota)
	assert.Equal(t, int64(70715), totals.ProfitQuota)
	assert.Zero(t, totals.CostQuota)
	logs, _, err := GetAllLogs(LogTypeBillingCorrection, 0, 0, "", "", "", 0, 100, 0, "", "", "", false)
	require.NoError(t, err)
	require.NoError(t, AttachLogAccounting(logs))
	for _, log := range logs {
		require.NotNil(t, log.CostQuota)
		assert.Zero(t, *log.CostQuota)
		assert.Equal(t, int64(log.Quota), *log.ProfitQuota)
	}
}

func TestBillingCorrectionRejectsAlteredPreviewAndSource(t *testing.T) {
	for _, mode := range []string{"total", "owner", "expired", "source", "pending"} {
		t.Run(mode, func(t *testing.T) {
			input := seedBillingCorrection(t)
			batch, err := PreviewBillingCorrection(input, 1, "0.77", "customer_group")
			require.NoError(t, err)
			switch mode {
			case "total":
				require.NoError(t, DB.Model(&BillingCorrection{}).Where("id = ?", batch.ID).Update("net_delta", 0).Error)
			case "owner":
				require.NoError(t, DB.Model(&BillingCorrection{}).Where("id = ?", batch.ID).Update("created_by", 2).Error)
			case "expired":
				require.NoError(t, DB.Model(&BillingCorrection{}).Where("id = ?", batch.ID).Update("expires_at", common.GetTimestamp()-1).Error)
			case "source":
				require.NoError(t, DB.Model(&Task{}).Where("task_id = ?", "success-task").Update("channel_id", 905).Error)
			case "pending":
				require.NoError(t, DB.Create(&BillingOperation{ID: "pending", UserID: 901, State: "reserved", Reserved: 1}).Error)
			}
			_, err = ApplyBillingCorrection(batch.ID, batch.SHA256, "0.77", "customer_group", 1, false, "")
			require.Error(t, err)
			var count int64
			require.NoError(t, DB.Model(&BillingEntry{}).Where("kind = ?", "rate_correction").Count(&count).Error)
			assert.Zero(t, count)
			quota, err := GetUserQuota(901, true)
			require.NoError(t, err)
			assert.Equal(t, 1000000, quota)
		})
	}
}

func TestBillingCorrectionLowerRateCreditsWalletAndCanBeReversed(t *testing.T) {
	input := seedBillingCorrection(t)
	batch, err := PreviewBillingCorrection(input, 1, "0.55", "customer_group")
	require.NoError(t, err)
	assert.Equal(t, int64(-70710), batch.NetDelta)
	_, err = ApplyBillingCorrection(batch.ID, batch.SHA256, "0.55", "customer_group", 1, false, "")
	require.NoError(t, err)
	quota, err := GetUserQuota(901, true)
	require.NoError(t, err)
	assert.Equal(t, 1070710, quota)
	_, err = ApplyBillingCorrection(batch.ID, batch.SHA256, "", "", 1, true, "Undo lower group rate")
	require.NoError(t, err)
	require.NoError(t, PublishBillingCorrectionAudit(batch.ID))
	require.NoError(t, PublishBillingCorrectionAudit(batch.ID))
	quota, err = GetUserQuota(901, true)
	require.NoError(t, err)
	assert.Equal(t, 1000000, quota)
	logs, total, err := GetAllLogs(LogTypeBillingCorrection, 0, 0, "", "", "", 0, 100, 0, "", "", "", false)
	require.NoError(t, err)
	require.Equal(t, int64(6), total)
	var delta int64
	for _, log := range logs {
		delta += int64(log.Quota)
	}
	assert.Zero(t, delta)
	customer, total, err := GetUserLogs(901, LogTypeUnknown, 0, 0, "", "", 0, 100, "", "", "", false)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, customer, 3)
}

func TestBillingCorrectionRejectsInvalidHistoricalMultipliers(t *testing.T) {
	input := seedBillingCorrection(t)
	var task Task
	require.NoError(t, DB.First(&task, "task_id = ?", "success-task").Error)
	task.PrivateData.BillingContext.OtherRatios["invalid"] = 0
	require.NoError(t, DB.Model(&task).Update("private_data", task.PrivateData).Error)
	batch, err := PreviewBillingCorrection(input, 1, "0.77", "customer_group")
	require.NoError(t, err)
	assert.False(t, batch.CanApply, "a valid duration must not hide another invalid multiplier")
	_, err = ApplyBillingCorrection(batch.ID, batch.SHA256, "0.77", "customer_group", 1, false, "")
	assert.ErrorIs(t, err, ErrBillingCorrectionBlocked)
}

func TestBillingCorrectionUsesActiveFormalSequenceAfterHistoryImport(t *testing.T) {
	input := seedBillingCorrection(t)
	var originals []BillingEntry
	require.NoError(t, DB.Order("sequence asc").Find(&originals).Error)
	for _, entry := range originals {
		entry.ID = 0
		entry.EventKey = "import:" + entry.EventKey
		entry.Sequence += 3
		entry.Kind = "history_usage"
		if entry.Quota < 0 {
			entry.Kind = "history_refund"
			var refund Log
			require.NoError(t, DB.Where("user_id = ? AND type = ?", 901, LogTypeRefund).First(&refund).Error)
			entry.SourceLogID = refund.Id
		}
		require.NoError(t, DB.Create(&entry).Error)
	}
	require.NoError(t, DB.Model(&BillingAccount{}).Where("user_id = ?", 901).Updates(map[string]any{"start_sequence": 4, "sequence": 6}).Error)
	batch, err := PreviewBillingCorrection(input, 1, "0.77", "customer_group")
	require.NoError(t, err)
	require.True(t, batch.CanApply)
	require.Len(t, batch.Rows, 3, "pre-bootstrap aliases must not duplicate active imported charges/refunds")
	assert.Equal(t, int64(70715), batch.NetDelta)
	for _, row := range batch.Rows {
		assert.Greater(t, row.SourceEntryID, originals[2].ID)
	}
	_, err = ApplyBillingCorrection(batch.ID, batch.SHA256, "0.77", "customer_group", 1, false, "")
	require.NoError(t, err)
	quota, err := GetUserQuota(901, true)
	require.NoError(t, err)
	assert.Equal(t, 929285, quota)
}

func TestBillingCorrectionPerCallFlagKeepsDurationAndFullRefund(t *testing.T) {
	input := seedBillingCorrection(t)
	var tasks []Task
	require.NoError(t, DB.Find(&tasks).Error)
	for _, task := range tasks {
		task.PrivateData.BillingContext.PerCallBilling = true
		require.NoError(t, DB.Model(&task).Update("private_data", task.PrivateData).Error)
	}
	batch, err := PreviewBillingCorrection(input, 1, "0.77", "customer_group")
	require.NoError(t, err)
	require.True(t, batch.CanApply)
	assert.Equal(t, int64(494995), batch.Rows[0].CorrectedQuota)
	assert.Equal(t, int64(70715), batch.NetDelta)
	assert.Equal(t, int64(0), batch.Rows[1].Delta+batch.Rows[2].Delta, "fully refunded tasks must have no net adjustment")
}

func configureBillingCorrectionPricing(t *testing.T, name string, price *float64, ratio *float64) {
	t.Helper()
	oldPrices, err := common.Marshal(ratio_setting.GetModelPriceCopy())
	require.NoError(t, err)
	oldRatios, err := common.Marshal(ratio_setting.GetModelRatioCopy())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(string(oldPrices)))
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(string(oldRatios)))
	})
	prices, ratios := ratio_setting.GetModelPriceCopy(), ratio_setting.GetModelRatioCopy()
	delete(prices, name)
	delete(ratios, name)
	if price != nil {
		prices[name] = *price
	}
	if ratio != nil {
		ratios[name] = *ratio
	}
	encoded, err := common.Marshal(prices)
	require.NoError(t, err)
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(string(encoded)))
	encoded, err = common.Marshal(ratios)
	require.NoError(t, err)
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(string(encoded)))
}

func TestBillingModelPricingCorrectionRepricesChargesRefundsAndWallet(t *testing.T) {
	for _, tc := range []struct {
		name             string
		price            float64
		corrected, delta int64
	}{
		{"increase", .3, 495000, 70720}, {"decrease", .2, 330000, -94280}, {"unchanged", .257142, 424280, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := seedBillingCorrection(t)
			input.Mode, input.TargetGroup = BillingCorrectionModelPricing, ""
			configureBillingCorrectionPricing(t, "wan-prime", &tc.price, nil)
			batch, err := PreviewBillingCorrection(input, 1, "", "customer_group")
			require.NoError(t, err)
			assert.Equal(t, BillingCorrectionModelPricing, batch.Mode)
			assert.Equal(t, tc.delta, batch.NetDelta)
			assert.Equal(t, tc.corrected, batch.Rows[0].CorrectedQuota)
			assert.Equal(t, -tc.corrected, batch.Rows[2].CorrectedQuota)
			for _, row := range batch.Rows {
				assert.Equal(t, "wan", row.TargetGroup)
				assert.Equal(t, "0.66", row.TargetRate)
			}
			if tc.delta == 0 {
				assert.False(t, batch.CanApply)
				_, err = ApplyBillingCorrection(batch.ID, batch.SHA256, "", "customer_group", 1, false, "")
				assert.ErrorIs(t, err, ErrBillingCorrectionBlocked)
				return
			}
			require.True(t, batch.CanApply)
			_, err = ApplyBillingCorrection(batch.ID, batch.SHA256, "", "customer_group", 1, false, "")
			require.NoError(t, err)
			_, err = ApplyBillingCorrection(batch.ID, batch.SHA256, "", "customer_group", 1, false, "")
			require.NoError(t, err)
			var user User
			require.NoError(t, DB.First(&user, 901).Error)
			assert.Equal(t, int64(1000000)-tc.delta, int64(user.Quota))
			assert.Equal(t, tc.corrected, int64(user.UsedQuota))
			_, err = ApplyBillingCorrection(batch.ID, batch.SHA256, "", "", 1, true, "Reverse reviewed model repricing")
			require.NoError(t, err)
			require.NoError(t, DB.First(&user, 901).Error)
			assert.Equal(t, 1000000, user.Quota)
			assert.Equal(t, 424280, user.UsedQuota)
		})
	}
}

func TestBillingModelPricingCorrectionRejectsConfigurationChangedAfterPreview(t *testing.T) {
	input := seedBillingCorrection(t)
	input.Mode, input.TargetGroup = BillingCorrectionModelPricing, ""
	price := .3
	configureBillingCorrectionPricing(t, "wan-prime", &price, nil)
	batch, err := PreviewBillingCorrection(input, 1, "", "customer_group")
	require.NoError(t, err)
	prices := ratio_setting.GetModelPriceCopy()
	prices["wan-prime"] = .4
	encoded, err := common.Marshal(prices)
	require.NoError(t, err)
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(string(encoded)))
	_, err = ApplyBillingCorrection(batch.ID, batch.SHA256, "", "customer_group", 1, false, "")
	assert.ErrorIs(t, err, ErrBillingConflict)
	var count int64
	require.NoError(t, DB.Model(&BillingEntry{}).Where("kind = ?", "rate_correction").Count(&count).Error)
	assert.Zero(t, count)
	quota, err := GetUserQuota(901, true)
	require.NoError(t, err)
	assert.Equal(t, 1000000, quota)
}

func TestBillingModelPricingAfterGroupCorrectionUsesCurrentDiscountAndCanReverseInOrder(t *testing.T) {
	input := seedBillingCorrection(t)
	group, err := PreviewBillingCorrection(input, 1, "0.77", "customer_group")
	require.NoError(t, err)
	_, err = ApplyBillingCorrection(group.ID, group.SHA256, "0.77", "customer_group", 1, false, "")
	require.NoError(t, err)
	price := .3
	configureBillingCorrectionPricing(t, "wan-prime", &price, nil)
	input.Mode, input.TargetGroup = BillingCorrectionModelPricing, ""
	pricing, err := PreviewBillingCorrection(input, 1, "", "customer_group")
	require.NoError(t, err)
	require.True(t, pricing.CanApply)
	assert.Equal(t, int64(577500), pricing.Rows[0].CorrectedQuota)
	assert.Equal(t, int64(494995), pricing.Rows[0].EffectiveQuota)
	assert.Equal(t, int64(82505), pricing.NetDelta)
	for _, row := range pricing.Rows {
		assert.Equal(t, "wan-prime", row.TargetGroup)
		assert.Equal(t, "0.77", row.TargetRate)
		assert.Equal(t, group.ID, row.PreviousBatchID)
	}
	_, err = ApplyBillingCorrection(pricing.ID, pricing.SHA256, "", "customer_group", 1, false, "")
	require.NoError(t, err)
	_, err = ApplyBillingCorrection(group.ID, group.SHA256, "", "", 1, true, "Attempt older reversal")
	assert.ErrorIs(t, err, ErrBillingCorrectionDependency, "later corrections must be reversed first")
	var user User
	require.NoError(t, DB.First(&user, 901).Error)
	assert.Equal(t, 846780, user.Quota)
	_, err = ApplyBillingCorrection(pricing.ID, pricing.SHA256, "", "", 1, true, "Reverse model repricing first")
	require.NoError(t, err)
	require.NoError(t, DB.First(&user, 901).Error)
	assert.Equal(t, 929285, user.Quota)
	_, err = ApplyBillingCorrection(group.ID, group.SHA256, "", "", 1, true, "Then reverse group change")
	require.NoError(t, err)
	require.NoError(t, DB.First(&user, 901).Error)
	assert.Equal(t, 1000000, user.Quota)
}

func seedBillingCorrectionSeedance(t *testing.T, name, resolution string, hasVideo bool, settled bool, settledRatio ...float64) BillingCorrectionInput {
	t.Helper()
	return seedBillingCorrectionSeedanceAt(t, name, resolution, hasVideo, settled, common.GetTimestamp()-60, settledRatio...)
}

func seedBillingCorrectionSeedanceAt(t *testing.T, name, resolution string, hasVideo bool, settled bool, posted int64, settledRatio ...float64) BillingCorrectionInput {
	t.Helper()
	truncateTables(t)
	initial := int64(13200)
	final := initial
	bc := &TaskBillingContext{OriginModelName: name, ModelPrice: .04, GroupRatio: .66, PerCallBilling: true}
	if settled {
		bc.ModelPrice, bc.ModelRatio = -1, 4
		bc.PerCallBilling = false
		initial, final = 660000, 26400
		if len(settledRatio) > 0 {
			quota, err := common.QuotaFromFloatStrict(10000 * settledRatio[0] * .66)
			require.NoError(t, err)
			final = int64(quota)
		}
	}
	require.NoError(t, DB.Create(&User{Id: 901, Username: "seedance_customer", Group: "customer_group", Quota: 1000000, UsedQuota: int(final), Status: common.UserStatusEnabled}).Error)
	sequence := int64(1)
	if settled {
		sequence = 2
	}
	require.NoError(t, DB.Create(&BillingAccount{UserID: 901, AccountingStartAt: posted - 7140, Sequence: sequence}).Error)
	require.NoError(t, DB.Create(&Token{Id: 903, UserId: 901, Key: "seedance-secret", RemainQuota: 1000000, UsedQuota: int(final)}).Error)
	require.NoError(t, DB.Create(&Channel{Id: 904, UsedQuota: final}).Error)
	contentType := "text"
	if hasVideo {
		contentType = "video_url"
	}
	body, err := common.Marshal(map[string]any{"resolution": resolution, "content": []map[string]any{{"type": contentType}}})
	require.NoError(t, err)
	task := Task{TaskID: "seedance-task", UserId: 901, Group: "wan", ChannelId: 904, Quota: int(final), Status: TaskStatusSuccess, PrivateData: TaskPrivateData{TokenId: 903, BillingContext: bc}, Properties: Properties{RequestBody: body}}
	task.SetData(map[string]any{"usage": map[string]any{"total_tokens": 10000}})
	require.NoError(t, DB.Create(&task).Error)
	metadata, err := common.Marshal(map[string]any{"task_id": task.TaskID, "group_ratio": .66})
	require.NoError(t, err)
	log := Log{UserId: 901, Type: LogTypeConsume, CreatedAt: posted, ModelName: name, Quota: int(initial), Group: "wan", TokenId: 903, ChannelId: 904, Other: string(metadata)}
	require.NoError(t, DB.Create(&log).Error)
	require.NoError(t, DB.Create(&BillingEntry{EventKey: "seedance-submit", UserID: 901, Sequence: 1, PostedAt: posted, Kind: "usage", ModelName: name, Quota: initial, TokenID: 903, SourceLogID: log.Id}).Error)
	if settled {
		require.NoError(t, DB.Create(&BillingEntry{EventKey: "seedance-settle", UserID: 901, Sequence: 2, PostedAt: posted + 1, Kind: "task_adjustment", ModelName: name, Quota: final - initial, TokenID: 903, RequestID: task.TaskID}).Error)
	}
	return BillingCorrectionInput{UserID: 901, StartAt: posted - 60, EndAt: posted + 59, Models: []string{name}, Mode: BillingCorrectionModelPricing, Reason: "Reprice Seedance historical resolution"}
}

func TestBillingModelPricingSeedance25UsesSavedResolutionAndVideoEvidence(t *testing.T) {
	for _, tc := range []struct {
		name, resolution string
		video            bool
		expected         int64
	}{
		{"doubao-seedance-2-5", "720p", false, 31643},
		{"doubao-seedance-2-5-260628", "1080p", false, 34808},
		{"doubao-seedance-2-5", "720p", true, 18986},
		{"doubao-seedance-2-5-260628", "1080p", true, 20794},
	} {
		t.Run(fmt.Sprintf("%s_%s_video_%t", tc.name, tc.resolution, tc.video), func(t *testing.T) {
			input := seedBillingCorrectionSeedance(t, tc.name, tc.resolution, tc.video, false)
			ratio := 70 / (2 * 7.3)
			configureBillingCorrectionPricing(t, tc.name, nil, &ratio)
			batch, err := PreviewBillingCorrection(input, 1, "", "customer_group")
			require.NoError(t, err)
			require.True(t, batch.CanApply)
			assert.Equal(t, tc.expected, batch.Rows[0].CorrectedQuota)
			var pricing billingCorrectionPricing
			require.NoError(t, common.UnmarshalJsonStr(batch.Rows[0].TargetPricing, &pricing))
			assert.Equal(t, tc.resolution, pricing.Resolution)
			require.NotNil(t, pricing.HasVideo)
			assert.Equal(t, tc.video, *pricing.HasVideo)
			_, err = ApplyBillingCorrection(batch.ID, batch.SHA256, "", "customer_group", 1, false, "")
			require.NoError(t, err)
			var user User
			require.NoError(t, DB.First(&user, 901).Error)
			assert.Equal(t, int64(1000000)-tc.expected+13200, int64(user.Quota))
			assert.Equal(t, tc.expected, int64(user.UsedQuota))
			var task Task
			require.NoError(t, DB.First(&task, "task_id = ?", "seedance-task").Error)
			assert.Equal(t, 13200, task.Quota)
		})
	}
}

func TestBillingModelPricingSeedanceTokenSettlementRepricesBothChargeAndRefund(t *testing.T) {
	input := seedBillingCorrectionSeedance(t, "doubao-seedance-2-5", "720p", false, true)
	ratio := 70 / (2 * 7.3)
	configureBillingCorrectionPricing(t, "doubao-seedance-2-5", nil, &ratio)
	batch, err := PreviewBillingCorrection(input, 1, "", "customer_group")
	require.NoError(t, err)
	require.True(t, batch.CanApply)
	require.Len(t, batch.Rows, 2)
	assert.Equal(t, int64(791095), batch.Rows[0].CorrectedQuota)
	assert.Equal(t, int64(-759452), batch.Rows[1].CorrectedQuota)
	assert.Equal(t, int64(5243), batch.NetDelta)
	_, err = ApplyBillingCorrection(batch.ID, batch.SHA256, "", "customer_group", 1, false, "")
	require.NoError(t, err)
	var user User
	require.NoError(t, DB.First(&user, 901).Error)
	assert.Equal(t, 994757, user.Quota)
	assert.Equal(t, 31643, user.UsedQuota)
}

func TestBillingModelPricingSeedanceFailsClosedWithoutEvidence(t *testing.T) {
	for _, missing := range []string{"request", "resolution", "usage", "excessive_usage", "unknown_content", "unsupported_resolution"} {
		t.Run(missing, func(t *testing.T) {
			input := seedBillingCorrectionSeedance(t, "doubao-seedance-2-5", "720p", false, false)
			ratio := 70 / (2 * 7.3)
			configureBillingCorrectionPricing(t, "doubao-seedance-2-5", nil, &ratio)
			var task Task
			require.NoError(t, DB.First(&task, "task_id = ?", "seedance-task").Error)
			switch missing {
			case "request":
				task.Properties.RequestBody = nil
			case "resolution":
				task.Properties.RequestBody = []byte(`{"content":[{"type":"text"}]}`)
			case "unsupported_resolution":
				task.Properties.RequestBody = []byte(`{"resolution":"4k","content":[{"type":"text"}]}`)
			case "unknown_content":
				task.Properties.RequestBody = []byte(`{"resolution":"720p","content":[{}]}`)
			case "usage":
				task.Data = nil
			case "excessive_usage":
				task.SetData(map[string]any{"usage": map[string]any{"total_tokens": int64(9223372036854775807)}})
			}
			require.NoError(t, DB.Model(&task).Updates(map[string]any{"properties": task.Properties, "data": task.Data}).Error)
			batch, err := PreviewBillingCorrection(input, 1, "", "customer_group")
			require.NoError(t, err)
			assert.False(t, batch.CanApply)
			_, err = ApplyBillingCorrection(batch.ID, batch.SHA256, "", "customer_group", 1, false, "")
			assert.ErrorIs(t, err, ErrBillingCorrectionBlocked)
		})
	}
}

func TestBillingGroupCorrectionAfterModelPricingKeepsEffectiveModelPrice(t *testing.T) {
	input := seedBillingCorrection(t)
	input.Mode, input.TargetGroup = BillingCorrectionModelPricing, ""
	price := .3
	configureBillingCorrectionPricing(t, "wan-prime", &price, nil)
	pricing, err := PreviewBillingCorrection(input, 1, "", "customer_group")
	require.NoError(t, err)
	_, err = ApplyBillingCorrection(pricing.ID, pricing.SHA256, "", "customer_group", 1, false, "")
	require.NoError(t, err)
	input.Mode, input.TargetGroup = BillingCorrectionGroupRate, "wan-prime"
	group, err := PreviewBillingCorrection(input, 1, "0.77", "customer_group")
	require.NoError(t, err)
	require.True(t, group.CanApply)
	assert.Equal(t, int64(495000), group.Rows[0].EffectiveQuota)
	assert.Equal(t, int64(577500), group.Rows[0].CorrectedQuota)
	assert.Equal(t, int64(82500), group.NetDelta)
	var frozen billingCorrectionPricing
	require.NoError(t, common.UnmarshalJsonStr(group.Rows[0].TargetPricing, &frozen))
	assert.Equal(t, .3, frozen.ModelPrice)
	_, err = ApplyBillingCorrection(group.ID, group.SHA256, "0.77", "customer_group", 1, false, "")
	require.NoError(t, err)
	_, err = ApplyBillingCorrection(group.ID, group.SHA256, "", "", 1, true, "Reverse group change after pricing")
	require.NoError(t, err)
	var user User
	require.NoError(t, DB.First(&user, 901).Error)
	assert.Equal(t, 929280, user.Quota)
	assert.Equal(t, 495000, user.UsedQuota)
	_, err = ApplyBillingCorrection(pricing.ID, pricing.SHA256, "", "", 1, true, "Reverse original model repricing")
	require.NoError(t, err)
	require.NoError(t, DB.First(&user, 901).Error)
	assert.Equal(t, 1000000, user.Quota)
}

func TestBillingCorrectionCanReverseAnAppliedBatchFromBeforePricingModes(t *testing.T) {
	input := seedBillingCorrection(t)
	batch, err := PreviewBillingCorrection(input, 1, "0.77", "customer_group")
	require.NoError(t, err)
	_, err = ApplyBillingCorrection(batch.ID, batch.SHA256, "0.77", "customer_group", 1, false, "")
	require.NoError(t, err)
	// Persist the pre-upgrade wire format and digest, independently of the new
	// serializer. Applied financial batches must remain reversible after schema
	// migration even though every newly added column contains its zero value.
	legacyRows := make([]string, 0, len(batch.Rows))
	for _, row := range batch.Rows {
		legacyRows = append(legacyRows, fmt.Sprintf(`{"batch_id":%q,"source_entry_id":%d,"model_name":%q,"posted_at":%d,"task_id":%q,"token_id":%d,"channel_id":%d,"original_group":%q,"original_rate":%q,"original_quota":%d,"corrected_quota":%d,"delta":%d,"source_sha256":%q,"blocked":%q}`,
			row.BatchID, row.SourceEntryID, row.ModelName, row.PostedAt, row.TaskID, row.TokenID, row.ChannelID, row.OriginalGroup, row.OriginalRate, row.OriginalQuota, row.CorrectedQuota, row.Delta, row.SourceSHA256, row.Blocked))
	}
	legacyJSON := fmt.Sprintf(`{"ID":%q,"UserID":%d,"CreatedBy":%d,"UserGroup":%q,"CreatedAt":%d,"ExpiresAt":%d,"StartAt":%d,"EndAt":%d,"Sequence":%d,"Models":%q,"Group":%q,"Rate":%q,"Reason":%q,"CanApply":true,"ChargeDelta":%d,"RefundDelta":%d,"NetDelta":%d,"Rows":[%s]}`,
		batch.ID, batch.UserID, batch.CreatedBy, batch.UserGroup, batch.CreatedAt, batch.ExpiresAt, batch.StartAt, batch.EndAt, batch.SourceSequence, batch.Models, batch.TargetGroup, batch.TargetRate, batch.Reason, batch.ChargeDelta, batch.RefundDelta, batch.NetDelta, strings.Join(legacyRows, ","))
	legacyDigest := fmt.Sprintf("%x", sha256.Sum256([]byte(legacyJSON)))
	require.NoError(t, DB.Model(&BillingCorrection{}).Where("id = ?", batch.ID).Updates(map[string]any{"mode": "", "sha256": legacyDigest}).Error)
	require.NoError(t, DB.Model(&BillingCorrectionRow{}).Where("batch_id = ?", batch.ID).Updates(map[string]any{
		"effective_quota": 0, "effective_group": "", "effective_rate": "", "previous_batch_id": "", "target_group": "", "target_rate": "", "target_pricing": "",
	}).Error)
	_, err = ApplyBillingCorrection(batch.ID, legacyDigest, "", "", 1, true, "Reverse a pre-upgrade adjustment")
	require.NoError(t, err)
	var user User
	require.NoError(t, DB.First(&user, 901).Error)
	assert.Equal(t, 1000000, user.Quota)
	assert.Equal(t, 424280, user.UsedQuota)
	stored, err := GetBillingCorrection(batch.ID)
	require.NoError(t, err)
	assert.Equal(t, "reversed", stored.Status)
}

func TestBillingModelPricingCorrectionAppliesFullyRefundedAmountsWithZeroNetWalletChange(t *testing.T) {
	input := seedBillingCorrection(t)
	// Refund the remaining task as well. Corrected gross amounts must be
	// visible for fully refunded tasks even though no money is owed.
	require.NoError(t, DB.Model(&Task{}).Where("task_id = ?", "success-task").Updates(map[string]any{"status": TaskStatusFailure, "quota": 0}).Error)
	require.NoError(t, DB.Create(&BillingEntry{EventKey: "success-task-refunded", UserID: 901, Sequence: 4, PostedAt: input.StartAt + 61, Kind: "task_adjustment", ModelName: "wan-prime", Quota: -424280, TokenID: 903, RequestID: "success-task"}).Error)
	require.NoError(t, DB.Model(&BillingAccount{}).Where("user_id = ?", 901).Update("sequence", 4).Error)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", 901).Update("used_quota", 0).Error)
	require.NoError(t, DB.Model(&Token{}).Where("id = ?", 903).Update("used_quota", 0).Error)
	require.NoError(t, DB.Model(&Channel{}).Where("id = ?", 904).Update("used_quota", 0).Error)
	input.Mode, input.TargetGroup = BillingCorrectionModelPricing, ""
	price := .3
	configureBillingCorrectionPricing(t, "wan-prime", &price, nil)
	batch, err := PreviewBillingCorrection(input, 1, "", "customer_group")
	require.NoError(t, err)
	require.True(t, batch.CanApply)
	assert.Equal(t, int64(141440), batch.ChargeDelta)
	assert.Equal(t, int64(141440), batch.RefundDelta)
	assert.Zero(t, batch.NetDelta)
	_, err = ApplyBillingCorrection(batch.ID, batch.SHA256, "", "customer_group", 1, false, "")
	require.NoError(t, err)
	var user User
	require.NoError(t, DB.First(&user, 901).Error)
	assert.Equal(t, 1000000, user.Quota)
	assert.Zero(t, user.UsedQuota)
	var entries []BillingEntry
	require.NoError(t, DB.Where("kind = ?", "rate_correction").Find(&entries).Error)
	require.Len(t, entries, 4)
	assert.Equal(t, int64(0), entries[0].Quota+entries[1].Quota+entries[2].Quota+entries[3].Quota)
	_, err = ApplyBillingCorrection(batch.ID, batch.SHA256, "", "", 1, true, "Reverse zero-net repricing")
	require.NoError(t, err)
	require.NoError(t, DB.First(&user, 901).Error)
	assert.Equal(t, 1000000, user.Quota)
	assert.Zero(t, user.UsedQuota)
}

func TestBillingModelPricingSeedanceFailedTaskDoesNotRequireSuccessfulUsage(t *testing.T) {
	input := seedBillingCorrectionSeedance(t, "doubao-seedance-2-5", "720p", false, false)
	var task Task
	require.NoError(t, DB.First(&task, "task_id = ?", "seedance-task").Error)
	require.NoError(t, DB.Model(&task).Updates(map[string]any{"status": TaskStatusFailure, "quota": 0, "data": nil}).Error)
	require.NoError(t, DB.Create(&BillingEntry{EventKey: "failed-seedance-refund", UserID: 901, Sequence: 2, PostedAt: input.StartAt + 61, Kind: "task_adjustment", ModelName: "doubao-seedance-2-5", Quota: -13200, TokenID: 903, RequestID: task.TaskID}).Error)
	require.NoError(t, DB.Model(&BillingAccount{}).Where("user_id = ?", 901).Update("sequence", 2).Error)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", 901).Update("used_quota", 0).Error)
	require.NoError(t, DB.Model(&Token{}).Where("id = ?", 903).Update("used_quota", 0).Error)
	require.NoError(t, DB.Model(&Channel{}).Where("id = ?", 904).Update("used_quota", 0).Error)
	ratio := 70 / (2 * 7.3)
	configureBillingCorrectionPricing(t, "doubao-seedance-2-5", nil, &ratio)
	batch, err := PreviewBillingCorrection(input, 1, "", "customer_group")
	require.NoError(t, err)
	require.True(t, batch.CanApply)
	require.Len(t, batch.Rows, 2)
	assert.Equal(t, int64(791095), batch.Rows[0].CorrectedQuota)
	assert.Equal(t, int64(-791095), batch.Rows[1].CorrectedQuota)
	assert.Zero(t, batch.NetDelta)
	_, err = ApplyBillingCorrection(batch.ID, batch.SHA256, "", "customer_group", 1, false, "")
	require.NoError(t, err)
	var user User
	require.NoError(t, DB.First(&user, 901).Error)
	assert.Equal(t, 1000000, user.Quota)
	assert.Zero(t, user.UsedQuota)
}

func TestBillingModelPricingSeedanceUsesArchivedGenericNativeAndUpstreamRequests(t *testing.T) {
	for _, tc := range []struct {
		name               string
		original, upstream string
		resolution         string
		video              bool
		expected           int64
	}{
		{"generic_original", `{"model":"doubao-seedance-2-5","metadata":{"resolution":"1080p","content":[{"type":"text"}]}}`, "", "1080p", false, 34808},
		{"native_original", `{"model":"doubao-seedance-2-5","resolution":"720p","content":[{"type":"video_url"}]}`, "", "720p", true, 18986},
		{"upstream_priority", `{"metadata":{"resolution":"720p","content":[{"type":"text"}]}}`, `{"resolution":"1080p","content":[{"type":"video_url","video_url":{"url":"https://example.invalid/private-request-video"}}]}`, "1080p", true, 20794},
		{"video_field_without_type", `{"resolution":"720p","content":[{"video_url":{"url":"https://example.invalid/private-request-video"}}]}`, "", "720p", true, 18986},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := seedBillingCorrectionSeedance(t, "doubao-seedance-2-5", "480p", false, false)
			var task Task
			require.NoError(t, DB.First(&task, "task_id = ?", "seedance-task").Error)
			task.Properties.RequestBody = nil
			require.NoError(t, DB.Model(&task).Update("properties", task.Properties).Error)
			require.NoError(t, SaveTaskRequestSnapshots(task.TaskID, "request-archive", []byte(tc.original), []byte(tc.upstream)))
			ratio := 70 / (2 * 7.3)
			configureBillingCorrectionPricing(t, "doubao-seedance-2-5", nil, &ratio)
			batch, err := PreviewBillingCorrection(input, 1, "", "customer_group")
			require.NoError(t, err)
			require.True(t, batch.CanApply)
			assert.Equal(t, tc.expected, batch.Rows[0].CorrectedQuota)
			var frozen billingCorrectionPricing
			require.NoError(t, common.UnmarshalJsonStr(batch.Rows[0].TargetPricing, &frozen))
			assert.Equal(t, tc.resolution, frozen.Resolution)
			require.NotNil(t, frozen.HasVideo)
			assert.Equal(t, tc.video, *frozen.HasVideo)
			assert.False(t, frozen.PerCallBilling, "current token pricing must replace legacy fixed-price flag")
			assert.NotContains(t, batch.Rows[0].TargetPricing, "private-request-video")
			_, err = ApplyBillingCorrection(batch.ID, batch.SHA256, "", "customer_group", 1, false, "")
			require.NoError(t, err, "reading archives inside a single-connection transaction must not deadlock")
			var user User
			require.NoError(t, DB.First(&user, 901).Error)
			assert.Equal(t, tc.expected, int64(user.UsedQuota))
		})
	}
}

func TestBillingModelPricingSeedanceTokenToFixedPriceStopsTokenSettlement(t *testing.T) {
	input := seedBillingCorrectionSeedance(t, "doubao-seedance-2-5", "720p", false, true)
	price := .1
	ratio := 70 / (2 * 7.3)
	configureBillingCorrectionPricing(t, "doubao-seedance-2-5", &price, &ratio)
	batch, err := PreviewBillingCorrection(input, 1, "", "customer_group")
	require.NoError(t, err)
	require.True(t, batch.CanApply)
	assert.Equal(t, int64(33000), batch.Rows[0].CorrectedQuota)
	assert.Zero(t, batch.Rows[1].CorrectedQuota)
	assert.Equal(t, int64(6600), batch.NetDelta)
	var frozen billingCorrectionPricing
	require.NoError(t, common.UnmarshalJsonStr(batch.Rows[0].TargetPricing, &frozen))
	assert.True(t, frozen.PerCallBilling)
	_, err = ApplyBillingCorrection(batch.ID, batch.SHA256, "", "customer_group", 1, false, "")
	require.NoError(t, err)
	var user User
	require.NoError(t, DB.First(&user, 901).Error)
	assert.Equal(t, 993400, user.Quota)
	assert.Equal(t, 33000, user.UsedQuota)
}

func TestBillingGroupCorrectionInheritsRepricingWhenCompletionUsedDifferentRate(t *testing.T) {
	// Polling historically read the current token rate rather than the
	// submission snapshot. The terminal task quota plus the complete original
	// ledger proves that amount even when the two rates differ.
	input := seedBillingCorrectionSeedance(t, "doubao-seedance-2-5", "720p", false, true, 5)
	ratio := 6.0
	configureBillingCorrectionPricing(t, "doubao-seedance-2-5", nil, &ratio)
	pricing, err := PreviewBillingCorrection(input, 1, "", "customer_group")
	require.NoError(t, err)
	require.True(t, pricing.CanApply)
	assert.Equal(t, int64(6600), pricing.NetDelta)
	_, err = ApplyBillingCorrection(pricing.ID, pricing.SHA256, "", "customer_group", 1, false, "")
	require.NoError(t, err)
	input.Mode, input.TargetGroup = BillingCorrectionGroupRate, "wan-prime"
	group, err := PreviewBillingCorrection(input, 1, "0.77", "customer_group")
	require.NoError(t, err)
	require.True(t, group.CanApply)
	assert.Equal(t, int64(6600), group.NetDelta)
	_, err = ApplyBillingCorrection(group.ID, group.SHA256, "0.77", "customer_group", 1, false, "")
	require.NoError(t, err)
	var user User
	require.NoError(t, DB.First(&user, 901).Error)
	assert.Equal(t, 986800, user.Quota)
	assert.Equal(t, 46200, user.UsedQuota)
}
