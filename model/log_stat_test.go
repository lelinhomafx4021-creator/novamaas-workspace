package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSumLogStatisticsUsesNetRevenueAndConsistentFilters(t *testing.T) {
	truncateTables(t)
	now := time.Now().Unix()
	logs := []Log{
		{
			UserId: 2, CreatedAt: now, Type: LogTypeConsume, Username: "regular-user",
			Quota: 12345, PromptTokens: 100, CompletionTokens: 20,
			RequestId: "request-a", UpstreamRequestId: "upstream-a",
		},
		{
			UserId: 2, CreatedAt: now, Type: LogTypeRefund, Username: "regular-user",
			Quota: 2345, RequestId: "request-b", UpstreamRequestId: "upstream-b",
		},
	}
	require.NoError(t, LOG_DB.Create(&logs).Error)

	stat, err := SumLogStatistics(CostAccountingFilter{
		StartTimestamp: now - 60,
		EndTimestamp:   now + 60,
		Username:       "regular-user",
	})

	require.NoError(t, err)
	assert.Equal(t, int64(12345), stat.Quota)
	assert.Equal(t, int64(2345), stat.RefundQuota)
	assert.Equal(t, int64(10000), stat.RevenueQuota)
	assert.Equal(t, int64(2), stat.Records)
	assert.Equal(t, 1, stat.Rpm)
	assert.Equal(t, 120, stat.Tpm)

	consumeOnly, err := SumLogStatistics(CostAccountingFilter{
		Username:  "regular-user",
		RequestID: "request-a",
		LogType:   LogTypeConsume,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(12345), consumeOnly.Quota)
	assert.Zero(t, consumeOnly.RefundQuota)
	assert.Equal(t, int64(12345), consumeOnly.RevenueQuota)
	assert.Equal(t, int64(1), consumeOnly.Records)

	refundOnly, err := SumLogStatistics(CostAccountingFilter{
		Username:          "regular-user",
		UpstreamRequestID: "upstream-b",
		LogType:           LogTypeRefund,
	})
	require.NoError(t, err)
	assert.Zero(t, refundOnly.Quota)
	assert.Equal(t, int64(2345), refundOnly.RefundQuota)
	assert.Equal(t, int64(-2345), refundOnly.RevenueQuota)
	assert.Equal(t, int64(1), refundOnly.Records)
}
