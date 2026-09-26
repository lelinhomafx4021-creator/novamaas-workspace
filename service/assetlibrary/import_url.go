package assetlibrary

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	coreService "github.com/QuantumNous/new-api/service"
	storageService "github.com/QuantumNous/new-api/service/storage"
)

func CreateAssetFromURL(ctx context.Context, ownerUserID int, groupPublicID string, name string, assetType string, sourceURL string) (*AssetView, error) {
	parsedURL, err := url.Parse(strings.TrimSpace(sourceURL))
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
		return nil, &RequestError{StatusCode: http.StatusBadRequest, Err: errors.New("asset URL must be an absolute HTTP or HTTPS URL")}
	}
	policy, err := storageService.GetAssetLibraryPolicy()
	if err != nil {
		return nil, err
	}
	if policy.MaxFileBytes <= 0 {
		return nil, &RequestError{StatusCode: http.StatusServiceUnavailable, Err: errors.New("asset library storage is not configured")}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return nil, &RequestError{StatusCode: http.StatusBadRequest, Err: errors.New("invalid asset URL")}
	}
	request.Header.Set("Accept", "image/*,video/*,audio/*")
	response, err := coreService.GetSSRFProtectedHTTPClient().Do(request)
	if err != nil {
		return nil, &RequestError{StatusCode: http.StatusBadGateway, Err: fmt.Errorf("download asset URL: %w", err)}
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, &RequestError{StatusCode: http.StatusBadGateway, Err: fmt.Errorf("asset URL returned HTTP %d", response.StatusCode)}
	}
	if response.ContentLength > policy.MaxFileBytes {
		return nil, &RequestError{StatusCode: http.StatusRequestEntityTooLarge, Err: errors.New("asset URL exceeds the configured size limit")}
	}

	temporaryFile, err := os.CreateTemp("", "asset-library-import-*")
	if err != nil {
		return nil, err
	}
	temporaryPath := temporaryFile.Name()
	defer os.Remove(temporaryPath)
	defer temporaryFile.Close()
	written, err := io.Copy(temporaryFile, io.LimitReader(response.Body, policy.MaxFileBytes+1))
	if err != nil {
		return nil, &RequestError{StatusCode: http.StatusBadGateway, Err: fmt.Errorf("download asset URL: %w", err)}
	}
	if written <= 0 || written > policy.MaxFileBytes {
		return nil, &RequestError{StatusCode: http.StatusRequestEntityTooLarge, Err: errors.New("asset URL is empty or exceeds the configured size limit")}
	}
	if _, err = temporaryFile.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	header := make([]byte, 512)
	read, readErr := io.ReadFull(temporaryFile, header)
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) && !errors.Is(readErr, io.EOF) {
		return nil, readErr
	}
	contentType := strings.ToLower(strings.TrimSpace(strings.SplitN(http.DetectContentType(header[:read]), ";", 2)[0]))
	declaredType := strings.ToLower(strings.TrimSpace(strings.SplitN(response.Header.Get("Content-Type"), ";", 2)[0]))
	if contentType == "application/octet-stream" && (strings.HasPrefix(declaredType, "image/") || strings.HasPrefix(declaredType, "video/") || strings.HasPrefix(declaredType, "audio/")) {
		contentType = declaredType
	}
	if _, err = temporaryFile.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	fileName := path.Base(parsedURL.Path)
	if fileName == "." || fileName == "/" || fileName == "" {
		fileName = name
	}
	return CreateAsset(ctx, ownerUserID, AssetInput{
		GroupPublicID: groupPublicID,
		Name:          name,
		AssetType:     strings.ToLower(strings.TrimSpace(assetType)),
		FileName:      fileName,
		ContentType:   contentType,
		Size:          written,
		Body:          temporaryFile,
	})
}
