package model

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MaxTaskRequestBodyBytes keeps each archived snapshot below MySQL 5.7's
// common 4 MiB packet default with room for statement and row overhead. The
// original and upstream snapshots are persisted as separate rows so their
// payload sizes are never combined into one database write.
const MaxTaskRequestBodyBytes = constant.MaxVideoTaskRequestBodyBytes

var ErrTaskRequestBodyTooLarge = errors.New("request body snapshot exceeds the 2 MiB persistence limit")

const upstreamRequestBodyReferencePrefix = "upstream:"

const (
	TaskRequestSnapshotOriginal = "original"
	TaskRequestSnapshotUpstream = "upstream"
	TaskRequestSnapshotPending  = "pending"
	TaskRequestSnapshotReady    = "ready"
	TaskRequestSnapshotFailed   = "failed"
	TaskRequestEvidenceVersion  = 1
)

// TaskRequestBody keeps large administrator-only request payloads out of the
// hot task and log rows. ReferenceID is a task ID for normal task requests and
// "request:<request-id>" for legacy error logs that have no task linkage.
type TaskRequestBody struct {
	ID               int64           `json:"id" gorm:"primary_key"`
	ReferenceID      string          `json:"reference_id" gorm:"type:varchar(191);uniqueIndex"`
	TaskID           string          `json:"task_id" gorm:"type:varchar(191);index"`
	RequestID        string          `json:"request_id" gorm:"type:varchar(64);index"`
	Body             json.RawMessage `json:"body" gorm:"type:json"`
	BodySize         int64           `json:"body_size"`
	BodySHA256       string          `json:"body_sha256" gorm:"type:varchar(64)"`
	Kind             string          `json:"kind" gorm:"type:varchar(16)"`
	StorageProfileID int             `json:"storage_profile_id" gorm:"index"`
	ObjectKey        string          `json:"object_key" gorm:"type:varchar(1024)"`
	StorageState     string          `json:"storage_state" gorm:"type:varchar(16);index"`
	NodeName         string          `json:"node_name" gorm:"type:varchar(128)"`
	EvidenceVersion  int             `json:"evidence_version"`
	EvidenceValid    bool            `json:"evidence_valid"`
	Resolution       string          `json:"resolution" gorm:"type:varchar(32)"`
	HasVideoInput    bool            `json:"has_video_input"`
	CapturedAt       int64           `json:"captured_at" gorm:"bigint"`
	RequestCreatedAt int64           `json:"request_created_at" gorm:"bigint;index"`
	CreatedAt        int64           `json:"created_at" gorm:"bigint;index"`
	UpdatedAt        int64           `json:"updated_at" gorm:"bigint"`
}

// TaskRequestSnapshots is the administrator-only audit representation of a
// client request and the exact JSON body built for the upstream provider.
type TaskRequestSnapshots struct {
	Original json.RawMessage `json:"original,omitempty"`
	Upstream json.RawMessage `json:"upstream,omitempty"`
}

// TaskRequestEvidence is the bounded input evidence required by Seedance
// repricing. Keeping it next to the object index avoids remote reads inside
// billing transactions, including after the inline request has been cleared.
type TaskRequestEvidence struct {
	Version       int    `json:"version"`
	Valid         bool   `json:"valid"`
	Resolution    string `json:"resolution"`
	HasVideoInput bool   `json:"has_video_input"`
}

// Only field presence affects video input pricing. Retaining the value would
// duplicate potentially large Base64 media while extracting this small record.
type snapshotVideoURLPresence struct {
	Present bool
}

func (presence *snapshotVideoURLPresence) UnmarshalJSON(data []byte) error {
	presence.Present = len(data) > 0
	return nil
}

func ExtractTaskRequestEvidence(body []byte) TaskRequestEvidence {
	type contentItem struct {
		Type     string                   `json:"type"`
		VideoURL snapshotVideoURLPresence `json:"video_url"`
	}
	type videoRequest struct {
		Resolution json.RawMessage `json:"resolution"`
		Content    *[]contentItem  `json:"content"`
	}
	var request struct {
		videoRequest
		Metadata *videoRequest `json:"metadata"`
	}
	invalid := TaskRequestEvidence{Version: TaskRequestEvidenceVersion}
	if len(body) == 0 || common.Unmarshal(body, &request) != nil {
		return invalid
	}
	data := request.videoRequest
	if data.Content == nil && request.Metadata != nil {
		data = *request.Metadata
	}
	if data.Content == nil || len(*data.Content) == 0 {
		return invalid
	}
	evidence := TaskRequestEvidence{Version: TaskRequestEvidenceVersion, Valid: true}
	if len(data.Resolution) > 0 {
		if common.GetJsonType(data.Resolution) != "string" || common.Unmarshal(data.Resolution, &evidence.Resolution) != nil {
			return invalid
		}
		evidence.Resolution = strings.ToLower(strings.TrimSpace(evidence.Resolution))
		if evidence.Resolution == "" || len(evidence.Resolution) > 32 {
			return invalid
		}
	}
	for _, item := range *data.Content {
		if item.Type == "video_url" || item.VideoURL.Present {
			evidence.HasVideoInput = true
			continue
		}
		if item.Type != "text" && item.Type != "image_url" && item.Type != "audio_url" {
			return invalid
		}
	}
	return evidence
}

