//go:build ignore

package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"go/format"
	"math/big"
	"os"
	"strings"
	"text/template"

	"github.com/ethereum-optimism/optimism/packages/contracts-bedrock/scripts/checks/common"
	_common "github.com/ethereum/go-ethereum/common"
)

//go:embed source.go.tpl
var templateString string

type templateData struct {
	Package string
	Methods []templateMethod
}

type templateMethod struct {
	Name    string
	Inputs  string
	Outputs string
}

type processedFile struct {
	File string
	Code string
}

type RunArgs struct {
	SuperchainProxyAdminOwner  _common.Address `toml:"superchainProxyAdminOwner"`
	ProtocolVersionsOwner      _common.Address `toml:"protocolVersionsOwner"`
	Guardian                   _common.Address `toml:"guardian"`
	Paused                     bool            `toml:"paused"`
	RequiredProtocolVersion    *big.Int        `toml:"requiredProtocolVersion"`
	RecommendedProtocolVersion *big.Int        `toml:"recommendedProtocolVersion"`
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
		[]string{"forge-artifacts/DeploySuperchain2.s.sol/*.json"},
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

	// var run = w3.MustNewFunc("run((address,address,address,bool,uint256,uint256) _input)", "(address,address,address,address,address) _output")

	runArgs := new(RunArgs)
	packed, err := artifact.Abi.Parsed.Pack("run", runArgs)
	if err != nil {
		return nil, []error{fmt.Errorf("failed to pack %s: %w", file, err)}
	}
	fmt.Printf("Packed: %s", packed)

	methods := []templateMethod{}
	for name, method := range artifact.Abi.Parsed.Methods {
		// if method.RawName == "emittedDeployOutputs" {
		// 	packed, err := method.Inputs.Pack([]byte{})
		// }

		methodInputs := []string{}
		for _, input := range method.Inputs {
			methodInputs = append(methodInputs, fmt.Sprintf("%s %s", input.Type.String(), input.Name))
		}

		methodOutputs := []string{}
		for _, output := range method.Outputs {
			methodOutputs = append(methodOutputs, fmt.Sprintf("%s %s", output.Type.String(), output.Name))
		}

		methods = append(methods, templateMethod{
			Name:    method.RawName,
			Inputs:  strings.Join(methodInputs, ","),
			Outputs: strings.Join(methodOutputs, ","),
		})

		fmt.Printf("Processing %s: Method %s: %v\n", file, name, method)
	}

	buffer := new(bytes.Buffer)
	tmpl := template.Must(template.New("").Parse(templateString))
	data := &templateData{
		Package: "fixme",
		Methods: methods,
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

// func formatMethod(m abi.Method) string {
// 	return fmt.Sprintf("%s()", m.RawName)
// }

// func formatArgument(a abi.Argument) string {
// 	if a.Indexed {
// 		return fmt.Sprintf("%s indexed %s", a.Name)
// 	}
// }

//Unpack
// def := fmt.Sprintf(`[{ "name" : "method", "type": "function", "outputs": %s}]`, test.def)
// abi, err := JSON(strings.NewReader(def))
// if err != nil {
// 	t.Fatalf("invalid ABI definition %s: %v", def, err)
// }
// encb, err := hex.DecodeString(test.packed)
// if err != nil {
// 	t.Fatalf("invalid hex %s: %v", test.packed, err)
// }
// out, err := abi.Unpack("method", encb)
// if err != nil {
// 	t.Errorf("test %d (%v) failed: %v", i, test.def, err)
// 	return
// }
// if !reflect.DeepEqual(test.unpacked, ConvertType(out[0], test.unpacked)) {
// 	t.Errorf("test %d (%v) failed: expected %v, got %v", i, test.def, test.unpacked, out[0])
// }
