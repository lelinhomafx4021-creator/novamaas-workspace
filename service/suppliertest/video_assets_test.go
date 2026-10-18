package suppliertest

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVideoUploadsReuseOneDefaultGroup(t *testing.T) {
	created := false
	creates := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		assert.Equal(t, "/api/asset-library/groups", request.URL.Path)
		assert.Equal(t, "Bearer generation-token", request.Header.Get("Authorization"))
		if request.Method == http.MethodPost {
			creates++
			body, err := io.ReadAll(request.Body)
			require.NoError(t, err)
			assert.JSONEq(t, `{"name":"default"}`, string(body))
			created = true
			_, _ = io.WriteString(w, `{"success":true,"data":{"id":"default-group","name":"default","status":"ready"}}`)
			return
		}
		if created {
			_, _ = io.WriteString(w, `{"success":true,"data":[{"id":"default-group","name":"default","status":"ready"}]}`)
		} else {
			_, _ = io.WriteString(w, `{"success":true,"data":[]}`)
		}
	}))
	defer server.Close()
	for _, expectedStages := range []int{2, 1} {
		id, stages, err := EnsureVideoDefaultGroup(context.Background(), server.Client(), server.URL, "generation-token")
		require.NoError(t, err)
		assert.Equal(t, "default-group", id)
		require.Len(t, stages, expectedStages)
		assert.Equal(t, "pass", stages[len(stages)-1].Status)
	}
	assert.Equal(t, 1, creates)
}

func TestMaterialPreparationKeepsIdentityReadinessAndOrder(t *testing.T) {
	for _, accessible := range []bool{true, false} {
		t.Run(map[bool]string{true: "ready", false: "inaccessible"}[accessible], func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				assert.Equal(t, "Bearer actual-generation-token", request.Header.Get("Authorization"))
				assert.Equal(t, "asset-one", request.URL.Query().Get("search"))
				if accessible {
					_, _ = io.WriteString(w, `{"success":true,"data":{"items":[{"id":"asset-one","name":"reference","status":"ready","size":42,"sha256":"hash"}]}}`)
				} else {
					_, _ = io.WriteString(w, `{"success":true,"data":{"items":[]}}`)
				}
			}))
			defer server.Close()
			report := &VideoRunReport{}
			err := prepareVideoAssets(context.Background(), server.Client(), RunRequest{BaseURL: server.URL, APIKey: "actual-generation-token", Video: VideoConfig{PlatformMode: true, RawPayload: `{"content":[{"type":"text","text":"animate"},{"type":"image_url","role":"reference_image","image_url":{"url":"asset://asset-one"}}]}`}}, report, func(*VideoRunReport) {})
			if accessible {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			require.Len(t, report.Assets, 1)
			assert.Equal(t, 0, report.Assets[0].Index)
			assert.True(t, report.Assets[0].Reused)
			require.Len(t, report.Stages, 1)
			assert.Contains(t, report.Stages[0].Path, "search=asset-one")
		})
	}
}
