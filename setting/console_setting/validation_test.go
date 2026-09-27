package console_setting

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReleaseAnnouncementValidation(t *testing.T) {
	announcement := map[string]interface{}{
		"id":          1,
		"kind":        "release",
		"releaseKey":  "2026.09-assets",
		"title":       "Asset Library",
		"content":     "New asset workflows",
		"publishDate": time.Now().UTC().Format(time.RFC3339),
		"imageUrl":    "/releases/assets.png",
	}
	encode := func(items ...map[string]interface{}) string {
		data, err := common.Marshal(items)
		require.NoError(t, err)
		return string(data)
	}

	require.NoError(t, validateAnnouncements(encode(announcement)))

	duplicate := map[string]interface{}{}
	for key, value := range announcement {
		duplicate[key] = value
	}
	duplicate["id"] = 2
	assert.Error(t, validateAnnouncements(encode(announcement, duplicate)))

	announcement["imageUrl"] = "javascript:alert(1)"
	assert.Error(t, validateAnnouncements(encode(announcement)))
}

func TestGetAnnouncementsPublishesScheduledReleasesOnTime(t *testing.T) {
	previous := GetConsoleSetting().Announcements
	t.Cleanup(func() { GetConsoleSetting().Announcements = previous })
	now := time.Now().UTC()
	items := []map[string]interface{}{
		{"id": 1, "content": "Live", "publishDate": now.Add(-time.Hour).Format(time.RFC3339)},
		{"id": 2, "content": "Preview", "publishDate": now.Add(time.Hour).Format(time.RFC3339)},
	}
	data, err := common.Marshal(items)
	require.NoError(t, err)
	GetConsoleSetting().Announcements = string(data)

	announcements := GetAnnouncements()
	require.Len(t, announcements, 1)
	assert.Equal(t, "Live", announcements[0]["content"])
}
