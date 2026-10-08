package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBillingModelsSplitByConsumedGroupAndHistoricalRate(t *testing.T) {
	id := seedBillingCustomer(t)
	logs := []Log{
		{Id: 100, UserId: id, Type: LogTypeConsume, ModelName: "seedance", Quota: 86, Group: "spe_sd_86", Other: `{"group_ratio":0.86}`},
		{Id: 101, UserId: id, Type: LogTypeConsume, ModelName: "seedance", Quota: 86, Group: "spe_sd_86", RequestId: "second", Other: `{"group_ratio":"0.860000"}`},
		{Id: 102, UserId: id, Type: LogTypeConsume, ModelName: "seedance", Quota: 100, Group: "sd", RequestId: "standard", Other: `{"group_ratio":1}`},
		{Id: 103, UserId: id, Type: LogTypeConsume, ModelName: "seedance", Quota: 80, Group: "spe_sd_86", RequestId: "old-discount", Other: `{"group_ratio":0.8}`},
	}
	require.NoError(t, LOG_DB.Create(&logs).Error)
	task := Task{TaskID: "refund-task", UserId: id, Group: "spe_sd_86", PrivateData: TaskPrivateData{BillingContext: &TaskBillingContext{GroupRatio: 0.86}}}
	require.NoError(t, DB.Create(&task).Error)
	entries := []BillingEntry{
		{EventKey: "import", UserID: id, Sequence: 1, PostedAt: 10, Kind: "history_usage", ModelName: "seedance", Quota: 86, SourceLogID: 100},
		{EventKey: "second", UserID: id, Sequence: 2, PostedAt: 11, Kind: "usage", ModelName: "seedance", Quota: 86, RequestID: "second"},
		{EventKey: "standard", UserID: id, Sequence: 3, PostedAt: 12, Kind: "usage", ModelName: "seedance", Quota: 100, RequestID: "standard"},
		{EventKey: "old-discount", UserID: id, Sequence: 4, PostedAt: 13, Kind: "usage", ModelName: "seedance", Quota: 80, RequestID: "old-discount"},
		{EventKey: "refund", UserID: id, Sequence: 5, PostedAt: 14, Kind: "task_adjustment", ModelName: "seedance", Quota: -43, RequestID: "refund-task"},
		{EventKey: "missing", UserID: id, Sequence: 6, PostedAt: 15, Kind: "usage", ModelName: "seedance", Quota: 6, RequestID: "missing"},
	}
	require.NoError(t, DB.Create(&entries).Error)
	totals, err := summarizeBillingModels(DB.Model(&BillingEntry{}), id, 1, 6, 0, 100, true)
	require.NoError(t, err)
	require.Len(t, totals, 4)
	assert.Equal(t, "", totals[0].BillingRate, "missing evidence must not borrow a current rate")
	assert.Equal(t, int64(6), totals[0].Charge)
	assert.Equal(t, "sd", totals[1].BillingGroup)
	assert.Equal(t, "1", totals[1].BillingRate)
	assert.Equal(t, int64(100), totals[1].Charge)
	assert.Equal(t, "0.8", totals[2].BillingRate, "a historical rate change splits the same group")
	assert.Equal(t, int64(80), totals[2].Charge)
	assert.Equal(t, "spe_sd_86", totals[3].BillingGroup)
	assert.Equal(t, "0.86", totals[3].BillingRate)
	assert.Equal(t, int64(172), totals[3].Charge)
	assert.Equal(t, int64(43), totals[3].Refund)
	assert.Equal(t, int64(3), totals[3].Count)
}

func TestBillingHistoricalRateDoesNotGuessFromAmbiguousOrUnrelatedLogs(t *testing.T) {
	id := seedBillingCustomer(t)
	logs := []Log{
		{UserId: id, Type: LogTypeConsume, ModelName: "model", Quota: 10, Group: "a", RequestId: "duplicate", Other: `{"group_ratio":0.8}`},
		{UserId: id, Type: LogTypeConsume, ModelName: "model", Quota: 10, Group: "b", RequestId: "duplicate", Other: `{"group_ratio":1}`},
		{UserId: id + 1, Type: LogTypeConsume, ModelName: "model", Quota: 10, Group: "other-user", RequestId: "other", Other: `{"group_ratio":0.5}`},
		{UserId: id, Type: LogTypeConsume, ModelName: "model", Quota: 0, Group: "free", RequestId: "free", Other: `{"group_ratio":0}`},
	}
	require.NoError(t, LOG_DB.Create(&logs).Error)
	rates, err := billingEntryHistoricalRates(DB, id, []BillingEntry{
		{Sequence: 1, ModelName: "model", Quota: 10, RequestID: "duplicate"},
		{Sequence: 2, ModelName: "model", Quota: 10, RequestID: "other"},
		{Sequence: 3, ModelName: "model", Quota: 0, RequestID: "free"},
	})
	require.NoError(t, err)
	assert.Empty(t, rates[1])
	assert.Empty(t, rates[2])
	assert.Equal(t, billingHistoricalRate{Group: "free", Ratio: "0"}, rates[3])
}
