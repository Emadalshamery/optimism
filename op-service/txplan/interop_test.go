package txplan

import (
	"context"
	"math/big"
	"math/rand"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
	"github.com/lmittmann/w3"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

func TestSimpleTx(t *testing.T) {
	// t.Skip() // temporal addition for make CI pass.
	// To run tests, hardcode the endpoint/private key below
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	// To run tests against kt-devnet, hardcode the endpoint/private key below.
	// Fetch from system descriptor
	privHex := "0xf214f2b2cd398c806f84e317254e0f0b801d0643303237d97a22a48e01628897"
	rpcURL := "http://127.0.0.1:32948"

	logger := testlog.Logger(t, log.LevelInfo)

	wallet, err := newWallet(ctx, rpcURL, privHex, nil, logger)
	require.NoError(t, err)

	// send eth to null address
	opts := CombineOptions(
		WithTo(&common.Address{}),
		DefaultTxSubmitOptions(wallet),
		DefaultTxInclusionOptions(wallet),
	)

	txSimple := NewPlannedTx(opts, WithValue(big.NewInt(1000)))
	res, err := txSimple.IncludedBlock.Eval(ctx)
	require.NoError(t, err)

	t.Logf("included simple tx in block %s", res)
}

func buildSendMessageCalldata(chainID eth.ChainID, addr common.Address, msg []byte) ([]byte, error) {
	// construct call input, ugly but no bindings...
	sendMessage := w3.MustNewFunc("sendMessage(uint256,address,bytes calldata)", "bytes32")
	return sendMessage.EncodeArgs(chainID.ToBig(), addr, msg)
}

func TestInteropTxUsingL2toL2CDM(t *testing.T) {
	// t.Skip() // temporal addition for make CI pass.

	rng := rand.New(rand.NewSource(1234))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	logger := testlog.Logger(t, log.LevelInfo)

	// To run tests against kt-devnet, hardcode the endpoint/private key below.
	// Fetch from system descriptor
	privHexA := "0xf214f2b2cd398c806f84e317254e0f0b801d0643303237d97a22a48e01628897"
	rpcURLA := "http://127.0.0.1:32948"
	privHexB := "0xeaa861a9a01391ed3d587d8a5a84ca56ee277629a8b02c22093a419bf240e65d"
	rpcURLB := "http://127.0.0.1:32961"

	walletA, err := newWallet(ctx, rpcURLA, privHexA, nil, logger)
	require.NoError(t, err)
	walletB, err := newWallet(ctx, rpcURLB, privHexB, nil, logger)
	require.NoError(t, err)

	optsFunc := func(w Wallet) Option {
		opts := CombineOptions(
			DefaultTxSubmitOptions(w),
			DefaultTxInclusionOptions(w),
		)
		return opts
	}

	optsA := optsFunc(walletA)
	optsB := optsFunc(walletB)

	txB := NewIntent[*ExecTrigger, *InteropOutput](optsB)

	// eventLogger can be a contract the emits any logs
	// for easy testing, we use existing L2toL2CDM predeploy
	eventLogger := common.HexToAddress(predeploys.L2toL2CrossDomainMessenger)
	randomAddr := testutils.RandomAddress(rng)
	randomData := testutils.RandomData(rng, 200)

	destChainID, err := txB.PlannedTx.ChainID.Eval(ctx)
	require.NoError(t, err)
	opaqueData, err := buildSendMessageCalldata(destChainID, randomAddr, randomData)
	require.NoError(t, err)

	txA := NewIntent[*InitTrigger, *InteropOutput](optsA)
	txA.Content.Set(&InitTrigger{
		Emitter:    eventLogger,
		Topics:     []common.Hash{},
		OpaqueData: opaqueData,
	})

	txB.Content.DependOn(&txA.Result)
	CrossL2InboxAddr := common.HexToAddress(predeploys.CrossL2Inbox)
	txB.Content.Fn(executeIndexed(CrossL2InboxAddr, &txA.Result, 0))

	recA, err := txA.PlannedTx.Included.Eval(ctx)
	require.NoError(t, err)
	t.Logf("included initiating tx in block %s", recA.BlockHash)

	recB, err := txB.PlannedTx.Included.Eval(ctx)
	require.NoError(t, err)
	t.Logf("included executing tx in block %s", recB.BlockHash)
}
