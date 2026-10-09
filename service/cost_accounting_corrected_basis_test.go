package service

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSupplierDiscountRepriceRetainsCorrectedModelBasisAcrossClearingAndRestoringDiscount(t *testing.T) {
	truncate(t)
	pre := decimal.NewFromInt(1250000).Mul(decimal.NewFromInt(46)).Div(decimal.NewFromInt(70))
	final := decimal.NewFromInt(50000).Mul(decimal.NewFromInt(46)).Div(decimal.NewFromInt(70))
	for i, amount := range []int64{1075000, -1032000} {
		logType, quota := model.LogTypeConsume, amount
		if quota < 0 {
			logType, quota = model.LogTypeRefund, -quota
		}
		log := model.Log{UserId: 7, Type: logType, Quota: int(quota), CreatedAt: int64(100 + i), ChannelId: 9, ModelName: "doubao-seedance-2-5", Other: common.MapToJsonStr(map[string]any{"group_ratio": .86})}
		require.NoError(t, model.LOG_DB.Create(&log).Error)
		originalBasis := decimal.NewFromInt(amount).Div(decimal.RequireFromString("0.86"))
		originalQuota, err := common.QuotaFromDecimalStrict(originalBasis.Mul(decimal.RequireFromString("0.82")))
		require.NoError(t, err)
		snapshot, err := model.BuildCostAccountingSnapshot(&log, &model.CostAccountingInput{CostBasisQuota: originalBasis.String(), CostDiscount: "0.82", CostQuota: int64(originalQuota)})
		require.NoError(t, err)
		require.NoError(t, model.DB.Create(snapshot).Error)
		before, after, cost := decimal.Zero, pre, int64(673571)
		if i == 1 {
			before, after, cost = pre, final, -646628
		}
		event := fmt.Sprintf("model-cost-proof-%d", i)
		require.NoError(t, model.DB.Create(&model.CostAccountingAdjustment{EventKey: &event, SnapshotID: snapshot.ID, DeltaCostQuota: cost - int64(originalQuota), NewCostQuota: cost,
			CostDiscount: "0.820000", CostBasisQuota: after.Sub(before).String(), CostBasisBefore: before.String(), CostBasisAfter: after.String(), BasisVersion: model.CostAccountingCorrectedBasisVersion, Action: "apply", CorrectionID: "corrected-model-proof"}).Error)
	}
	filter := model.CostAccountingFilter{ChannelID: 9}
	for _, tc := range []struct {
		discount string
		want     int64
	}{{"0.5", 16429}, {"", 43000}, {"0.82", 26943}} {
		input := CostAccountingRepriceInput{StartTimestamp: 1, EndTimestamp: 200, ChannelID: 9, CostDiscount: tc.discount, Limit: 10, Apply: true, Reason: "Change supplier discount without restoring old model price", ActorID: 1}
		result, err := RepriceCostAccounting(input)
		require.NoError(t, err)
		assert.Equal(t, tc.want, result.NewCostQuota)
		totals, err := model.SumCostAccounting(filter)
		require.NoError(t, err)
		assert.Equal(t, tc.want, totals.CostQuota)
		views, _, err := model.ListCostAccountingSnapshots(filter, 0, 10)
		require.NoError(t, err)
		basis := decimal.Zero
		for _, view := range views {
			assert.Equal(t, model.CostAccountingCorrectedBasisVersion, view.EffectiveBasisVersion)
			basis = basis.Add(decimal.RequireFromString(view.EffectiveCostBasisQuota))
		}
		assert.True(t, final.Equal(basis), "supplier discount changes retain the corrected 46/70 price basis")
		repeated, err := RepriceCostAccounting(input)
		require.NoError(t, err)
		assert.Zero(t, repeated.Applied, "the same effective basis and discount must not append another adjustment")
	}
}
