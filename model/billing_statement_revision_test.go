package model

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBillingRegenerationRetainsConfirmedEvidenceAndOriginalLedgerRange(t *testing.T) {
	id := seedBillingCustomer(t)
	start, end, err := BillingMonthBounds("2020-02")
	require.NoError(t, err)
	entries := []BillingEntry{
		{EventKey: "frozen-source", UserID: id, Sequence: 1, PostedAt: start, Kind: "usage", ModelName: "original", Quota: 500},
		{EventKey: "later-entry", UserID: id, Sequence: 2, PostedAt: start + 1, Kind: "usage", ModelName: "later", Quota: 700},
	}
	require.NoError(t, DB.Create(&entries).Error)
	hash := sha256.Sum256([]byte(`{"pdf_template_version":8}`))
	key := "901:2020-02"
	source := &BillingStatement{ID: "confirmed-original", UserID: id, Month: "2020-02", Revision: 1, ActiveKey: &key, Status: StatementConfirmed,
		StartAt: start, EndAt: end, FromSequence: 1, ToSequence: 1, StorageProfileID: 7, ProfileVersion: 4,
		Snapshot: `{"pdf_template_version":8}`, SnapshotSHA256: hex.EncodeToString(hash[:]), ManifestSHA256: "original-manifest", PDFSHA256: "original-pdf", IssuedAt: 100, ConfirmedAt: 200, CreatedBy: 1, CreatedAt: 50}
	require.NoError(t, DB.Create(source).Error)
	artifact := &BillingArtifact{StatementID: source.ID, Kind: "pdf", SHA256: source.PDFSHA256, ObjectKey: "immutable-key"}
	require.NoError(t, DB.Create(artifact).Error)
	require.NoError(t, DB.Create(&BillingStatementEvent{StatementID: source.ID, Action: "confirm", ActorID: id, CreatedAt: 200, ManifestSHA256: source.ManifestSHA256}).Error)
	next, err := RegenerateBillingStatement(source, 1, func(locked *BillingStatement, totals []BillingModelTotal) (string, string, error) {
		assert.Equal(t, source.SnapshotSHA256, locked.SnapshotSHA256)
		require.Len(t, totals, 1)
		assert.Equal(t, "original", totals[0].ModelName)
		assert.Equal(t, int64(500), totals[0].Charge)
		return locked.Snapshot, locked.SnapshotSHA256, nil
	})
	require.NoError(t, err)
	assert.Equal(t, 2, next.Revision)
	assert.Equal(t, source.ID, next.SourceStatementID)
	assert.Equal(t, source.StartAt, next.StartAt)
	assert.Equal(t, source.EndAt, next.EndAt)
	assert.Equal(t, source.FromSequence, next.FromSequence)
	assert.Equal(t, source.ToSequence, next.ToSequence)
	assert.Equal(t, source.ProfileVersion, next.ProfileVersion)
	assert.Equal(t, source.StorageProfileID, next.StorageProfileID)
	assert.Equal(t, StatementPreparing, next.Status)
	assert.Zero(t, next.IssuedAt)
	assert.Zero(t, next.ConfirmedAt)
	assert.Empty(t, next.PDFSHA256)
	original, err := GetBillingStatement(source.ID)
	require.NoError(t, err)
	assert.Equal(t, StatementConfirmed, original.Status)
	assert.Equal(t, int64(200), original.ConfirmedAt)
	assert.Equal(t, source.Snapshot, original.Snapshot)
	assert.Equal(t, source.PDFSHA256, original.PDFSHA256)
	assert.Equal(t, next.ID, original.SupersededBy)
	assert.Nil(t, original.ActiveKey)
	require.NotNil(t, next.ActiveKey)
	assert.Equal(t, key, *next.ActiveKey)
	var oldArtifact BillingArtifact
	require.NoError(t, DB.First(&oldArtifact, artifact.ID).Error)
	assert.Equal(t, *artifact, oldArtifact)
	var events []BillingStatementEvent
	require.NoError(t, DB.Where("statement_id = ?", source.ID).Order("id asc").Find(&events).Error)
	require.Len(t, events, 2)
	assert.Equal(t, "confirm", events[0].Action)
	assert.Equal(t, "regenerate", events[1].Action)
	assert.Equal(t, next.ID, events[1].Note)
	_, err = ChangeBillingStatement(source.ID, "confirm", source.ManifestSHA256, "", "owner", id, false)
	assert.ErrorIs(t, err, ErrBillingConflict)
	_, err = RegenerateBillingStatement(source, 1, nil)
	assert.ErrorIs(t, err, ErrBillingConflict, "duplicate regeneration cannot fork another active revision")
}

func TestBillingRegenerationFailureLeavesOriginalActive(t *testing.T) {
	id := seedBillingCustomer(t)
	key := "901:2020-02"
	hash := sha256.Sum256([]byte("{}"))
	source := &BillingStatement{ID: "draft-source", UserID: id, Month: "2020-02", Revision: 1, ActiveKey: &key, Status: StatementDraft, Snapshot: "{}", SnapshotSHA256: hex.EncodeToString(hash[:]), FromSequence: 1}
	require.NoError(t, DB.Create(source).Error)
	_, err := RegenerateBillingStatement(source, 1, func(*BillingStatement, []BillingModelTotal) (string, string, error) { return "{}", "wrong-digest", nil })
	assert.ErrorIs(t, err, ErrBillingEvidenceIntegrity)
	original, err := GetBillingStatement(source.ID)
	require.NoError(t, err)
	assert.Empty(t, original.SupersededBy)
	require.NotNil(t, original.ActiveKey)
	assert.Equal(t, key, *original.ActiveKey)
	var count int64
	require.NoError(t, DB.Model(&BillingStatement{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

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
