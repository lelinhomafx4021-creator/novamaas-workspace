package videostudio

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
)

// RunStatusSubmitting marks a run whose task is not linked yet; every other
// status is the linked Task's own video status.
const RunStatusSubmitting = "submitting"

// A submission still unlinked after this long ended without creating a task.
const staleSubmissionAfter = 10 * time.Minute

const (
	maxProjectNameRunes = 128
	// Drafts carry prompts and layout only; reference files are stored by ID.
	// Stays under the 64 KiB MySQL TEXT limit shared by every supported DB.
	maxProjectJSONBytes = 60 * 1024
)

type ProjectInput struct {
	Name     *string `json:"name"`
	Draft    *string `json:"draft"`
	Canvas   *string `json:"canvas"`
	Revision int64   `json:"revision"`
}

type ProjectPage struct {
	Items    []model.VideoProject `json:"items"`
	Total    int64                `json:"total"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"page_size"`
}

type RunView struct {
	model.VideoCreationRun
	Status     string `json:"status"`
	Progress   string `json:"progress"`
	FailReason string `json:"fail_reason,omitempty"`
	Quota      int    `json:"quota"`
	FinishedAt int64  `json:"finished_at,omitempty"`
}

type RunPage struct {
	Items    []RunView `json:"items"`
	Total    int64     `json:"total"`
	Page     int       `json:"page"`
	PageSize int       `json:"page_size"`
}

func CreateProject(ownerUserID int, input ProjectInput) (*model.VideoProject, error) {
	project := &model.VideoProject{OwnerUserID: ownerUserID, Name: "Untitled project", Draft: "{}", Canvas: "{}", Revision: 1}
	if err := applyProjectInput(project, input); err != nil {
		return nil, err
	}
	key, err := common.GenerateRandomCharsKey(16)
	if err != nil {
		return nil, err
	}
	now := common.GetTimestamp()
	project.PublicID = "vp_" + strings.ToLower(key)
	project.CreatedAt = now
	project.UpdatedAt = now
	if err = model.DB.Create(project).Error; err != nil {
		return nil, err
	}
	return project, nil
}

func ListProjects(ownerUserID int, page int, pageSize int) (*ProjectPage, error) {
	page, pageSize = normalizePage(page, pageSize)
	projects, total, err := model.ListVideoProjects(ownerUserID, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, err
	}
	return &ProjectPage{Items: projects, Total: total, Page: page, PageSize: pageSize}, nil
}

// SaveProject writes only the fields the client sent, so the mini program
// can save the draft without touching the web canvas layout.
func SaveProject(ownerUserID int, publicID string, input ProjectInput) (*model.VideoProject, error) {
	if input.Revision <= 0 {
		return nil, badRequest("revision is required")
	}
	staged := &model.VideoProject{}
	if err := applyProjectInput(staged, input); err != nil {
		return nil, err
	}
	updates := map[string]any{}
	if input.Name != nil {
		updates["name"] = staged.Name
	}
	if input.Draft != nil {
		updates["draft"] = staged.Draft
	}
	if input.Canvas != nil {
		updates["canvas"] = staged.Canvas
	}
	project, err := model.UpdateVideoProject(ownerUserID, publicID, input.Revision, updates)
	if errors.Is(err, model.ErrVideoProjectRevisionConflict) {
		return nil, &RequestError{StatusCode: http.StatusConflict, Err: err}
	}
	return project, err
}

func applyProjectInput(project *model.VideoProject, input ProjectInput) error {
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" || utf8.RuneCountInString(name) > maxProjectNameRunes {
			return badRequest("project name must contain 1 to %d characters", maxProjectNameRunes)
		}
		project.Name = name
	}
	for _, field := range []struct {
		value  *string
		target *string
		name   string
	}{{input.Draft, &project.Draft, "draft"}, {input.Canvas, &project.Canvas, "canvas"}} {
		if field.value == nil {
			continue
		}
		if len(*field.value) > maxProjectJSONBytes {
			return &RequestError{StatusCode: http.StatusRequestEntityTooLarge, Err: fmt.Errorf("%s exceeds %d bytes", field.name, maxProjectJSONBytes)}
		}
		var decoded map[string]any
		if err := common.UnmarshalJsonStr(*field.value, &decoded); err != nil {
			return badRequest("%s must be a JSON object", field.name)
		}
		*field.target = *field.value
	}
	return nil
}

