package model

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBillingSupplierCostCorrectionsPreserveGroupIndependenceAndReverseDependencies(t *testing.T) {
	fixture := seedBillingCorrectionCosts(t, "doubao-seedance-2-5", "720p", true, false)
	first, err := PreviewBillingCorrection(fixture.Input, 1, "", "customer_group")
	require.NoError(t, err)
	_, err = ApplyBillingCorrection(first.ID, first.SHA256, "", "customer_group", 1, false, "")
	require.NoError(t, err)
	group := fixture.Input
	group.Mode, group.TargetGroup, group.Reason = BillingCorrectionGroupRate, "customer_discount", "Change customer group without changing supplier price"
	second, err := PreviewBillingCorrection(group, 1, "0.77", "customer_group")
	require.NoError(t, err)
	require.True(t, second.CanApply, "rows: %+v", second.Rows)
	assert.Zero(t, second.CostDelta)
	for _, row := range second.Rows {
		assert.False(t, row.CostChanged)
	}
	_, err = ApplyBillingCorrection(second.ID, second.SHA256, "0.77", "customer_group", 1, false, "")
	require.NoError(t, err)
	_, err = ApplyBillingCorrection(first.ID, first.SHA256, "", "", 1, true, "Reverse earlier model correction")
	require.ErrorIs(t, err, ErrBillingCorrectionDependency)
	_, err = ApplyBillingCorrection(second.ID, second.SHA256, "", "", 1, true, "Reverse later customer group correction")
	require.NoError(t, err)
	_, err = ApplyBillingCorrection(first.ID, first.SHA256, "", "", 1, true, "Reverse earlier model correction after its dependency")
	require.NoError(t, err)
	totals, err := SumCostAccounting(CostAccountingFilter{UserID: 901})
	require.NoError(t, err)
	assert.Equal(t, fixture.OriginalCost, totals.CostQuota)
}

func TestCostRepriceRejectsStaleModelBasisAndPreventsEarlierCorrectionReversal(t *testing.T) {
	fixture := seedBillingCorrectionCosts(t, "doubao-seedance-2-5", "720p", true, false)
	before, _, err := ListCostAccountingSnapshots(CostAccountingFilter{UserID: 901}, 0, 10)
	require.NoError(t, err)
	batch, err := PreviewBillingCorrection(fixture.Input, 1, "", "customer_group")
	require.NoError(t, err)
	_, err = ApplyBillingCorrection(batch.ID, batch.SHA256, "", "customer_group", 1, false, "")
	require.NoError(t, err)
	old := before[0]
	_, err = AdjustCostAccountingSnapshots([]CostAccountingAdjustmentTarget{{SnapshotID: old.ID, NewCostQuota: old.EffectiveCostQuota, CostDiscount: old.EffectiveCostDiscount,
		CostBasisQuota: old.EffectiveCostBasisQuota, BasisVersion: old.EffectiveBasisVersion, ExpectedAdjustmentID: &old.LatestAdjustmentID}}, "Stale discount preview", 1, "stale-preview")
	require.ErrorIs(t, err, ErrBillingConflict)
	current, _, err := ListCostAccountingSnapshots(CostAccountingFilter{UserID: 901}, 0, 10)
	require.NoError(t, err)
	view := current[0]
	newCost, err := RepriceCostAccountingBasis(view, "0.5")
	require.NoError(t, err)
	_, err = AdjustCostAccountingSnapshots([]CostAccountingAdjustmentTarget{{SnapshotID: view.ID, NewCostQuota: newCost, CostDiscount: "0.5", CostBasisQuota: view.EffectiveCostBasisQuota,
		CostBasisBefore: view.EffectiveCostBasisBefore, CostBasisAfter: view.EffectiveCostBasisAfter, BasisVersion: view.EffectiveBasisVersion, ExpectedAdjustmentID: &view.LatestAdjustmentID}}, "Later supplier discount correction", 1, "later-discount")
	require.NoError(t, err)
	_, err = ApplyBillingCorrection(batch.ID, batch.SHA256, "", "", 1, true, "Reversal must not erase a later cost reprice")
	require.ErrorIs(t, err, ErrBillingCorrectionDependency)
	var user User
	require.NoError(t, DB.First(&user, 901).Error)
	assert.Equal(t, int64(2000000)-batch.NetDelta, int64(user.Quota), "rejected cost reversal rolls back its wallet and ledger transaction")
}

