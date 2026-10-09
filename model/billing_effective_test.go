package model

import (
	"context"
	"math"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type billingEffectiveFixture struct {
	Charge, Refund Log
	Sources        []BillingEntry
	Hour           int64
}

func seedBillingEffectiveViews(t *testing.T) billingEffectiveFixture {
	t.Helper()
	oldDB, oldLogs := DB, LOG_DB
	mainDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	logsDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	for _, db := range []*gorm.DB{mainDB, logsDB} {
		sqlDB, err := db.DB()
		require.NoError(t, err)
		sqlDB.SetMaxOpenConns(1)
		t.Cleanup(func() { assert.NoError(t, sqlDB.Close()) })
	}
	require.NoError(t, mainDB.AutoMigrate(&BillingEntry{}, &BillingCorrectionRow{}, &BillingAccount{}, &QuotaData{}, &Task{}, &Channel{}, &Token{}, &CostAccountingSnapshot{}, &CostAccountingAdjustment{}))
	require.NoError(t, logsDB.AutoMigrate(&Log{}))
	DB, LOG_DB = mainDB, logsDB
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	initCol()
	t.Cleanup(func() {
		DB, LOG_DB = oldDB, oldLogs
		common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
		initCol()
	})
	start, _, err := BillingMonthBounds("2026-09")
	require.NoError(t, err)
	hour := start + 86400 + 3600
	other := common.MapToJsonStr(map[string]any{"task_id": "video-source", "group_ratio": 1, "model_price": 0.01, "fee_quota": 1000})
	fixture := billingEffectiveFixture{
		Charge: Log{UserId: 50, Username: "customer", CreatedAt: hour + 10, Type: LogTypeConsume, ModelName: "video", Quota: 1000, TokenId: 4, ChannelId: 3, TokenName: "token", Group: "old", RequestId: "same-request", PromptTokens: 10, CompletionTokens: 5, Other: other},
		Refund: Log{UserId: 50, Username: "customer", CreatedAt: hour + 20, Type: LogTypeRefund, ModelName: "video", Quota: 400, TokenId: 4, ChannelId: 3, TokenName: "token", Group: "old", RequestId: "same-request", Other: other},
		Hour:   hour,
	}
	require.NoError(t, LOG_DB.Create(&fixture.Charge).Error)
	require.NoError(t, LOG_DB.Create(&fixture.Refund).Error)
	fixture.Sources = []BillingEntry{
		{EventKey: "source-charge", UserID: 50, Sequence: 1, PostedAt: fixture.Charge.CreatedAt, Kind: "usage", Quota: 1000, ModelName: "video", TokenID: 4, RequestID: "same-request", SourceLogID: fixture.Charge.Id},
		{EventKey: "source-refund", UserID: 50, Sequence: 2, PostedAt: fixture.Refund.CreatedAt, Kind: "refund", Quota: -400, ModelName: "video", TokenID: 4, RequestID: "same-request", SourceLogID: fixture.Refund.Id},
	}
	require.NoError(t, DB.Create(&fixture.Sources).Error)
	require.NoError(t, DB.Create(&Channel{Id: 3, Name: "upstream", CostDiscount: "0.9"}).Error)
	require.NoError(t, DB.Create(&Token{Id: 4, UserId: 50, Key: "test-key", Name: "token"}).Error)
	require.NoError(t, DB.Create(&Task{UserId: 50, TaskID: "video-source", PrivateData: TaskPrivateData{NodeName: "node-a"}}).Error)
	require.NoError(t, DB.Create(&QuotaData{UserID: 50, Username: "customer", CreatedAt: hour, ModelName: "video", UseGroup: "old", TokenID: 4, ChannelID: 3, NodeName: "node-a", Quota: 1000, Count: 1, TokenUsed: 15}).Error)
	for i, log := range []*Log{&fixture.Charge, &fixture.Refund} {
		sign := int64(1)
		if i == 1 {
			sign = -1
		}
		require.NoError(t, DB.Create(&CostAccountingSnapshot{EventKey: CostSnapshotEventKey(log), SourceLogID: int64(log.Id), UserID: 50, RequestID: log.RequestId, Username: log.Username, ModelName: log.ModelName,
			TokenName: "token", ChannelID: 3, LogType: log.Type, OccurredAt: log.CreatedAt, GroupName: "old", RevenueQuota: sign * int64(log.Quota), CostQuota: sign * int64(log.Quota) * 9 / 10}).Error)
	}
	return fixture
}

func appendBillingEffectiveFixtureEvents(t *testing.T, fixture billingEffectiveFixture, reverse bool) {
	t.Helper()
	action, group, rate, direction, seq := "apply", "new", "1.2", int64(1), int64(3)
	if reverse {
		action, group, rate, direction, seq = "reverse", "old", "1", -1, 5
	}
	for i, source := range fixture.Sources {
		delta := int64(200)
		if i == 1 {
			delta = -80
		}
		require.NoError(t, DB.Create(&BillingEntry{EventKey: "correction:" + action + ":fixture:" + source.EventKey, UserID: 50, Sequence: seq + int64(i), PostedAt: fixture.Hour + 40*86400, Kind: "rate_correction",
			Quota: direction * delta, SourceEntryID: source.ID, CorrectionID: "fixture", ModelName: source.ModelName, TokenID: 4, BillingGroup: group, BillingRate: rate}).Error)
		if !reverse {
			require.NoError(t, DB.Create(&BillingCorrectionRow{BatchID: "fixture", SourceEntryID: source.ID, TaskID: "video-source", TargetPricing: `{"pricing_mode":"per_call","model_price":0.012,"model_ratio":0,"other_ratios":{"seconds":5}}`}).Error)
		}
		require.NoError(t, LOG_DB.Create(&Log{UserId: 50, Username: "customer", Type: LogTypeBillingCorrection, CreatedAt: fixture.Hour + 40*86400, ModelName: "video", Quota: int(direction * delta), RequestId: action + source.EventKey, Group: group}).Error)
	}
}

func TestBillingEffectiveViewsProjectSourceAmountsAndPreserveEvidence(t *testing.T) {
	fixture := seedBillingEffectiveViews(t)
	before, err := GetBillingUsageHours(context.Background(), 50, fixture.Hour, fixture.Hour+3600)
	require.NoError(t, err)
	require.Len(t, before.Hours, 1)
	assert.Equal(t, int64(1000), before.Hours[0].Charge)
	appendBillingEffectiveFixtureEvents(t, fixture, false)
	logs, total, err := GetUserLogs(50, LogTypeUnknown, 0, 0, "", "", 0, 10, "new", "", "", true)
	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
	require.Len(t, logs, 2)
	for _, log := range logs {
		assert.Equal(t, "new", log.Group)
		metadata, err := common.StrToMap(log.Other)
		require.NoError(t, err)
		assert.Equal(t, float64(1.2), metadata["group_ratio"])
		assert.Equal(t, float64(0.012), metadata["model_price"])
		assert.Equal(t, true, metadata["billing_correction_applied"])
		if log.Type == LogTypeConsume {
			assert.Equal(t, 1200, log.Quota)
			assert.Equal(t, float64(1200), metadata["fee_quota"])
			require.NotNil(t, log.CostQuota)
			assert.Equal(t, int64(900), *log.CostQuota)
			assert.Equal(t, int64(300), *log.ProfitQuota)
			assert.Equal(t, 10, log.PromptTokens)
		} else {
			assert.Equal(t, 480, log.Quota)
			assert.Equal(t, int64(-360), *log.CostQuota)
		}
	}
	_, oldTotal, err := GetUserLogs(50, LogTypeUnknown, 0, 0, "", "", 0, 10, "old", "", "", false)
	require.NoError(t, err)
	assert.Zero(t, oldTotal, "group filtering and pagination counts follow the effective billing group")
	page, total, err := GetUserLogs(50, LogTypeConsume, 0, 0, "", "", 0, 1, "new", "", "", false)
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, page, 1)
	assert.Equal(t, 1200, page[0].Quota)
	window, err := GetBillingUsageHours(context.Background(), 50, fixture.Hour, fixture.Hour+3600)
	require.NoError(t, err)
	require.Len(t, window.Hours, 1)
	assert.Equal(t, int64(1200), window.Hours[0].Charge)
	assert.Equal(t, int64(480), window.Hours[0].Refund)
	assert.Equal(t, int64(2), window.Hours[0].Count, "correction events do not become new requests")
	details, err := GetBillingUsageRecords(context.Background(), 50, fixture.Hour, fixture.Hour+3600, nil)
	require.NoError(t, err)
	require.Len(t, details, 2)
	assert.Equal(t, 480, details[0].Quota)
	assert.Equal(t, 1200, details[1].Quota)
	assert.Empty(t, details[0].Other)
	assert.Zero(t, details[0].ChannelId, "reference details preserve their original safe subset")
	assert.Zero(t, details[0].TokenId)
	var original Log
	require.NoError(t, LOG_DB.First(&original, fixture.Charge.Id).Error)
	assert.Equal(t, 1000, original.Quota)
	assert.Equal(t, "old", original.Group)
	assert.Equal(t, fixture.Charge.Other, original.Other)
	var source BillingEntry
	require.NoError(t, DB.First(&source, fixture.Sources[0].ID).Error)
	assert.Equal(t, int64(1000), source.Quota)
	appendBillingEffectiveFixtureEvents(t, fixture, true)
	restored, total, err := GetUserLogs(50, LogTypeConsume, 0, 0, "", "", 0, 10, "old", "", "", false)
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, restored, 1)
	assert.Equal(t, 1000, restored[0].Quota)
	metadata, err := common.StrToMap(restored[0].Other)
	require.NoError(t, err)
	assert.Equal(t, float64(0.01), metadata["model_price"])
	window, err = GetBillingUsageHours(context.Background(), 50, fixture.Hour, fixture.Hour+3600)
	require.NoError(t, err)
	assert.Equal(t, int64(1000), window.Hours[0].Charge, "reversal invalidates previously cached effective totals")
}

