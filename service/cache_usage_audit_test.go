package service

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testCacheAuditMoney = cacheAuditMoney{rate: decimal.NewFromInt(1), unit: decimal.NewFromInt(500000)}

const convertedMessages = `"request_path":"/v1/messages","request_conversion":["Claude Messages","OpenAI Compatible"]`

func auditRow(t *testing.T, log model.Log) CacheUsageAuditRow {
	t.Helper()
	if log.Type == 0 {
		log.Type = model.LogTypeConsume
	}
	row, _ := buildCacheUsageAuditRow(&log, testCacheAuditMoney, 0, nil)
	return row
}

// Mirrors the reported production request: 4921 total input, 4825 cached.
func TestCacheUsageAuditSeparatesDuplicatedCacheRead(t *testing.T) {
	row := auditRow(t, model.Log{PromptTokens: 4921, CompletionTokens: 86, Quota: 5043,
		Other: `{` + convertedMessages + `,"model_ratio":10,"completion_ratio":5,"group_ratio":0.5,"cache_ratio":0.1,"cache_tokens":4825}`})

	assert.Equal(t, CacheAuditAffected, row.Status)
	assert.Equal(t, CacheUsageTokens{TotalInput: 4921, Input: 96, CacheRead: 4825, Output: 86}, row.Tokens)
	assert.Equal(t, int64(4921), row.ReportedInputTokens)
	assert.Equal(t, int64(96), row.CorrectInputTokens)
	assert.Equal(t, int64(4825), row.DuplicateTokens)
	require.NotNil(t, row.ListCost)
	assert.Equal(t, CacheUsageCost{Input: "0.001920", CacheRead: "0.009650", CacheWrite: "0.000000", Output: "0.008600", Total: "0.020170"}, *row.ListCost)
	assert.Equal(t, "0.010085", row.DiscountedCost.Total)
	assert.Equal(t, "0.010086", row.ActualAmount)
	assert.False(t, row.PriceMismatch)
}

func TestCacheUsageAuditClassification(t *testing.T) {
	prices := `"model_ratio":1,"completion_ratio":1,"group_ratio":1,"cache_ratio":0.1,"cache_creation_ratio":1.25`
	cases := []struct {
		name, other, status, reason string
		createdAt                   int64
		duplicate, duplicateMax     int64
	}{
		{"native Claude upstream", `{"request_path":"/v1/messages","request_conversion":["Claude Messages"],"claude":true,"usage_semantic":"anthropic","cache_tokens":800,` + prices + `}`, CacheAuditNotAffected, "native_claude_upstream", 0, 0, 0},
		{"anthropic usage preserved", `{` + convertedMessages + `,"claude":true,"usage_semantic":"anthropic","cache_tokens":800,` + prices + `}`, CacheAuditNotAffected, "anthropic_usage_preserved", 0, 0, 0},
		{"OpenAI endpoint", `{"request_path":"/v1/chat/completions","request_conversion":["OpenAI Compatible"],"cache_tokens":800,` + prices + `}`, CacheAuditNotAffected, "not_messages_endpoint", 0, 0, 0},
		{"no cache", `{` + convertedMessages + `,` + prices + `}`, CacheAuditNotAffected, "no_cache", 0, 0, 0},
		{"cache write needs review", `{` + convertedMessages + `,"cache_tokens":800,"cache_creation_tokens":100,"cache_write_tokens":100,` + prices + `}`, CacheAuditReview, "cache_write_present", 0, 0, 900},
		{"old log without metadata", `{` + prices + `,"cache_tokens":800}`, CacheAuditNoEvidence, "missing_request_path", 0, 0, 0},
		{"path but no chain", `{"request_path":"/v1/messages","cache_tokens":800,` + prices + `}`, CacheAuditNoEvidence, "missing_conversion_chain", 0, 0, 0},
		{"stamped by fixed converter", `{` + convertedMessages + `,"claude_wire_usage_version":2,"cache_tokens":800,` + prices + `}`, CacheAuditNotAffected, "fixed_converter", 0, 0, 0},
		{"after configured fix time", `{` + convertedMessages + `,"cache_tokens":800,` + prices + `}`, CacheAuditNotAffected, "fixed_converter", 2000, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			log := model.Log{Type: model.LogTypeConsume, CreatedAt: tc.createdAt, PromptTokens: 1000, Other: tc.other}
			row, _ := buildCacheUsageAuditRow(&log, testCacheAuditMoney, 1500, nil)
			assert.Equal(t, tc.status, row.Status)
			assert.Equal(t, tc.reason, row.Reason)
			assert.Equal(t, tc.duplicate, row.DuplicateTokens)
			assert.Equal(t, tc.duplicateMax, row.DuplicateTokensMax)
		})
	}
}