func (body *TaskRequestBody) BeforeCreate(_ *gorm.DB) error {
	now := common.GetTimestamp()
	if body.CreatedAt == 0 {
		body.CreatedAt = now
	}
	if body.UpdatedAt == 0 {
		body.UpdatedAt = now
	}
	return nil
}

func taskRequestBodyReference(taskID string, requestID string) (string, error) {
	if taskID != "" {
		return taskID, nil
	}
	if requestID != "" {
		return "request:" + requestID, nil
	}
	return "", errors.New("task id or request id is required")
}

func upstreamRequestBodyReference(taskID string, requestID string) (string, error) {
	referenceID, err := taskRequestBodyReference(taskID, requestID)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(referenceID))
	return upstreamRequestBodyReferencePrefix + fmt.Sprintf("%x", digest), nil
}

func saveTaskRequestBody(db *gorm.DB, taskID string, requestID string, body []byte, requestCreatedAt int64) error {
	referenceID, err := taskRequestBodyReference(taskID, requestID)
	if err != nil {
		return err
	}
	return saveTaskRequestBodyAtReference(db, referenceID, taskID, requestID, body, requestCreatedAt)
}

func saveTaskRequestBodyAtReference(db *gorm.DB, referenceID string, taskID string, requestID string, body []byte, requestCreatedAt int64) error {
	if len(body) == 0 {
		return errors.New("request body is required")
	}
	var decoded any
	if err := common.Unmarshal(body, &decoded); err != nil {
		return fmt.Errorf("invalid request body JSON: %w", err)
	}
	digest := sha256.Sum256(body)
	evidence := ExtractTaskRequestEvidence(body)
	kind := TaskRequestSnapshotOriginal
	if strings.HasPrefix(referenceID, upstreamRequestBodyReferencePrefix) {
		kind = TaskRequestSnapshotUpstream
	}
	record := TaskRequestBody{
		ReferenceID:      referenceID,
		TaskID:           taskID,
		RequestID:        requestID,
		Body:             append(json.RawMessage(nil), body...),
		BodySize:         int64(len(body)),
		BodySHA256:       fmt.Sprintf("%x", digest),
		Kind:             kind,
		EvidenceVersion:  evidence.Version,
		EvidenceValid:    evidence.Valid,
		Resolution:       evidence.Resolution,
		HasVideoInput:    evidence.HasVideoInput,
		CapturedAt:       requestCreatedAt * int64(time.Second),
		RequestCreatedAt: requestCreatedAt,
		UpdatedAt:        common.GetTimestamp(),
	}
	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "reference_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"task_id", "request_id", "body", "body_size", "body_sha256", "kind", "storage_profile_id", "object_key", "storage_state", "node_name",
			"evidence_version", "evidence_valid", "resolution", "has_video_input", "captured_at", "request_created_at", "updated_at",
		}),
	}).Create(&record).Error
}

func SaveTaskRequestBody(taskID string, requestID string, body []byte) error {
	return saveTaskRequestBody(DB, taskID, requestID, body, common.GetTimestamp())
}

// SaveTaskRequestSnapshots stores the original and upstream JSON bodies in
// separate rows. Keeping the legacy reference for the original body preserves
// compatibility with existing readers and archive jobs; the reserved upstream
// reference is a bounded SHA-256-derived key, adding the second snapshot
// without a cross-database index migration or exceeding varchar(191).
func SaveTaskRequestSnapshots(taskID string, requestID string, originalBody []byte, upstreamBody []byte) error {
	if len(originalBody) > MaxTaskRequestBodyBytes || len(upstreamBody) > MaxTaskRequestBodyBytes {
		return ErrTaskRequestBodyTooLarge
	}
	upstreamReference, err := upstreamRequestBodyReference(taskID, requestID)
	if err != nil {
		return err
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		requestCreatedAt := common.GetTimestamp()
		if err := saveTaskRequestBody(tx, taskID, requestID, originalBody, requestCreatedAt); err != nil {
			return err
		}
		if len(upstreamBody) == 0 {
			return nil
		}
		return saveTaskRequestBodyAtReference(tx, upstreamReference, taskID, requestID, upstreamBody, requestCreatedAt)
	})
}

