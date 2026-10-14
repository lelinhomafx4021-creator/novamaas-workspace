package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
)

type SnapshotCapture struct {
	ReferenceID string                    `json:"reference_id,omitempty"`
	TaskID      string                    `json:"task_id"`
	RequestID   string                    `json:"request_id"`
	Kind        string                    `json:"kind"`
	NodeName    string                    `json:"node_name"`
	UserID      int                       `json:"user_id"`
	Attempt     int                       `json:"attempt,omitempty"`
	ContentType string                    `json:"content_type,omitempty"`
	Evidence    model.TaskRequestEvidence `json:"evidence"`
	CapturedAt  int64                     `json:"captured_at"`
}

type snapshotSpoolJob struct {
	ID string `json:"id"`
	SnapshotCapture
	Profile          model.StorageProfile `json:"profile"`
	PolicyID         int64                `json:"policy_id"`
	Prefix           string               `json:"prefix"`
	RetentionSeconds int64                `json:"retention_seconds"`
	Size             int64                `json:"size"`
	SHA256           string               `json:"sha256"`
	CreatedAt        int64                `json:"created_at"`
	Attempts         int                  `json:"attempts"`
	NextAttemptAt    int64                `json:"next_attempt_at"`
	LastError        string               `json:"last_error,omitempty"`
	processing       bool
}

func (job *snapshotSpoolJob) reference() string {
	if job.ReferenceID != "" {
		return job.ReferenceID
	}
	if job.RequestID != "" {
		return job.RequestID
	}
	return job.TaskID
}

type RequestSnapshotWorkerStatus struct {
	NodeName       string `json:"node_name"`
	SpoolDirectory string `json:"spool_directory"`
	Workers        int    `json:"workers"`
	PendingFiles   int    `json:"pending_files"`
	PendingBytes   int64  `json:"pending_bytes"`
	LastError      string `json:"last_error,omitempty"`
}

type snapshotSpool struct {
	mu           sync.Mutex
	dir          string
	lock         *os.File
	jobs         map[string]*snapshotSpoolJob
	bytes        int64
	reservations int
	workers      int
	maxJobs      int
	lastError    string
	wake         chan struct{}
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	stopping     bool
}

var requestSnapshotWorkers struct {
	sync.Mutex
	spool     *snapshotSpool
	lastError string
}

func StartRequestSnapshotWorkers() error {
	requestSnapshotWorkers.Lock()
	defer requestSnapshotWorkers.Unlock()
	if requestSnapshotWorkers.spool != nil {
		return nil
	}
	dir := common.GetEnvOrDefaultString("SNAPSHOT_SPOOL_DIR", filepath.Join("data", "request-snapshots"))
	spool, err := openSnapshotSpool(dir, min(max(common.GetEnvOrDefault("SNAPSHOT_WORKERS", 2), 1), 16))
	if err != nil {
		requestSnapshotWorkers.lastError = err.Error()
		return err
	}
	requestSnapshotWorkers.spool = spool
	requestSnapshotWorkers.lastError = ""
	for range spool.workers {
		spool.wg.Add(1)
		go spool.runWorker()
	}
	return nil
}

func StopRequestSnapshotWorkers(ctx context.Context) error {
	requestSnapshotWorkers.Lock()
	spool := requestSnapshotWorkers.spool
	requestSnapshotWorkers.Unlock()
	if spool == nil {
		return nil
	}
	spool.mu.Lock()
	spool.stopping = true
	spool.mu.Unlock()
	spool.cancel()
	done := make(chan struct{})
	go func() {
		spool.wg.Wait()
		close(done)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
	}
	requestSnapshotWorkers.Lock()
	defer requestSnapshotWorkers.Unlock()
	if requestSnapshotWorkers.spool == spool {
		requestSnapshotWorkers.spool = nil
		_ = unlockSnapshotSpool(spool.lock)
		return spool.lock.Close()
	}
	return nil
}

func SnapshotWorkerStatus() RequestSnapshotWorkerStatus {
	requestSnapshotWorkers.Lock()
	spool := requestSnapshotWorkers.spool
	startError := requestSnapshotWorkers.lastError
	requestSnapshotWorkers.Unlock()
	status := RequestSnapshotWorkerStatus{NodeName: common.NodeName, LastError: startError}
	if spool == nil {
		return status
	}
	spool.mu.Lock()
	defer spool.mu.Unlock()
	status.SpoolDirectory, status.Workers = spool.dir, spool.workers
	status.PendingFiles, status.PendingBytes, status.LastError = len(spool.jobs)+spool.reservations, spool.bytes, spool.lastError
	return status
}

