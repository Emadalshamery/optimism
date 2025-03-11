package proofs_test

import (
	"testing"

	"github.com/ethereum-optimism/optimism/op-chain-ops/genesis"
	actionsHelpers "github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/actions/proofs/helpers"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
)

type upgradeBlockTestCfg struct {
	fork          rollup.ForkName
	numUpgradeTxs int
}

// TestUpgradeBlockGas tests that the upgrade block correctly adds ugprade gas
func TestUpgradeBlockGas(gt *testing.T) {
	matrix := helpers.NewMatrix[upgradeBlockTestCfg]()
	defer matrix.Run(gt)

	matrix.AddDefaultTestCases(
		upgradeBlockTestCfg{fork: rollup.Isthmus, numUpgradeTxs: 8},
		helpers.NewForkMatrix(helpers.Holocene),
		testUpgradeBlockGas,
	) //.AddDefaultTestCases(
	// 	upgradeBlockTestCfg{fork: rollup.Interop, numUpgradeTxs: 0},
	// 	helpers.NewForkMatrix(helpers.Isthmus),
	// 	testUpgradeBlock,
	// )
}

func testUpgradeBlockGas(gt *testing.T, testCfg *helpers.TestCfg[upgradeBlockTestCfg]) {
	tcfg := testCfg.Custom
	t := actionsHelpers.NewDefaultTesting(gt)
	testSetup := func(dc *genesis.DeployConfig) {
		dc.L1PragueTimeOffset = ptr(hexutil.Uint64(0))
		// activate fork after a few blocks
		dc.SetForkTimeOffset(tcfg.fork, ptr(uint64(4)))
	}
	env := helpers.NewL2FaultProofEnv(t, testCfg, helpers.NewTestParams(), helpers.NewBatcherCfg(), testSetup)

	engine := env.Engine
	sequencer := env.Sequencer
	miner := env.Miner
	rollupCfg := env.Sd.RollupCfg

	miner.ActEmptyBlock(t)

	sequencer.ActL1HeadSignal(t)
	sequencer.ActBuildL2ToFork(t, tcfg.fork)

	upgradeHeader := engine.L2Chain().CurrentHeader()
	require.Equal(t, tcfg.fork, rollupCfg.IsActivationBlock(upgradeHeader.Time-1, upgradeHeader.Time))

	upgradeBlock := engine.L2Chain().GetBlockByHash(upgradeHeader.Hash())
	require.Len(t, upgradeBlock.Transactions(), tcfg.numUpgradeTxs+1)
	var upgradeGas uint64
	for _, tx := range upgradeBlock.Transactions()[1:] /* skip l1 info deposit */ {
		upgradeGas += tx.Gas()
	}
	require.Equal(t, rollupCfg.Genesis.SystemConfig.GasLimit+upgradeGas, upgradeBlock.GasLimit())

	env.BatchAndMine(t)
	env.Sequencer.ActL1HeadSignal(t)
	env.Sequencer.ActL2PipelineFull(t)

	l2SafeHead := engine.L2Chain().CurrentSafeBlock()
	require.Equal(t, eth.HeaderBlockID(l2SafeHead), eth.HeaderBlockID(upgradeHeader), "derivation leads to the same block")

	env.RunFaultProofProgram(t, l2SafeHead.Number.Uint64(), testCfg.CheckResult, testCfg.InputParams...)
}

// TestUpgradeBlockTxOmission tests that the sequencer omits user transactions in upgrade blocks
// and that batches that contain user transactions in an upgrade block are dropped.
func TestUpgradeBlockTxOmission(gt *testing.T) {
	matrix := helpers.NewMatrix[upgradeBlockTestCfg]()
	defer matrix.Run(gt)

	matrix.AddDefaultTestCases(
		upgradeBlockTestCfg{fork: rollup.Isthmus, numUpgradeTxs: 8},
		helpers.NewForkMatrix(helpers.Holocene),
		testUpgradeBlockTxOmission,
	) //.AddDefaultTestCases(
	// 	upgradeBlockTestCfg{fork: rollup.Interop, numUpgradeTxs: 0},
	// 	helpers.NewForkMatrix(helpers.Isthmus),
	// 	testUpgradeBlockTxOmission,
	// )
}

func testUpgradeBlockTxOmission(gt *testing.T, testCfg *helpers.TestCfg[upgradeBlockTestCfg]) {
	tcfg := testCfg.Custom
	t := actionsHelpers.NewDefaultTesting(gt)
	offset := uint64(4)
	testSetup := func(dc *genesis.DeployConfig) {
		dc.L1PragueTimeOffset = ptr(hexutil.Uint64(0))
		// activate fork after a few blocks
		dc.SetForkTimeOffset(tcfg.fork, &offset)
	}
	env := helpers.NewL2FaultProofEnv(t, testCfg, helpers.NewTestParams(), helpers.NewBatcherCfg(), testSetup)

	engine := env.Engine
	sequencer := env.Sequencer
	miner := env.Miner
	rollupCfg := env.Sd.RollupCfg
	blockTime := rollupCfg.BlockTime

	miner.ActEmptyBlock(t)

	sequencer.ActL1HeadSignal(t)
	for i := 0; i < int(offset)-1; i++ {
		sequencer.ActL2EmptyBlock(t)
	}
	tx := env.Alice.L2.ActMakeTx(t)
	sequencer.ActL2StartBlock(t)
	// we assert later that the sequencer actually omits this tx in the upgrade block
	engine.ActL2IncludeTx(env.Alice.Address())
	sequencer.ActL2EndBlock(t)

	upgradeHeader := engine.L2Chain().CurrentHeader()
	require.Equal(t, tcfg.fork,
		rollupCfg.IsActivationBlock(upgradeHeader.Time-blockTime, upgradeHeader.Time),
		"this block should be upgrade block")
	upgradeBlock := engine.L2Chain().GetBlockByHash(upgradeHeader.Hash())
	require.Len(t, upgradeBlock.Transactions(), tcfg.numUpgradeTxs+1, "upgrade block doesn't contain alice tx")

	batcher := env.Batcher
	for i := 0; i < int(offset)-1; i++ {
		batcher.ActL2BatchBuffer(t)
	}
	batcher.ActL2BatchBuffer(t, func(block *types.Block) *types.Block {
		// inject user tx into upgrade batch
		return block.WithBody(types.Body{Transactions: append(block.Transactions(), tx)})
	})

	batcher.ActL2ChannelClose(t)
	batcher.ActL2BatchSubmit(t)
	env.Miner.ActL1StartBlock(12)(t)
	env.Miner.ActL1IncludeTxByHash(env.Batcher.LastSubmitted.Hash())(t)
	env.Miner.ActL1EndBlock(t)

	env.Sequencer.ActL1HeadSignal(t)
	env.Sequencer.ActL2PipelineFull(t)

	recs := env.Logs.FindLogs(testlog.NewMessageFilter("dropping batch with user transactions in fork activation block"))
	require.Len(t, recs, 1)

	l2SafeHead := engine.L2Chain().CurrentSafeBlock()
	preUpgradeHeader := engine.L2Chain().GetHeaderByHash(upgradeHeader.ParentHash)
	require.Equal(t, eth.HeaderBlockID(preUpgradeHeader), eth.HeaderBlockID(l2SafeHead), "derivation only reaches pre-upgrade block")

	env.RunFaultProofProgram(t, l2SafeHead.Number.Uint64(), testCfg.CheckResult, testCfg.InputParams...)
}
