package apidocs

import (
	"embed"
	"io/fs"
	"strings"
)

//go:embed *.md
var content embed.FS

type Document struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

// Documents is embedded in the server, never in the public frontend bundle.
var Documents []Document

func init() {
	names, err := fs.Glob(content, "*.md")
	if err != nil {
		panic(err)
	}
	for _, name := range names {
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
