package controller

import (
	"bytes"
	"errors"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	assetService "github.com/QuantumNous/new-api/service/assetlibrary"
	storageService "github.com/QuantumNous/new-api/service/storage"
	"github.com/QuantumNous/new-api/service/videostudio"
	"github.com/QuantumNous/new-api/setting/model_setting"

	"github.com/gin-gonic/gin"
)

const videoStudioMaxBase64Bytes = 16 * 1024 * 1024

func GetVideoStudioStoragePolicy(c *gin.Context) {
	policy, err := storageService.GetVideoStudioPolicy()
	if err != nil {
		storageAPIError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": policy})
}

func UpdateVideoStudioStoragePolicy(c *gin.Context) {
	var input storageService.PolicyInput
	if err := common.DecodeJson(c.Request.Body, &input); err != nil {
		storageAPIError(c, http.StatusBadRequest, errors.New("invalid storage policy request"))
		return
	}
	policy, err := storageService.SaveVideoStudioPolicy(input)
	if err != nil {
		storageAPIError(c, http.StatusBadRequest, err)
		return
	}
	recordManageAudit(c, "storage.policy_update", map[string]interface{}{"key": policy.Key, "profile_id": policy.StorageProfileID, "enabled": policy.Enabled})
	c.JSON(http.StatusOK, gin.H{"success": true, "data": policy})
}

// UploadVideoStudioReference accepts a multipart "file" or a JSON body with a
// base64 data URI, so the web client and the mini program share one path.
func UploadVideoStudioReference(c *gin.Context) {
	policy, err := storageService.GetVideoStudioPolicy()
	if err != nil {
		storageAPIError(c, http.StatusInternalServerError, err)
		return
	}
	// Base64 inflates the payload by 4/3; leave headroom for multipart framing.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, min(policy.MaxFileBytes*4/3+64*1024, videoStudioMaxBase64Bytes))
	if strings.HasPrefix(c.ContentType(), "application/json") {
		var input struct {
			FileName string `json:"file_name"`
			DataURI  string `json:"data_uri"`
		}
		if err = common.DecodeJson(c.Request.Body, &input); err != nil {
			storageAPIError(c, http.StatusRequestEntityTooLarge, errors.New("reference image is invalid or exceeds the size limit"))
			return
		}
		payload, decodeErr := storageService.DecodeImageDataURI(input.DataURI)
		if decodeErr != nil {
			storageAPIError(c, http.StatusBadRequest, decodeErr)
			return
		}
		reference, uploadErr := storageService.UploadVideoStudioReference(c.Request.Context(), c.GetInt("id"), input.FileName, bytes.NewReader(payload))
		respondVideoStudioReference(c, reference, uploadErr)
		return
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		storageAPIError(c, http.StatusRequestEntityTooLarge, errors.New("reference image is required or exceeds the size limit"))
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		storageAPIError(c, http.StatusBadRequest, err)
		return
	}
	defer file.Close()
	reference, err := storageService.UploadVideoStudioReference(c.Request.Context(), c.GetInt("id"), fileHeader.Filename, file)
	respondVideoStudioReference(c, reference, err)
}

func ListVideoStudioReferences(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	references, err := storageService.ListVideoStudioReferences(c.GetInt("id"), c.Query("include_temporary") == "true", pageInfo.GetPage(), pageInfo.GetPageSize())
	if err != nil {
		storageAPIError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": references})
}

func GetVideoStudioReferencePreview(c *gin.Context) {
	value, expiresAt, err := storageService.PresignVideoStudioReference(c.Request.Context(), c.GetInt("id"), c.Param("id"), c.Query("variant") == "thumbnail")
	if err != nil {
		storageAPIError(c, storageAPIStatus(err, http.StatusInternalServerError), err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"url": value, "expires_at": expiresAt}})
}

func SaveVideoStudioReference(c *gin.Context) {
	reference, err := storageService.SaveVideoStudioReference(c.GetInt("id"), c.Param("id"))
	respondVideoStudioReference(c, reference, err)
}

func DeleteVideoStudioReference(c *gin.Context) {
	if err := storageService.DeleteVideoStudioReference(c.GetInt("id"), c.Param("id")); err != nil {
		storageAPIError(c, storageAPIStatus(err, http.StatusInternalServerError), err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func RegisterVideoStudioReferenceAsset(c *gin.Context) {
	var input struct {
		GroupID string `json:"group_id"`
		Name    string `json:"name"`
	}
	if err := common.DecodeJson(c.Request.Body, &input); err != nil || strings.TrimSpace(input.GroupID) == "" {
		assetLibraryError(c, &assetService.RequestError{StatusCode: http.StatusBadRequest, Err: errors.New("asset group is required")})
		return
	}
	asset, err := assetService.CreateAssetFromVideoStudioReference(c.GetInt("id"), strings.TrimSpace(input.GroupID), c.Param("id"), input.Name)
	if err != nil {
		assetLibraryError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": asset})
}

func respondVideoStudioReference(c *gin.Context, reference *storageService.VideoStudioReference, err error) {
	if err != nil {
		storageAPIError(c, storageAPIStatus(err, http.StatusInternalServerError), err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": reference})
}

// videoStudioKey resolves the API key a studio request runs under. A zero ID
// means the account's default group with no model whitelist.
func videoStudioKey(userID int, tokenID int) (*model.Token, error) {
	if tokenID == 0 {
		return nil, nil
	}
	token, err := model.GetTokenByIds(tokenID, userID)
	if err != nil {
		return nil, &videostudio.RequestError{StatusCode: http.StatusNotFound, Err: errors.New("api key does not exist")}
	}
	if token.Status != common.TokenStatusEnabled || (token.ExpiredTime != -1 && token.ExpiredTime < common.GetTimestamp()) {
		return nil, &videostudio.RequestError{StatusCode: http.StatusForbidden, Err: errors.New("api key is disabled or expired")}
	}
	return token, nil
}

// availableForKey narrows the group's models to the key's whitelist.
func availableForKey(available map[string]bool, token *model.Token) map[string]bool {
	if token == nil || !token.ModelLimitsEnabled {
		return available
	}
	limits := token.GetModelLimitsMap()
	allowed := make(map[string]bool, len(limits))
	for name := range available {
		if limits[name] {
			allowed[name] = true
		}
	}
	return allowed
}

// ListVideoStudioModels returns studio-capable models the chosen key can
// reach, so the canvas never offers a model without an available channel.
func ListVideoStudioModels(c *gin.Context) {
	token, err := videoStudioKey(c.GetInt("id"), common.String2Int(c.Query("token_id")))
	if err != nil {
		abortVideoStudio(c, err)
		return
	}
	group := c.Query("group")
	if token != nil {
		group = token.Group
	}
	available, ok := playgroundAvailableModels(c, group)
	if !ok {
		return
	}
	available = availableForKey(available, token)
	enabled := make([]string, 0, len(available))
	for name := range available {
		enabled = append(enabled, name)
	}
	promptModel := model_setting.GetVideoStudioPromptModel()
	if !available[promptModel] {
		promptModel = ""
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"models":       videostudio.AvailableCapabilities(enabled),
		"prompt_model": promptModel,
	}})
}

func ListVideoStudioProjects(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	projects, err := videostudio.ListProjects(c.GetInt("id"), pageInfo.GetPage(), pageInfo.GetPageSize())
	respondVideoStudio(c, projects, err)
}

func CreateVideoStudioProject(c *gin.Context) {
	var input videostudio.ProjectInput
	if err := common.DecodeJson(c.Request.Body, &input); err != nil {
		storageAPIError(c, http.StatusBadRequest, errors.New("invalid project request"))
		return
	}
	project, err := videostudio.CreateProject(c.GetInt("id"), input)
	respondVideoStudio(c, project, err)
}

func GetVideoStudioProject(c *gin.Context) {
	project, err := model.FindVideoProject(c.GetInt("id"), c.Param("id"))
	respondVideoStudio(c, project, err)
}

func UpdateVideoStudioProject(c *gin.Context) {
	var input videostudio.ProjectInput
	if err := common.DecodeJson(c.Request.Body, &input); err != nil {
		storageAPIError(c, http.StatusBadRequest, errors.New("invalid project request"))
		return
	}
	project, err := videostudio.SaveProject(c.GetInt("id"), c.Param("id"), input)
	respondVideoStudio(c, project, err)
}

func DeleteVideoStudioProject(c *gin.Context) {
	respondVideoStudio(c, nil, model.DeleteVideoProject(c.GetInt("id"), c.Param("id")))
}

func ListVideoStudioRuns(c *gin.Context) {
	project, err := model.FindVideoProject(c.GetInt("id"), c.Param("id"))
	if err != nil {
		respondVideoStudio(c, nil, err)
		return
	}
	pageInfo := common.GetPageQuery(c)
	runs, err := videostudio.ListRuns(c.GetInt("id"), project, pageInfo.GetPage(), pageInfo.GetPageSize())
	respondVideoStudio(c, runs, err)
}

// ValidatePlaygroundVideo checks a studio submission and replaces the body with
// the standard /v1/video/generations task request before channel selection,
// so Distribute, retries, billing, and provider adaptors run unchanged. It
// writes nothing: a submission rejected before a channel is chosen leaves no
// run behind.
func ValidatePlaygroundVideo(c *gin.Context) {
	if c.GetBool("use_access_token") {
		abortVideoStudio(c, &videostudio.RequestError{StatusCode: http.StatusForbidden, Err: errors.New("access tokens cannot submit studio videos")})
		return
	}
	var input videostudio.SubmitInput
	if err := common.UnmarshalBodyReusable(c, &input); err != nil {
		abortVideoStudio(c, &videostudio.RequestError{StatusCode: http.StatusBadRequest, Err: errors.New("invalid video studio request")})
		return
	}
	userID := c.GetInt("id")
	key, err := videoStudioKey(userID, input.TokenID)
	if err != nil {
		abortVideoStudio(c, err)
		return
	}
	if key != nil {
		// The chosen key decides the group; a client-sent group is ignored.
		input.Group = key.Group
	}
	available, ok := playgroundAvailableModels(c, input.Group)
	if !ok {
		return
	}
	available = availableForKey(available, key)
	// A replayed request answers from the stored run before any revalidation:
	// its references may have expired since, but the task already exists.
	if run, err := model.FindVideoCreationRunByClientRequest(userID, input.ClientRequestID); err == nil {
		if err = videostudio.CheckReplay(run, &input); err != nil {
			abortVideoStudio(c, err)
			return
		}
		c.AbortWithStatusJSON(http.StatusOK, gin.H{"success": true, "data": videostudio.RunViewFor(userID, run)})
		return
	}
	prepared, err := videostudio.PrepareSubmission(c.Request.Context(), userID, &input, available[input.Model])
	if err != nil {
		abortVideoStudio(c, err)
		return
	}
	data, err := common.Marshal(prepared.TaskBody)
	if err != nil {
		abortVideoStudio(c, err)
		return
	}
	storage, err := common.CreateBodyStorage(data)
	if err != nil {
		abortVideoStudio(c, err)
		return
	}
	common.CleanupBodyStorage(c)
	c.Set(common.KeyBodyStorage, storage)
	c.Request.Body = storage
	c.Request.ContentLength = int64(len(data))
	c.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(c, constant.ContextKeyVideoStudioSubmission, prepared)
	if err = setupPlaygroundToken(c, input.Group); err != nil {
		abortVideoStudio(c, err)
		return
	}
	if key != nil && key.ModelLimitsEnabled {
		// Keep the key's whitelist so Distribute enforces it on the real channel pick.
		c.Set("token_model_limit_enabled", true)
		c.Set("token_model_limit", key.GetModelLimitsMap())
	}
}

// PlaygroundVideo runs after a channel is selected. It reserves the run (and
// its public task ID) before the upstream call, submits through the shared
// task path, and answers with the run instead of the provider-shaped body.
func PlaygroundVideo(c *gin.Context) {
	prepared, ok := common.GetContextKeyType[*videostudio.PreparedSubmission](c, constant.ContextKeyVideoStudioSubmission)
	if !ok {
		respondVideoStudio(c, nil, errors.New("video studio submission was not prepared"))
		return
	}
	run, replayed, err := videostudio.CreateRun(prepared)
	if err != nil {
		respondVideoStudio(c, nil, err)
		return
	}
	userID := c.GetInt("id")
	if replayed {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": videostudio.RunViewFor(userID, run)})
		return
	}
	if _, taskErr := submitVideoTask(c, run.TaskID, true); taskErr != nil {
		videostudio.FailRun(run, taskErr.Message)
		respondTaskError(c, taskErr)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": videostudio.RunViewFor(userID, run)})
}

func respondVideoStudio(c *gin.Context, data any, err error) {
	if err != nil {
		storageAPIError(c, storageAPIStatus(err, http.StatusInternalServerError), err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func abortVideoStudio(c *gin.Context, err error) {
	respondVideoStudio(c, nil, err)
	c.Abort()
}