func SaveTaskRequestBodyAt(ctx context.Context, taskID string, requestID string, body []byte, requestCreatedAt int64) error {
	if requestCreatedAt <= 0 {
		requestCreatedAt = common.GetTimestamp()
	}
	return saveTaskRequestBody(DB.WithContext(ctx), taskID, requestID, body, requestCreatedAt)
}

// SaveExternalTaskRequestSnapshot publishes a verified object index. An inline
// legacy body is retained on conflict until a separate verified cleanup clears
// it, so publishing and clearing never make an interrupted migration destructive.
func SaveExternalTaskRequestSnapshot(taskID, requestID, kind, nodeName string, profileID int, objectKey string, size int64, bodySHA256 string, evidence TaskRequestEvidence, capturedAt ...int64) error {
	return SaveExternalTaskRequestSnapshotAtReference("", taskID, requestID, kind, nodeName, profileID, objectKey, size, bodySHA256, evidence, capturedAt...)
}

// SaveExternalTaskRequestSnapshotAtReference keeps archive-table migrations on
// their scanned reference even when a task and request trace share linkage.
// An empty reference preserves the normal publication and late-binding behavior.
func SaveExternalTaskRequestSnapshotAtReference(referenceID, taskID, requestID, kind, nodeName string, profileID int, objectKey string, size int64, bodySHA256 string, evidence TaskRequestEvidence, capturedAt ...int64) error {
	return SaveExternalTaskRequestSnapshotAtReferenceContext(context.Background(), referenceID, taskID, requestID, kind, nodeName, profileID, objectKey, size, bodySHA256, evidence, capturedAt...)
}

// SaveExternalTaskRequestSnapshotAtReferenceContext includes configuration
// lookup and metadata publication in the caller's archive deadline.
func SaveExternalTaskRequestSnapshotAtReferenceContext(ctx context.Context, referenceID, taskID, requestID, kind, nodeName string, profileID int, objectKey string, size int64, bodySHA256 string, evidence TaskRequestEvidence, capturedAt ...int64) error {
	if ctx == nil {
		ctx = context.Background()
	}
	db := DB.WithContext(ctx)
	var defaultReference string
	var err error
	switch kind {
	case TaskRequestSnapshotOriginal:
		defaultReference, err = taskRequestBodyReference(taskID, requestID)
	case TaskRequestSnapshotUpstream:
		defaultReference, err = upstreamRequestBodyReference(taskID, requestID)
	default:
		return errors.New("invalid request snapshot kind")
	}
	if err != nil {
		return err
	}
	requestReference := ""
	if requestID != "" {
		requestReference, _ = taskRequestBodyReference("", requestID)
		if kind == TaskRequestSnapshotUpstream {
			requestReference, _ = upstreamRequestBodyReference("", requestID)
		}
	}
	explicitReference := referenceID != ""
	if explicitReference {
		if len(referenceID) > 191 || (referenceID != defaultReference && referenceID != requestReference) || strings.HasPrefix(referenceID, taskPollHistoryPrefix) {
			return errors.New("request snapshot reference does not match its source or kind")
		}
	} else {
		referenceID = defaultReference
	}
	// Request-only snapshots can be published before the upstream task exists.
	// Reuse that reference after binding instead of creating a duplicate row.
	if !explicitReference && taskID != "" && requestID != "" {
		var existing TaskRequestBody
		err := db.Select("reference_id").Where("reference_id IN ?", []string{referenceID, requestReference}).
			Where("task_id = ? OR task_id = ?", taskID, "").Order("id ASC").First(&existing).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			referenceID = existing.ReferenceID
		}
	}
	digest, err := hex.DecodeString(bodySHA256)
	if profileID <= 0 || strings.TrimSpace(objectKey) == "" || size <= 0 || err != nil || len(digest) != sha256.Size {
		return errors.New("incomplete request snapshot object metadata")
	}
	if evidence.Version != TaskRequestEvidenceVersion || len(evidence.Resolution) > 32 {
		return errors.New("unsupported request snapshot evidence")
	}
	now := common.GetTimestamp()
	captured := time.Now().UnixNano()
	if len(capturedAt) > 0 && capturedAt[0] > 0 {
		captured = capturedAt[0]
	}
	record := TaskRequestBody{
		ReferenceID: referenceID, TaskID: taskID, RequestID: requestID,
		Kind: kind, StorageProfileID: profileID, ObjectKey: objectKey,
		StorageState: TaskRequestSnapshotReady, NodeName: nodeName,
		BodySize: size, BodySHA256: strings.ToLower(bodySHA256),
		EvidenceVersion: evidence.Version, EvidenceValid: evidence.Valid,
		Resolution: evidence.Resolution, HasVideoInput: evidence.HasVideoInput,
		CapturedAt:       captured,
		RequestCreatedAt: captured / int64(time.Second), UpdatedAt: now,
	}
	return db.Transaction(func(tx *gorm.DB) error {
		// Inserting with conflict ignored acquires the row before inspecting its
		// capture time, including the concurrent first-publication case.
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "reference_id"}}, DoNothing: true}).Create(&record).Error; err != nil {
			return err
		}
		var existing TaskRequestBody
		if err := lockForUpdate(tx).Select("id", "captured_at").Where("reference_id = ?", referenceID).First(&existing).Error; err != nil {
			return err
		}
		if existing.CapturedAt > captured {
			return nil
		}
		return tx.Model(&TaskRequestBody{}).Where("id = ?", existing.ID).
			Select("TaskID", "RequestID", "Kind", "StorageProfileID", "ObjectKey", "StorageState", "NodeName", "BodySize", "BodySHA256",
				"EvidenceVersion", "EvidenceValid", "Resolution", "HasVideoInput", "CapturedAt", "UpdatedAt").Updates(&record).Error
	})
}

