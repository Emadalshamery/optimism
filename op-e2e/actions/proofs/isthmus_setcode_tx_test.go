package proofs_test

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"io"
	"math/big"
	"testing"

	"math/rand"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm/program"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	actionsHelpers "github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/actions/proofs/helpers"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
)

func Test_ProgramAction_SetCodeTx(gt *testing.T) {
	matrix := helpers.NewMatrix[any]()

	matrix.AddDefaultTestCases(
		nil,
		helpers.LatestForkOnly,
		runSetCodeTxTypeTest,
	)

	matrix.Run(gt)
}

func runSetCodeTxTypeTest(gt *testing.T, testCfg *helpers.TestCfg[any]) {
	var (
		aa = common.HexToAddress("0x000000000000000000000000000000000000aaaa")
		bb = common.HexToAddress("0x000000000000000000000000000000000000bbbb")
	)

	t := actionsHelpers.NewDefaultTesting(gt)

	// hardcoded because it's not available until after we need it
	bobAddr := common.HexToAddress("0x14dC79964da2C08b23698B3D3cc7Ca32193d9955")

	// Create 2 contracts, (1) writes 42 to slot 42, (2) calls (1)
	store42Program := program.New().Sstore(0x42, 0x42)
	callBobProgram := program.New().Call(nil, bobAddr, 1, 0, 0, 0, 0)

	alloc := *actionsHelpers.DefaultAlloc
	alloc.L2Alloc = make(map[common.Address]types.Account)
	alloc.L2Alloc[aa] = types.Account{
		Code: store42Program.Bytes(),
	}
	alloc.L2Alloc[bb] = types.Account{
		Code: callBobProgram.Bytes(),
	}

	testCfg.Allocs = &alloc

	tp := helpers.NewTestParams()
	env := helpers.NewL2FaultProofEnv(t, testCfg, tp, helpers.NewBatcherCfg())
	require.Equal(gt, env.Bob.Address(), bobAddr)

	cl := env.Engine.EthClient()

	env.Sequencer.ActL2PipelineFull(t)
	env.Miner.ActEmptyBlock(t)
	env.Sequencer.ActL2StartBlock(t)

	aliceSecret := env.Alice.L2.Secret()
	bobSecret := env.Bob.L2.Secret()

	chainID := env.Sequencer.RollupCfg.L2ChainID

	// Sign authorization tuples.
	// The way the auths are combined, it becomes
	// 1. tx -> addr1 which is delegated to 0xaaaa
	// 2. addr1:0xaaaa calls into addr2:0xbbbb
	// 3. addr2:0xbbbb  writes to storage
	auth1, err := types.SignSetCode(aliceSecret, types.SetCodeAuthorization{
		ChainID: *uint256.MustFromBig(chainID),
		Address: bb,
		Nonce:   1,
	})
	require.NoError(gt, err, "failed to sign auth1")
	auth2, err := types.SignSetCode(bobSecret, types.SetCodeAuthorization{
		Address: aa,
		Nonce:   0,
	})
	require.NoError(gt, err, "failed to sign auth2")

	txdata := &types.SetCodeTx{
		ChainID:   uint256.MustFromBig(chainID),
		Nonce:     0,
		To:        env.Alice.Address(),
		Gas:       500000,
		GasFeeCap: uint256.NewInt(5000000000),
		GasTipCap: uint256.NewInt(2),
		AuthList:  []types.SetCodeAuthorization{auth1, auth2},
	}
	signer := types.NewIsthmusSigner(chainID)
	tx := types.MustSignNewTx(aliceSecret, signer, txdata)

	err = cl.SendTransaction(t.Ctx(), tx)
	require.NoError(gt, err, "failed to send set code tx")

	_, err = env.Engine.EngineApi.IncludeTx(tx, env.Alice.Address())
	require.NoError(t, err, "failed to include set code tx")

	env.Sequencer.ActL2EndBlock(t)

	// Verify delegation designations were deployed.
	bobCode, err := cl.PendingCodeAt(t.Ctx(), env.Bob.Address())
	require.NoError(gt, err, "failed to get bob code")
	want := types.AddressToDelegation(auth2.Address)
	if !bytes.Equal(bobCode, want) {
		t.Fatalf("addr1 code incorrect: got %s, want %s", common.Bytes2Hex(bobCode), common.Bytes2Hex(want))
	}
	aliceCode, err := cl.PendingCodeAt(t.Ctx(), env.Alice.Address())
	require.NoError(gt, err, "failed to get alice code")
	want = types.AddressToDelegation(auth1.Address)
	if !bytes.Equal(aliceCode, want) {
		t.Fatalf("addr2 code incorrect: got %s, want %s", common.Bytes2Hex(aliceCode), common.Bytes2Hex(want))
	}

	// Verify delegation executed the correct code.
	fortyTwo := common.BytesToHash([]byte{0x42})
	actual, err := cl.PendingStorageAt(t.Ctx(), env.Bob.Address(), fortyTwo)
	require.NoError(gt, err, "failed to get addr1 storage")

	if !bytes.Equal(actual, fortyTwo[:]) {
		t.Fatalf("addr2 storage wrong: expected %d, got %d", fortyTwo, actual)
	}

	// batch submit to L1. batcher should submit span batches.
	env.BatchAndMine(t)

	env.Sequencer.ActL1HeadSignal(t)
	env.Sequencer.ActL2PipelineFull(t)

	latestBlock, err := cl.BlockByNumber(t.Ctx(), nil)
	require.NoError(t, err, "error fetching latest block")

	env.RunFaultProofProgram(t, latestBlock.NumberU64(), testCfg.CheckResult, testCfg.InputParams...)
}

