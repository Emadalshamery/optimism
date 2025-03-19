package interop

import (
	"encoding/hex"
	"math/big"
	"math/rand"
	"testing"

	"github.com/ethereum-optimism/optimism/devnet-sdk/contracts/constants"
	"github.com/ethereum-optimism/optimism/devnet-sdk/system"
	"github.com/ethereum-optimism/optimism/devnet-sdk/testing/systest"
	"github.com/ethereum-optimism/optimism/devnet-sdk/testing/testlib/validators"
	sdktypes "github.com/ethereum-optimism/optimism/devnet-sdk/types"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/wait"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
	"github.com/ethereum-optimism/optimism/op-service/txplan"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/log"
	"github.com/lmittmann/w3"
	"github.com/stretchr/testify/require"
)

func simpleTxWalletV2(lowLevelSystemGetter validators.LowLevelSystemGetter, chainIdx uint64, sourceWalletGetter validators.WalletGetter) systest.InteropSystemTestFunc {
	return func(t systest.T, sys system.InteropSystem) {
		ctx := t.Context()
		llsys := lowLevelSystemGetter(ctx)

		logger := testlog.Logger(t, log.LevelInfo)
		chain := llsys.L2s()[chainIdx]

		wallet, err := system.NewWalletV2FromWalletAndChain(ctx, sourceWalletGetter(ctx), chain)
		require.NoError(t, err)

		opts := txplan.CombineOptions(
			txplan.WithTo(&common.Address{}),
			system.DefaultTxSubmitOptions(wallet),
			system.DefaultTxInclusionOptions(wallet),
		)

		txSimple := txplan.NewPlannedTx(opts, txplan.WithValue(big.NewInt(1000)))
		res, err := txSimple.IncludedBlock.Eval(ctx)
		require.NoError(t, err)

		logger.Info("included simple tx", "block", res)
	}
}

func TestSimpleTxWalletV2(t *testing.T) {
	chainIdx := uint64(0)
	walletGetter, sourcefundsValidator := validators.AcquireL2WalletWithFunds(chainIdx, sdktypes.NewBalance(big.NewInt(1.0*constants.ETH)))
	lowLevelSystemGetter, lowLevelSystemValidator := validators.AcquireLowLevelSystem()

	systest.InteropSystemTest(t,
		simpleTxWalletV2(lowLevelSystemGetter, chainIdx, walletGetter),
		sourcefundsValidator,
		lowLevelSystemValidator,
	)
}

func BuildSendMessageCalldata(chainID eth.ChainID, addr common.Address, msg []byte) ([]byte, error) {
	// TODO: Need to do better construct call input than this
	sendMessage := w3.MustNewFunc("sendMessage(uint256,address,bytes calldata)", "bytes32")
	return sendMessage.EncodeArgs(chainID.ToBig(), addr, msg)
}