func RequestSnapshotPending(taskID, requestID string) bool {
	requestSnapshotWorkers.Lock()
	spool := requestSnapshotWorkers.spool
	requestSnapshotWorkers.Unlock()
	if spool == nil {
		return false
	}
	spool.mu.Lock()
	defer spool.mu.Unlock()
	for _, job := range spool.jobs {
		if (taskID != "" && job.TaskID == taskID) || (requestID != "" && job.RequestID == requestID) {
			return true
		}
	}
	return false
}

func openSnapshotSpool(dir string, workers int) (*snapshotSpool, error) {
	if err := validateSnapshotSpoolDirectory(dir); err != nil {
		return nil, err
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("snapshot spool must be a regular directory")
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return nil, err
	}
	lock, err := os.OpenFile(filepath.Join(dir, ".lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := lockSnapshotSpool(lock); err != nil {
		_ = lock.Close()
		return nil, fmt.Errorf("snapshot spool is already owned by another process: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	spool := &snapshotSpool{
		dir: dir, lock: lock, jobs: make(map[string]*snapshotSpoolJob), workers: workers,
		maxJobs: min(max(common.GetEnvOrDefault("SNAPSHOT_SPOOL_MAX_JOBS", 4096), 1), 100000),
		ctx:     ctx, cancel: cancel, wake: make(chan struct{}, workers),
	}
	if err := spool.recoverJobs(); err != nil {
		cancel()
		_ = unlockSnapshotSpool(lock)
		_ = lock.Close()
		return nil, err
	}
	return spool, nil
}

// CaptureRequestSnapshot takes ownership of bytes by durably staging them before
// it returns. Only local streaming writes happen here; object uploads and all
// archive metadata writes run in workers independent of the client context.
func CaptureRequestSnapshot(metadata SnapshotCapture, reader io.Reader, size int64) error {
	policy, profile, err := resolveRequestSnapshotPolicy()
	if err != nil {
		if !errors.Is(err, ErrRequestSnapshotsDisabled) {
			message := err.Error()
			if len(message) > 2000 {
				message = message[:2000]
			}
			requestSnapshotWorkers.Lock()
			spool := requestSnapshotWorkers.spool
			requestSnapshotWorkers.lastError = message
			requestSnapshotWorkers.Unlock()
			if spool != nil {
				spool.mu.Lock()
				spool.lastError = message
				spool.mu.Unlock()
			}
		}
		return err
	}
	if err := StartRequestSnapshotWorkers(); err != nil {
		return err
	}
	requestSnapshotWorkers.Lock()
	spool := requestSnapshotWorkers.spool
	requestSnapshotWorkers.Unlock()
	if spool == nil {
		return errors.New("request snapshot workers are stopping")
	}
	return spool.capture(metadata, reader, size, policy, profile)
}

func (spool *snapshotSpool) capture(metadata SnapshotCapture, reader io.Reader, size int64, policy *model.StoragePolicy, profile *model.StorageProfile) (captureErr error) {
	defer func() {
		if captureErr == nil {
			return
		}
		spool.mu.Lock()
		spool.lastError = captureErr.Error()
		if len(spool.lastError) > 2000 {
			spool.lastError = spool.lastError[:2000]
		}
		spool.mu.Unlock()
	}()
	if metadata.Kind != "original" && metadata.Kind != "upstream" {
		return errors.New("invalid request snapshot kind")
	}
	if metadata.TaskID == "" && metadata.RequestID == "" {
		return errors.New("request snapshot requires a task or request ID")
	}
	if reader == nil || size <= 0 || size > policy.MaxFileBytes {
		return errors.New("request snapshot exceeds the configured file limit")
	}
	if metadata.ContentType != "" && !strings.HasPrefix(strings.ToLower(metadata.ContentType), "application/json") {
		return errors.New("request snapshots currently require JSON bodies")
	}
	metadata.ContentType = "application/json"
	if metadata.CapturedAt <= 0 {
		metadata.CapturedAt = time.Now().UnixNano()
	}
	if metadata.Evidence.Version == 0 {
		metadata.Evidence = model.TaskRequestEvidence{Version: model.TaskRequestEvidenceVersion}
	}
	if metadata.NodeName == "" {
		metadata.NodeName = common.NodeName
	}
	spool.mu.Lock()
	if spool.stopping || len(spool.jobs)+spool.reservations >= spool.maxJobs || size > policy.MaxTotalBytes-spool.bytes {
		spool.mu.Unlock()
		return errors.New("request snapshot spool capacity is exhausted or stopping")
	}
	spool.bytes += size
	spool.reservations++
	spool.wg.Add(1)
	spool.mu.Unlock()
	committed := false
	defer func() {
		spool.mu.Lock()
		spool.reservations--
		if !committed {
			spool.bytes -= size
		}
		spool.mu.Unlock()
		spool.wg.Done()
	}()
	identifier, err := common.GenerateRandomCharsKey(32)
	if err != nil {
		return err
	}
	id := strings.ToLower(identifier)
	bodyPath := filepath.Join(spool.dir, id+".body")
	file, err := os.OpenFile(bodyPath+".tmp", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
		_ = os.Remove(bodyPath + ".tmp")
		if !committed {
			_ = os.Remove(bodyPath)
			_ = os.Remove(filepath.Join(spool.dir, id+".json"))
		}
	}()
	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(file, hash), io.LimitReader(reader, size+1))
	if err != nil {
		return err
	}
	if written != size {
		return errors.New("request snapshot size changed during capture")
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(bodyPath+".tmp", bodyPath); err != nil {
		return err
	}
	job := &snapshotSpoolJob{
		ID: id, SnapshotCapture: metadata,
		Profile: model.StorageProfile{ID: profile.ID, ProviderType: profile.ProviderType,
			Endpoint: profile.Endpoint, Region: profile.Region, Bucket: profile.Bucket}, PolicyID: policy.ID,
		Prefix: policy.ObjectPrefix, RetentionSeconds: policy.RetentionSeconds,
		Size: size, SHA256: hex.EncodeToString(hash.Sum(nil)), CreatedAt: time.Now().Unix(),
	}
	if err := spool.persistJob(job); err != nil {
		return err
	}
	spool.mu.Lock()
	spool.jobs[id] = job
	committed = true
	spool.mu.Unlock()
	for range spool.workers {
		select {
		case spool.wake <- struct{}{}:
		default:
		}
	}
	return nil
}

func (spool *snapshotSpool) persistJob(job *snapshotSpoolJob) error {
	body, err := common.Marshal(job)
	if err != nil {
		return err
	}
	if len(body) > 64*1024 {
		return errors.New("request snapshot manifest exceeds the metadata limit")
	}
	file, err := os.CreateTemp(spool.dir, ".manifest-*.tmp")
	if err != nil {
		return err
	}
	defer func() { _ = file.Close(); _ = os.Remove(file.Name()) }()
	if err := file.Chmod(0600); err != nil {
		return err
	}
	if _, err := file.Write(body); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(file.Name(), filepath.Join(spool.dir, job.ID+".json")); err != nil {
		return err
	}
	return syncSnapshotDirectory(spool.dir)
}

func syncSnapshotDirectory(dir string) error {
	directory, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func (spool *snapshotSpool) recoverJobs() error {
	entries, err := os.ReadDir(spool.dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || len(entry.Name()) != 37 || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("snapshot spool contains a symlink manifest")
		}
		manifest, err := os.Open(filepath.Join(spool.dir, entry.Name()))
		if err != nil {
			return err
		}
		data, readErr := io.ReadAll(io.LimitReader(manifest, 64*1024+1))
		_ = manifest.Close()
		if readErr != nil || len(data) > 64*1024 {
			return fmt.Errorf("invalid snapshot manifest: %s", entry.Name())
		}
		var job snapshotSpoolJob
		if err := common.Unmarshal(data, &job); err != nil {
			return fmt.Errorf("invalid snapshot manifest %s: %w", entry.Name(), err)
		}
		if job.ID+".json" != entry.Name() || len(job.ID) != 32 || strings.ContainsAny(job.ID, "/\\.") || job.Size <= 0 || job.Size > 512*1024*1024 || len(job.SHA256) != 64 {
			return fmt.Errorf("invalid snapshot manifest identity: %s", entry.Name())
		}
		if job.CapturedAt <= 0 {
			job.CapturedAt = job.CreatedAt * int64(time.Second)
		}
		bodyInfo, err := os.Lstat(filepath.Join(spool.dir, job.ID+".body"))
		if err != nil || !bodyInfo.Mode().IsRegular() || bodyInfo.Size() != job.Size {
			return fmt.Errorf("missing or invalid snapshot spool body: %s", job.ID)
		}
		// Lowering the admission limit must not prevent already accepted jobs
		// from recovering and draining after a restart.
		if len(spool.jobs) >= 100000 || spool.bytes > (64*1024*1024*1024)-job.Size {
			return errors.New("recovered snapshot spool exceeds the safety capacity")
		}
		spool.jobs[job.ID] = &job
		spool.bytes += job.Size
	}
	// An interrupted capture may leave a body without its committed manifest.
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".done") || strings.HasSuffix(entry.Name(), ".tmp") || (strings.HasSuffix(entry.Name(), ".body") && spool.jobs[strings.TrimSuffix(entry.Name(), ".body")] == nil) {
			if err := os.Remove(filepath.Join(spool.dir, entry.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

func (spool *snapshotSpool) runWorker() {
	defer spool.wg.Done()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		if spool.ctx.Err() != nil {
			return
		}
		job := spool.claimDueJob(time.Now().Unix())
		if job == nil {
			select {
			case <-spool.ctx.Done():
				return
			case <-spool.wake:
			case <-ticker.C:
			}
			continue
		}
		ctx, cancel := context.WithTimeout(spool.ctx, 5*time.Minute)
		err := uploadSnapshotJob(ctx, spool.dir, job)
		cancel()
		spool.finishJob(job, err)
	}
}

func (spool *snapshotSpool) claimDueJob(now int64) *snapshotSpoolJob {
	spool.mu.Lock()
	defer spool.mu.Unlock()
	activeReferences := make(map[string]bool)
	for _, job := range spool.jobs {
		if job.processing {
			activeReferences[job.Kind+":"+job.reference()] = true
		}
	}
	var selected *snapshotSpoolJob
	for _, job := range spool.jobs {
		if job.processing || job.NextAttemptAt > now || activeReferences[job.Kind+":"+job.reference()] {
			continue
		}
		if selected == nil || job.CapturedAt < selected.CapturedAt {
			selected = job
		}
	}
	if selected != nil {
		selected.processing = true
	}
	return selected
}

func (spool *snapshotSpool) finishJob(job *snapshotSpoolJob, uploadErr error) {
	spool.mu.Lock()
	defer spool.mu.Unlock()
	if uploadErr == nil {
		// Durably retire the manifest before removing its body. On a crash the
		// .done marker remains distinguishable from a pending upload manifest.
		manifest := filepath.Join(spool.dir, job.ID+".json")
		completed := filepath.Join(spool.dir, job.ID+".done")
		if err := os.Rename(manifest, completed); err != nil {
			uploadErr = err
		} else if err := syncSnapshotDirectory(spool.dir); err != nil {
			_ = os.Rename(completed, manifest)
			uploadErr = err
		} else if err := os.Remove(filepath.Join(spool.dir, job.ID+".body")); err != nil && !errors.Is(err, os.ErrNotExist) {
			_ = os.Rename(completed, manifest)
			uploadErr = err
		} else {
			_ = os.Remove(completed)
			delete(spool.jobs, job.ID)
			spool.bytes -= job.Size
			return
		}
	}
	job.Attempts++
	job.processing = false
	job.NextAttemptAt = time.Now().Add(time.Second * time.Duration(1<<min(job.Attempts, 10))).Unix()
	job.LastError = uploadErr.Error()
	if len(job.LastError) > 2000 {
		job.LastError = job.LastError[:2000]
	}
	spool.lastError = job.LastError
	if err := spool.persistJob(job); err != nil {
		spool.lastError = fmt.Sprintf("persist snapshot retry state: %v", err)
	}
	logger.LogWarn(context.Background(), fmt.Sprintf("request snapshot archive failed: request_id=%s task_id=%s kind=%s error=%v", job.RequestID, job.TaskID, job.Kind, uploadErr))
}