// CheckReplay accepts a retried submission only when it repeats the stored
// one exactly; reusing a request ID for different content is a conflict.
func CheckReplay(run *model.VideoCreationRun, input *SubmitInput) error {
	hash, err := input.Hash()
	if err != nil {
		return err
	}
	if run.RequestHash != hash {
		return &RequestError{StatusCode: http.StatusConflict, Err: errors.New("client_request_id was already used for a different submission")}
	}
	return nil
}

// CreateRun reserves the generation record and its public task ID before the
// upstream call, so a retried submission never sends a second task.
func CreateRun(prepared *PreparedSubmission) (*model.VideoCreationRun, bool, error) {
	run := &model.VideoCreationRun{
		PublicID:        "vr_" + strings.TrimPrefix(model.GenerateTaskID(), "task_"),
		ProjectID:       prepared.Project.ID,
		NodeID:          prepared.Input.NodeID,
		OwnerUserID:     prepared.Project.OwnerUserID,
		ClientRequestID: prepared.Input.ClientRequestID,
		RequestHash:     prepared.RequestHash,
		Snapshot:        prepared.Snapshot,
		TaskID:          model.GenerateTaskID(),
		CreatedAt:       common.GetTimestamp(),
	}
	if err := model.DB.Create(run).Error; err != nil {
		// A concurrent request with the same client ID won the unique index.
		existing, findErr := model.FindVideoCreationRunByClientRequest(run.OwnerUserID, run.ClientRequestID)
		if findErr != nil {
			return nil, false, err
		}
		if existing.RequestHash != run.RequestHash {
			return nil, false, &RequestError{StatusCode: http.StatusConflict, Err: errors.New("client_request_id was already used for a different submission")}
		}
		return existing, true, nil
	}
	return run, false, nil
}

// FailRun records a submission that never produced a task so the canvas can
// show the reason; the client must create a new request ID to retry.
func FailRun(run *model.VideoCreationRun, message string) {
	if len(message) > 2000 {
		message = message[:2000]
	}
	if err := model.DB.Model(&model.VideoCreationRun{}).Where("id = ?", run.ID).Update("error_message", message).Error; err != nil {
		common.SysError("record video studio run failure: " + err.Error())
	}
	run.ErrorMessage = message
}

func ListRuns(ownerUserID int, project *model.VideoProject, page int, pageSize int) (*RunPage, error) {
	page, pageSize = normalizePage(page, pageSize)
	runs, total, err := model.ListVideoCreationRuns(ownerUserID, project.ID, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, err
	}
	taskIDs := make([]any, 0, len(runs))
	for _, run := range runs {
		taskIDs = append(taskIDs, run.TaskID)
	}
	tasks, err := model.GetByTaskIds(ownerUserID, taskIDs)
	if err != nil {
		return nil, err
	}
	tasksByID := make(map[string]*model.Task, len(tasks))
	for _, task := range tasks {
		tasksByID[task.TaskID] = task
	}
	items := make([]RunView, 0, len(runs))
	for _, run := range runs {
		items = append(items, runView(run, tasksByID[run.TaskID]))
	}
	return &RunPage{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func RunViewFor(ownerUserID int, run *model.VideoCreationRun) RunView {
	task, exists, err := model.GetByTaskId(ownerUserID, run.TaskID)
	if err != nil || !exists {
		task = nil
	}
	return runView(*run, task)
}

// runView reports the Task's own status. A run without a task either failed
// before submission (error_message) or is still being submitted.
func runView(run model.VideoCreationRun, task *model.Task) RunView {
	view := RunView{VideoCreationRun: run}
	if task == nil {
		view.Status = RunStatusSubmitting
		if run.ErrorMessage != "" {
			view.Status = dto.VideoStatusFailed
			view.FailReason = run.ErrorMessage
		} else if common.GetTimestamp()-run.CreatedAt > int64(staleSubmissionAfter/time.Second) {
			// The submit request ended without linking a task; nothing was charged.
			view.Status = dto.VideoStatusFailed
			view.FailReason = "submission did not complete"
		}
		return view
	}
	view.Status = task.Status.ToVideoStatus()
	// A freshly inserted task stays NOT_START until the first poll.
	if task.Status == model.TaskStatusNotStart {
		view.Status = dto.VideoStatusQueued
	}
	view.Progress = task.Progress
	view.Quota = task.Quota
	view.FinishedAt = task.FinishTime
	if task.Status == model.TaskStatusFailure {
		view.FailReason = task.FailReason
	}
	return view
}

func normalizePage(page int, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize
}
