package templates

import (
	"bytes"
	"embed"
	"fmt"
	"path/filepath"
	"text/template"
)

//go:embed *.tmpl
var files embed.FS

func Render(name string, data any) ([]byte, error) {
	parsed, err := template.ParseFS(files, filepath.Base(name))
	if err != nil {
		return nil, fmt.Errorf("parse template %s: %w", name, err)
	}
	var output bytes.Buffer
	if err := parsed.ExecuteTemplate(&output, filepath.Base(name), data); err != nil {
		return nil, fmt.Errorf("render template %s: %w", name, err)
	}
	return output.Bytes(), nil
}