func TestBillingEffectiveStatisticsUseSourcePeriodWithoutAuditDoubleCounting(t *testing.T) {
	fixture := seedBillingEffectiveViews(t)
	appendBillingEffectiveFixtureEvents(t, fixture, false)
	for _, administrative := range []bool{false, true} {
		filter := CostAccountingFilter{UserID: 50, StartTimestamp: fixture.Hour, EndTimestamp: fixture.Hour + 3599, IncludeBillingCorrections: administrative}
		statistics, err := SumLogStatistics(filter)
		require.NoError(t, err)
		assert.Equal(t, int64(1200), statistics.Quota)
		assert.Equal(t, int64(480), statistics.RefundQuota)
		assert.Equal(t, int64(720), statistics.RevenueQuota)
		assert.Equal(t, int64(2), statistics.Records)
		assert.Equal(t, int64(1), statistics.Requests)
		costs, err := SumCostAccounting(filter)
		require.NoError(t, err)
		assert.Equal(t, int64(720), costs.RevenueQuota)
		assert.Equal(t, int64(540), costs.CostQuota)
		assert.Equal(t, int64(180), costs.ProfitQuota)
		buckets, err := SumCostAccountingBuckets(filter, 3600, 0)
		require.NoError(t, err)
		require.Len(t, buckets, 1)
		assert.Equal(t, fixture.Hour, buckets[0].Bucket)
		assert.Equal(t, int64(720), buckets[0].RevenueQuota)
		assert.Equal(t, int64(540), buckets[0].CostQuota)
	}
	execution, err := SumLogStatistics(CostAccountingFilter{UserID: 50, StartTimestamp: fixture.Hour + 40*86400, EndTimestamp: fixture.Hour + 40*86400 + 3600, IncludeBillingCorrections: true})
	require.NoError(t, err)
	assert.Zero(t, execution.RevenueQuota, "mixed usage excludes correction audits posted in the later month")
	audits, err := SumLogStatistics(CostAccountingFilter{UserID: 50, LogType: LogTypeBillingCorrection, IncludeBillingCorrections: true})
	require.NoError(t, err)
	assert.Equal(t, int64(120), audits.RevenueQuota)
	assert.Equal(t, int64(2), audits.Records)
	newGroup, err := SumCostAccounting(CostAccountingFilter{UserID: 50, Group: "new"})
	require.NoError(t, err)
	assert.Equal(t, int64(540), newGroup.CostQuota, "original upstream costs follow their source when filtering by its effective group")
	assert.Equal(t, int64(720), newGroup.RevenueQuota)
}