// GetSnapshotRecords prefers the explicit request trace, or the canonical task
// source in task views. Large inline bodies are read only after selecting at
// most one record per kind; ready object indices need no inline body read.
func GetSnapshotRecords(taskID, requestID string, metadataOnly ...bool) ([]TaskRequestBody, error) {
	return GetSnapshotRecordsContext(context.Background(), taskID, requestID, metadataOnly...)
}

func GetSnapshotRecordsContext(ctx context.Context, taskID, requestID string, metadataOnly ...bool) ([]TaskRequestBody, error) {
	if taskID == "" && requestID == "" {
		return nil, errors.New("task id or request id is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	db := DB.WithContext(ctx)
	metadata := len(metadataOnly) > 0 && metadataOnly[0]
	preferRequest := requestID != ""
	references := make([]string, 0, 4)
	taskReferences := make(map[string]bool, 2)
	requestReferences := make(map[string]bool, 2)
	for _, ids := range [][2]string{{taskID, ""}, {"", requestID}} {
		if ids[0] == "" && ids[1] == "" {
			continue
		}
		original, _ := taskRequestBodyReference(ids[0], ids[1])
		upstream, _ := upstreamRequestBodyReference(ids[0], ids[1])
		references = append(references, original, upstream)
		if ids[0] != "" {
			taskReferences[original], taskReferences[upstream] = true, true
		} else {
			requestReferences[original], requestReferences[upstream] = true, true
		}
	}
	var records []TaskRequestBody
	// Each reference is unique. Separating this lookup from task_id avoids the
	// OR/filesort plan that exhausted MySQL's sort buffer on large JSON bodies.
	if err := db.Omit("Body").Where("reference_id IN ?", references).
		Where("reference_id NOT LIKE ?", taskPollHistoryPrefix+"%").Find(&records).Error; err != nil {
		return nil, err
	}
	foundKinds := make(map[string]bool, 2)
	for _, record := range records {
		foundKinds[taskRequestSnapshotKind(record)] = true
	}
	if taskID != "" {
		for _, kind := range []string{TaskRequestSnapshotOriginal, TaskRequestSnapshotUpstream} {
			if foundKinds[kind] {
				continue
			}
			var fallback []TaskRequestBody
			query := db.Omit("Body").Where("task_id = ?", taskID).
				Where("reference_id NOT LIKE ?", taskPollHistoryPrefix+"%")
			if kind == TaskRequestSnapshotUpstream {
				query = query.Where("kind = ? OR ((kind = ? OR kind IS NULL) AND reference_id LIKE ?)", kind, "", upstreamRequestBodyReferencePrefix+"%")
			} else {
				query = query.Where("kind = ? OR ((kind = ? OR kind IS NULL) AND reference_id NOT LIKE ?)", kind, "", upstreamRequestBodyReferencePrefix+"%")
			}
			if err := query.Order("id DESC").Limit(1).Find(&fallback).Error; err != nil {
				return nil, err
			}
			if len(fallback) > 0 {
				foundKinds[kind] = true
				records = append(records, fallback[0])
			}
		}
	}
	// An upstream index can outlive its task or precede task insertion. Recover
	// the shared request ID from metadata before consulting the task's property.
	if taskID != "" && requestID == "" && (!foundKinds[TaskRequestSnapshotOriginal] || !foundKinds[TaskRequestSnapshotUpstream]) {
		var newestID int64
		for _, record := range records {
			if record.RequestID != "" && record.ID > newestID {
				requestID, newestID = record.RequestID, record.ID
			}
		}
		if requestID == "" {
			var linked TaskRequestBody
			if err := db.Select("id", "request_id").Where("task_id = ? AND request_id <> ?", taskID, "").
				Where("reference_id NOT LIKE ?", taskPollHistoryPrefix+"%").Order("id DESC").Limit(1).Find(&linked).Error; err != nil {
				return nil, err
			}
			requestID = linked.RequestID
		}
		if requestID == "" && !metadata {
			// Select only the small linkage value. Legacy properties can contain
			// another large request body and must not be transferred for lookup.
			requestIDExpression := "json_extract(properties, '$.request_id')"
			switch {
			case common.UsingMainDatabase(common.DatabaseTypeMySQL):
				requestIDExpression = "JSON_UNQUOTE(JSON_EXTRACT(properties, '$.request_id'))"
			case common.UsingMainDatabase(common.DatabaseTypePostgreSQL):
				requestIDExpression = "properties->>'request_id'"
			}
			var linkage struct{ RequestID string }
			if err := db.Model(&Task{}).Select("COALESCE("+requestIDExpression+", '') AS request_id").
				Where("task_id = ?", taskID).Order("id ASC").Limit(1).Find(&linkage).Error; err != nil {
				return nil, err
			}
			requestID = linkage.RequestID
		}
		if requestID != "" {
			original, _ := taskRequestBodyReference("", requestID)
			upstream, _ := upstreamRequestBodyReference("", requestID)
			requestReferences[original], requestReferences[upstream] = true, true
			var requestRecords []TaskRequestBody
			if err := db.Omit("Body").Where("reference_id IN ?", []string{original, upstream}).Find(&requestRecords).Error; err != nil {
				return nil, err
			}
			records = append(records, requestRecords...)
		}
	}
	selected := make(map[string]TaskRequestBody, 2)
	selectedPriority := make(map[string]int, 2)
	for _, record := range records {
		record.Kind = taskRequestSnapshotKind(record)
		priority := 0
		switch {
		case preferRequest && requestReferences[record.ReferenceID]:
			priority = 3
		case taskReferences[record.ReferenceID]:
			priority = 2
		case taskID != "" && record.TaskID == taskID:
			priority = 1
		}
		previous, exists := selected[record.Kind]
		if !exists || priority > selectedPriority[record.Kind] || (priority == selectedPriority[record.Kind] && record.ID > previous.ID) {
			selected[record.Kind] = record
			selectedPriority[record.Kind] = priority
		}
	}
	if !metadata {
		inlineIDs := make([]int64, 0, 2)
		for _, kind := range []string{TaskRequestSnapshotOriginal, TaskRequestSnapshotUpstream} {
			if record, exists := selected[kind]; exists && !(record.StorageState == TaskRequestSnapshotReady && record.StorageProfileID > 0 && record.ObjectKey != "") {
				inlineIDs = append(inlineIDs, record.ID)
			}
		}
		if len(inlineIDs) > 0 {
			// A concurrent migration can turn an inline row ready after selection.
			// Refresh its metadata and omit its body in the same PK-only read.
			columns := "id, reference_id, task_id, request_id, body_size, body_sha256, kind, storage_profile_id, object_key, storage_state, node_name, evidence_version, evidence_valid, resolution, has_video_input, captured_at, request_created_at, created_at, updated_at"
			columns += ", CASE WHEN storage_state = 'ready' AND storage_profile_id > 0 AND object_key <> '' THEN NULL ELSE body END AS body"
			var inlineRecords []TaskRequestBody
			if err := db.Select(columns).Where("id IN ?", inlineIDs).Find(&inlineRecords).Error; err != nil {
				return nil, err
			}
			for _, record := range inlineRecords {
				record.Kind = taskRequestSnapshotKind(record)
				selected[record.Kind] = record
			}
		}
	}
	result := make([]TaskRequestBody, 0, 2)
	for _, kind := range []string{TaskRequestSnapshotOriginal, TaskRequestSnapshotUpstream} {
		if record, exists := selected[kind]; exists {
			result = append(result, record)
		}
	}
	return result, nil
}

func taskRequestSnapshotKind(record TaskRequestBody) string {
	if record.Kind != "" {
		return record.Kind
	}
	if strings.HasPrefix(record.ReferenceID, upstreamRequestBodyReferencePrefix) {
		return TaskRequestSnapshotUpstream
	}
	return TaskRequestSnapshotOriginal
}

func BindTaskRequestSnapshots(taskID, requestID string, contexts ...context.Context) error {
	if taskID == "" || requestID == "" {
		return errors.New("task id and request id are required")
	}
	db := DB
	if len(contexts) > 0 && contexts[0] != nil {
		db = db.WithContext(contexts[0])
	}
	return db.Model(&TaskRequestBody{}).
		Where("request_id = ? AND task_id = ?", requestID, "").
		Where("reference_id NOT LIKE ?", taskPollHistoryPrefix+"%").
		Updates(map[string]any{"task_id": taskID, "updated_at": common.GetTimestamp()}).Error
}

func FindDatabaseSnapshotBatch(afterID int64, limit int, contexts ...context.Context) ([]TaskRequestBody, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	var records []TaskRequestBody
	db := DB
	if len(contexts) > 0 && contexts[0] != nil {
		db = db.WithContext(contexts[0])
	}
	err := db.Omit("Body").Where("id > ? AND body IS NOT NULL AND body_size > 0", afterID).
		Where("reference_id NOT LIKE ?", taskPollHistoryPrefix+"%").
		Order("id ASC").Limit(limit).Find(&records).Error
	for i := range records {
		if records[i].Kind == "" {
			records[i].Kind = TaskRequestSnapshotOriginal
			if strings.HasPrefix(records[i].ReferenceID, upstreamRequestBodyReferencePrefix) {
				records[i].Kind = TaskRequestSnapshotUpstream
			}
		}
	}
	return records, err
}

// GetDatabaseSnapshotBody reads one migration payload at a time. Batch scans
// only fetch metadata so large historical Base64 requests remain bounded.
func GetDatabaseSnapshotBody(id int64, contexts ...context.Context) (TaskRequestBody, error) {
	var record TaskRequestBody
	db := DB
	if len(contexts) > 0 && contexts[0] != nil {
		db = db.WithContext(contexts[0])
	}
	err := db.Where("id = ? AND body IS NOT NULL AND body_size > 0", id).
		Where("reference_id NOT LIKE ?", taskPollHistoryPrefix+"%").First(&record).Error
	if record.Kind == "" {
		record.Kind = TaskRequestSnapshotOriginal
		if strings.HasPrefix(record.ReferenceID, upstreamRequestBodyReferencePrefix) {
			record.Kind = TaskRequestSnapshotUpstream
		}
	}
	return record, err
}

// ClearMigratedSnapshotBody refuses to remove the last durable copy or a body
// changed since migration. The caller must additionally verify the remote
// object's bytes before requesting this database cleanup.
func ClearMigratedSnapshotBody(id int64, expectedSHA256 string, contexts ...context.Context) (bool, error) {
	cleared := false
	db := DB
	if len(contexts) > 0 && contexts[0] != nil {
		db = db.WithContext(contexts[0])
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		var record TaskRequestBody
		err := lockForUpdate(tx).Where("id = ?", id).First(&record).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if strings.HasPrefix(record.ReferenceID, taskPollHistoryPrefix) || len(record.Body) == 0 || record.BodySize != int64(len(record.Body)) || record.StorageState != TaskRequestSnapshotReady || record.StorageProfileID <= 0 || record.ObjectKey == "" || record.BodySHA256 != expectedSHA256 {
			return nil
		}
		digest := sha256.Sum256(record.Body)
		if fmt.Sprintf("%x", digest) != expectedSHA256 {
			return nil
		}
		result := tx.Model(&TaskRequestBody{}).Where("id = ?", record.ID).
			Updates(map[string]any{"body": nil, "updated_at": common.GetTimestamp()})
		cleared = result.RowsAffected == 1
		return result.Error
	})
	return cleared, err
}

func GetTaskRequestBody(taskID string) (json.RawMessage, bool, error) {
	return GetArchivedRequestBody(taskID, "")
}

func GetArchivedRequestBody(taskID string, requestID string) (json.RawMessage, bool, error) {
	referenceID, err := taskRequestBodyReference(taskID, requestID)
	if err != nil {
		return nil, false, err
	}
	var record TaskRequestBody
	err = DB.Select("body").Where("reference_id = ?", referenceID).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return record.Body, true, nil
}

func GetArchivedRequestSnapshots(taskID string, requestID string) (TaskRequestSnapshots, bool, error) {
	originalReference, err := taskRequestBodyReference(taskID, requestID)
	if err != nil {
		return TaskRequestSnapshots{}, false, err
	}
	upstreamReference, err := upstreamRequestBodyReference(taskID, requestID)
	if err != nil {
		return TaskRequestSnapshots{}, false, err
	}
	var records []TaskRequestBody
	if err := DB.Select("reference_id", "body").
		Where("reference_id IN ?", []string{originalReference, upstreamReference}).
		Find(&records).Error; err != nil {
		return TaskRequestSnapshots{}, false, err
	}
	snapshots := TaskRequestSnapshots{}
	for _, record := range records {
		if record.ReferenceID == upstreamReference {
			snapshots.Upstream = record.Body
			continue
		}
		if record.ReferenceID == originalReference {
			snapshots.Original = record.Body
		}
	}
	return snapshots, len(records) > 0, nil
}

type LegacyTaskRequestBodyRow struct {
	ID         int64
	TaskID     string
	CreatedAt  int64
	SubmitTime int64
	Properties Properties `gorm:"type:json"`
}

func GetTaskRequestBodyArchiveUpperID(ctx context.Context) (int64, error) {
	var upperID int64
	err := DB.WithContext(ctx).Model(&Task{}).Select("COALESCE(MAX(id), 0)").Scan(&upperID).Error
	return upperID, err
}

func GetDatabaseSnapshotArchiveUpperID(ctx context.Context) (int64, error) {
	var upperID int64
	err := DB.WithContext(ctx).Model(&TaskRequestBody{}).Select("COALESCE(MAX(id), 0)").Scan(&upperID).Error
	return upperID, err
}

func CountTasksThroughID(ctx context.Context, upperID int64) (int64, error) {
	var total int64
	err := DB.WithContext(ctx).Model(&Task{}).Where("id <= ?", upperID).Count(&total).Error
	return total, err
}

func FindLegacyTaskRequestBodyBatch(ctx context.Context, cursor int64, upperID int64, limit int, metadataOnly ...bool) ([]LegacyTaskRequestBodyRow, error) {
	if limit <= 0 {
		limit = 100
	}
	var rows []LegacyTaskRequestBodyRow
	columns := "id, task_id, created_at, submit_time, properties"
	if len(metadataOnly) > 0 && metadataOnly[0] {
		columns = "id, task_id, created_at, submit_time"
	}
	err := DB.WithContext(ctx).Model(&Task{}).
		Select(columns).
		Where("id > ? AND id <= ?", cursor, upperID).
		Order("id ASC").
		Limit(limit).
		Scan(&rows).Error
	return rows, err
}

// ArchiveLegacyTaskRequestBody writes the payload first and only then removes
// it from the hot task row. Re-running after interruption is therefore safe.
func ArchiveLegacyTaskRequestBody(ctx context.Context, row LegacyTaskRequestBodyRow) (bool, error) {
	if len(row.Properties.RequestBody) == 0 {
		return false, nil
	}
	err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		requestCreatedAt := row.SubmitTime
		if requestCreatedAt <= 0 {
			requestCreatedAt = row.CreatedAt
		}
		if err := saveTaskRequestBody(tx, row.TaskID, "", row.Properties.RequestBody, requestCreatedAt); err != nil {
			return err
		}
		row.Properties.RequestBody = nil
		return tx.Model(&Task{}).Where("id = ?", row.ID).Updates(map[string]any{
			"properties":             row.Properties,
			"request_body_available": true,
		}).Error
	})
	return err == nil, err
}

