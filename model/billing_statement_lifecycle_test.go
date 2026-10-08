package model

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBillingStatementVoidThenCreatePreservesEvidenceAndAllowsOnlyOneEffectiveVersion(t *testing.T) {
	id := seedBillingCustomer(t)
	start, end, err := BillingMonthBounds("2020-02")
	require.NoError(t, err)
	require.NoError(t, DB.Model(&BillingAccount{}).Where("user_id = ?", id).Updates(map[string]any{"accounting_start_at": start, "sequence": 1}).Error)
	require.NoError(t, DB.Create(&BillingEntry{EventKey: "latest-source", UserID: id, Sequence: 1, PostedAt: start, Kind: "history_usage", Quota: 66}).Error)
	require.NoError(t, DB.Create(&BillingHour{UserID: id, Hour: start, Charge: 66, Count: 1, FirstSequence: 1, LastSequence: 1}).Error)
	originalImport := BillingHistoryImport{ID: "permanent-import", UserID: id, Month: "2020-02", SourceSHA256: "source-digest", Records: 1, FromSequence: 1, ToSequence: 1}
	require.NoError(t, DB.Create(&originalImport).Error)
	body, digest := correctedStatementTestSnapshot(t)
	key := "901:2020-02"
	original := &BillingStatement{ID: "confirmed-old", UserID: id, Month: "2020-02", Revision: 1, ActiveKey: &key, Status: StatementConfirmed, Snapshot: body, SnapshotSHA256: digest, PDFSHA256: "old-pdf", ExcelSHA256: "old-excel", ManifestSHA256: "old-manifest", ConfirmedAt: 200, IssuedAt: 100, StartAt: start, EndAt: end, FromSequence: 1, ToSequence: 1}
	require.NoError(t, DB.Create(original).Error)
	artifact := BillingArtifact{StatementID: original.ID, Kind: "pdf", ObjectKey: "immutable-pdf", SHA256: "old-pdf"}
	require.NoError(t, DB.Create(&artifact).Error)
	require.NoError(t, DB.Create(&BillingStatementEvent{StatementID: original.ID, Action: "confirm", ActorID: id, SessionID: "original-session", CreatedAt: 200, ManifestSHA256: original.ManifestSHA256}).Error)
	builder := func(_ *BillingAccount, hours []BillingHour, totals []BillingModelTotal) (string, string, error) {
		require.Len(t, hours, 1)
		assert.Equal(t, int64(66), hours[0].Charge)
		return body, digest, nil
	}
	next := &BillingStatement{ID: "next", UserID: id, Month: "2020-02", CreatedBy: 1}
	assert.ErrorContains(t, CreateBillingStatement(next, builder, true), "active statement")
	for _, test := range []struct {
		actor  int
		admin  bool
		reason string
	}{{id, false, "replace"}, {1, true, ""}, {1, true, "  "}} {
		_, err = ChangeBillingStatement(original.ID, "void", "", test.reason, "", test.actor, test.admin)
		assert.ErrorIs(t, err, ErrBillingConflict)
	}
	voided, err := ChangeBillingStatement(original.ID, "void", "", "Correct billing rate", "", 1, true)
	require.NoError(t, err)
	assert.Equal(t, StatementVoid, voided.Status)
	assert.Nil(t, voided.ActiveKey)
	assert.Equal(t, original.ConfirmedAt, voided.ConfirmedAt)
	assert.Equal(t, original.IssuedAt, voided.IssuedAt)
	assert.Equal(t, original.Snapshot, voided.Snapshot)
	assert.Equal(t, original.SnapshotSHA256, voided.SnapshotSHA256)
	assert.Equal(t, original.PDFSHA256, voided.PDFSHA256)
	assert.Equal(t, original.ExcelSHA256, voided.ExcelSHA256)
	assert.Equal(t, original.ManifestSHA256, voided.ManifestSHA256)
	_, err = ChangeBillingStatement(original.ID, "void", "", "duplicate", "", 1, true)
	assert.ErrorIs(t, err, ErrBillingConflict)
	_, err = ChangeBillingStatement(original.ID, "confirm", original.ManifestSHA256, "", "original-session", id, false)
	assert.ErrorIs(t, err, ErrBillingConflict)
	require.NoError(t, CreateBillingStatement(next, builder, true))
	assert.Equal(t, 2, next.Revision)
	assert.Equal(t, StatementPreparing, next.Status)
	assert.Zero(t, next.ConfirmedAt)
	assert.Zero(t, next.IssuedAt)
	assert.Empty(t, next.ManifestSHA256)
	assert.ErrorContains(t, CreateBillingStatement(&BillingStatement{ID: "duplicate", UserID: id, Month: "2020-02"}, builder, true), "active statement")
	state, err := GetBillingPreparationState(context.Background(), id, start, end)
	require.NoError(t, err)
	assert.Equal(t, next.ID, state.ExistingStatement)
	history, err := GetBillingHistoryState(context.Background(), id, "2020-02")
	require.NoError(t, err)
	assert.Equal(t, next.ID, history.ActiveStatement)
	var preservedImport BillingHistoryImport
	require.NoError(t, DB.First(&preservedImport, "id = ?", originalImport.ID).Error)
	assert.Equal(t, originalImport, preservedImport)
	var preservedArtifact BillingArtifact
	require.NoError(t, DB.First(&preservedArtifact, artifact.ID).Error)
	assert.Equal(t, artifact, preservedArtifact)
	var events []BillingStatementEvent
	require.NoError(t, DB.Where("statement_id = ?", original.ID).Order("id").Find(&events).Error)
	require.Len(t, events, 2)
	assert.Equal(t, "confirm", events[0].Action)
	assert.Equal(t, "original-session", events[0].SessionID)
	assert.Equal(t, "void", events[1].Action)
	assert.Equal(t, "Correct billing rate", events[1].Note)
	var user User
	require.NoError(t, DB.First(&user, id).Error)
	assert.Equal(t, 10000, user.Quota)
}

