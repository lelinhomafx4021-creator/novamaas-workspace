package service

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBillingReadableVerificationPreservesFrozenOriginalAndCorporateIdentity(t *testing.T) {
	for _, test := range []struct{ name, company, taxID string }{
		{"standard", "上海示例科技有限公司", "DEMO-TAX"},
		{"maximum identity", strings.Repeat("企", 200), strings.Repeat("X", 64)},
	} {
		t.Run(test.name, func(t *testing.T) {
			statement, snapshot := frozenBillingExportFixture(t)
			snapshot.PDFTemplateVersion = 17
			snapshot.CompanyTitle, snapshot.TaxID = test.company, test.taxID
			frozen, err := common.Marshal(snapshot)
			require.NoError(t, err)
			snapshotHash := sha256.Sum256(frozen)
			statement.Snapshot, statement.SnapshotSHA256 = string(frozen), hex.EncodeToString(snapshotHash[:])
			workbook, err := RenderBillingStatementExcel(statement, snapshot)
			require.NoError(t, err)
			excelHash := sha256.Sum256(workbook)
			statement.ExcelSHA256 = hex.EncodeToString(excelHash[:])
			original, err := RenderBillingStatementPDF(statement, snapshot, false)
			require.NoError(t, err, "accepted corporate identities and complete fingerprints must fit")
			pdfHash := sha256.Sum256(original)
			statement.PDFSHA256 = hex.EncodeToString(pdfHash[:])
			manifest, err := common.Marshal(BillingManifest{StatementID: statement.ID, PDFSHA256: statement.PDFSHA256, ExcelSHA256: statement.ExcelSHA256, SnapshotSHA256: statement.SnapshotSHA256})
			require.NoError(t, err)
			manifestHash := sha256.Sum256(manifest)
			statement.ManifestSHA256 = hex.EncodeToString(manifestHash[:])
			statement.ConfirmedAt = statement.CreatedAt + 3600
			_, err = RenderBillingStatementPDF(statement, snapshot, true)
			require.NoError(t, err, "the receipt must retain all four fingerprints")
			retry, err := RenderBillingStatementPDF(statement, snapshot, false)
			require.NoError(t, err)
			assert.Equal(t, original, retry, "confirmation must not change the original PDF or its fingerprint")
			after, err := common.Marshal(snapshot)
			require.NoError(t, err)
			assert.Equal(t, frozen, after)
			require.NoError(t, statement.VerifySnapshot())
		})
	}
}
