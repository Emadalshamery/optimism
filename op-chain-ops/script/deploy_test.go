package script

import (
	_ "embed"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

const (
	// reusable definition of tuple components with some common types
	deployScriptFieldListTupleComponents = `
		[
			{
				"name": "addressField",
				"type": "address"
			},
			{
				"name": "boolField",
				"type": "bool"
			},
			{
				"name": "bytesField",
				"type": "bytes"
			},
			{
				"name": "bytes32Field",
				"type": "bytes32"
			},
			{
				"name": "int256Field",
				"type": "int256"
			},
			{
				"name": "stringField",
				"type": "string"
			},
			{
				"name": "uint256Field",
				"type": "uint256",
				"internalType": "ProtocolVersion"
			}
		]`
)

type deployScriptFieldListTuple struct {
	AddressField common.Address `abi:"addressField"`
	BoolField    bool           `abi:"boolField"`
	BytesField   []uint8        `abi:"bytesField"`
	Bytes32Field [32]uint8      `abi:"bytes32Field"`
	Int256Field  *big.Int       `abi:"int256Field"`
	StringField  string         `abi:"stringField"`
	Uint256Field *big.Int       `abi:"uint256Field"`
}

func TestUnpackIntoInterface(t *testing.T) {

	t.Run("should unpack struct with primitive fields", func(t *testing.T) {
		deployScriptAbiJson := fmt.Sprintf(`[{"type": "function", "name": "fn", "inputs": [], "outputs": [{"type": "tuple", "components": %s}]}]`, deployScriptFieldListTupleComponents)
		deployScriptAbi, err := abi.JSON(strings.NewReader(deployScriptAbiJson))
		require.NoError(t, err)

		abiMethod, ok := deployScriptAbi.Methods["fn"]
		require.True(t, ok)

		output := deployScriptFieldListTuple{
			AddressField: common.BigToAddress(big.NewInt(9)),
			BoolField:    false,
			BytesField:   []byte{1, 0, 1, 0},
			Bytes32Field: [32]byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			Int256Field:  big.NewInt(-1),
			StringField:  "i'm a smartcontract and i'm okay, i sleep all night and i work all day",
			Uint256Field: big.NewInt(7),
		}

		packed, err := abiMethod.Outputs.Pack(output)
		require.NoError(t, err)

		unpacked, err := unpackIntoInterface[deployScriptFieldListTuple](&deployScriptAbi, "fn", packed)
		require.NoError(t, err)

		require.Equal(t, &output, unpacked)
	})

	t.Run("should unpack struct with nested structs", func(t *testing.T) {
		type deployScriptNestedFieldListTuple struct {
			NestedField deployScriptFieldListTuple
		}

		deployScriptAbiJson := fmt.Sprintf(`[{"type": "function", "name": "fn", "inputs": [], "outputs": [{"type": "tuple", "components": [{ "type": "tuple", "name": "nestedField", "components": %s }] }]}]`, deployScriptFieldListTupleComponents)
		deployScriptAbi, err := abi.JSON(strings.NewReader(deployScriptAbiJson))
		require.NoError(t, err)

		abiMethod, ok := deployScriptAbi.Methods["fn"]
		require.True(t, ok)

		output := deployScriptNestedFieldListTuple{
			NestedField: deployScriptFieldListTuple{
				AddressField: common.BigToAddress(big.NewInt(9)),
				BoolField:    false,
				BytesField:   []byte{1, 0, 1, 0},
				Bytes32Field: [32]byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
				Int256Field:  big.NewInt(-1),
				StringField:  "i'm a smartcontract and i'm okay, i sleep all night and i work all day",
				Uint256Field: big.NewInt(7),
			},
		}

		packed, err := abiMethod.Outputs.Pack(output)
		require.NoError(t, err)

		unpacked, err := unpackIntoInterface[deployScriptNestedFieldListTuple](&deployScriptAbi, "fn", packed)
		require.NoError(t, err)

		require.Equal(t, &output, unpacked)
	})

	t.Run("should what", func(t *testing.T) {
		type deployScriptTupleWithSwappedFields struct {
			Field2Field common.Address `abi:"field2" json:"field2"`
			Field1Field common.Address `abi:"field1" json:"field1"`
		}

		deployScriptAbiJson := `[{"type": "function", "name": "fn", "inputs": [], "outputs": [{"type": "tuple", "components": [{ "type": "address", "name": "field1" }, { "type": "address", "name": "field2" }] }]}]`
		deployScriptAbi, err := abi.JSON(strings.NewReader(deployScriptAbiJson))
		require.NoError(t, err)

		abiMethod, ok := deployScriptAbi.Methods["fn"]
		require.True(t, ok)

		output := deployScriptTupleWithSwappedFields{
			Field2Field: common.BigToAddress(big.NewInt(2)),
			Field1Field: common.BigToAddress(big.NewInt(1)),
		}

		packed, err := abiMethod.Outputs.Pack(output)
		require.NoError(t, err)

		unpacked, err := unpackIntoInterface[deployScriptTupleWithSwappedFields](&deployScriptAbi, "fn", packed)
		require.NoError(t, err)

		require.Equal(t, &output, unpacked)
	})

	t.Run("should what again", func(t *testing.T) {
		type deployScriptTupleWithSwappedFields struct {
			Field2Field common.Address `abi:"field2"`
			Field1Field common.Address `abi:"field1"`
		}

		deployScriptAbiJson := `[{"type": "function", "name": "fn", "inputs": [], "outputs": [{"type": "tuple", "components": [{ "type": "address", "name": "field1" }, { "type": "address", "name": "field2" }] }]}]`
		deployScriptAbi, err := abi.JSON(strings.NewReader(deployScriptAbiJson))
		require.NoError(t, err)

		abiMethod, ok := deployScriptAbi.Methods["fn"]
		require.True(t, ok)

		output := deployScriptTupleWithSwappedFields{
			Field2Field: common.BigToAddress(big.NewInt(2)),
			Field1Field: common.BigToAddress(big.NewInt(1)),
		}

		packed, err := abiMethod.Outputs.Pack(output)
		require.NoError(t, err)

		unpacked, err := unpackIntoInterface[deployScriptTupleWithSwappedFields](&deployScriptAbi, "fn", packed)
		require.NoError(t, err)

		require.Equal(t, &output, unpacked)
	})
}

func TestNewDeployScriptAbiValidator(t *testing.T) {
	t.Run("should not fail with swapped fields", func(t *testing.T) {
		type deployScriptTupleWithSwappedFields struct {
			Field2 common.Address `abi:"field1"`
			Field1 common.Address `abi:"field2"`
		}

		deployScriptAbiJson := `[{"type": "function", "name": "run", "inputs": [{ "type": "uint256", "name": "what" }], "outputs": [{"type": "tuple", "components": [{ "type": "address", "name": "field1" }, { "type": "address", "name": "field2" }] }]}]`
		deployScriptAbi, err := abi.JSON(strings.NewReader(deployScriptAbiJson))
		require.NoError(t, err)

		validator := NewDeployScriptAbiValidator(big.NewInt(0), deployScriptTupleWithSwappedFields{
			Field2: common.BigToAddress(big.NewInt(2)),
			Field1: common.BigToAddress(big.NewInt(1)),
		})

		err = validator(deployScriptAbi)
		require.NoError(t, err)
	})
}

func TestDeployScriptIOFactoryImpl(t *testing.T) {
	t.Run("should not fail with correctly formated struct fields", func(t *testing.T) {
		type Input struct {
			Field1 common.Address `abi:"field1"`
			Field2 common.Address `abi:"field2"`
		}

		type Output struct {
			Field3 common.Address `abi:"field3"`
			Field4 common.Address `abi:"field4"`
		}

		deployScriptAbiJson := `[{
			"type": "function",
			"name": "run",
			"inputs": [{"type": "tuple", "components": [{ "type": "address", "name": "field1" }, { "type": "address", "name": "field2" }] }],
			"outputs": [{"type": "tuple", "components": [{ "type": "address", "name": "field3" }, { "type": "address", "name": "field4" }] }]
		}]`
		deployScriptAbi, err := abi.JSON(strings.NewReader(deployScriptAbiJson))
		require.NoError(t, err)

		factory := &deployScriptIOFactoryImpl[Input, Output]{
			methodName: "run",
			testInput: Input{
				Field1: common.BigToAddress(big.NewInt(1)),
				Field2: common.BigToAddress(big.NewInt(2)),
			},
			testOutput: Output{
				Field3: common.BigToAddress(big.NewInt(1)),
				Field4: common.BigToAddress(big.NewInt(2)),
			},
		}

		io, err := factory.Create(deployScriptAbi)
		require.NoError(t, err)
		require.NotNil(t, io)
	})

	t.Run("should fail when input is missing", func(t *testing.T) {
		type Input struct {
			Field1 common.Address `abi:"field1"`
			Field2 common.Address `abi:"field2"`
		}

		type Output struct {
			Field3 common.Address `abi:"field3"`
			Field4 common.Address `abi:"field4"`
		}

		deployScriptAbiJson := `[{
			"type": "function",
			"name": "run",
			"inputs": [],
			"outputs": [{"type": "tuple", "components": [{ "type": "address", "name": "field3" }, { "type": "address", "name": "field4" }] }]
		}]`
		deployScriptAbi, err := abi.JSON(strings.NewReader(deployScriptAbiJson))
		require.NoError(t, err)

		factory := &deployScriptIOFactoryImpl[Input, Output]{
			methodName: "run",
			testInput: Input{
				Field1: common.BigToAddress(big.NewInt(1)),
				Field2: common.BigToAddress(big.NewInt(2)),
			},
			testOutput: Output{
				Field3: common.BigToAddress(big.NewInt(1)),
				Field4: common.BigToAddress(big.NewInt(2)),
			},
		}

		io, err := factory.Create(deployScriptAbi)
		require.ErrorContains(t, err, "contract has an incompatible run method: expected single argument, got run()")
		require.Nil(t, io)
	})

	t.Run("should fail when input is not a tuple", func(t *testing.T) {
		type Input struct {
			Field1 common.Address `abi:"field1"`
			Field2 common.Address `abi:"field2"`
		}

		type Output struct {
			Field3 common.Address `abi:"field3"`
			Field4 common.Address `abi:"field4"`
		}

		deployScriptAbiJson := `[{
			"type": "function",
			"name": "run",
			"inputs": [{ "type": "uint256", "name": "amount" }],
			"outputs": [{"type": "tuple", "components": [{ "type": "address", "name": "field3" }, { "type": "address", "name": "field4" }] }]
		}]`
		deployScriptAbi, err := abi.JSON(strings.NewReader(deployScriptAbiJson))
		require.NoError(t, err)

		factory := &deployScriptIOFactoryImpl[Input, Output]{
			methodName: "run",
			testInput: Input{
				Field1: common.BigToAddress(big.NewInt(1)),
				Field2: common.BigToAddress(big.NewInt(2)),
			},
			testOutput: Output{
				Field3: common.BigToAddress(big.NewInt(1)),
				Field4: common.BigToAddress(big.NewInt(2)),
			},
		}

		io, err := factory.Create(deployScriptAbi)
		require.ErrorContains(t, err, "input type mismatch: ABI type uint256 (represented in go as *big.Int) is not assignable to go type script.Input")
		require.Nil(t, io)
	})

	t.Run("should fail when input field types don't match", func(t *testing.T) {
		type Input struct {
			Field1 common.Address `abi:"field1"`
			Field2 *big.Int       `abi:"field2"`
		}

		type Output struct {
			Field3 common.Address `abi:"field3"`
			Field4 common.Address `abi:"field4"`
		}

		deployScriptAbiJson := `[{
			"type": "function",
			"name": "run",
			"inputs": [{"type": "tuple", "components": [{ "type": "address", "name": "field1" }, { "type": "address", "name": "field2" }] }],
			"outputs": [{"type": "tuple", "components": [{ "type": "address", "name": "field3" }, { "type": "address", "name": "field4" }] }]
		}]`
		deployScriptAbi, err := abi.JSON(strings.NewReader(deployScriptAbiJson))
		require.NoError(t, err)

		factory := &deployScriptIOFactoryImpl[Input, Output]{
			methodName: "run",
			testInput: Input{
				Field1: common.BigToAddress(big.NewInt(1)),
				Field2: big.NewInt(2),
			},
			testOutput: Output{
				Field3: common.BigToAddress(big.NewInt(1)),
				Field4: common.BigToAddress(big.NewInt(2)),
			},
		}

		io, err := factory.Create(deployScriptAbi)
		require.ErrorContains(t, err, "input type mismatch: field type mismatch at index 1 (Field2): ABI type address (represented in go as common.Address) is not assignable to go type *big.Int")
		require.Nil(t, io)
	})

	t.Run("should fail when input field tags don't match", func(t *testing.T) {
		type Input struct {
			Field1 common.Address `abi:"field7"`
			Field2 common.Address `abi:"field8"`
		}

		type Output struct {
			Field3 common.Address `abi:"field3"`
			Field4 common.Address `abi:"field4"`
		}

		deployScriptAbiJson := `[{
			"type": "function",
			"name": "run",
			"inputs": [{"type": "tuple", "components": [{ "type": "address", "name": "field1" }, { "type": "address", "name": "field2" }] }],
			"outputs": [{"type": "tuple", "components": [{ "type": "address", "name": "field3" }, { "type": "address", "name": "field4" }] }]
		}]`
		deployScriptAbi, err := abi.JSON(strings.NewReader(deployScriptAbiJson))
		require.NoError(t, err)

		factory := &deployScriptIOFactoryImpl[Input, Output]{
			methodName: "run",
			testInput: Input{
				Field1: common.BigToAddress(big.NewInt(1)),
				Field2: common.BigToAddress(big.NewInt(2)),
			},
			testOutput: Output{
				Field3: common.BigToAddress(big.NewInt(1)),
				Field4: common.BigToAddress(big.NewInt(2)),
			},
		}

		io, err := factory.Create(deployScriptAbi)
		require.ErrorContains(t, err, "contract has an incompatible run method (has signature run((address,address))): struct: abi tag 'field7' defined but not found in abi")
		require.Nil(t, io)
	})

	t.Run("should fail when input field names don't match", func(t *testing.T) {
		type Input struct {
			Field7 common.Address
			Field8 common.Address
		}

		type Output struct {
			Field3 common.Address `abi:"field3"`
			Field4 common.Address `abi:"field4"`
		}

		deployScriptAbiJson := `[{
			"type": "function",
			"name": "run",
			"inputs": [{"type": "tuple", "components": [{ "type": "address", "name": "field1" }, { "type": "address", "name": "field2" }] }],
			"outputs": [{"type": "tuple", "components": [{ "type": "address", "name": "field3" }, { "type": "address", "name": "field4" }] }]
		}]`
		deployScriptAbi, err := abi.JSON(strings.NewReader(deployScriptAbiJson))
		require.NoError(t, err)

		factory := &deployScriptIOFactoryImpl[Input, Output]{
			methodName: "run",
			testInput: Input{
				Field7: common.BigToAddress(big.NewInt(1)),
				Field8: common.BigToAddress(big.NewInt(2)),
			},
			testOutput: Output{
				Field3: common.BigToAddress(big.NewInt(1)),
				Field4: common.BigToAddress(big.NewInt(2)),
			},
		}

		io, err := factory.Create(deployScriptAbi)
		require.ErrorContains(t, err, "input type mismatch: field name mismatch at index 0: expected Field1, got Field7")
		require.Nil(t, io)
	})

	t.Run("should fail when input fields are out of order", func(t *testing.T) {
		type Input struct {
			Field2 common.Address `abi:"field2"`
			Field1 common.Address `abi:"field1"`
		}

		type Output struct {
			Field3 common.Address `abi:"field3"`
			Field4 common.Address `abi:"field4"`
		}

		deployScriptAbiJson := `[{
			"type": "function",
			"name": "run",
			"inputs": [{"type": "tuple", "components": [{ "type": "address", "name": "field1" }, { "type": "address", "name": "field2" }] }],
			"outputs": [{"type": "tuple", "components": [{ "type": "address", "name": "field3" }, { "type": "address", "name": "field4" }] }]
		}]`
		deployScriptAbi, err := abi.JSON(strings.NewReader(deployScriptAbiJson))
		require.NoError(t, err)

		factory := &deployScriptIOFactoryImpl[Input, Output]{
			methodName: "run",
			testInput: Input{
				Field1: common.BigToAddress(big.NewInt(1)),
				Field2: common.BigToAddress(big.NewInt(2)),
			},
			testOutput: Output{
				Field3: common.BigToAddress(big.NewInt(1)),
				Field4: common.BigToAddress(big.NewInt(2)),
			},
		}

		io, err := factory.Create(deployScriptAbi)
		require.ErrorContains(t, err, "input type mismatch: field name mismatch at index 0: expected Field1, got Field2")
		require.Nil(t, io)
	})

	t.Run("should not fail when input has a compatible nested field", func(t *testing.T) {
		type NestedNestedInput struct {
			Field1 common.Address `abi:"field1"`
			Field2 common.Address `abi:"field2"`
		}

		type NestedInput struct {
			NestedNested NestedNestedInput `abi:"nestedNested"`
		}

		type Input struct {
			Nested NestedInput `abi:"nested"`
		}

		type Output struct {
			Field3 common.Address `abi:"field3"`
			Field4 common.Address `abi:"field4"`
		}

		deployScriptAbiJson := `[{
			"type": "function",
			"name": "run",
			"inputs": [{"type": "tuple", "components": [{"type": "tuple", "name": "nested", "components": [{"type": "tuple", "name": "nestedNested", "components": [{ "type": "address", "name": "field1" }, { "type": "address", "name": "field2" }] }] }] }],
			"outputs": [{"type": "tuple", "components": [{ "type": "address", "name": "field3" }, { "type": "address", "name": "field4" }] }]
		}]`
		deployScriptAbi, err := abi.JSON(strings.NewReader(deployScriptAbiJson))
		require.NoError(t, err)

		factory := &deployScriptIOFactoryImpl[Input, Output]{
			methodName: "run",
			testInput: Input{
				Nested: NestedInput{
					NestedNested: NestedNestedInput{
						Field1: common.BigToAddress(big.NewInt(1)),
						Field2: common.BigToAddress(big.NewInt(2)),
					},
				},
			},
			testOutput: Output{
				Field3: common.BigToAddress(big.NewInt(1)),
				Field4: common.BigToAddress(big.NewInt(2)),
			},
		}

		io, err := factory.Create(deployScriptAbi)
		require.NoError(t, err)
		require.NotNil(t, io)
	})

	t.Run("should not fail when input has an array field", func(t *testing.T) {
		type NestedInput struct {
			Field1 common.Address `abi:"field1"`
			Field2 common.Address `abi:"field2"`
		}

		type Input2 struct {
			Nested [2]NestedInput `abi:"nested"`
		}

		type Input3 struct {
			Nested [3]NestedInput `abi:"nested"`
		}

		type Output struct {
			Field3 common.Address `abi:"field3"`
			Field4 common.Address `abi:"field4"`
		}

		deployScriptAbiJson := `[{
			"type": "function",
			"name": "run",
			"inputs": [{"type": "tuple", "components": [{"type": "tuple[2]", "name": "nested", "components": [{ "type": "address", "name": "field1" }, { "type": "address", "name": "field2" }] }] }],
			"outputs": [{"type": "tuple", "components": [{ "type": "address", "name": "field3" }, { "type": "address", "name": "field4" }] }]
		}]`
		deployScriptAbi, err := abi.JSON(strings.NewReader(deployScriptAbiJson))
		require.NoError(t, err)

		factory2 := &deployScriptIOFactoryImpl[Input2, Output]{
			methodName: "run",
			testInput: Input2{
				Nested: [2]NestedInput{
					{
						Field1: common.BigToAddress(big.NewInt(1)),
						Field2: common.BigToAddress(big.NewInt(2)),
					},
					{
						Field1: common.BigToAddress(big.NewInt(3)),
						Field2: common.BigToAddress(big.NewInt(4)),
					},
				},
			},
			testOutput: Output{
				Field3: common.BigToAddress(big.NewInt(1)),
				Field4: common.BigToAddress(big.NewInt(2)),
			},
		}

		io2, err := factory2.Create(deployScriptAbi)
		require.NoError(t, err)
		require.NotNil(t, io2)

		factory3 := &deployScriptIOFactoryImpl[Input3, Output]{
			methodName: "run",
			testInput: Input3{
				Nested: [3]NestedInput{
					{
						Field1: common.BigToAddress(big.NewInt(1)),
						Field2: common.BigToAddress(big.NewInt(2)),
					},
					{
						Field1: common.BigToAddress(big.NewInt(3)),
						Field2: common.BigToAddress(big.NewInt(4)),
					},
					{
						Field1: common.BigToAddress(big.NewInt(3)),
						Field2: common.BigToAddress(big.NewInt(4)),
					},
				},
			},
			testOutput: Output{
				Field3: common.BigToAddress(big.NewInt(1)),
				Field4: common.BigToAddress(big.NewInt(2)),
			},
		}

		io3, err := factory3.Create(deployScriptAbi)
		require.ErrorContains(t, err, "input type mismatch: field type mismatch at index 0 (Nested): ABI type (address,address)[2] is not assignable to go type [3]script.NestedInput: expected an array of length 2, got length 3")
		require.Nil(t, io3)
	})

	t.Run("should not fail when input has a slice field", func(t *testing.T) {
		type NestedInput struct {
			Field1 common.Address `abi:"field1"`
			Field2 common.Address `abi:"field2"`
		}

		type Input struct {
			Nested []NestedInput `abi:"nested"`
		}

		type Output struct {
			Field3 common.Address `abi:"field3"`
			Field4 common.Address `abi:"field4"`
		}

		deployScriptAbiJson := `[{
			"type": "function",
			"name": "run",
			"inputs": [{"type": "tuple", "components": [{"type": "tuple[]", "name": "nested", "components": [{ "type": "address", "name": "field1" }, { "type": "address", "name": "field2" }] }] }],
			"outputs": [{"type": "tuple", "components": [{ "type": "address", "name": "field3" }, { "type": "address", "name": "field4" }] }]
		}]`
		deployScriptAbi, err := abi.JSON(strings.NewReader(deployScriptAbiJson))
		require.NoError(t, err)

		factory := &deployScriptIOFactoryImpl[Input, Output]{
			methodName: "run",
			testInput: Input{
				Nested: []NestedInput{
					{
						Field1: common.BigToAddress(big.NewInt(1)),
						Field2: common.BigToAddress(big.NewInt(2)),
					},
					{
						Field1: common.BigToAddress(big.NewInt(3)),
						Field2: common.BigToAddress(big.NewInt(4)),
					},
				},
			},
			testOutput: Output{
				Field3: common.BigToAddress(big.NewInt(1)),
				Field4: common.BigToAddress(big.NewInt(2)),
			},
		}

		io, err := factory.Create(deployScriptAbi)
		require.NoError(t, err)
		require.NotNil(t, io)
	})

	t.Run("should fail when input has incompatible nested field", func(t *testing.T) {
		type NestedInput struct {
			Field1 common.Address `abi:"field1"`
			Field2 *big.Int       `abi:"field2"`
		}

		type Input struct {
			Nested NestedInput `abi:"nested"`
		}

		type Output struct {
			Field3 common.Address `abi:"field3"`
			Field4 common.Address `abi:"field4"`
		}

		deployScriptAbiJson := `[{
			"type": "function",
			"name": "run",
			"inputs": [{"type": "tuple", "components": [{"type": "tuple", "name": "nested", "components": [{ "type": "address", "name": "field1" }, { "type": "address", "name": "field2" }] }] }],
			"outputs": [{"type": "tuple", "components": [{ "type": "address", "name": "field3" }, { "type": "address", "name": "field4" }] }]
		}]`
		deployScriptAbi, err := abi.JSON(strings.NewReader(deployScriptAbiJson))
		require.NoError(t, err)

		factory := &deployScriptIOFactoryImpl[Input, Output]{
			methodName: "run",
			testInput: Input{
				Nested: NestedInput{
					Field1: common.BigToAddress(big.NewInt(1)),
					Field2: big.NewInt(2),
				},
			},
			testOutput: Output{
				Field3: common.BigToAddress(big.NewInt(1)),
				Field4: common.BigToAddress(big.NewInt(2)),
			},
		}

		io, err := factory.Create(deployScriptAbi)
		require.ErrorContains(t, err, "input type mismatch: field type mismatch at index 0 (Nested): field type mismatch at index 1 (Field2): ABI type address (represented in go as common.Address) is not assignable to go type *big.Int")
		require.Nil(t, io)
	})

	t.Run("should fail", func(t *testing.T) {
		type Input struct {
			Field1 []byte `abi:"field1"`
		}

		type Output struct {
			Field3 common.Address `abi:"field3"`
			Field4 common.Address `abi:"field4"`
		}

		deployScriptAbiJson := `[{
			"type": "function",
			"name": "run",
			"inputs": [{"type": "tuple", "components": [{"type": "bytes32", "name": "field1" }] }],
			"outputs": [{"type": "tuple", "components": [{ "type": "address", "name": "field3" }, { "type": "address", "name": "field4" }] }]
		}]`
		deployScriptAbi, err := abi.JSON(strings.NewReader(deployScriptAbiJson))
		require.NoError(t, err)

		factory := &deployScriptIOFactoryImpl[Input, Output]{
			methodName: "run",
			testInput: Input{
				Field1: common.Hex2Bytes("100001"),
			},
			testOutput: Output{
				Field3: common.BigToAddress(big.NewInt(1)),
				Field4: common.BigToAddress(big.NewInt(2)),
			},
		}

		io, err := factory.Create(deployScriptAbi)
		require.ErrorContains(t, err, "input type mismatch: field type mismatch at index 0 (Field1): ABI type bytes32 (represented in go as [32]uint8) is not assignable to go type []uint8")
		require.Nil(t, io)
	})
}
