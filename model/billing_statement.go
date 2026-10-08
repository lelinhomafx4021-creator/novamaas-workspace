package model

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	StatementPreparing = "preparing"
	StatementDraft     = "draft"
	StatementIssued    = "issued"
	StatementDisputed  = "disputed"
	StatementConfirmed = "confirmed"
	StatementVoid      = "void"
	StatementFailed    = "failed"
)

type BillingStatement struct {
	ID                string  `json:"id" gorm:"primaryKey;type:varchar(64)"`
	UserID            int     `json:"user_id" gorm:"uniqueIndex:idx_billing_statement_revision,priority:1;index:idx_billing_statement_list,priority:1"`
	Month             string  `json:"month" gorm:"type:varchar(7);uniqueIndex:idx_billing_statement_revision,priority:2"`
	Revision          int     `json:"revision" gorm:"uniqueIndex:idx_billing_statement_revision,priority:3"`
	ActiveKey         *string `json:"-" gorm:"type:varchar(64);uniqueIndex"`
	Status            string  `json:"status" gorm:"type:varchar(16);index:idx_billing_statement_jobs,priority:1"`
	StartAt           int64   `json:"start_at" gorm:"bigint"`
	EndAt             int64   `json:"end_at" gorm:"bigint"`
	FromSequence      int64   `json:"-" gorm:"bigint"`
	ToSequence        int64   `json:"-" gorm:"bigint"`
	ProfileVersion    int64   `json:"profile_version" gorm:"bigint"`
	Snapshot          string  `json:"snapshot" gorm:"type:text"`
	SnapshotSHA256    string  `json:"snapshot_sha256" gorm:"type:char(64)"`
	ManifestSHA256    string  `json:"manifest_sha256" gorm:"type:char(64)"`
	PDFSHA256         string  `json:"pdf_sha256" gorm:"type:char(64)"`
	ExcelSHA256       string  `json:"excel_sha256" gorm:"type:char(64)"`
	SourceStatementID string  `json:"source_statement_id,omitempty" gorm:"type:varchar(64)"`
	SupersededBy      string  `json:"superseded_by,omitempty" gorm:"type:varchar(64)"`
	StorageProfileID  int     `json:"storage_profile_id"`
	CreatedBy         int     `json:"created_by"`
	CreatedAt         int64   `json:"created_at" gorm:"bigint;index:idx_billing_statement_list,priority:2"`
	IssuedBy          int     `json:"issued_by"`
	IssuedAt          int64   `json:"issued_at" gorm:"bigint"`
	DueAt             int64   `json:"due_at" gorm:"bigint"`
	ConfirmedAt       int64   `json:"confirmed_at" gorm:"bigint"`
	LeaseOwner        string  `json:"-" gorm:"type:varchar(64)"`
	LeaseUntil        int64   `json:"-" gorm:"bigint;index:idx_billing_statement_jobs,priority:2"`
	LastError         string  `json:"last_error,omitempty" gorm:"type:text"`
}

type BillingStatementEvent struct {
	ID             int64  `json:"id" gorm:"primaryKey"`
	StatementID    string `json:"statement_id" gorm:"type:varchar(64);index:idx_billing_statement_events,priority:1"`
	ActorID        int    `json:"actor_id"`
	Action         string `json:"action" gorm:"type:varchar(32)"`
	Note           string `json:"note" gorm:"type:text"`
	ManifestSHA256 string `json:"manifest_sha256" gorm:"type:char(64)"`
	PDFSHA256      string `json:"pdf_sha256" gorm:"type:char(64)"`
	ExcelSHA256    string `json:"excel_sha256" gorm:"type:char(64)"`
	SessionID      string `json:"-" gorm:"type:varchar(128)"`
	CreatedAt      int64  `json:"created_at" gorm:"bigint;index:idx_billing_statement_events,priority:2"`
}

// Artifact metadata is permanent, and also registered with StorageObject so
// profile identity cannot be changed underneath archived statements.
type BillingArtifact struct {
	ID          int64  `json:"id" gorm:"primaryKey"`
	StatementID string `json:"statement_id" gorm:"type:varchar(64);uniqueIndex:idx_billing_artifact,priority:1"`
	Kind        string `json:"kind" gorm:"type:varchar(16);uniqueIndex:idx_billing_artifact,priority:2"`
	Ordinal     int    `json:"ordinal" gorm:"uniqueIndex:idx_billing_artifact,priority:3"`
	ObjectID    string `json:"-" gorm:"type:varchar(64)"`
	ObjectKey   string `json:"-" gorm:"type:varchar(1024)"`
	SHA256      string `json:"sha256" gorm:"type:char(64)"`
	Size        int64  `json:"size" gorm:"bigint"`
	Rows        int64  `json:"rows" gorm:"bigint"`
}