func TestCacheUsageAuditTokenSemantics(t *testing.T) {
	// Anthropic semantics log uncached input in prompt_tokens.
	row := auditRow(t, model.Log{PromptTokens: 10, CompletionTokens: 5,
		Other: `{"claude":true,"usage_semantic":"anthropic","cache_tokens":100,"cache_creation_tokens":30,"cache_creation_tokens_5m":20,"cache_creation_tokens_1h":10,"cache_write_tokens":30,"model_ratio":1,"completion_ratio":5,"group_ratio":0,"cache_ratio":0.1,"cache_creation_ratio":1.25,"cache_creation_ratio_5m":1.25,"cache_creation_ratio_1h":2}`})
	assert.Equal(t, CacheUsageTokens{TotalInput: 140, Input: 10, CacheRead: 100, CacheWrite: 30, Output: 5}, row.Tokens)
	require.NotNil(t, row.ListCost)
	// 10 + 100*0.1 + 20*1.25 + 10*2 + 5*5 = 90 quota
	assert.Equal(t, "0.000180", row.ListCost.Total)
	assert.Equal(t, "0.000000", row.DiscountedCost.Total)

	// OpenAI semantics: cache write is part of prompt_tokens and must not go negative.
	row = auditRow(t, model.Log{PromptTokens: 50,
		Other: `{"cache_tokens":40,"cache_write_tokens":30,"model_ratio":1,"completion_ratio":1,"group_ratio":1,"cache_ratio":0.1,"cache_creation_ratio":1.25}`})
	assert.Equal(t, int64(0), row.Tokens.Input)
	assert.Equal(t, int64(50), row.Tokens.TotalInput)
}

func TestCacheUsageAuditKeepsActualChargeWhenPriceCannotBeRebuilt(t *testing.T) {
	for issue, other := range map[string]string{
		"missing_price":         `{` + convertedMessages + `,"cache_tokens":800}`,
		"missing_cache_price":   `{` + convertedMessages + `,"cache_tokens":800,"model_ratio":1,"completion_ratio":1,"group_ratio":1}`,
		"tiered_billing":        `{` + convertedMessages + `,"cache_tokens":800,"billing_mode":"tiered_expr","model_ratio":1,"completion_ratio":1,"group_ratio":1,"cache_ratio":0.1}`,
		"per_call_price":        `{` + convertedMessages + `,"cache_tokens":800,"model_price":0.02,"model_ratio":1,"completion_ratio":1,"group_ratio":1,"cache_ratio":0.1}`,
		"unsupported_surcharge": `{` + convertedMessages + `,"cache_tokens":800,"tool_surcharges":[{"name":"web_search"}],"model_ratio":1,"completion_ratio":1,"group_ratio":1,"cache_ratio":0.1}`,
	} {
		row := auditRow(t, model.Log{PromptTokens: 1000, Quota: 250000, Other: other})
		assert.Equal(t, issue, row.PriceIssue, issue)
		assert.Nil(t, row.ListCost, issue)
		assert.Equal(t, "0.500000", row.ActualAmount, issue)
		// Token evidence does not depend on prices.
		assert.Equal(t, CacheAuditAffected, row.Status, issue)
		assert.Equal(t, int64(800), row.DuplicateTokens, issue)
	}
	row := auditRow(t, model.Log{PromptTokens: 1000, Quota: 1, Other: `not json`})
	assert.Equal(t, CacheAuditNoEvidence, row.Status)
	assert.Equal(t, "invalid_metadata", row.Reason)
}

func TestCacheUsageAuditFlagsPriceMismatch(t *testing.T) {
	row := auditRow(t, model.Log{PromptTokens: 1000, Quota: 900,
		Other: `{` + convertedMessages + `,"model_ratio":1,"completion_ratio":1,"group_ratio":1}`})
	assert.True(t, row.PriceMismatch)
}