func interopTxUsingL2toL2CDMWalletV2(lowLevelSystemGetter validators.LowLevelSystemGetter, sourceChainIdx, destChainIdx uint64, sourceWalletGetter, destWalletGetter validators.WalletGetter) systest.InteropSystemTestFunc {
	return func(t systest.T, sys system.InteropSystem) {
		ctx := t.Context()
		llsys := lowLevelSystemGetter(ctx)
		rng := rand.New(rand.NewSource(1234))

		logger := testlog.Logger(t, log.LevelInfo)
		logger = logger.With("test", "TestMessagePassing", "devnet", sys.Identifier())

		chainA := llsys.L2s()[sourceChainIdx]
		chainB := llsys.L2s()[destChainIdx]

		// walletA is funded at chainA and want to initialize message at chain A
		walletA, err := system.NewWalletV2FromWalletAndChain(ctx, sourceWalletGetter(ctx), chainA)
		require.NoError(t, err)
		// walletB is funded at chainB and want to execute message to chainB
		walletB, err := system.NewWalletV2FromWalletAndChain(ctx, destWalletGetter(ctx), chainB)
		require.NoError(t, err)

		optsFunc := func(w system.WalletV2) txplan.Option {
			opts := txplan.CombineOptions(
				system.DefaultTxSubmitOptions(w),
				system.DefaultTxInclusionOptions(w),
			)
			return opts
		}

		optsA := optsFunc(walletA)
		optsB := optsFunc(walletB)

		txB := system.NewIntent[*system.ExecTrigger, *system.InteropOutput](optsB)

		// eventLogger can be a contract the emits any logs
		// for easy testing, we use existing L2toL2CDM predeploy
		eventLogger := common.HexToAddress(predeploys.L2toL2CrossDomainMessenger)
		randomAddr := testutils.RandomAddress(rng)
		randomData := testutils.RandomData(rng, 200)

		destChainID, err := txB.PlannedTx.ChainID.Eval(ctx)
		require.NoError(t, err)
		opaqueData, err := BuildSendMessageCalldata(destChainID, randomAddr, randomData)
		require.NoError(t, err)

		txA := system.NewIntent[*system.InitTrigger, *system.InteropOutput](optsA)

		// Topics field is only needed when we call EventLogger contract
		// We are using L2toL2CDM so make it empty
		txA.Content.Set(&system.InitTrigger{
			Emitter:    eventLogger,
			Topics:     []common.Hash{},
			OpaqueData: opaqueData,
		})

		txB.Content.DependOn(&txA.Result)
		CrossL2InboxAddr := common.HexToAddress(predeploys.CrossL2Inbox)
		txB.Content.Fn(system.ExecuteIndexed(CrossL2InboxAddr, &txA.Result, 0))

		recA, err := txA.PlannedTx.Included.Eval(ctx)
		require.NoError(t, err)
		logger.Info("included initiating tx", "block", recA.BlockHash)

		recB, err := txB.PlannedTx.Included.Eval(ctx)
		require.NoError(t, err)
		logger.Info("included executing tx", "block", recB.BlockHash)
	}
}

func TestInteropTxUsingL2toL2CDMWalletV2(t *testing.T) {
	sourceChainIdx := uint64(0)
	destChainIdx := uint64(1)
	sourceWalletGetter, sourcefundsValidator := validators.AcquireL2WalletWithFunds(sourceChainIdx, sdktypes.NewBalance(big.NewInt(1.0*constants.ETH)))
	destWalletGetter, destfundsValiator := validators.AcquireL2WalletWithFunds(destChainIdx, sdktypes.NewBalance(big.NewInt(1.0*constants.ETH)))
	lowLevelSystemGetter, lowLevelSystemValidator := validators.AcquireLowLevelSystem()

	systest.InteropSystemTest(t,
		interopTxUsingL2toL2CDMWalletV2(lowLevelSystemGetter, sourceChainIdx, destChainIdx, sourceWalletGetter, destWalletGetter),
		sourcefundsValidator,
		destfundsValiator,
		lowLevelSystemValidator,
	)
}