// BillingModelTotal is the customer-visible amount breakdown frozen with a
// statement. It contains no channel, provider-cost, margin, or administrator
// accounting data.
type BillingModelTotal struct {
	ModelName    string `json:"model_name"`
	BillingGroup string `json:"billing_group,omitempty"`
	BillingRate  string `json:"billing_rate,omitempty"`
	Charge       int64  `json:"charge"`
	Refund       int64  `json:"refund"`
	Count        int64  `json:"count" gorm:"column:records"`
	ChargeCount  int64  `json:"charge_count"`
	RefundCount  int64  `json:"refund_count"`
	ActiveDays   int64  `json:"active_days"`
	FirstPosted  int64  `json:"first_posted_at" gorm:"column:first_posted_at"`
	LastPosted   int64  `json:"last_posted_at" gorm:"column:last_posted_at"`
}

// summarizeBillingModels uses the original model name from each ledger entry.
// SQL GROUP BY follows the database column collation, which can merge names
// such as Foo and foo on MySQL even though the archive compares them exactly.
func summarizeBillingModels(entriesScope *gorm.DB, userID int, fromSequence, toSequence, start, end int64, historicalRates bool) ([]BillingModelTotal, error) {
	const batchSize = 1000
	type modelRateKey struct{ Model, Group, Rate string }
	totals := make(map[modelRateKey]BillingModelTotal)
	modelDays := make(map[modelRateKey]map[int64]struct{})
	cursor := fromSequence - 1
	for {
		var entries []BillingEntry
		err := entriesScope.Session(&gorm.Session{}).
			Select("sequence", "model_name", "posted_at", "quota", "kind", "request_id", "source_log_id", "billing_group", "billing_rate").
			Where("user_id = ? AND sequence >= ? AND sequence > ? AND sequence <= ? AND posted_at >= ? AND posted_at < ? AND kind <> ?", userID, fromSequence, cursor, toSequence, start, end, "funding").
			Order("sequence asc").Limit(batchSize).Find(&entries).Error
		if err != nil {
			return nil, err
		}
		rates := make(map[int64]billingHistoricalRate)
		if historicalRates {
			rates, err = billingEntryHistoricalRates(entriesScope, userID, entries)
			if err != nil {
				return nil, err
			}
		}
		for _, entry := range entries {
			rate := rates[entry.Sequence]
			key := modelRateKey{entry.ModelName, rate.Group, rate.Ratio}
			total := totals[key]
			total.BillingGroup, total.BillingRate = rate.Group, rate.Ratio
			total.ModelName = entry.ModelName
			if total.Count == 0 || entry.PostedAt < total.FirstPosted {
				total.FirstPosted = entry.PostedAt
			}
			if entry.PostedAt > total.LastPosted {
				total.LastPosted = entry.PostedAt
			}
			day := (entry.PostedAt + 28800) / 86400
			days := modelDays[key]
			if days == nil {
				days = make(map[int64]struct{})
				modelDays[key] = days
			}
			days[day] = struct{}{}
			total.ActiveDays = int64(len(days))
			if entry.Quota >= 0 {
				total.Charge += entry.Quota
				total.ChargeCount++
			} else {
				total.Refund -= entry.Quota
				total.RefundCount++
			}
			total.Count++
			totals[key] = total
		}
		if len(entries) < batchSize {
			break
		}
		cursor = entries[len(entries)-1].Sequence
	}
	result := make([]BillingModelTotal, 0, len(totals))
	for _, total := range totals {
		result = append(result, total)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].ModelName != result[j].ModelName {
			return result[i].ModelName < result[j].ModelName
		}
		if result[i].BillingGroup != result[j].BillingGroup {
			return result[i].BillingGroup < result[j].BillingGroup
		}
		return result[i].BillingRate < result[j].BillingRate
	})
	return result, nil
}

