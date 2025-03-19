package rollup

import (
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/superchain"
	"github.com/lmittmann/w3"
	"github.com/lmittmann/w3/module/eth"
	"github.com/lmittmann/w3/w3types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpgradeTxGas(t *testing.T) {

	for _, chain := range superchain.ChainNames() {
		id, err := superchain.ChainIDByName(chain)
		if err != nil {
			t.Errorf("Error: %v", err)
		}
		ch, err := superchain.GetChain(id)
		if err != nil {
			t.Errorf("Error: %v", err)
		}

		cfg, err := ch.Config()
		if err != nil {
			t.Errorf("Error: %v", err)
		}

		sysCfg := cfg.Addresses.SystemConfigProxy
		funcGasLimit := w3.MustNewFunc("gasLimit()", "uint64")
		sc, err := superchain.GetSuperchain(ch.Network)
		if err != nil {
			t.Errorf("Error: %v", err)
		}

		client := w3.MustDial(sc.L1.PublicRPC)
		defer client.Close()
		var gasLimit uint64
		err = client.Call(eth.CallFunc(*sysCfg, funcGasLimit).Returns(&gasLimit))
		if err != nil {
			t.Errorf("Error: %v", err)
		}
		t.Log("Chain:", chain, "GasLimit", fmt.Sprintf("%.1fM", float64(gasLimit)/1000000))
		assert.GreaterOrEqual(t, gasLimit, uint64(30_000_000))
	}
}

func TestSuperchainGasLimit(t *testing.T) {

	var sysCfgAddressGetter AddressGetter = func(cfg superchain.ChainConfig) *common.Address {
		return cfg.Addresses.SystemConfigProxy
	}

	r := GetL1SuperchainInformation([]call{
		{
			a: sysCfgAddressGetter,
			f: w3.MustNewFunc("gasLimit()", "uint64"),
			r: func() any { return new(uint64) },
		},
		// {
		// 	a: sysCfgAddressGetter,
		// 	f: w3.MustNewFunc("resourceConfig()", "(uint32,uint8,uint8,uint32,uint32,uint128)"),
		// 	r: func() any { return []any{new(uint32), new(uint8), new(uint8), new(uint32), new(uint32), new(big.Int)} },
		// },
	})
	require.NotNil(t, r)

	gl := *r["op-mainnet"]["gasLimit()"].(*uint64)
	t.Logf("%d", gl)
	for chainID, chainResults := range r {
		t.Log("Chain:", chainID, "GasLimit", fmt.Sprintf("%.1fM", float64(*chainResults["gasLimit()"].(*uint64))/1000000))
		assert.GreaterOrEqual(t, *chainResults["gasLimit()"].(*uint64), uint64(30_000_000))
	}
}

type AddressGetter func(superchain.ChainConfig) *common.Address
type ReturnsMaker func() any
type call struct {
	a AddressGetter
	f *w3.Func
	r ReturnsMaker
}

// maps chain name to function signature to result
type results map[string]map[string]interface{}

func GetL1SuperchainInformation(calls []call) results {
	results := make(results)

	superchains := []string{"sepolia", "mainnet"}

	for _, sc := range superchains {
		var client *w3.Client
		rawCalls := make([]w3types.RPCCaller, 0, len(calls)*len(superchain.ChainNames()))
		for _, chain := range superchain.ChainNames() {
			id, err := superchain.ChainIDByName(chain)
			if err != nil {
				panic(err)
			}
			ch, err := superchain.GetChain(id)
			if err != nil {
				panic(err)
			}
			if ch.Network != sc {
				continue
			}
			results[chain] = make(map[string]any)
			cfg, err := ch.Config()
			if err != nil {
				panic(err)
			}
			superC, err := superchain.GetSuperchain(ch.Network)
			if err != nil {
				panic(err)
			}
			if client == nil {
				fmt.Printf("dialing client at %s", superC.L1.PublicRPC)
				client = w3.MustDial(superC.L1.PublicRPC)
			}
			// loop over calls
			for _, call := range calls {
				results[chain][call.f.Signature] = call.r()
				rawCalls = append(rawCalls, eth.CallFunc(*(call.a(*cfg)), call.f).Returns(results[chain][call.f.Signature]))
			}
		}
		fmt.Println("making batch call")
		err := client.Call(rawCalls...)
		fmt.Println("finished batch call")
		if err != nil {
			panic(err)
		}
		client.Close()
	}
	return results
}
