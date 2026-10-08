package model

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBillingV9RequiresExcelBeforeArchivePublicationAndIssuing(t *testing.T) {
	id := seedBillingCustomer(t)
	body := `{"pdf_template_version":9}`
	hash := sha256.Sum256([]byte(body))
	statement := &BillingStatement{ID: "paired-archive", UserID: id, Month: "2020-02", Revision: 1, Status: StatementPreparing, LeaseOwner: "worker", Snapshot: body, SnapshotSHA256: hex.EncodeToString(hash[:]), PDFSHA256: "pdf", ManifestSHA256: "manifest"}
	require.NoError(t, DB.Create(statement).Error)
	assert.ErrorIs(t, CompleteBillingStatementArchive(statement, "worker", true), ErrBillingEvidenceIntegrity)
	statement.ExcelSHA256 = "excel"
	require.NoError(t, CompleteBillingStatementArchive(statement, "worker", true))
	stored, err := GetBillingStatement(statement.ID)
	require.NoError(t, err)
	assert.Equal(t, "excel", stored.ExcelSHA256)
	require.NoError(t, DB.Model(stored).Update("ExcelSHA256", "").Error)
	_, err = ChangeBillingStatement(statement.ID, "issue", "", "", "", 1, true)
	assert.ErrorIs(t, err, ErrBillingEvidenceIntegrity)
}