func GetBillingStatement(id string) (*BillingStatement, error) {
	var statement BillingStatement
	err := DB.First(&statement, "id = ?", id).Error
	return &statement, err
}

// CompleteBillingStatementArchive publishes only the current lease owner's
// result. Metadata is updated atomically with the state and lease release.
func CompleteBillingStatementArchive(statement *BillingStatement, owner string, succeeded bool) error {
	updates := map[string]interface{}{"lease_until": 0, "lease_owner": ""}
	if succeeded {
		if statement.ManifestSHA256 == "" || statement.PDFSHA256 == "" {
			return ErrBillingEvidenceIntegrity
		}
		var snapshot struct {
			PDFTemplateVersion int `json:"pdf_template_version"`
		}
		if err := common.UnmarshalJsonStr(statement.Snapshot, &snapshot); err != nil {
			return err
		}
		if snapshot.PDFTemplateVersion >= 9 && statement.ExcelSHA256 == "" {
			return ErrBillingEvidenceIntegrity
		}
		updates["status"], updates["last_error"] = StatementDraft, ""
		// Use Go field names so GORM resolves acronym-heavy fields against the
		// same schema used by AutoMigrate, without renaming existing columns.
		updates["ManifestSHA256"], updates["PDFSHA256"] = statement.ManifestSHA256, statement.PDFSHA256
		updates["ExcelSHA256"] = statement.ExcelSHA256
	} else {
		updates["status"], updates["last_error"] = StatementFailed, "Archive preparation failed. Check server logs and retry."
	}
	result := DB.Model(&BillingStatement{}).Where("id = ? AND status = ? AND lease_owner = ?", statement.ID, StatementPreparing, owner).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrBillingConflict
	}
	return nil
}

func ListBillingStatements(userID int, before int64, beforeID, status string, customer bool) ([]BillingStatement, error) {
	statements := make([]BillingStatement, 0)
	query := DB.Where("user_id = ?", userID)
	if customer {
		query = query.Where("issued_at > 0")
	}
	if status != "" && status != "all" {
		query = query.Where("status = ?", status)
	}
	if before > 0 {
		query = query.Where("created_at < ? OR (created_at = ? AND id < ?)", before, before, beforeID)
	}
	err := query.Order("created_at desc").Order("id desc").Limit(100).Find(&statements).Error
	return statements, err
}

// Effective statements include legacy rows without an active key. Superseded
// versions are retained as evidence, but cannot block a replacement.
func effectiveBillingStatements(tx *gorm.DB) *gorm.DB {
	return tx.Where("status <> ? AND (superseded_by IS NULL OR superseded_by = ?)", StatementVoid, "")
}

