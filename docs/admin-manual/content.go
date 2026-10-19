package adminmanual

import (
	"embed"
	"strings"
)

// Content is served only by the authenticated administrator-manual endpoints.
// It must never be imported by the customer documentation or frontend build.
//
//go:embed *.md
var content embed.FS

type Document struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

var Documents []Document

func init() {
	for _, name := range []string{
		"01-account-login.md",
		"02-subscriptions.md",
		"07-plugins.md",
		"03-api-keys.md",
		"04-text-models.md",
		"05-image-models.md",
		"06-video-models.md",
	} {
		body, err := content.ReadFile(name)
		if err != nil {
			panic(err)
		}
		title, markdown, _ := strings.Cut(string(body), "\n")
		_, id, _ := strings.Cut(strings.TrimSuffix(name, ".md"), "-")
		Documents = append(Documents, Document{
			ID: id, Title: strings.TrimPrefix(title, "# "), Content: strings.TrimSpace(markdown),
		})
	}
}
