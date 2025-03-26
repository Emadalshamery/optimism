//go:build ignore

package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"go/format"
	"os"
	"text/template"

	"github.com/ethereum-optimism/optimism/packages/contracts-bedrock/scripts/checks/common"
)

//go:embed source.go.tpl
var templateString string

type templateData struct {
	Package string
	Events  []string
	Methods []string
}

type processedFile struct {
	File string
	Code string
}

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Printf("CWD is %s\n", cwd)

	// Grab all the forge script artifacts
	results, err := common.ProcessFilesGlob(
		// FIXME
		[]string{"forge-artifacts/*.s.sol/*.json"},
		[]string{},
		processFile,
	)

	// Panic panic
	if err != nil {
		fmt.Printf("failed to process forge scripts artifacts: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Got %d artifacts\n", len(results))
}

func processFile(file string) (*processedFile, []error) {
	fmt.Printf("Processing %s\n", file)

	artifact, err := common.ReadForgeArtifact(file)
	if err != nil {
		return nil, []error{fmt.Errorf("failed to read forge artifact from %s: %w", file, err)}
	}

	for name, event := range artifact.Abi.Parsed.Events {
		fmt.Printf("Processing %s: Event %s: %v\n", file, name, event)
	}

	buffer := new(bytes.Buffer)
	tmpl := template.Must(template.New("").Parse(templateString))
	data := &templateData{
		Package: "fixme",
	}
	if err := tmpl.Execute(buffer, data); err != nil {
		return nil, []error{err}
	}

	code, err := format.Source(buffer.Bytes())
	if err != nil {
		return nil, []error{fmt.Errorf("%v\n%s", err, buffer)}
	}

	fmt.Printf("Processed %s:\n\n%s\n", file, code)

	return &processedFile{
		File: file,
		Code: string(code),
	}, nil
}