func TestBillingEffectiveDashboardKeepsGrossConsumptionAndRequestDimensions(t *testing.T) {
	fixture := seedBillingEffectiveViews(t)
	appendBillingEffectiveFixtureEvents(t, fixture, false)
	for _, lookup := range []func() ([]*QuotaData, error){
		func() ([]*QuotaData, error) { return GetQuotaDataByUserId(50, fixture.Hour, fixture.Hour+3599) },
		func() ([]*QuotaData, error) {
			return GetQuotaDataByUsername("customer", fixture.Hour, fixture.Hour+3599)
		},
		func() ([]*QuotaData, error) { return GetQuotaDataGroupByUser(fixture.Hour, fixture.Hour+3599) },
		func() ([]*QuotaData, error) { return GetAllQuotaDates(fixture.Hour, fixture.Hour+3599, "") },
	} {
		rows, err := lookup()
		require.NoError(t, err)
		require.Len(t, rows, 1)
		assert.Equal(t, 1200, rows[0].Quota, "dashboard retains its established gross consumption basis")
		assert.Equal(t, 1, rows[0].Count)
		assert.Equal(t, 15, rows[0].TokenUsed)
	}
	for _, role := range []int{common.RoleCommonUser, common.RoleAdminUser, common.RoleRootUser} {
		rows, err := GetFlowQuotaData(fixture.Hour, fixture.Hour+3599, "", 50, role)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		assert.Equal(t, "new", rows[0].UseGroup)
		assert.Equal(t, 1200, rows[0].Quota)
		assert.Equal(t, 1, rows[0].Count)
		assert.Equal(t, 15, rows[0].TokenUsed)
		if role >= common.RoleRootUser {
			assert.Equal(t, "node-a", rows[0].NodeName)
		}
	}
	var raw QuotaData
	require.NoError(t, DB.First(&raw).Error)
	assert.Equal(t, 1000, raw.Quota)
	assert.Equal(t, "old", raw.UseGroup)
}