func DeleteOrphanTaskRequestBodiesBefore(ctx context.Context, targetTimestamp int64) (int64, error) {
	result := DB.WithContext(ctx).
		Where("request_created_at > 0 AND request_created_at < ?", targetTimestamp).
		Where("storage_profile_id IS NULL OR storage_profile_id = ?", 0).
		Where("task_id = ? OR NOT EXISTS (SELECT 1 FROM tasks WHERE tasks.task_id = task_request_bodies.task_id)", "").
		Delete(&TaskRequestBody{})
	return result.RowsAffected, result.Error
}

type LogRequestBodyArchiveRow struct {
	ID          int
	CreatedAt   int64
	RequestID   string
	Other       string
	OtherSHA256 string `gorm:"column:other_sha256"`
}

type LogRequestBodyArchiveCursor struct {
	ID          int
	CreatedAt   int64
	RequestID   string
	OtherSHA256 string
}

func GetLogRequestBodyArchiveUpperCursor(ctx context.Context) (LogRequestBodyArchiveCursor, error) {
	var row LogRequestBodyArchiveRow
	order := "id DESC"
	selectColumns := "id, created_at, request_id"
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		selectColumns += ", lower(hex(SHA256(other))) AS other_sha256"
		order = "created_at DESC, request_id DESC, other_sha256 DESC"
	}
	err := LOG_DB.WithContext(ctx).Model(&Log{}).
		Select(selectColumns).
		Order(order).
		Limit(1).
		Scan(&row).Error
	return LogRequestBodyArchiveCursor{
		ID: row.ID, CreatedAt: row.CreatedAt, RequestID: row.RequestID, OtherSHA256: row.OtherSHA256,
	}, err
}

