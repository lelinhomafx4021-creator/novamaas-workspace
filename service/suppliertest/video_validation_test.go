package suppliertest

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVideoMaterialValidationPreservesProviderSceneBoundaries(t *testing.T) {
	for _, test := range []struct {
		name, body string
		invalid    bool
	}{
		{"multiple references", `{"content":[{"type":"image_url","role":"reference_image","image_url":{"url":"https://example/a.png"}},{"type":"image_url","role":"reference_image","image_url":{"url":"asset://asset-b"}}]}`, false},
		{"reference plus last frame", `{"content":[{"type":"image_url","role":"reference_image","image_url":{"url":"https://example/a.png"}},{"type":"image_url","role":"last_frame","image_url":{"url":"https://example/b.png"}}]}`, true},
		{"end without start", `{"content":[{"type":"image_url","role":"last_frame","image_url":{"url":"https://example/b.png"}}]}`, true},
		{"excess duration", `{"duration":18446744073686646784}`, true},
		{"explicit zero preserved", `{"duration":0,"seed":0,"watermark":false}`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := RunRequest{Video: VideoConfig{RawPayload: test.body}}
			err := ValidateVideoDraft(req)
			if test.invalid {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
	for _, count := range []int{9, 10} {
		t.Run(fmt.Sprintf("seedance 2.0 count %d", count), func(t *testing.T) {
			body := `{"model":"doubao-seedance-2-0","content":[`
			for index := 0; index < count; index++ {
				if index > 0 {
					body += ","
				}
				body += `{"type":"image_url","role":"reference_image","image_url":{"url":"https://example/ref.png"}}`
			}
			body += "]}"
			err := ValidateVideoDraft(RunRequest{Video: VideoConfig{RawPayload: body}})
			if count == 9 {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