func TestRunCacheUsageAuditAggregatesAndFilters(t *testing.T) {
	truncate(t)
	t.Cleanup(func() { model.LOG_DB.Exec("DELETE FROM logs") })
	prices := `"model_ratio":10,"completion_ratio":5,"group_ratio":0.5,"cache_ratio":0.1`
	day := int64(1_760_000_000)
	logs := []model.Log{
		{UserId: 7, TokenId: 3, TokenName: "prod", ModelName: "gpt-5", CreatedAt: day, Type: model.LogTypeConsume, PromptTokens: 4921, CompletionTokens: 86, Quota: 5043, Other: `{` + convertedMessages + `,"cache_tokens":4825,` + prices + `}`},
		{UserId: 7, TokenId: 3, TokenName: "prod", ModelName: "gpt-5", CreatedAt: day + 60, Type: model.LogTypeConsume, PromptTokens: 1000, CompletionTokens: 10, Quota: 5250, Other: `{"request_path":"/v1/chat/completions","request_conversion":["OpenAI Compatible"],"cache_tokens":500,` + prices + `}`},
		{UserId: 7, TokenId: 4, TokenName: "test", ModelName: "gpt-5-mini", CreatedAt: day + 120, Type: model.LogTypeConsume, PromptTokens: 200, Quota: 100, Other: `{"cache_tokens":100,` + prices + `}`},
		{UserId: 7, TokenId: 3, TokenName: "prod", ModelName: "gpt-5", CreatedAt: day + 180, Type: model.LogTypeRefund, Quota: 43},
		{UserId: 8, TokenId: 9, ModelName: "gpt-5", CreatedAt: day, Type: model.LogTypeConsume, PromptTokens: 9999, Quota: 1, Other: `{` + convertedMessages + `,"cache_tokens":9000,` + prices + `}`},
		{UserId: 7, TokenId: 3, ModelName: "gpt-5", CreatedAt: day + 86400*2, Type: model.LogTypeConsume, PromptTokens: 9999, Quota: 1, Other: `{` + convertedMessages + `,"cache_tokens":9000}`},
	}
	require.NoError(t, model.LOG_DB.Create(&logs).Error)

	req := CacheUsageAuditRequest{UserID: 7, StartAt: day, EndAt: day + 86400}
	report, err := RunCacheUsageAudit(context.Background(), req, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(3), report.Total.Requests)
	assert.Equal(t, int64(1), report.Total.AffectedRequests)
	assert.Equal(t, int64(1), report.Total.NoEvidenceRequests)
	assert.Equal(t, int64(4825), report.Total.DuplicateTokens)
	assert.Equal(t, CacheUsageTokens{TotalInput: 6121, Input: 696, CacheRead: 5425, Output: 96}, report.Total.Tokens)
	assert.Equal(t, int64(10350), report.Total.actual.IntPart()) // 5043 + 5250 + 100 - 43 refund
	assert.Len(t, report.Rows, 4)
	require.Len(t, report.ByToken, 2)
	assert.Equal(t, "prod (#3)", report.ByToken[0].Key)
	assert.Equal(t, int64(4825), report.ByToken[0].DuplicateTokens)
	require.Len(t, report.ByModel, 2)
	require.Len(t, report.ByDay, 1)

	req.Status, req.TokenID = CacheAuditAffected, 3
	var exported []CacheUsageAuditRow
	report, err = RunCacheUsageAudit(context.Background(), req, func(row CacheUsageAuditRow) error {
		exported = append(exported, row)
		return nil
	})
	require.NoError(t, err)
	require.Len(t, exported, 1)
	assert.Equal(t, int64(4825), exported[0].DuplicateTokens)
	assert.Equal(t, report.Rows, exported)
	assert.Equal(t, int64(2), report.Total.Requests) // totals ignore the status filter

	_, err = RunCacheUsageAudit(context.Background(), CacheUsageAuditRequest{StartAt: day, EndAt: day + 94*86400}, nil)
	assert.ErrorIs(t, err, ErrCacheUsageAuditPeriod)
}

func TestConvertsToClaudeResponseStampsOnlyConvertedMessages(t *testing.T) {
	for _, tc := range []struct {
		chain []types.RelayFormat
		want  bool
	}{
		{[]types.RelayFormat{types.RelayFormatClaude, types.RelayFormatOpenAI}, true},
		{[]types.RelayFormat{types.RelayFormatClaude, types.RelayFormatOpenAIResponses}, true},
		{[]types.RelayFormat{types.RelayFormatClaude}, false},
		{[]types.RelayFormat{types.RelayFormatOpenAI, types.RelayFormatClaude}, false},
		{nil, false},
	} {
		info := &relaycommon.RelayInfo{RequestConversionChain: tc.chain}
		assert.Equal(t, tc.want, convertsToClaudeResponse(info), "%v", tc.chain)
	}
}

