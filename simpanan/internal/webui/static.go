package webui

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static
var staticFS embed.FS

// staticFileSystem strips the "static/" prefix so the URL
// /static/app.css maps to the embedded file static/app.css.
func staticFileSystem() http.FileSystem {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err) // an embed misconfiguration is a programmer bug
	}
	return http.FS(sub)
}

// indexHTML returns the bundled index.html bytes. Used by the root
// handler so / serves the SPA shell directly without a redirect.
func indexHTML() []byte {
	data, err := staticFS.ReadFile("static/index.html")
	if err != nil {
		panic(err)
	}
	return data
}
