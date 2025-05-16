package base

import (
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-devstack/dsl"
	"github.com/ethereum-optimism/optimism/op-devstack/presets"
	"github.com/ethereum-optimism/optimism/op-devstack/stack/match"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

func TestL1ToL2Deposit(gt *testing.T) {
	// Create a test environment using op-devstack
	t := devtest.ParallelT(gt)
	sys := presets.NewSimpleInterop(t)

	// Fund Alice on L1
	fundingAmount := eth.ThousandEther
	alice := sys.FunderL1.NewFundedEOA(fundingAmount)
	alice.VerifyBalanceExact(fundingAmount)

	alicel2 := dsl.NewEOA(alice.Key(), sys.L2ELA)

	// Get the optimism portal address
	rollupConfig := sys.L2Networks()[0].Escape().RollupConfig()
	portalAddr := rollupConfig.DepositContractAddress

	l1Client := sys.L1Network.Escape().L1ELNode(match.FirstL1EL).EthClient()

	depositAmount := eth.OneEther

	// Deposit flow
	alicePrivKey := alice.Key().PrivateKey()

	// TODO: We should use an ABI wrapper for this
	// But right now the OptimismPortal Helper requires a geth client
	const optimismPortalABI = `[{"inputs":[{"internalType":"address","name":"_to","type":"address"},{"internalType":"uint256","name":"_value","type":"uint256"},{"internalType":"uint64","name":"_gasLimit","type":"uint64"},{"internalType":"bool","name":"_isCreation","type":"bool"},{"internalType":"bytes","name":"_data","type":"bytes"}],"name":"depositTransaction","outputs":[],"stateMutability":"payable","type":"function"}]`
	parsedABI, err := abi.JSON(strings.NewReader(optimismPortalABI))
	require.NoError(t, err)

	data, err := parsedABI.Pack(
		"depositTransaction",
		alice.Address(),       // _to
		depositAmount.ToBig(), // _value
		uint64(300_000),       // _gasLimit
		false,                 // _isCreation
		[]byte{},              // _data
	)
	require.NoError(t, err)

	// Prepare the transaction
	nonce, err := l1Client.PendingNonceAt(t.Ctx(), alice.Address())
	require.NoError(t, err)
	gasPrice, err := l1Client.SuggestGasPrice(t.Ctx())
	require.NoError(t, err)
	chainID, err := l1Client.ChainID(t.Ctx())
	require.NoError(t, err)

	tx := types.NewTransaction(
		nonce,
		portalAddr,
		depositAmount.ToBig(), // ETH to deposit
		500_000,               // L1 gas limit (estimate higher for contract call)
		gasPrice,
		data, // ABI-encoded call data
	)

	// Sign and send
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), alicePrivKey)
	require.NoError(t, err)
	err = l1Client.SendTransaction(t.Ctx(), signedTx)
	require.NoError(t, err)

	// Wait for the transaction to be mined as above
	// Then wait for a few more blocks (simulate finalization)
	latestBlock, err := l1Client.InfoByLabel(t.Ctx(), eth.Unsafe)
	require.NoError(t, err)
	targetBlock := latestBlock.NumberU64() + 5

	for {
		blk, err := l1Client.InfoByLabel(t.Ctx(), eth.Unsafe)
		require.NoError(t, err)
		if blk.NumberU64() >= targetBlock {
			break
		}
		time.Sleep(time.Second)
	}

	receipt, err := l1Client.TransactionReceipt(t.Ctx(), signedTx.Hash())

	require.NoError(t, err)
	require.Equal(t, uint64(1), receipt.Status, "deposit tx failed")

	// Verify the deposit was successful
	gasCost := new(big.Int).Mul(new(big.Int).SetUint64(receipt.GasUsed), gasPrice)
	expectedFinalL1 := new(big.Int).Sub(fundingAmount.ToBig(), depositAmount.ToBig())
	expectedFinalL1.Sub(expectedFinalL1, gasCost)

	alice.VerifyBalanceExact(eth.WeiBig(expectedFinalL1))

	alicel2.VerifyBalanceExact(depositAmount)
}