func TestCacheUsageAuditCompensationPricesDuplicatedInputOnly(t *testing.T) {
	prices := map[string]decimal.Decimal{"gpt-5": decimal.NewFromInt(3)}
	build := func(other string) CacheUsageAuditRow {
		log := model.Log{Type: model.LogTypeConsume, ModelName: "gpt-5", PromptTokens: 4921, CompletionTokens: 86, Other: other}
		row, _ := buildCacheUsageAuditRow(&log, testCacheAuditMoney, 0, prices)
		return row
	}
	ratios := `"model_ratio":10,"completion_ratio":5,"group_ratio":0.5,"cache_ratio":0.1`
	row := build(`{` + convertedMessages + `,"cache_tokens":4825,` + ratios + `}`)
	require.NotNil(t, row.Compensation)
	// 4825 tokens x ratio 10 = 48250 quota = $0.0965 at the base input price ($20/M).
	assert.Equal(t, "20.000000", row.InputPricePerMillion)
	assert.Equal(t, CacheUsageCompensation{List: "0.096500", Group: "0.048250", Downstream: "0.014475",
		ListMax: "0.096500", GroupMax: "0.048250", DownstreamMax: "0.014475"}, *row.Compensation)

	// Review rows: nothing confirmed, compensation upper bound covers read + write.
	row = build(`{` + convertedMessages + `,"cache_tokens":4000,"cache_write_tokens":825,"cache_creation_ratio":1.25,` + ratios + `}`)
	assert.Equal(t, CacheAuditReview, row.Status)
	assert.Equal(t, "0.000000", row.Compensation.List)
	assert.Equal(t, "0.096500", row.Compensation.ListMax)

	// Tiered rows cannot use the base input ratio but still get the downstream estimate.
	row = build(`{` + convertedMessages + `,"cache_tokens":4825,"billing_mode":"tiered_expr",` + ratios + `}`)
	assert.Empty(t, row.Compensation.List)
	assert.Equal(t, "0.014475", row.Compensation.Downstream)

	// Estimated usage is not evidence of a duplicate.
	row = build(`{` + convertedMessages + `,"cache_tokens":4825,"admin_info":{"usage_billing_path":"billing-usage-openai-estimated"},` + ratios + `}`)
	assert.Equal(t, CacheAuditNoEvidence, row.Status)
	assert.Equal(t, "estimated_usage", row.Reason)
	assert.Nil(t, row.Compensation)
}

func TestRunCacheUsageAuditTotalsCompensationAndUsers(t *testing.T) {
	truncate(t)
	t.Cleanup(func() { model.LOG_DB.Exec("DELETE FROM logs") })
	day := int64(1_760_000_000)
	ratios := `"model_ratio":10,"completion_ratio":5,"group_ratio":0.5,"cache_ratio":0.1`
	require.NoError(t, model.LOG_DB.Create(&[]model.Log{
		{UserId: 7, Username: "acme", ModelName: "gpt-5", CreatedAt: day, Type: model.LogTypeConsume, PromptTokens: 4921, Quota: 1, Other: `{` + convertedMessages + `,"cache_tokens":4825,` + ratios + `}`},
		{UserId: 7, Username: "acme", ModelName: "gpt-5", CreatedAt: day + 1, Type: model.LogTypeConsume, PromptTokens: 1175, Quota: 1, Other: `{` + convertedMessages + `,"cache_tokens":1175,` + ratios + `}`},
		{UserId: 7, Username: "acme", ModelName: "other", CreatedAt: day + 2, Type: model.LogTypeConsume, PromptTokens: 600, Quota: 1, Other: `{` + convertedMessages + `,"cache_tokens":500}`},
	}).Error)
	report, err := RunCacheUsageAudit(context.Background(), CacheUsageAuditRequest{StartAt: day, EndAt: day + 60,
		DownstreamInputPrices: map[string]string{"gpt-5": "3"}}, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(6500), report.Total.DuplicateTokens)
	assert.Equal(t, int64(6696), report.Total.AffectedReportedInput)
	assert.Equal(t, int64(196), report.Total.AffectedCorrectInput)
	require.Len(t, report.ByModel, 2)
	assert.Equal(t, "gpt-5", report.ByModel[0].Key)
	assert.Equal(t, "20.000000", report.ByModel[0].InputPricePerMillion)
	assert.Equal(t, "2.000000", report.ByModel[0].CacheReadPricePerMillion)
	assert.False(t, report.ByModel[0].PriceMixed)
	// gpt-5: input 96+0, cache read 6000 x 0.1, output 0, model ratio 10.
	assert.Equal(t, CacheUsageCost{Input: "0.001920", CacheRead: "0.012000", CacheWrite: "0.000000", Output: "0.000000", Total: "0.013920"}, report.ByModel[0].ListCost)
	assert.Equal(t, "0.006960", report.ByModel[0].DiscountedCost.Total)
	assert.Equal(t, []string{" × 0.5"}, report.ByModel[0].Rates)
	assert.Equal(t, int64(6000), report.ByModel[0].DuplicateTokens)
	assert.Equal(t, "0.120000", report.Total.Compensation.List) // 6000 x $20/M
	assert.Equal(t, "0.060000", report.Total.Compensation.Group)
	assert.Equal(t, "0.018000", report.Total.Compensation.Downstream)
	assert.Equal(t, int64(500), report.Total.UnpricedDuplicateTokens)
	require.Len(t, report.ByUser, 1)
	assert.Equal(t, "acme (#7)", report.ByUser[0].Key)

	_, err = RunCacheUsageAudit(context.Background(), CacheUsageAuditRequest{StartAt: day, EndAt: day + 60,
		DownstreamInputPrices: map[string]string{"gpt-5": "-1"}}, nil)
	assert.Error(t, err)
}

