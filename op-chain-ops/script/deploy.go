package script

import (
	"fmt"
	"reflect"

	"github.com/ethereum-optimism/optimism/op-chain-ops/foundry"
	"github.com/ethereum-optimism/optimism/op-chain-ops/script/addresses"
	"github.com/holiman/uint256"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	deployScriptRunMethodName = "run"
)

// DeployScriptFactory can deploy DeployScript instances
type DeployScriptFactory[I any, O any] interface {
	Create() (script DeployScript[I, O], err error)
}

// DeployScript is an instance of a forge script deployed to a chain
type DeployScript[I any, O any] interface {
	Run(input I) (output *O, err error)
	Teardown()
}

// DeployScriptIOFactory creates a DeployScriptIO based on an ABI
//
// Its main responsibility is to validate the ABI agains I and O
type DeployScriptIOFactory[I any, O any] interface {
	Create(abi abi.ABI) (io DeployScriptIO[I, O], err error)
}

// DeployScriptIO is an input/output decoder for a DeployScript
type DeployScriptIO[I any, O any] interface {
	EncodeInput(input I) (data []byte, err error)
	DecodeOutput(data []byte) (output O, err error)
}

var (
	_ DeployScriptIO[any, any]        = (*deployScriptIOImpl[any, any])(nil)
	_ DeployScriptIOFactory[any, any] = (*deployScriptIOFactoryImpl[any, any])(nil)
)

type deployScriptIOFactoryImpl[I any, O any] struct {
	testInput  I
	testOutput O
	methodName string
}

// Create implements DeployScriptIOFactory.
func (f *deployScriptIOFactoryImpl[I, O]) Create(abi abi.ABI) (DeployScriptIO[I, O], error) {
	// make sure:
	//
	// - the constructor can be packed using no arguments
	// - the run() can be packed using the provided input
	// - the run() return value can be packed using the provided output

	// First we make sure that we can pack the constructor without any args
	_, err := abi.Pack("")
	if err != nil {
		return nil, fmt.Errorf("script can't be instantiated without arguments (constructor has signature %s): %w", abi.Constructor.Sig, err)
	}

	// Now we make sure the run method exists
	methodAbi, ok := abi.Methods[f.methodName]
	if !ok {
		return nil, fmt.Errorf("contract is missing a %s method", f.methodName)
	}

	if len(methodAbi.Inputs) != 1 {
		return nil, fmt.Errorf("contract has an incompatible %s method: expected single argument, got %s", f.methodName, methodAbi.Sig)
	}

	err = matchTypes(methodAbi.Inputs[0].Type, reflect.TypeOf(*new(I)))
	if err != nil {
		return nil, fmt.Errorf("input type mismatch: %w", err)
	}

	// Now we make sure that the run method can be packed using the provided input
	_, err = methodAbi.Inputs.Pack(f.testInput)
	if err != nil {
		return nil, fmt.Errorf("contract has an incompatible %s method (has signature %s): %w", f.methodName, methodAbi.Sig, err)
	}

	if len(methodAbi.Outputs) != 1 {
		return nil, fmt.Errorf("contract has an incompatible %s method: expected single return value, got %d", f.methodName, len(methodAbi.Outputs))
	}

	// Now we make sure that the run output can be packed using the provided output
	_, err = methodAbi.Outputs.Pack(f.testOutput)
	if err != nil {
		return nil, fmt.Errorf("contract has an incompatible run method return value: %w", err)
	}

	err = matchTypes(methodAbi.Outputs[0].Type, reflect.TypeOf(*new(O)))
	if err != nil {
		return nil, fmt.Errorf("output type mismatch: %w", err)
	}

	// At this point we know that the script has the ABI we'd like it to have
	return &deployScriptIOImpl[I, O]{
		abi:        abi,
		methodName: f.methodName,
	}, nil
}

type deployScriptIOImpl[I any, O any] struct {
	abi        abi.ABI
	methodName string
}

func (io *deployScriptIOImpl[I, O]) DecodeOutput(data []byte) (output O, err error) {
	unpacked, err := io.abi.Unpack(io.methodName, data)
	if err != nil {
		return output, fmt.Errorf("failed to decode output data (%s): %w", common.Bytes2Hex(data), err)
	}

	output = *abi.ConvertType(unpacked, new(O)).(*O)

	return output, nil
}

func (io *deployScriptIOImpl[I, O]) EncodeInput(input I) (data []byte, err error) {
	packed, err := io.abi.Pack(io.methodName, input)
	if err != nil {
		return nil, fmt.Errorf("failed to encode input (%v): %w", input, err)
	}

	return packed, nil
}

type deployScriptFactoryImpl[I any, O any] struct {
	artifact     *foundry.Artifact
	contractName string
	host         *Host
}

func (f *deployScriptFactoryImpl[I, O]) Create() (script DeployScript[I, O], err error) {
	deployer := addresses.ScriptDeployer
	deployNonce := f.host.state.GetNonce(deployer)

	// Compute address of script contract to be deployed
	address := crypto.CreateAddress(deployer, deployNonce)

	// Label the address using the contract name
	f.host.Label(address, f.contractName)

	// TODO Check that we can run other scripts if we only enable cheatcodes for this address
	f.host.AllowCheatcodes(address)    // before constructor execution, give our script cheatcode access
	f.host.state.MakeExcluded(address) // scripts are persistent across forks

	// disable contract size constraints
	f.host.EnforceMaxCodeSize(false)
	defer f.host.EnforceMaxCodeSize(true)

	// deploy the script
	deployedAddr, err := f.host.Create(deployer, f.artifact.Bytecode.Object)
	if err != nil {
		return nil, fmt.Errorf("failed to deploy script %s: %w", f.contractName, err)
	}

	// make sure we deployed to the expected address
	if deployedAddr != address {
		return nil, fmt.Errorf("deployed script %s to unexpected address %s, expected %s", f.contractName, deployedAddr, address)
	}

	// save the contract source map
	f.host.RememberArtifact(address, f.artifact, f.contractName)

	// and return a script
	return &deployScriptImpl[I, O]{
		abi:          &f.artifact.ABI,
		address:      address,
		contractName: f.contractName,
		host:         f.host,
		torndown:     false,
	}, nil
}