func TestBillingEffectiveClickHouseIdentityIsOwnedAndRejectsAmbiguousEvidence(t *testing.T) {
	fixture := seedBillingEffectiveViews(t)
	appendBillingEffectiveFixtureEvents(t, fixture, false)
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeClickHouse)
	initCol()
	// Persisted IDs can be absent or unrelated on the ClickHouse backend.
	require.NoError(t, DB.Table("billing_entries").Where("id IN ?", []int64{fixture.Sources[0].ID, fixture.Sources[1].ID}).Update("source_log_id", 987654).Error)
	foreign := fixture.Charge
	foreign.Id, foreign.UserId = 0, 51
	require.NoError(t, LOG_DB.Create(&foreign).Error)
	logs, total, err := GetUserLogs(50, LogTypeConsume, 0, 0, "", "", 0, 10, "new", "", "", false)
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, logs, 1)
	assert.Equal(t, 1200, logs[0].Quota)
	foreignLogs, _, err := GetUserLogs(51, LogTypeConsume, 0, 0, "", "", 0, 10, "", "", "", false)
	require.NoError(t, err)
	require.Len(t, foreignLogs, 1)
	assert.Equal(t, 1000, foreignLogs[0].Quota)
	duplicate := fixture.Charge
	duplicate.Id = 0
	require.NoError(t, LOG_DB.Create(&duplicate).Error)
	_, _, err = GetUserLogs(50, LogTypeConsume, 0, 0, "", "", 0, 10, "", "", "", false)
	require.ErrorContains(t, err, "ambiguous original log evidence")
}

func TestBillingEffectiveTaskRefundUsesStableTaskEvidence(t *testing.T) {
	fixture := seedBillingEffectiveViews(t)
	require.NoError(t, DB.Table("billing_entries").Where("id = ?", fixture.Sources[1].ID).Updates(map[string]any{"kind": "task_adjustment", "source_log_id": 0, "request_id": "video-source"}).Error)
	appendBillingEffectiveFixtureEvents(t, fixture, false)
	logs, _, err := GetUserLogs(50, LogTypeRefund, 0, 0, "", "", 0, 10, "", "", "", false)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Equal(t, 480, logs[0].Quota)
	// A nearby unrelated request with the same amount cannot steal the refund.
	unrelated := fixture.Refund
	unrelated.Id = 0
	unrelated.Other = common.MapToJsonStr(map[string]any{"task_id": "unrelated-video"})
	unrelated.RequestId = "unrelated-request"
	unrelated.CreatedAt++
	require.NoError(t, LOG_DB.Create(&unrelated).Error)
	logs, _, err = GetUserLogs(50, LogTypeRefund, 0, 0, "", "", 0, 10, "", "", "", false)
	require.NoError(t, err)
	require.Len(t, logs, 2)
	assert.Equal(t, 400, logs[0].Quota)
	assert.Equal(t, 480, logs[1].Quota)
}

