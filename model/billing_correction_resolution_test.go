package model

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBillingModelPricingSeedanceUsesActualOutputResolutionWithCompleteInput(t *testing.T) {
	for _, tc := range []struct {
		name, request, response, resolution, requested, source string
		video                                                  bool
		final                                                  int64
	}{
		{"omitted_request_no_video", `{"content":[{"type":"text"}]}`, `"720p"`, "720p", "", "upstream_response", false, 33000},
		{"omitted_generic_request_with_video", `{"metadata":{"content":[{"type":"video_url"}]}}`, `"720p"`, "720p", "", "upstream_response", true, 19800},
		{"actual_output_takes_priority", `{"resolution":"1080p","content":[{"type":"text"}]}`, `"720p"`, "720p", "1080p", "upstream_response", false, 33000},
		{"480p_to_720p_same_price", `{"resolution":"480p","content":[{"type":"text"}]}`, `"720p"`, "720p", "480p", "upstream_response", false, 33000},
		{"actual_1080p_video", `{"content":[{"type":"video_url"}]}`, `"1080p"`, "1080p", "", "upstream_response", true, 21685},
		{"archived_request_fallback", `{"resolution":"1080p","content":[{"type":"text"}]}`, "", "1080p", "1080p", "archived_request", false, 36300},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := seedBillingCorrectionSeedance(t, "doubao-seedance-2-5", "720p", false, true)
			ratio := 5.0 // Existing operator rate: 5 * 2 USD/M * 7 CNY/USD = 70 CNY/M.
			configureBillingCorrectionPricing(t, "doubao-seedance-2-5", nil, &ratio)
			var task Task
			require.NoError(t, DB.First(&task, "task_id = ?", "seedance-task").Error)
			task.Properties.RequestBody = nil
			data := map[string]any{"usage": map[string]any{"total_tokens": 10000}}
			if tc.response != "" {
				data["resolution"] = json.RawMessage(tc.response)
			}
			task.SetData(data)
			require.NoError(t, DB.Model(&task).Updates(map[string]any{"properties": task.Properties, "data": task.Data}).Error)
			require.NoError(t, SaveTaskRequestSnapshots(task.TaskID, "", []byte(tc.request), nil))
			batch, err := PreviewBillingCorrection(input, 1, "", "customer_group")
			require.NoError(t, err)
			require.True(t, batch.CanApply)
			require.Len(t, batch.Rows, 2)
			assert.Equal(t, tc.final, batch.Rows[0].CorrectedQuota+batch.Rows[1].CorrectedQuota)
			assert.Equal(t, tc.final-26400, batch.NetDelta)
			var frozen billingCorrectionPricing
			require.NoError(t, common.UnmarshalJsonStr(batch.Rows[0].TargetPricing, &frozen))
			assert.Equal(t, tc.resolution, frozen.Resolution)
			assert.Equal(t, tc.requested, frozen.RequestedResolution)
			assert.Equal(t, tc.source, frozen.ResolutionSource)
			require.NotNil(t, frozen.HasVideo)
			assert.Equal(t, tc.video, *frozen.HasVideo)
			if tc.requested == "" {
				assert.NotContains(t, batch.Rows[0].TargetPricing, "requested_resolution")
			}
			quota, err := GetUserQuota(901, true)
			require.NoError(t, err)
			assert.Equal(t, 1000000, quota, "preview must not change the wallet")
		})
	}
}