// Caller supplies a snapshot built under the account lock. It contains no
// sensitive request bodies or credentials.
func CreateBillingStatement(statement *BillingStatement, buildSnapshot func(*BillingAccount, []BillingHour, []BillingModelTotal) (string, string, error), corrected ...bool) error {
	useCorrections := len(corrected) > 0 && corrected[0]
	start, end, err := BillingMonthBounds(statement.Month)
	if err != nil {
		return err
	}
	if common.GetTimestamp() < end+86400 {
		return errors.New("statements can be prepared 24 hours after the month closes")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		account, err := lockBillingAccount(tx, statement.UserID)
		if err != nil {
			return err
		}
		if account.AccountingStartAt == 0 {
			return ErrBillingNotConfigured
		}
		if account.AccountingStartAt >= end {
			return errors.New("month precedes the accounting start")
		}
		if account.CompanyTitle == "" || account.TaxID == "" {
			return errors.New("company title and tax ID are required")
		}
		var active BillingStatement
		if err := effectiveBillingStatements(tx).Where("user_id = ? AND month = ?", statement.UserID, statement.Month).Select("id").Limit(1).Find(&active).Error; err != nil {
			return err
		}
		if active.ID != "" {
			return errors.New("an active statement already exists for this month; void it before creating a new version")
		}
		var pending int64
		if err := tx.Model(&BillingOperation{}).Where("user_id = ? AND state = ? AND created_at < ?", statement.UserID, "reserved", end).Count(&pending).Error; err != nil {
			return err
		}
		if pending > 0 {
			return errors.New("unfinished billing operations must be reconciled before preparing a statement")
		}
		if account.AccountingStartAt > start {
			start = account.AccountingStartAt
		}
		statement.StartAt, statement.EndAt = start, end
		statement.FromSequence, statement.ToSequence = account.Sequence+1, account.Sequence
		statement.ProfileVersion = account.ProfileVersion
		hours := make([]BillingHour, 0)
		if err := tx.Where("user_id = ? AND hour >= ? AND hour < ?", statement.UserID, start/3600*3600, end).Order("hour asc").Find(&hours).Error; err != nil {
			return err
		}
		// Summaries cannot define the evidence boundary: a missing first or last
		// hour would otherwise silently remove real entries from the archive.
		// This one-time, indexed customer/month query freezes canonical bounds;
		// the worker independently reconciles every included entry with the hours.
		var bounds struct {
			FirstSequence int64
			LastSequence  int64
		}
		if err := tx.Model(&BillingEntry{}).
			Where("user_id = ? AND posted_at >= ? AND posted_at < ? AND sequence >= ? AND sequence <= ? AND kind <> ?", statement.UserID, start, end, account.StartSequence, account.Sequence, "funding").
			Select("COALESCE(MIN(sequence), 0) AS first_sequence, COALESCE(MAX(sequence), 0) AS last_sequence").Scan(&bounds).Error; err != nil {
			return err
		}
		if bounds.FirstSequence > 0 {
			statement.FromSequence, statement.ToSequence = bounds.FirstSequence, bounds.LastSequence
		}
		for _, hour := range hours {
			if hour.FirstSequence < account.StartSequence || hour.LastSequence < hour.FirstSequence || hour.LastSequence > account.Sequence {
				return errors.New("invalid hourly billing sequence bounds")
			}
		}
		modelTotals := make([]BillingModelTotal, 0)
		if bounds.FirstSequence > 0 {
			modelTotals, err = summarizeBillingModels(tx.Model(&BillingEntry{}), statement.UserID, statement.FromSequence, statement.ToSequence, start, end, !useCorrections)
			if err != nil {
				return err
			}
		}
		if useCorrections {
			if err := validateBillingCorrectionPeriodStatements(tx, statement); err != nil {
				return err
			}
			var hourCharge, hourRefund, hourCount, entryCharge, entryRefund, entryCount int64
			for _, hour := range hours {
				if hour.Charge < 0 || hour.Refund < 0 || hour.Charge > int64(common.MaxWalletQuota)-hourCharge || hour.Refund > int64(common.MaxWalletQuota)-hourRefund {
					return ErrBillingEvidenceIntegrity
				}
				hourCharge += hour.Charge
				hourRefund += hour.Refund
				hourCount += hour.Count
			}
			for _, total := range modelTotals {
				if total.Charge < 0 || total.Refund < 0 || total.Charge > int64(common.MaxWalletQuota)-entryCharge || total.Refund > int64(common.MaxWalletQuota)-entryRefund {
					return ErrBillingEvidenceIntegrity
				}
				entryCharge += total.Charge
				entryRefund += total.Refund
				entryCount += total.Count
			}
			if hourCharge != entryCharge || hourRefund != entryRefund || hourCount != entryCount {
				return errors.New("formal hourly summaries do not reconcile with source ledger")
			}
			statement.ToSequence = account.Sequence
			hours, modelTotals, err = billingCorrectedPeriodTotals(tx, statement.UserID, statement.FromSequence, statement.ToSequence, start, end)
			if err != nil {
				return err
			}
		}
		statement.Snapshot, statement.SnapshotSHA256, err = buildSnapshot(account, hours, modelTotals)
		if err != nil {
			return err
		}
		if useCorrections {
			if err := statement.VerifySnapshot(); err != nil {
				return err
			}
			basis, err := statement.UsesSourcePeriodCorrections()
			if err != nil {
				return err
			}
			if !basis {
				return ErrBillingEvidenceIntegrity
			}
		}
		var latest BillingStatement
		err = tx.Where("user_id = ? AND month = ?", statement.UserID, statement.Month).Order("revision desc").First(&latest).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		statement.Revision = latest.Revision + 1
		key := fmt.Sprintf("%d:%s", statement.UserID, statement.Month)
		statement.ActiveKey = &key
		statement.Status, statement.CreatedAt = StatementPreparing, common.GetTimestamp()
		if err := tx.Create(statement).Error; err != nil {
			return err
		}
		return tx.Create(&BillingStatementEvent{StatementID: statement.ID, ActorID: statement.CreatedBy, Action: "prepare", CreatedAt: statement.CreatedAt}).Error
	})
}