func TestBillingSupplierCostsAllowLIFOReversalAfterLaterModelCorrectionIsReversed(t *testing.T) {
	fixture := seedBillingCorrectionCosts(t, "doubao-seedance-2-5", "720p", true, false)
	first, err := PreviewBillingCorrection(fixture.Input, 1, "", "customer_group")
	require.NoError(t, err)
	_, err = ApplyBillingCorrection(first.ID, first.SHA256, "", "customer_group", 1, false, "")
	require.NoError(t, err)
	ratio := 6.0
	configureBillingCorrectionPricing(t, "doubao-seedance-2-5", nil, &ratio)
	second, err := PreviewBillingCorrection(fixture.Input, 1, "", "customer_group")
	require.NoError(t, err)
	require.True(t, second.CanApply)
	_, err = ApplyBillingCorrection(second.ID, second.SHA256, "", "customer_group", 1, false, "")
	require.NoError(t, err)
	filter := CostAccountingFilter{UserID: 901}
	totals, err := SumCostAccounting(filter)
	require.NoError(t, err)
	assert.Equal(t, int64(29520), totals.CostQuota)
	_, err = ApplyBillingCorrection(first.ID, first.SHA256, "", "", 1, true, "Reject reversal of a depended-on supplier price")
	require.ErrorIs(t, err, ErrBillingCorrectionDependency)
	_, err = ApplyBillingCorrection(second.ID, second.SHA256, "", "", 1, true, "Reverse later supplier model price")
	require.NoError(t, err)
	totals, err = SumCostAccounting(filter)
	require.NoError(t, err)
	assert.Equal(t, int64(24600), totals.CostQuota)
	_, err = ApplyBillingCorrection(first.ID, first.SHA256, "", "", 1, true, "Reverse earlier supplier model price after dependency reversal")
	require.NoError(t, err)
	totals, err = SumCostAccounting(filter)
	require.NoError(t, err)
	assert.Equal(t, fixture.OriginalCost, totals.CostQuota)
	var user User
	require.NoError(t, DB.First(&user, 901).Error)
	assert.Equal(t, 2000000, user.Quota)
}

func TestCostViewsRestoreLegacyBasisAndRejectOverflowingHistoricalAdjustments(t *testing.T) {
	for _, tc := range []struct {
		name               string
		badQuota, badDelta bool
	}{{"restore legacy version zero", false, false}, {"reject oversized original cost", true, false}, {"reject adjustment overflow before addition", false, true}} {
		t.Run(tc.name, func(t *testing.T) {
			truncateTables(t)
			snapshot := CostAccountingSnapshot{EventKey: "legacy-proof", UserID: 7, ModelName: "legacy-model", CostBasisQuota: "1000", CostDiscount: "0.82", CostQuota: 820, SnapshotVersion: 0}
			if tc.badQuota {
				snapshot.CostQuota = math.MaxInt64
			}
			require.NoError(t, DB.Create(&snapshot).Error)
			if tc.badDelta {
				require.NoError(t, DB.Create(&CostAccountingAdjustment{SnapshotID: snapshot.ID, DeltaCostQuota: math.MaxInt64}).Error)
			} else if !tc.badQuota {
				require.NoError(t, DB.Create(&CostAccountingAdjustment{SnapshotID: snapshot.ID, DeltaCostQuota: -100, NewCostQuota: 720, CostBasisQuota: "900", CostDiscount: "0.8", BasisVersion: CostAccountingCorrectedBasisVersion, Action: "apply"}).Error)
				require.NoError(t, DB.Create(&CostAccountingAdjustment{SnapshotID: snapshot.ID, DeltaCostQuota: 100, NewCostQuota: 820, CostBasisQuota: "1000", CostDiscount: "0.82", BasisVersion: 0, Action: "reverse"}).Error)
			}
			views, _, err := ListCostAccountingSnapshots(CostAccountingFilter{UserID: 7}, 0, 10)
			if tc.badQuota || tc.badDelta {
				require.Error(t, err, "invalid historical cost must fail before a wrapped amount is returned")
				return
			}
			require.NoError(t, err)
			require.Len(t, views, 1)
			assert.Equal(t, int64(820), views[0].EffectiveCostQuota)
			assert.Equal(t, "1000", views[0].EffectiveCostBasisQuota)
			assert.Equal(t, "0.82", views[0].EffectiveCostDiscount)
			assert.Zero(t, views[0].EffectiveBasisVersion)
		})
	}
}
