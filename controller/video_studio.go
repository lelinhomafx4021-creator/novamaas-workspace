package controller

import (
	"bytes"
	"errors"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	assetService "github.com/QuantumNous/new-api/service/assetlibrary"
	storageService "github.com/QuantumNous/new-api/service/storage"

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