func ChangeBillingStatement(id, action, digest, note, sessionID string, actorID int, admin bool) (*BillingStatement, error) {
	var statement BillingStatement
	if err := DB.Select("id", "user_id").First(&statement, "id = ?", id).Error; err != nil {
		return nil, err
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		// Correction apply and statement creation also lock the account. Keep
		// issue/confirm validation atomic with respect to a new correction.
		if _, err := lockBillingAccount(tx, statement.UserID); err != nil {
			return err
		}
		if err := lockForUpdate(tx).First(&statement, "id = ?", id).Error; err != nil {
			return err
		}
		if !admin && statement.UserID != actorID {
			return gorm.ErrRecordNotFound
		}
		if statement.SupersededBy != "" {
			return ErrBillingConflict
		}
		if action == "issue" || action == "confirm" {
			if err := statement.VerifySnapshot(); err != nil {
				return err
			}
			var snapshot struct {
				PDFTemplateVersion int `json:"pdf_template_version"`
			}
			if err := common.UnmarshalJsonStr(statement.Snapshot, &snapshot); err != nil {
				return err
			}
			if snapshot.PDFTemplateVersion >= 9 && statement.ExcelSHA256 == "" {
				return ErrBillingEvidenceIntegrity
			}
			if statement.Status != StatementConfirmed {
				pending, err := billingStatementCorrectionsPending(tx, &statement)
				if err != nil {
					return err
				}
				if pending {
					return ErrBillingStatementCorrectionsPending
				}
			}
		}
		now := common.GetTimestamp()
		updates := map[string]interface{}{}
		switch action {
		case "issue":
			if !admin || statement.Status != StatementDraft || statement.ManifestSHA256 == "" || statement.PDFSHA256 == "" {
				return ErrBillingConflict
			}
			updates["status"], updates["issued_at"], updates["issued_by"], updates["due_at"] = StatementIssued, now, actorID, now+7*86400
		case "confirm":
			// The controller additionally requires a live first-party session.
			if statement.UserID != actorID || sessionID == "" || digest == "" || digest != statement.ManifestSHA256 {
				return ErrBillingConflict
			}
			if statement.Status == StatementConfirmed {
				return nil
			}
			if statement.Status != StatementIssued && statement.Status != StatementDisputed {
				return ErrBillingConflict
			}
			updates["status"], updates["confirmed_at"] = StatementConfirmed, now
		case "dispute":
			if statement.UserID != actorID || sessionID == "" || statement.Status != StatementIssued || note == "" {
				return ErrBillingConflict
			}
			updates["status"] = StatementDisputed
		case "reply":
			if !admin || statement.Status != StatementDisputed || note == "" {
				return ErrBillingConflict
			}
		case "void":
			if !admin || statement.Status == StatementVoid || strings.TrimSpace(note) == "" {
				return ErrBillingConflict
			}
			updates["status"], updates["active_key"] = StatementVoid, nil
		case "retry":
			if !admin || statement.Status != StatementFailed {
				return ErrBillingConflict
			}
			updates["status"], updates["last_error"], updates["lease_until"] = StatementPreparing, "", 0
		default:
			return errors.New("invalid statement action")
		}
		if len([]rune(note)) > 2000 {
			return errors.New("statement note exceeds 2000 characters")
		}
		if err := tx.Model(&statement).Updates(updates).Error; err != nil {
			return err
		}
		event := BillingStatementEvent{StatementID: id, ActorID: actorID, Action: action, Note: note, SessionID: sessionID, CreatedAt: now, ManifestSHA256: statement.ManifestSHA256, PDFSHA256: statement.PDFSHA256, ExcelSHA256: statement.ExcelSHA256}
		return tx.Create(&event).Error
	})
	if err != nil {
		return nil, err
	}
	return GetBillingStatement(id)
}

func GetBillingStatementEntries(statement *BillingStatement, after int64, limit int) ([]BillingEntry, error) {
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}
	corrected, err := statement.UsesSourcePeriodCorrections()
	if err != nil {
		return nil, err
	}
	return billingStatementEntryPage(DB, statement.UserID, statement.FromSequence, statement.ToSequence, statement.StartAt, statement.EndAt, after, limit, corrected)
}