// TestInvalidSetCodeTxBatch tests that batches that include SetCodeTxs are dropped before Isthmus
func Test_ProgramAction_InvalidSetCodeTxBatch(gt *testing.T) {
	matrix := helpers.NewMatrix[any]()
	matrix.AddDefaultTestCases(
		nil,
		helpers.NewForkMatrix(helpers.Holocene),
		testInvalidSetCodeTxBatch,
	)
	matrix.Run(gt)
}

func testInvalidSetCodeTxBatch(gt *testing.T, testCfg *helpers.TestCfg[any]) {
	t := actionsHelpers.NewDefaultTesting(gt)
	env := helpers.NewL2FaultProofEnv(t, testCfg, helpers.NewTestParams(), helpers.NewBatcherCfg())
	sequencer := env.Sequencer
	miner := env.Miner
	batcher := env.Batcher

	sequencer.ActL2EmptyBlock(t)
	u1 := sequencer.L2Unsafe()
	sequencer.ActL2EmptyBlock(t) // we'll inject the setcode tx in this block's batch

	rng := rand.New(rand.NewSource(0))
	setcodetx := testutils.RandomSetCodeTx(rng, types.NewPragueSigner(env.Sd.RollupCfg.L2ChainID))
	batcher.ActL2BatchBuffer(t)
	batcher.ActL2BatchBuffer(t, func(block *types.Block) *types.Block {
		// inject user tx into upgrade batch
		return block.WithBody(types.Body{Transactions: append(block.Transactions(), setcodetx)})
	})
	batcher.ActL2ChannelClose(t)
	batcher.ActL2BatchSubmit(t)
	miner.ActL1StartBlock(12)(t)
	miner.ActL1IncludeTxByHash(env.Batcher.LastSubmitted.Hash())(t)
	miner.ActL1EndBlock(t)

	sequencer.ActL1HeadSignal(t)
	sequencer.ActL2PipelineFull(t)

	l2safe := sequencer.L2Safe()
	s2block := env.Engine.L2Chain().GetBlockByHash(l2safe.Hash)
	require.Len(t, s2block.Transactions(), 1, "safe head should only contain l1 info deposit")
	require.Equal(t, u1, l2safe, "expected last block to be reorgd out due to setcode tx")

	recs := env.Logs.FindLogs(testlog.NewMessageFilter("sequencers may not embed any SetCode transactions before Isthmus"))
	require.Len(t, recs, 1)

	env.RunFaultProofProgram(t, l2safe.Number, testCfg.CheckResult, testCfg.InputParams...)
}

func Test_ProgramAction_SetCodeTxWithContractCreationBitSet(gt *testing.T) {
	matrix := helpers.NewMatrix[any]()

	matrix.AddDefaultTestCases(
		nil,
		helpers.LatestForkOnly,
		runSetCodeTxTypeWithContractCreationBitSetTest,
	)

	matrix.Run(gt)
}

type readerWithCurrPos struct {
	io.Reader
	currPos uint
}