func TestCacheUsageAuditProtocol(t *testing.T) {
	for _, tc := range []struct {
		path  string
		chain []string
		want  string
	}{
		{"/v1/messages", nil, CacheAuditProtocolMessages},
		{"/v1/chat/completions", nil, CacheAuditProtocolChat},
		{"/v1/responses", nil, CacheAuditProtocolResponses},
		{"", []string{"Claude Messages", "OpenAI Compatible"}, CacheAuditProtocolMessages},
		{"", nil, CacheAuditProtocolOther},
		{"/v1/embeddings", nil, CacheAuditProtocolOther},
	} {
		assert.Equal(t, tc.want, cacheAuditProtocol(tc.path, tc.chain), tc.path)
	}
}

func TestRunCacheUsageAuditFiltersWholeReportByProtocol(t *testing.T) {
	truncate(t)
	t.Cleanup(func() { model.LOG_DB.Exec("DELETE FROM logs") })
	day := int64(1_760_000_000)
	ratios := `"model_ratio":1,"completion_ratio":1,"group_ratio":1,"cache_ratio":0.1`
	require.NoError(t, model.LOG_DB.Create(&[]model.Log{
		{UserId: 7, ModelName: "a", CreatedAt: day, Type: model.LogTypeConsume, PromptTokens: 1000, Quota: 1, Other: `{` + convertedMessages + `,"cache_tokens":800,` + ratios + `}`},
		{UserId: 7, ModelName: "b", CreatedAt: day, Type: model.LogTypeConsume, PromptTokens: 500, Quota: 1, Other: `{"request_path":"/v1/chat/completions","request_conversion":["OpenAI Compatible"],"cache_tokens":100,` + ratios + `}`},
		// Estimated usage outside /v1/messages is simply not affected.
		{UserId: 7, ModelName: "b", CreatedAt: day, Type: model.LogTypeConsume, PromptTokens: 500, Quota: 1, Other: `{"request_path":"/v1/chat/completions","request_conversion":["OpenAI Compatible"],"admin_info":{"local_count_tokens":true},` + ratios + `}`},
	}).Error)
	req := CacheUsageAuditRequest{StartAt: day, EndAt: day + 60, Protocol: CacheAuditProtocolMessages}
	report, err := RunCacheUsageAudit(context.Background(), req, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(1), report.Total.Requests)
	assert.Equal(t, int64(800), report.Total.DuplicateTokens)
	require.Len(t, report.ByModel, 1)
	assert.Equal(t, "a", report.ByModel[0].Key)

	req.Protocol = CacheAuditProtocolChat
	report, err = RunCacheUsageAudit(context.Background(), req, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(2), report.Total.Requests)
	assert.Zero(t, report.Total.NoEvidenceRequests)
	assert.Zero(t, report.Total.DuplicateTokens)

	req.Protocol = "grpc"
	_, err = RunCacheUsageAudit(context.Background(), req, nil)
	assert.Error(t, err)
}