func TestBillingEffectiveUnconfiguredCostUsesOriginalAmountInEveryGroup(t *testing.T) {
	fixture := seedBillingEffectiveViews(t)
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&CostAccountingSnapshot{}).Error)
	require.NoError(t, DB.Model(&Channel{}).Where("id = ?", 3).Update("cost_discount", "").Error)
	appendBillingEffectiveFixtureEvents(t, fixture, false)
	for _, group := range []string{"", "new", "old"} {
		filter := CostAccountingFilter{UserID: 50, Group: group, StartTimestamp: fixture.Hour, EndTimestamp: fixture.Hour + 3599}
		totals, err := SumCostAccounting(filter)
		require.NoError(t, err)
		statistics, err := SumLogStatistics(filter)
		require.NoError(t, err)
		if group == "old" {
			assert.Zero(t, totals.RevenueQuota)
			assert.Zero(t, totals.CostQuota)
			assert.Zero(t, totals.DefaultedRecords)
		} else {
			assert.Equal(t, int64(720), totals.RevenueQuota)
			assert.Equal(t, int64(600), totals.CostQuota, "unconfigured upstream cost uses original revenue, not the corrected selling price")
			assert.Equal(t, int64(120), totals.ProfitQuota)
			assert.Equal(t, int64(2), totals.DefaultedRecords)
			assert.True(t, ReconcileCostAccountingTotals(statistics, totals).AccountingComplete)
			buckets, err := SumCostAccountingBuckets(filter, 3600, 0)
			require.NoError(t, err)
			require.Len(t, buckets, 1)
			assert.Equal(t, int64(720), buckets[0].RevenueQuota)
			assert.Equal(t, int64(600), buckets[0].CostQuota)
		}
	}
}

func TestBillingEffectiveSuccessiveCorrectionsAndReversalsRestoreEveryProjection(t *testing.T) {
	fixture := seedBillingEffectiveViews(t)
	appendBillingEffectiveFixtureEvents(t, fixture, false)
	for i, source := range fixture.Sources {
		delta := int64(300)
		if i == 1 {
			delta = -120
		}
		require.NoError(t, DB.Create(&BillingEntry{EventKey: "correction:apply:second:" + source.EventKey, UserID: 50, Sequence: 5 + int64(i), PostedAt: fixture.Hour + 41*86400, Kind: "rate_correction",
			Quota: delta, SourceEntryID: source.ID, CorrectionID: "second", ModelName: "video", TokenID: 4, BillingGroup: "final", BillingRate: "1.5"}).Error)
		require.NoError(t, DB.Create(&BillingCorrectionRow{BatchID: "second", SourceEntryID: source.ID, TaskID: "video-source", PreviousBatchID: "fixture", TargetPricing: `{"model_price":0.015,"model_ratio":0,"other_ratios":{"seconds":5}}`}).Error)
	}
	assertProjection := func(group string, charge, refund int, price float64) {
		t.Helper()
		logs, total, err := GetUserLogs(50, LogTypeUnknown, 0, 0, "", "", 0, 10, group, "", "", false)
		require.NoError(t, err)
		assert.EqualValues(t, 2, total)
		require.Len(t, logs, 2)
		assert.Equal(t, refund, logs[0].Quota)
		assert.Equal(t, charge, logs[1].Quota)
		metadata, err := common.StrToMap(logs[1].Other)
		require.NoError(t, err)
		assert.Equal(t, price, metadata["model_price"])
		statistics, err := SumLogStatistics(CostAccountingFilter{UserID: 50, Group: group})
		require.NoError(t, err)
		assert.Equal(t, int64(charge-refund), statistics.RevenueQuota)
		rows, err := GetQuotaDataByUserId(50, fixture.Hour, fixture.Hour+3599)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		assert.Equal(t, charge, rows[0].Quota)
	}
	assertProjection("final", 1500, 600, 0.015)
	for i, source := range fixture.Sources {
		delta := int64(-300)
		if i == 1 {
			delta = 120
		}
		require.NoError(t, DB.Create(&BillingEntry{EventKey: "correction:reverse:second:" + source.EventKey, UserID: 50, Sequence: 7 + int64(i), PostedAt: fixture.Hour + 42*86400, Kind: "rate_correction",
			Quota: delta, SourceEntryID: source.ID, CorrectionID: "second", ModelName: "video", TokenID: 4, BillingGroup: "new", BillingRate: "1.2"}).Error)
	}
	assertProjection("new", 1200, 480, 0.012)
	for i, source := range fixture.Sources {
		delta := int64(-200)
		if i == 1 {
			delta = 80
		}
		require.NoError(t, DB.Create(&BillingEntry{EventKey: "correction:reverse:fixture:" + source.EventKey, UserID: 50, Sequence: 9 + int64(i), PostedAt: fixture.Hour + 43*86400, Kind: "rate_correction",
			Quota: delta, SourceEntryID: source.ID, CorrectionID: "fixture", ModelName: "video", TokenID: 4, BillingGroup: "old", BillingRate: "1"}).Error)
	}
	assertProjection("old", 1000, 400, 0.01)
}