func CountLogsThroughCursor(ctx context.Context, upper LogRequestBodyArchiveCursor) (int64, error) {
	var total int64
	tx := LOG_DB.WithContext(ctx).Model(&Log{})
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		tx = tx.Where(
			"created_at < ? OR (created_at = ? AND request_id < ?) OR (created_at = ? AND request_id = ? AND lower(hex(SHA256(other))) <= ?)",
			upper.CreatedAt, upper.CreatedAt, upper.RequestID, upper.CreatedAt, upper.RequestID, upper.OtherSHA256,
		)
	} else {
		tx = tx.Where("id <= ?", upper.ID)
	}
	return total, tx.Count(&total).Error
}

func FindLogRequestBodyArchiveBatch(ctx context.Context, cursor LogRequestBodyArchiveCursor, upper LogRequestBodyArchiveCursor, limit int, metadataOnly ...bool) ([]LogRequestBodyArchiveRow, error) {
	if limit <= 0 {
		limit = 100
	}
	selectColumns := "id, created_at, request_id, other"
	if len(metadataOnly) > 0 && metadataOnly[0] {
		selectColumns = "id, created_at, request_id"
	}
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		selectColumns += ", lower(hex(SHA256(other))) AS other_sha256"
	}
	tx := LOG_DB.WithContext(ctx).Model(&Log{}).Select(selectColumns)
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		tx = tx.
			Where(
				"created_at > ? OR (created_at = ? AND request_id > ?) OR (created_at = ? AND request_id = ? AND lower(hex(SHA256(other))) > ?)",
				cursor.CreatedAt, cursor.CreatedAt, cursor.RequestID, cursor.CreatedAt, cursor.RequestID, cursor.OtherSHA256,
			).
			Where(
				"created_at < ? OR (created_at = ? AND request_id < ?) OR (created_at = ? AND request_id = ? AND lower(hex(SHA256(other))) <= ?)",
				upper.CreatedAt, upper.CreatedAt, upper.RequestID, upper.CreatedAt, upper.RequestID, upper.OtherSHA256,
			).
			Order("created_at ASC, request_id ASC, other_sha256 ASC")
	} else {
		tx = tx.Where("id > ? AND id <= ?", cursor.ID, upper.ID).Order("id ASC")
	}
	var rows []LogRequestBodyArchiveRow
	err := tx.Limit(limit).Scan(&rows).Error
	return rows, err
}

