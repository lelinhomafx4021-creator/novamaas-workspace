package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedBillingCorrectionSettlementWindow(t *testing.T, totalTokens int64) (BillingCorrectionInput, Task) {
	t.Helper()
	truncateTables(t)
	posted := common.GetTimestamp() - 60
	const sourceQuota = 660000
	const walletAfterSubmission = 1000000
	const userID = 901
	modelName := "doubao-seedance-2-5"
	ratio := 4.0
	configureBillingCorrectionPricing(t, modelName, nil, &ratio)
	require.NoError(t, DB.Create(&User{Id: userID, Username: "settlement_window_customer", Group: "customer_group", Quota: walletAfterSubmission, UsedQuota: sourceQuota, Status: common.UserStatusEnabled}).Error)
	require.NoError(t, DB.Create(&BillingAccount{UserID: userID, AccountingStartAt: posted - 60, Sequence: 1}).Error)
	require.NoError(t, DB.Create(&Token{Id: 903, UserId: userID, Key: "window-test-token", RemainQuota: walletAfterSubmission, UsedQuota: sourceQuota}).Error)
	require.NoError(t, DB.Create(&Channel{Id: 904, UsedQuota: sourceQuota}).Error)
	// Submission finishes BillingOperation before any asynchronous completion.
	// Therefore the correction's reserved-operation guard sees no pending row.
	require.NoError(t, DB.Create(&BillingOperation{ID: "window-submission", UserID: userID, State: "settled", Reserved: sourceQuota, Actual: sourceQuota, RequestID: "window-submit-request", ModelName: modelName, TokenID: 903, CreatedAt: posted, UpdatedAt: posted}).Error)
	task := Task{TaskID: "window-task", UserId: userID, Group: "wan", ChannelId: 904, Quota: sourceQuota, Status: TaskStatusInProgress,
		PrivateData: TaskPrivateData{TokenId: 903, BillingContext: &TaskBillingContext{OriginModelName: modelName, ModelPrice: -1, ModelRatio: ratio, GroupRatio: .66}},
		Properties:  Properties{RequestBody: []byte(`{"resolution":"720p","content":[{"type":"text"}]}`)}}
	require.NoError(t, DB.Create(&task).Error)
	metadata, err := common.Marshal(map[string]any{"task_id": task.TaskID, "group_ratio": .66})
	require.NoError(t, err)
	log := Log{UserId: userID, Type: LogTypeConsume, CreatedAt: posted, ModelName: modelName, Quota: sourceQuota, Group: "wan", TokenId: 903, ChannelId: 904, Other: string(metadata)}
	require.NoError(t, DB.Create(&log).Error)
	require.NoError(t, DB.Create(&BillingEntry{EventKey: "settle:window-submission", UserID: userID, Sequence: 1, PostedAt: posted, Kind: "usage", ModelName: modelName, Quota: sourceQuota, TokenID: 903, SourceLogID: log.Id, RequestID: "window-submit-request"}).Error)
	// This is the exact production order in updateVideoSingleTask: publish
	// terminal status/data with CAS, then perform token settlement afterwards.
	task.Status, task.Progress, task.FinishTime = TaskStatusSuccess, "100%", common.GetTimestamp()
	task.SetData(map[string]any{"resolution": "720p", "usage": map[string]any{"total_tokens": totalTokens}})
	won, err := task.UpdateWithStatus(TaskStatusInProgress)
	require.NoError(t, err)
	require.True(t, won)
	return BillingCorrectionInput{UserID: userID, StartAt: posted - 10, EndAt: posted + 10, Models: []string{modelName}, Mode: BillingCorrectionModelPricing, Reason: "Reprice a completed task"}, task
}

func TestBillingModelPricingRejectsCompletedStatusBeforeDeferredTokenSettlement(t *testing.T) {
	input, task := seedBillingCorrectionSettlementWindow(t, 10000)
	const sourceQuota = 660000
	const actualQuota = 26400
	const walletAfterSubmission = 1000000
	const userID = 901
	modelName := "doubao-seedance-2-5"
	batch, err := PreviewBillingCorrection(input, 1, "", "customer_group")
	require.NoError(t, err)
	if batch.CanApply {
		_, err = ApplyBillingCorrection(batch.ID, batch.SHA256, "", "customer_group", 1, false, "")
		require.NoError(t, err)
	}
	// Resume the deferred live settlement through the real wallet/task API.
	// Its idempotency checks compare Task.Quota, which corrections preserve;
	// it does not inspect BillingCorrectionClaim or the completed operation.
	applied, err := AdjustBillingTaskQuota(&task, actualQuota, modelName)
	require.NoError(t, err)
	require.True(t, applied)
	var user User
	require.NoError(t, DB.First(&user, userID).Error)
	assert.False(t, batch.CanApply, "a terminal status alone must not authorize repricing before live settlement completes")
	assert.Equal(t, walletAfterSubmission+sourceQuota-actualQuota, user.Quota, "refund must happen exactly once across correction and deferred token settlement")
}

func TestBillingModelPricingRejectsUnprovenTokenSettlementEvenWhenSavedRateMatchesPrecharge(t *testing.T) {
	// The saved ratio reproduces the precharge, but live polling reads the
	// current ratio. Changing that ratio makes a pending live settlement post
	// exactly the same additional charge as a simultaneous model correction.
	input, task := seedBillingCorrectionSettlementWindow(t, 250000)
	currentRatio := 5.0
	configureBillingCorrectionPricing(t, "doubao-seedance-2-5", nil, &currentRatio)
	batch, err := PreviewBillingCorrection(input, 1, "", "customer_group")
	require.NoError(t, err)
	if batch.CanApply {
		_, err = ApplyBillingCorrection(batch.ID, batch.SHA256, "", "customer_group", 1, false, "")
		require.NoError(t, err)
	}
	applied, err := AdjustBillingTaskQuota(&task, 825000, "doubao-seedance-2-5")
	require.NoError(t, err)
	require.True(t, applied)
	var user User
	require.NoError(t, DB.First(&user, input.UserID).Error)
	assert.False(t, batch.CanApply, "single-entry token tasks need durable proof of settlement completion")
	assert.Equal(t, "incomplete_task_range", batch.Rows[0].Blocked)
	assert.Equal(t, 835000, user.Quota, "the additional 165000 charge must be applied only by live settlement")
}