type deployScriptImpl[I any, O any] struct {
	abi          *abi.ABI
	address      common.Address
	contractName string
	host         *Host
	torndown     bool
}

func (s *deployScriptImpl[I, O]) Run(input I) (output *O, err error) {
	if s.torndown {
		return nil, fmt.Errorf("script %s is alredy torn down", s.contractName)
	}

	data, err := s.abi.Pack(deployScriptRunMethodName, &input)
	if err != nil {
		return nil, fmt.Errorf("failed to encode run call for %s: %w", s.contractName, err)
	}

	result, _, err := s.host.Call(s.host.env.TxContext().Origin, s.address, data, DefaultFoundryGasLimit, uint256.NewInt(0))
	if err != nil {
		return nil, fmt.Errorf("failed to run script for %s: %w", s.contractName, err)
	}

	output, err = unpackIntoInterface[O](s.abi, deployScriptRunMethodName, result)
	if err != nil {
		return nil, fmt.Errorf("failed to encode run call for %s: %w", s.contractName, err)
	}

	return output, nil
}

func (s *deployScriptImpl[I, O]) Teardown() {
	s.torndown = true

	s.host.Wipe(s.address)
}

func LoadDeployScript[I any, O any](host *Host, fileName string, contractName string, validator DeployScriptAbiValidator[I, O], zeroInput O) (DeployScriptFactory[I, O], error) {
	// First we load the contract artifact
	artifact, err := host.af.ReadArtifact(fileName, contractName)
	if err != nil {
		return nil, fmt.Errorf("could not load deploy script artifact for contract %s from %s: %w", contractName, fileName, err)
	}

	// Now we validate the ABI against the validator
	if err = validator(artifact.ABI); err != nil {
		return nil, fmt.Errorf("deploy script %s from %s does not have matching ABI: %w", contractName, fileName, err)
	}

	// Now that we know we have a valid ABI, we can create the factory
	return &deployScriptFactoryImpl[I, O]{
		artifact:     artifact,
		contractName: contractName,
		host:         host,
	}, nil
}

type DeployScriptAbiValidator[I any, O any] func(scriptAbi abi.ABI) error

func NewDeployScriptAbiValidator[I any, O any](
	zeroInput I,
	zeroOutput O,
) DeployScriptAbiValidator[I, O] {
	return func(scriptAbi abi.ABI) error {
		// make sure:
		//
		// - the constructor can be packed using no arguments
		// - the run() can be packed using the provided input
		// - the run() return value can be packed using the provided output

		// First we make sure that we can pack the constructor without any args
		_, err := scriptAbi.Pack("")
		if err != nil {
			return fmt.Errorf("script can't be instantiated without arguments (has signature %s): %w", scriptAbi.Constructor.Sig, err)
		}

		// Now we make sure the run method exists
		methodAbi, ok := scriptAbi.Methods[deployScriptRunMethodName]
		if !ok {
			return fmt.Errorf("contract is missing a run method")
		}

		// // Now we make sure that the run method can be packed using the provided input
		// _, err = methodAbi.Inputs.Pack(zeroInput)
		// if err != nil {
		// 	return fmt.Errorf("contract has an incompatible run method (has signature %s): %w", methodAbi.Sig, err)
		// }

		// // Now we make sure that the run output can be packed using the provided output
		// _, err = methodAbi.Outputs.Pack(zeroOutput)
		// if err != nil {
		// 	return fmt.Errorf("contract has an incompatible run method return value: %w", err)
		// }

		err = matchTypes(methodAbi.Outputs[0].Type, reflect.TypeOf(*new(O)))
		if err != nil {
			return fmt.Errorf("could not match args and values: %w", err)
		}

		// At this point we know that the script has the ABI we'd like it to have
		return nil
	}
}

// func matchType(abiType abi.Type, reflectType reflect.Type) error {
// 	if abiType.T != abi.TupleTy {
// 		return fmt.Errorf("only tuple arguments are supported")
// 	}

// 	for index := range arg.Type.TupleType.NumField() {
// 		tupleElem := arg.Type.TupleElems[index]
// 		field := arg.Type.TupleType.Field(index)
// 		goField := oType.Field(index)

// 		if tupleElem.T == abi.TupleTy {
// 			return fmt.Errorf("field type mismatch at index %d (%s): nested tuples are not supported", index, field.Name)
// 		}

// 		if field.Name != goField.Name {
// 			return fmt.Errorf("field name mismatch at index %d: expected %s, got %s", index, field.Name, goField.Name)
// 		}

// 		if !field.Type.AssignableTo(goField.Type) {
// 			return fmt.Errorf("field type mismatch at index %d (%s): expected %s, got %s", index, field.Name, field.Type.String(), goField.Type.String())
// 		}
// 	}

// 	return nil
// }

// unpackIntoInterface is a helper function that wraps ABI.UnpackIntoInterface for testing purposes
func unpackIntoInterface[O any](deployScriptAbi *abi.ABI, methodName string, data []byte) (*O, error) {
	output, err := deployScriptAbi.Unpack(methodName, data)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack call result: %w", err)
	}

	result := *abi.ConvertType(output[0], new(O)).(*O)

	return &result, nil
}