type LogRequestBodyArchiveUpdate struct {
	ID            int
	CreatedAt     int64
	RequestID     string
	OriginalOther string
	Other         string
}

func UpdateArchivedLogRequestBodies(ctx context.Context, updates []LogRequestBodyArchiveUpdate) error {
	if len(updates) == 0 {
		return nil
	}
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		args := make([]any, 0, len(updates)*6)
		assignment := "multiIf("
		where := ""
		for i, update := range updates {
			if i > 0 {
				assignment += ", "
				where += " OR "
			}
			originalDigest := sha256.Sum256([]byte(update.OriginalOther))
			originalSHA256 := fmt.Sprintf("%x", originalDigest)
			assignment += "(created_at = ? AND request_id = ? AND lower(hex(SHA256(other))) = ?), ?"
			args = append(args, update.CreatedAt, update.RequestID, originalSHA256, update.Other)
			where += "(created_at = ? AND request_id = ? AND lower(hex(SHA256(other))) = ?)"
			updates[i].OriginalOther = originalSHA256
		}
		assignment += ", other)"
		for _, update := range updates {
			args = append(args, update.CreatedAt, update.RequestID, update.OriginalOther)
		}
		query := "ALTER TABLE logs UPDATE other = " + assignment + " WHERE " + where + " SETTINGS mutations_sync = 1"
		return LOG_DB.WithContext(ctx).Exec(query, args...).Error
	}

	return LOG_DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, update := range updates {
			if err := tx.Model(&Log{}).Where("id = ?", update.ID).Update("other", update.Other).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
