package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
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
	assert.Equal(t, "already_corrected", blocked.Rows[0].Blocked)
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
	for _, counter := range []string{"wallet", "token"} {
		t.Run(counter, func(t *testing.T) {
			input := seedBillingCorrection(t)
			batch, err := PreviewBillingCorrection(input, 1, "0.77", "customer_group")
			require.NoError(t, err)
			if counter == "wallet" {
				require.NoError(t, DB.Model(&User{}).Where("id = ?", 901).Update("quota", 0).Error)
			} else {
				require.NoError(t, DB.Model(&Token{}).Where("id = ?", 903).Update("remain_quota", 0).Error)
			}
			_, err = ApplyBillingCorrection(batch.ID, batch.SHA256, "0.77", "customer_group", 1, false, "")
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
	assert.Equal(t, int64(424280), customer.RevenueQuota)
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
