package rollup

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/superchain"
	"github.com/lmittmann/w3"
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
		funcBalanceOf = w3.MustNewFunc("balanceOf(address)", "uint256")

		addrA = common.Address{0x0a}

		addrWETH = w3.A("0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2")

		client = w3.MustDial("https://rpc.ankr.com/eth")
	}
}
