package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetCostAccountingOverviewSeparatesNaturalDaysAndDefaultsUnconfiguredCostToRevenue(t *testing.T) {
	truncate(t)
	location := time.FixedZone("Asia/Shanghai", 8*3600)
	now := time.Date(2026, 9, 20, 10, 30, 0, 0, location)
	todayStart := time.Date(2026, 9, 20, 0, 0, 0, 0, location).Unix()
	yesterdayStart := time.Date(2026, 9, 19, 0, 0, 0, 0, location).Unix()
	logs := []model.Log{
		{UserId: 1, Type: model.LogTypeConsume, CreatedAt: todayStart + 3600, Quota: 1000, RequestId: "today-consume"},
		{UserId: 1, Type: model.LogTypeRefund, CreatedAt: todayStart + 7200, Quota: 200, RequestId: "today-refund"},
		{UserId: 1, Type: model.LogTypeConsume, CreatedAt: yesterdayStart + 3600, Quota: 900, RequestId: "yesterday-consume", ChannelId: 9},
	}
	require.NoError(t, model.DB.Create(&model.Channel{Id: 9, Name: "unconfigured-cost", Key: "key", CostDiscount: ""}).Error)
	require.NoError(t, model.LOG_DB.Create(&logs).Error)

	todayConsume, err := model.BuildCostAccountingSnapshot(&logs[0], &model.CostAccountingInput{
		CostBasisQuota: "625", CostDiscount: "0.8", CostQuota: 500,
	})
	require.NoError(t, err)
	todayRefund, err := model.BuildCostAccountingSnapshot(&logs[1], &model.CostAccountingInput{
		CostBasisQuota: "-125", CostDiscount: "0.8", CostQuota: -100,
	})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create([]*model.CostAccountingSnapshot{todayConsume, todayRefund}).Error)

	overview, err := GetCostAccountingOverview(now, model.CostAccountingFilter{})
	require.NoError(t, err)

	assert.Equal(t, todayStart, overview.Today.StartTimestamp)
	assert.Equal(t, now.Unix(), overview.Today.EndTimestamp)
	assert.Equal(t, int64(800), overview.Today.RevenueQuota)
	assert.Equal(t, int64(400), overview.Today.CostQuota)
	require.NotNil(t, overview.Today.ProfitQuota)
	assert.Equal(t, int64(400), *overview.Today.ProfitQuota)
	assert.True(t, overview.Today.AccountingComplete)
	assert.Zero(t, overview.Today.MissingCostRecords)

	assert.Equal(t, yesterdayStart, overview.Yesterday.StartTimestamp)
	assert.Equal(t, todayStart-1, overview.Yesterday.EndTimestamp)
	assert.Equal(t, int64(900), overview.Yesterday.RevenueQuota)
	assert.Equal(t, int64(900), overview.Yesterday.CostQuota)
	require.NotNil(t, overview.Yesterday.ProfitQuota)
	assert.Zero(t, *overview.Yesterday.ProfitQuota)
	assert.True(t, overview.Yesterday.AccountingComplete)
	assert.Zero(t, overview.Yesterday.MissingCostRecords)
}

func TestGetCostAccountingOverviewWithholdsProfitForConfiguredChannelWithoutSnapshot(t *testing.T) {
	truncate(t)
	location := time.FixedZone("Asia/Shanghai", 8*3600)
	now := time.Date(2026, 9, 20, 10, 30, 0, 0, location)
	todayStart := time.Date(2026, 9, 20, 0, 0, 0, 0, location).Unix()
	require.NoError(t, model.DB.Create(&model.Channel{Id: 10, Name: "configured-cost", Key: "key", CostDiscount: "0.8"}).Error)
	require.NoError(t, model.LOG_DB.Create(&model.Log{
		UserId: 1, Type: model.LogTypeConsume, CreatedAt: todayStart + 3600,
		Quota: 1000, RequestId: "configured-missing", ChannelId: 10,
	}).Error)

	overview, err := GetCostAccountingOverview(now, model.CostAccountingFilter{})
	require.NoError(t, err)

	assert.Equal(t, int64(1000), overview.Today.RevenueQuota)
	assert.Nil(t, overview.Today.ProfitQuota)
	assert.False(t, overview.Today.AccountingComplete)
	assert.Equal(t, int64(1), overview.Today.MissingCostRecords)
}