func (r *readerWithCurrPos) Read(p []byte) (n int, err error) {
	n, err = r.Reader.Read(p)
	r.currPos += uint(n)
	return n, err
}

func (r *readerWithCurrPos) CurrPos() uint {
	return r.currPos
}

func (r *readerWithCurrPos) ReadByte() (byte, error) {
	var b [1]byte
	_, err := r.Read(b[:])
	return b[0], err
}

func runSetCodeTxTypeWithContractCreationBitSetTest(gt *testing.T, testCfg *helpers.TestCfg[any]) {
	var (
		aa = common.HexToAddress("0x000000000000000000000000000000000000aaaa")
		bb = common.HexToAddress("0x000000000000000000000000000000000000bbbb")
	)

	t := actionsHelpers.NewDefaultTesting(gt)

	// hardcoded because it's not available until after we need it
	bobAddr := common.HexToAddress("0x14dC79964da2C08b23698B3D3cc7Ca32193d9955")

	// Create 2 contracts, (1) writes 42 to slot 42, (2) calls (1)
	store42Program := program.New().Sstore(0x42, 0x42)
	callBobProgram := program.New().Call(nil, bobAddr, 1, 0, 0, 0, 0)

	alloc := *actionsHelpers.DefaultAlloc
	alloc.L2Alloc = make(map[common.Address]types.Account)
	alloc.L2Alloc[aa] = types.Account{
		Code: store42Program.Bytes(),
	}
	alloc.L2Alloc[bb] = types.Account{
		Code: callBobProgram.Bytes(),
	}

	testCfg.Allocs = &alloc

	tp := helpers.NewTestParams()
	cfg := helpers.NewBatcherCfg()
	env := helpers.NewL2FaultProofEnv(t, testCfg, tp, cfg)
	sequencer := env.Sequencer
	miner := env.Miner
	batcher := env.Batcher

	require.Equal(gt, env.Bob.Address(), bobAddr)
	u1 := sequencer.L2Unsafe()

	sequencer.ActL2EmptyBlock(t) // we'll inject the setcode tx in this block's batch

	aliceSecret := env.Alice.L2.Secret()

	chainID := env.Sequencer.RollupCfg.L2ChainID

	// Sign authorization tuples.
	// The way the auths are combined, it becomes
	// 1. tx -> addr1 which is delegated to 0xaaaa
	// 2. addr1:0xaaaa calls into addr2:0xbbbb
	// 3. addr2:0xbbbb  writes to storage
	auth1, err := types.SignSetCode(aliceSecret, types.SetCodeAuthorization{
		ChainID: *uint256.MustFromBig(chainID),
		Address: bb,
		Nonce:   0,
	})
	require.NoError(gt, err, "failed to sign auth1")

	txdata := &types.SetCodeTx{
		ChainID:   uint256.MustFromBig(chainID),
		Nonce:     0,
		To:        env.Alice.Address(),
		Gas:       500000,
		GasFeeCap: uint256.NewInt(5000000000),
		GasTipCap: uint256.NewInt(2),
		AuthList:  []types.SetCodeAuthorization{auth1},
	}
	signer := types.NewIsthmusSigner(chainID)
	tx := types.MustSignNewTx(aliceSecret, signer, txdata)

	txdata2 := &types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     1,
		To:        nil,
		Gas:       500000,
		GasFeeCap: big.NewInt(5000000000),
		GasTipCap: big.NewInt(2),
	}
	tx2 := types.MustSignNewTx(aliceSecret, signer, txdata2)

	batcher.ActL2BatchBuffer(t, func(block *types.Block) *types.Block {
		// inject user tx into upgrade batch
		return block.WithBody(types.Body{Transactions: append(block.Transactions(), tx, tx2)})
	})
	batcher.ActL2ChannelClose(t)

	outputFrame := batcher.ReadNextOutputFrame(t)

	// Zlib decompress
	require.Equal(t, outputFrame[0], byte(0), "expected zlib compression")

	// extract frame data from frame
	frameDataLen := binary.BigEndian.Uint32(outputFrame[1+16+2 : 1+16+2+4])
	frameData := outputFrame[1+16+2+4 : 1+16+2+4+frameDataLen]

	// decompress with zlib
	bufReader := bytes.NewBuffer(frameData)
	decompressor, err := zlib.NewReader(bufReader)
	require.NoError(t, err)
	decompressedData, err := io.ReadAll(decompressor)
	require.NoError(t, err)

	// rlp decode
	var rlpDecoded []byte
	err = rlp.DecodeBytes(decompressedData, &rlpDecoded)
	require.NoError(t, err)

	// decode batch data to find contract creation bit offset
	batchType := rlpDecoded[0]
	require.Equal(t, batchType, byte(derive.SpanBatchType))
	batchBuf := &readerWithCurrPos{Reader: bytes.NewReader(rlpDecoded[1:])}

	// read prefix
	_, err = binary.ReadUvarint(batchBuf)
	require.NoError(t, err, "failed to read rel timestamp")
	_, err = binary.ReadUvarint(batchBuf)
	require.NoError(t, err, "failed to read l1 origin num timestamp")
	_, err = batchBuf.Read(make([]byte, 40))
	require.NoError(t, err, "failed to read parent and origin checks")

	// read payload
	blockCount, err := binary.ReadUvarint(batchBuf)
	require.NoError(t, err, "failed to read block count")

	originBitfieldSize := (blockCount + 7) / 8
	originBitfield := make([]byte, originBitfieldSize)
	_, err = batchBuf.Read(originBitfield)
	require.NoError(t, err, "failed to read origin bitfield")

	blockTxCounts := make([]uint64, blockCount)
	totalBlockTxCount := uint64(0)

	for i := uint64(0); i < blockCount; i++ {
		blockTxCount, err := binary.ReadUvarint(batchBuf)
		require.NoError(t, err, "failed to read block tx count")

		blockTxCounts[i] = blockTxCount
		totalBlockTxCount += blockTxCount
	}

	// read contract creation bits
	contractCreationBits := make([]byte, (totalBlockTxCount+7)/8)
	_, err = batchBuf.Read(contractCreationBits)
	require.NoError(t, err, "failed to read contract creation bits")

	offsetStart := uint64(batchBuf.CurrPos()) - uint64(totalBlockTxCount+7)/8 + 1
	require.Equal(t, contractCreationBits[0], byte(0b10), "expected contract creation bits to be 0b10")

	// flip the second bit (big endian)
	rlpDecoded[offsetStart] = 0b01 // unset bit 2 and set bit 1

	// re-encode rlp
	var buf bytes.Buffer
	err = rlp.Encode(&buf, rlpDecoded)
	require.NoError(t, err)

	// re-compress
	compressedBuf := new(bytes.Buffer)
	compressor, err := zlib.NewWriterLevel(compressedBuf, zlib.BestCompression)
	require.NoError(t, err)
	_, err = compressor.Write(buf.Bytes())
	require.NoError(t, err)
	err = compressor.Close()
	require.NoError(t, err)

	// replace the frame data with the new compressed data
	newOutputFrame := make([]byte, 1+16+2+4+len(compressedBuf.Bytes())+1)
	copy(newOutputFrame[:1+16+2], outputFrame[:1+16+2])
	binary.BigEndian.PutUint32(newOutputFrame[1+16+2:1+16+2+4], uint32(len(compressedBuf.Bytes())))
	copy(newOutputFrame[1+16+2+4:], compressedBuf.Bytes())
	newOutputFrame[len(newOutputFrame)-1] = outputFrame[len(outputFrame)-1]

	// submit new batch
	batcher.ActL2BatchSubmitRaw(t, newOutputFrame)

	miner.ActL1StartBlock(12)(t)
	miner.ActL1IncludeTxByHash(env.Batcher.LastSubmitted.Hash())(t)
	miner.ActL1EndBlock(t)

	sequencer.ActL1HeadSignal(t)
	sequencer.ActL2PipelineFull(t)

	l2safe := sequencer.L2Safe()
	s2block := env.Engine.L2Chain().GetBlockByHash(l2safe.Hash)
	require.Len(t, s2block.Transactions(), 0, "safe head should not contain either tx")
	require.Equal(t, u1, l2safe, "expected last block to be reorgd out due to setcode tx")

	// recs := env.Logs.FindLogs(testlog.NewMessageFilter("to address is required for SetCodeTx"))
	// require.Len(t, recs, 1)

	env.RunFaultProofProgram(t, l2safe.Number, testCfg.CheckResult, testCfg.InputParams...)
}