func TestBillingLegacySupersededDraftDoesNotBlockAfterReplacementVoided(t *testing.T) {
	id := seedBillingCustomer(t)
	start, end, err := BillingMonthBounds("2020-02")
	require.NoError(t, err)
	require.NoError(t, DB.Model(&BillingAccount{}).Where("user_id = ?", id).Update("accounting_start_at", start).Error)
	body := "{}"
	hash := sha256.Sum256([]byte(body))
	digest := hex.EncodeToString(hash[:])
	require.NoError(t, DB.Create(&[]BillingStatement{
		{ID: "legacy-v1", UserID: id, Month: "2020-02", Revision: 1, Status: StatementDraft, SupersededBy: "legacy-v2", Snapshot: body, SnapshotSHA256: digest},
		{ID: "legacy-v2", UserID: id, Month: "2020-02", Revision: 2, Status: StatementVoid, Snapshot: body, SnapshotSHA256: digest},
	}).Error)
	state, err := GetBillingPreparationState(context.Background(), id, start, end)
	require.NoError(t, err)
	assert.Empty(t, state.ExistingStatement)
	history, err := GetBillingHistoryState(context.Background(), id, "2020-02")
	require.NoError(t, err)
	assert.Empty(t, history.ActiveStatement)
	builder := func(*BillingAccount, []BillingHour, []BillingModelTotal) (string, string, error) {
		return body, digest, nil
	}
	next := &BillingStatement{ID: "v3", UserID: id, Month: "2020-02"}
	require.NoError(t, CreateBillingStatement(next, builder))
	assert.Equal(t, 3, next.Revision)
}

func TestBillingOlderUnsupersededVersionBlocksEvenIfLatestIsVoid(t *testing.T) {
	id := seedBillingCustomer(t)
	start, end, err := BillingMonthBounds("2020-02")
	require.NoError(t, err)
	require.NoError(t, DB.Model(&BillingAccount{}).Where("user_id = ?", id).Update("accounting_start_at", start).Error)
	require.NoError(t, DB.Create(&[]BillingStatement{{ID: "older-active", UserID: id, Month: "2020-02", Revision: 1, Status: StatementDraft}, {ID: "latest-void", UserID: id, Month: "2020-02", Revision: 2, Status: StatementVoid}}).Error)
	assert.ErrorContains(t, CreateBillingStatement(&BillingStatement{ID: "new", UserID: id, Month: "2020-02"}, nil), "active statement")
	state, err := GetBillingPreparationState(context.Background(), id, start, end)
	require.NoError(t, err)
	assert.Equal(t, "older-active", state.ExistingStatement)
	_, err = ChangeBillingStatement("older-active", "void", "", "Replace legacy bill", "", 1, true)
	require.NoError(t, err)
	body, digest := correctedStatementTestSnapshot(t)
	next := &BillingStatement{ID: "new", UserID: id, Month: "2020-02"}
	require.NoError(t, CreateBillingStatement(next, func(*BillingAccount, []BillingHour, []BillingModelTotal) (string, string, error) {
		return body, digest, nil
	}, true))
	assert.Equal(t, 3, next.Revision)
}
