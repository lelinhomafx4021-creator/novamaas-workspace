package service

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBillingPostpaidDailyBalancesPreservePrecisionAndSigns(t *testing.T) {
	for _, test := range []struct{ amount, balance string }{
		{"1.605898", "-1.605898"},
		{"12345.678900", "-12,345.678900"},
		{"0.000000", "0.000000"},
		{"-0.000000", "0.000000"},
		{"-1233.567890", "1,233.567890"},
		{"0.000001", "-0.000001"},
		{"1234567890.123456", "-1,234,567,890.123456"},
	} {
		t.Run(test.amount, func(t *testing.T) {
			balances, err := billingPostpaidDailyBalances([]BillingRow{{Amount: test.amount}})
			require.NoError(t, err)
			assert.Equal(t, []string{test.balance}, balances)
		})
	}
}

func TestBillingPostpaidDailyBalancesAccumulateConsumptionAndRefunds(t *testing.T) {
	days := []BillingRow{
		{State: "outside_period"},
		{Amount: "100.000000"},
		{Amount: "200.000000"},
		{Amount: "0.000000"},
		{Amount: "-50.000000"},
		{Amount: "0.000001"},
	}
	balances, err := billingPostpaidDailyBalances(days)
	require.NoError(t, err)
	assert.Equal(t, []string{"-", "-100.000000", "-300.000000", "-300.000000", "-250.000000", "-250.000001"}, balances)
	assert.Equal(t, "200.000000", days[2].Amount, "the daily source amount must not become a running total")
	_, err = billingPostpaidDailyBalances([]BillingRow{{Amount: "invalid"}})
	assert.Error(t, err)
}

func TestBillingPostpaidPDFPreservesFrozenAmountsAndOmitsDailyRecordCounts(t *testing.T) {
	statement, snapshot := frozenBillingExportFixture(t)
	snapshot.PDFTemplateVersion = 15
	statement.ExcelSHA256 = strings.Repeat("b", 64)
	body, err := common.Marshal(snapshot)
	require.NoError(t, err)
	hash := sha256.Sum256(body)
	statement.Snapshot, statement.SnapshotSHA256 = string(body), hex.EncodeToString(hash[:])
	pdf, err := RenderBillingStatementPDF(statement, snapshot, false)
	require.NoError(t, err)
	after, err := common.Marshal(snapshot)
	require.NoError(t, err)
	assert.Equal(t, body, after, "rendering postpaid balances must not change frozen billing amounts")
	snapshot.Days[28].Count++
	retry, err := RenderBillingStatementPDF(statement, snapshot, false)
	require.NoError(t, err)
	assert.Equal(t, pdf, retry, "daily record counts are not displayed")
}

func TestBillingPostpaidPDFFitsFullMonthAndLargeBalances(t *testing.T) {
	for _, test := range []struct {
		name           string
		charge, refund int64
	}{
		{"consumption", int64(common.MaxWalletQuota), 0},
		{"refund", 0, int64(common.MaxWalletQuota)},
	} {
		t.Run(test.name, func(t *testing.T) {
			statement, branding := frozenBillingExportFixture(t)
			start, end, err := model.BillingMonthBounds("2026-08")
			require.NoError(t, err)
			snapshot, err := buildBillingSnapshot(&model.BillingAccount{UserID: statement.UserID, CompanyTitle: branding.CompanyTitle, TaxID: branding.TaxID, AccountingStartAt: start}, "2026-08", []model.BillingHour{{Hour: start, Charge: test.charge, Refund: test.refund, Count: 1}}, branding.Currency)
			require.NoError(t, err)
			snapshot.PDFTemplateVersion = 15
			snapshot.Issuer, snapshot.PDFLogoPNG, snapshot.Username = branding.Issuer, branding.PDFLogoPNG, branding.Username
			statement.Month, statement.StartAt, statement.EndAt = snapshot.Month, start, end
			statement.ExcelSHA256 = strings.Repeat("b", 64)
			_, err = RenderBillingStatementPDF(statement, snapshot, false)
			require.NoError(t, err, "31 daily rows, opening row, totals and large signed balances must fit")
		})
	}
}