func TestBillingModelPricingSeedanceRejectsIncompleteOrInvalidResolutionEvidence(t *testing.T) {
	for _, tc := range []struct{ name, request, response string }{
		{"missing_content", `{}`, `"720p"`},
		{"empty_content", `{"content":[]}`, `"720p"`},
		{"unknown_content", `{"content":[{}]}`, `"720p"`},
		{"missing_both_resolutions", `{"content":[{"type":"text"}]}`, ""},
		{"unknown_response", `{"resolution":"720p","content":[{"type":"text"}]}`, `"1440p"`},
		{"unsupported_response", `{"resolution":"720p","content":[{"type":"text"}]}`, `"4k"`},
		{"empty_response", `{"resolution":"720p","content":[{"type":"text"}]}`, `""`},
		{"null_response", `{"resolution":"720p","content":[{"type":"text"}]}`, `null`},
		{"numeric_response", `{"resolution":"720p","content":[{"type":"text"}]}`, `720`},
		{"object_response", `{"resolution":"720p","content":[{"type":"text"}]}`, `{}`},
		{"unsupported_request_with_valid_output", `{"resolution":"4k","content":[{"type":"text"}]}`, `"720p"`},
		{"numeric_request_with_valid_output", `{"resolution":720,"content":[{"type":"text"}]}`, `"720p"`},
		{"null_request_with_valid_output", `{"resolution":null,"content":[{"type":"text"}]}`, `"720p"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := seedBillingCorrectionSeedance(t, "doubao-seedance-2-5", "720p", false, true)
			ratio := 5.0
			configureBillingCorrectionPricing(t, "doubao-seedance-2-5", nil, &ratio)
			var task Task
			require.NoError(t, DB.First(&task, "task_id = ?", "seedance-task").Error)
			task.Properties.RequestBody = []byte(tc.request)
			data := map[string]any{"usage": map[string]any{"total_tokens": 10000}}
			if tc.response != "" {
				data["resolution"] = json.RawMessage(tc.response)
			}
			task.SetData(data)
			require.NoError(t, DB.Model(&task).Updates(map[string]any{"properties": task.Properties, "data": task.Data}).Error)
			batch, err := PreviewBillingCorrection(input, 1, "", "customer_group")
			require.NoError(t, err)
			assert.False(t, batch.CanApply)
			for _, row := range batch.Rows {
				assert.NotEmpty(t, row.Blocked)
			}
			_, err = ApplyBillingCorrection(batch.ID, batch.SHA256, "", "customer_group", 1, false, "")
			assert.ErrorIs(t, err, ErrBillingCorrectionBlocked)
		})
	}
}

func TestBillingModelPricingSeedanceOutputResolutionIsFrozenThroughApplyAndReverse(t *testing.T) {
	input := seedBillingCorrectionSeedance(t, "doubao-seedance-2-5", "720p", true, true)
	ratio := 5.0
	configureBillingCorrectionPricing(t, "doubao-seedance-2-5", nil, &ratio)
	var task Task
	require.NoError(t, DB.First(&task, "task_id = ?", "seedance-task").Error)
	task.Properties.RequestBody = []byte(`{"content":[{"type":"video_url"}]}`)
	task.SetData(map[string]any{"resolution": "720p", "usage": map[string]any{"total_tokens": 10000}})
	require.NoError(t, DB.Model(&task).Updates(map[string]any{"properties": task.Properties, "data": task.Data}).Error)
	first, err := PreviewBillingCorrection(input, 1, "", "customer_group")
	require.NoError(t, err)
	require.True(t, first.CanApply)
	assert.Equal(t, int64(-6600), first.NetDelta)
	task.SetData(map[string]any{"resolution": "1080p", "usage": map[string]any{"total_tokens": 10000}})
	require.NoError(t, DB.Model(&task).Update("data", task.Data).Error)
	_, err = ApplyBillingCorrection(first.ID, first.SHA256, "", "customer_group", 1, false, "")
	assert.ErrorIs(t, err, ErrBillingConflict, "changed upstream evidence invalidates the frozen preview")
	second, err := PreviewBillingCorrection(input, 1, "", "customer_group")
	require.NoError(t, err)
	require.True(t, second.CanApply)
	assert.Equal(t, int64(-4715), second.NetDelta)
	_, err = ApplyBillingCorrection(second.ID, second.SHA256, "", "customer_group", 1, false, "")
	require.NoError(t, err)
	var user User
	require.NoError(t, DB.First(&user, 901).Error)
	assert.Equal(t, 1004715, user.Quota)
	assert.Equal(t, 21685, user.UsedQuota)
	_, err = ApplyBillingCorrection(second.ID, second.SHA256, "", "", 1, true, "Reverse repricing with output evidence")
	require.NoError(t, err)
	require.NoError(t, DB.First(&user, 901).Error)
	assert.Equal(t, 1000000, user.Quota)
	assert.Equal(t, 26400, user.UsedQuota)
}

func TestBillingCorrectionIgnoresUnrelatedProviderResolutionMetadata(t *testing.T) {
	for _, mode := range []string{BillingCorrectionGroupRate, BillingCorrectionModelPricing} {
		for _, resolution := range []string{`720`, `{}`} {
			t.Run(mode+resolution, func(t *testing.T) {
				input := seedBillingCorrection(t)
				input.Mode = mode
				if mode == BillingCorrectionModelPricing {
					input.TargetGroup = ""
					price := .3
					configureBillingCorrectionPricing(t, "wan-prime", &price, nil)
				}
				data, err := common.Marshal(map[string]any{"resolution": json.RawMessage(resolution)})
				require.NoError(t, err)
				require.NoError(t, DB.Model(&Task{}).Where("user_id = ?", 901).Update("data", data).Error)
				batch, err := PreviewBillingCorrection(input, 1, "0.77", "customer_group")
				require.NoError(t, err)
				assert.True(t, batch.CanApply, "non-Seedance resolution must not affect its existing billing contract")
			})
		}
	}
}
