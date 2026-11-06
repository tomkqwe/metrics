package main

import (
	"bytes"
	"fmt"
	"text/template"
)

// resetFileTemplate owns the file and method skeleton; AST traversal only
// collects method metadata and field reset statements.
const resetFileTemplate = `{{.Header}}
package {{.Package}}

{{range .Methods}}// Reset clears {{.Name}} while retaining slices, maps and pointers.
func (rs *{{.Receiver}}) Reset() {
 if rs == nil { return }
{{.Body}}}

{{end}}`

type resetMethod struct {
	Name     string
	Receiver string
	Body     string
}

func renderFile(packageName string, methods []resetMethod) ([]byte, error) {
	tmpl, err := template.New("reset.gen.go").Parse(resetFileTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse reset template: %w", err)
	}
	var out bytes.Buffer
	data := struct {
		Header  string
		Package string
		Methods []resetMethod
	}{generatedHeader, packageName, methods}
	if err := tmpl.Execute(&out, data); err != nil {
		return nil, fmt.Errorf("render reset template: %w", err)
	}
	return out.Bytes(), nil
}
