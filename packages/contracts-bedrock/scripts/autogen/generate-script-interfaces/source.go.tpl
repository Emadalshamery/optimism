// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package {{.Package}}

import (
	"github.com/lmittmann/w3"
)

{{range .Methods}}
	var {{.Name}} = w3.MustNewFunc("{{.Name}}({{.Inputs}})", "{{.Outputs}}")
{{end}}