func messagePassingScenarioWalletV2(lowLevelSystemGetter validators.LowLevelSystemGetter, sourceChainIdx, destChainIdx uint64, sourceWalletGetter, destWalletGetter validators.WalletGetter) systest.InteropSystemTestFunc {
	return func(t systest.T, sys system.InteropSystem) {
		ctx := t.Context()
		llsys := lowLevelSystemGetter(ctx)

		logger := testlog.Logger(t, log.LevelInfo)
		logger = logger.With("test", "TestMessagePassing", "devnet", sys.Identifier())

		chainA := llsys.L2s()[sourceChainIdx]
		chainB := llsys.L2s()[destChainIdx]

		logger.Info("chain info", "sourceChain", chainA.ID(), "destChain", chainB.ID())

		// walletA is funded at chainA and want to initialize message at chain A
		walletA, err := system.NewWalletV2FromWalletAndChain(ctx, sourceWalletGetter(ctx), chainA)
		require.NoError(t, err)
		// userB is funded at chainB and want to execute message to chainB
		walletB, err := system.NewWalletV2FromWalletAndChain(ctx, destWalletGetter(ctx), chainB)
		require.NoError(t, err)

		optsFunc := func(w system.WalletV2) txplan.Option {
			opts := txplan.CombineOptions(
				system.DefaultTxSubmitOptions(w),
				system.DefaultTxInclusionOptions(w),
			)
			return opts
		}

		optsA := optsFunc(walletA)
		optsB := optsFunc(walletB)

		eventLogger := common.HexToAddress(predeploys.L2toL2CrossDomainMessenger)
		sha256PrecompileAddr := common.BytesToAddress([]byte{0x2})
		dummyMessage := []byte("l33t message")

		// Initiate message
		logger.Info("Initiate message", "address", sha256PrecompileAddr, "message", dummyMessage)

		txB := system.NewIntent[*system.RelayTrigger, *system.InteropOutput](optsB)
		destChainID, err := txB.PlannedTx.ChainID.Eval(ctx)
		require.NoError(t, err)
		opaqueData, err := BuildSendMessageCalldata(destChainID, sha256PrecompileAddr, dummyMessage)
		require.NoError(t, err)

		txA := system.NewIntent[*system.InitTrigger, *system.InteropOutput](optsA)
		txA.Content.Set(&system.InitTrigger{
			Emitter:    eventLogger,
			Topics:     []common.Hash{},
			OpaqueData: opaqueData,
		})

		recA, err := txA.PlannedTx.Included.Eval(ctx)
		require.NoError(t, err)
		logger.Info("Initiate message", "txHash", recA.TxHash.Hex())

		txB.Content.DependOn(&txA.Result)

		txB.Content.DependOn(&txA.Result)
		txB.Content.Fn(system.RelayIndexed(eventLogger, &txA.Result, &txA.PlannedTx.Included, 0))

		// Execute message
		logger.Info("Execute message", "address", sha256PrecompileAddr, "message", dummyMessage)
		recB, err := txB.PlannedTx.Included.Eval(ctx)
		require.NoError(t, err)

		execTxHash := recB.TxHash
		logger.Info("Execute message", "txHash", execTxHash.Hex())

		blockB, err := txB.PlannedTx.IncludedBlock.Eval(ctx)
		require.NoError(t, err)
		blockTimeB := big.NewInt(int64(blockB.Time))
		logger.Info("Execute message was included at", "timestamp", blockTimeB.String())

		// Validation that message has passed and got executed successfully
		gethClient, err := chainB.GethClient()
		require.NoError(t, err)

		trace, err := wait.DebugTraceTx(ctx, gethClient, execTxHash)
		require.NoError(t, err)

		precompile := vm.PrecompiledContractsHomestead[sha256PrecompileAddr]
		expected, err := precompile.Run(dummyMessage)
		require.NoError(t, err)
		logger.Info("sha256 computed offchain", "value", hex.EncodeToString(expected))

		// length of sha256 image is 32
		output := trace.CallTrace.Output
		require.GreaterOrEqual(t, len(output), 32)
		actual := []byte(output[len(output)-32:])
		logger.Info("sha256 computed onchain", "value", hex.EncodeToString(actual))

		require.Equal(t, expected, actual)
	}
}

func TestMessagePassingWalletV2(t *testing.T) {
	sourceChainIdx := uint64(0)
	destChainIdx := uint64(1)
	sourceWalletGetter, sourcefundsValidator := validators.AcquireL2WalletWithFunds(sourceChainIdx, sdktypes.NewBalance(big.NewInt(1.0*constants.ETH)))
	destWalletGetter, destfundsValiator := validators.AcquireL2WalletWithFunds(destChainIdx, sdktypes.NewBalance(big.NewInt(1.0*constants.ETH)))
	lowLevelSystemGetter, lowLevelSystemValidator := validators.AcquireLowLevelSystem()

	systest.InteropSystemTest(t,
		messagePassingScenarioWalletV2(lowLevelSystemGetter, sourceChainIdx, destChainIdx, sourceWalletGetter, destWalletGetter),
		sourcefundsValidator,
		destfundsValiator,
		lowLevelSystemValidator,
	)
}