func TestBillingEffectiveDashboardKeepsHistoricalUsernameDimensions(t *testing.T) {
	fixture := seedBillingEffectiveViews(t)
	require.NoError(t, DB.Create(&QuotaData{UserID: 50, Username: "renamed-customer", CreatedAt: fixture.Hour, ModelName: "video", UseGroup: "old", TokenID: 4, ChannelID: 3, NodeName: "node-a", Quota: 500, Count: 1, TokenUsed: 20}).Error)
	appendBillingEffectiveFixtureEvents(t, fixture, false)
	rows, err := GetQuotaDataByUserId(50, fixture.Hour, fixture.Hour+3599)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	for _, row := range rows {
		if row.Username == "customer" {
			assert.Equal(t, 1200, row.Quota)
		} else {
			assert.Equal(t, 500, row.Quota)
		}
	}
	flow, err := GetFlowQuotaData(fixture.Hour, fixture.Hour+3599, "", 50, common.RoleRootUser)
	require.NoError(t, err)
	require.Len(t, flow, 2)
	assert.Equal(t, "customer", flow[0].Username)
	assert.Equal(t, "new", flow[0].UseGroup)
	assert.Equal(t, 1200, flow[0].Quota)
	assert.Equal(t, "renamed-customer", flow[1].Username)
	assert.Equal(t, "old", flow[1].UseGroup)
	assert.Equal(t, 500, flow[1].Quota)
}

func TestBillingEffectiveTokenSettlementUsesCurrentTokenPriceInVisibleFeeDetails(t *testing.T) {
	fixture := seedBillingEffectiveViews(t)
	appendBillingEffectiveFixtureEvents(t, fixture, false)
	require.NoError(t, DB.Model(&BillingCorrectionRow{}).Where("batch_id = ?", "fixture").Update("target_pricing", `{"pricing_mode":"tokens","model_price":0.015,"model_ratio":5.5,"total_tokens":200,"resolution":"1080p","other_ratios":{"resolution":1.1}}`).Error)
	logs, _, err := GetUserLogs(50, LogTypeConsume, 0, 0, "", "", 0, 10, "", "", "", false)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	metadata, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	assert.Equal(t, float64(0), metadata["model_price"], "submission reservation unit price must not make a token-settled correction look like per-call pricing")
	assert.Equal(t, float64(5.5), metadata["model_ratio"])
	assert.Equal(t, "1080p", metadata["resolution"])
	assert.Equal(t, float64(200), metadata["total_tokens"])
	assert.Equal(t, 10, logs[0].PromptTokens, "changing the visible pricing basis does not rewrite factual token usage")
}

func TestBillingEffectiveStatisticsRejectAggregateOverflowInsteadOfWrappingCharges(t *testing.T) {
	fixture := seedBillingEffectiveViews(t)
	appendBillingEffectiveFixtureEvents(t, fixture, false)
	require.NoError(t, LOG_DB.Create(&Log{UserId: 50, CreatedAt: fixture.Hour + 30, Type: LogTypeConsume, Quota: math.MaxInt64 - 1100, ModelName: "corrupt-historical-usage"}).Error)
	_, err := SumLogStatistics(CostAccountingFilter{UserID: 50})
	require.ErrorIs(t, err, ErrBillingEvidenceIntegrity)
}

func TestBillingEffectiveWideTimestampFiltersStillIncludeCorrections(t *testing.T) {
	fixture := seedBillingEffectiveViews(t)
	appendBillingEffectiveFixtureEvents(t, fixture, false)
	logs, total, err := GetUserLogs(50, LogTypeConsume, math.MinInt64, math.MaxInt64, "", "", 0, 10, "new", "", "", false)
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, logs, 1)
	assert.Equal(t, 1200, logs[0].Quota)
	statistics, err := SumLogStatistics(CostAccountingFilter{UserID: 50, StartTimestamp: math.MinInt64, EndTimestamp: math.MaxInt64})
	require.NoError(t, err)
	assert.Equal(t, int64(720), statistics.RevenueQuota)
}
