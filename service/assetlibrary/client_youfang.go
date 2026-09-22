package assetlibrary

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

type youfangRESTClient struct {
	config     *model.AssetChannelConfig
	secret     string
	httpClient *http.Client
}

func (client *youfangRESTClient) requiresGroup() bool { return false }

func (client *youfangRESTClient) createGroup(_ context.Context, _ string, _ string) (string, error) {
	return "", errors.New("YooFang REST uses its default upstream group")
}

func (client *youfangRESTClient) createAsset(ctx context.Context, _ string, name string, assetType string, sourceURL string) (string, error) {
	payload := map[string]any{
		"url":        sourceURL,
		"name":       name,
		"asset_type": strings.ToUpper(assetType[:1]) + strings.ToLower(assetType[1:]),
	}
	result, err := client.call(ctx, http.MethodPost, nil, nil, payload)
	if err != nil {
		return "", err
	}
	if result.ID == "" {
		return "", errors.New("YooFang REST returned an empty asset ID")
	}
	return result.ID, nil
}

func (client *youfangRESTClient) getAsset(ctx context.Context, id string) (providerResult, error) {
	return client.call(ctx, http.MethodGet, []string{id}, nil, nil)
}

func (client *youfangRESTClient) deleteAsset(ctx context.Context, id string) error {
	_, err := client.call(ctx, http.MethodDelete, []string{id}, nil, nil)
	return err
}

func (client *youfangRESTClient) test(ctx context.Context) error {
	query := url.Values{"page": {"1"}, "size": {"1"}}
	_, err := client.call(ctx, http.MethodGet, nil, query, nil)
	return err
}

func (client *youfangRESTClient) call(ctx context.Context, method string, pathParts []string, query url.Values, payload any) (providerResult, error) {
	if err := waitForChannelRate(ctx, client.config.ChannelID, client.config.QPM); err != nil {
		return providerResult{}, err
	}
	endpoint, err := url.Parse(client.config.BaseURL)
	if err != nil {
		return providerResult{}, err
	}
	endpoint = endpoint.JoinPath("api", "v1", "assets")
	if len(pathParts) > 0 {
		endpoint = endpoint.JoinPath(pathParts...)
	}
	if len(query) > 0 {
		endpoint.RawQuery = query.Encode()
	}
	var body io.Reader
	if payload != nil {
		encoded, marshalErr := common.Marshal(payload)
		if marshalErr != nil {
			return providerResult{}, marshalErr
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
	if err != nil {
		return providerResult{}, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+client.secret)
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.httpClient.Do(request)
	if err != nil {
		return providerResult{}, fmt.Errorf("call YooFang REST %s: %w", method, err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 4*1024*1024))
	if err != nil {
		return providerResult{}, fmt.Errorf("read YooFang REST response: %w", err)
	}
	envelope := map[string]any{}
	if len(responseBody) > 0 {
		if err = common.Unmarshal(responseBody, &envelope); err != nil {
			return providerResult{}, fmt.Errorf("decode YooFang REST response: %w", err)
		}
	}
	if providerErr := providerResponseError(response.StatusCode, envelope); providerErr != nil {
		return providerResult{}, fmt.Errorf("YooFang REST failed: %w", providerErr)
	}
	result := envelope
	if data := mapValue(envelope, "data"); len(data) > 0 {
		result = data
	}
	return providerResult{
		ID:      stringValue(result, "id", "asset_id"),
		Status:  stringValue(result, "status"),
		Message: nestedErrorMessage(result),
	}, nil
}
