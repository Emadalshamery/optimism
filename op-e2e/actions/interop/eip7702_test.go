package interop

import (
	"crypto/ecdsa"
	"math/big"
	"strings"
	"testing"

	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"

	"github.com/ethereum-optimism/optimism/op-chain-ops/devkeys"

	"github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/actions/interop/dsl"
	"github.com/ethereum-optimism/optimism/op-e2e/bindings"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/interop/contracts/bindings/wallet"
)

// WalletUser holds user keys and address
type WalletUser struct {
	Key     devkeys.ChainUserKey
	Secret  *ecdsa.PrivateKey
	Address common.Address
	ChainID *big.Int
}

// TestWalletDeployment tests basic wallet deployment for EIP-7702
func TestWalletDeployment(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)

	is := dsl.SetupInterop(t)
	actors := is.CreateActors()
	actors.PrepareChainState(t)

	// Setup user
	aliceKey := devkeys.ChainUserKeys(actors.ChainA.RollupCfg.L2ChainID)(0)
	secret, err := is.Keys.Secret(aliceKey)
	require.NoError(t, err)
	sender := crypto.PubkeyToAddress(secret.PublicKey)

	// Create transaction options
	auth, err := bind.NewKeyedTransactorWithChainID(secret, actors.ChainA.RollupCfg.L2ChainID)
	require.NoError(t, err)
	auth.GasTipCap = big.NewInt(params.GWei)
	auth.GasLimit = 3000000 // Set high gas limit for deployment

	// Deploy wallet contract
	addr, tx, _, err := wallet.DeployWallet(auth, actors.ChainA.SequencerEngine.EthClient())
	require.NoError(t, err)

	// Create block with the deployment transaction
	actors.ChainA.Sequencer.ActL2StartBlock(t)
	_, err = actors.ChainA.SequencerEngine.EngineApi.IncludeTx(tx, sender)
	require.NoError(t, err)
	actors.ChainA.Sequencer.ActL2EndBlock(t)

	// Sync the actors to process the block
	actors.ChainA.Sequencer.SyncSupervisor(t)
	actors.Supervisor.ProcessFull(t)
	actors.ChainA.Sequencer.ActL2PipelineFull(t)

	// Verify contract code exists at deployed address
	code, err := actors.ChainA.SequencerEngine.EthClient().CodeAt(t.Ctx(), addr, nil)
	require.NoError(t, err)
	require.NotEmpty(t, code, "Contract code should exist at deployed address")

	t.Log("Wallet deployed at address:", addr.Hex())
}

// setupWalletUser creates a new user for testing
func setupWalletUser(t helpers.Testing, is *dsl.InteropSetup, chain *dsl.Chain, keyIndex int) *WalletUser {
	userKey := devkeys.ChainUserKeys(chain.RollupCfg.L2ChainID)(uint64(keyIndex))
	secret, err := is.Keys.Secret(userKey)
	require.NoError(t, err)
	return &WalletUser{
		Key:     userKey,
		Secret:  secret,
		Address: crypto.PubkeyToAddress(secret.PublicKey),
		ChainID: new(big.Int).Set(chain.ChainID.ToBig()),
	}
}

// newWalletTxOpts creates transaction options for a chain
func newWalletTxOpts(t helpers.Testing, secret *ecdsa.PrivateKey, chain *dsl.Chain) *bind.TransactOpts {
	auth, err := bind.NewKeyedTransactorWithChainID(secret, chain.RollupCfg.L2ChainID)
	require.NoError(t, err)
	auth.GasTipCap = big.NewInt(params.GWei)
	auth.GasLimit = 3000000 // Set high gas limit
	return auth
}

// Helper function to create EIP-7702 authorization
func createAuthorization(t helpers.Testing, key *ecdsa.PrivateKey, chainID *big.Int, code []byte) types.SetCodeAuthorization {
	auth := types.SetCodeAuthorization{
		ChainID: *uint256.MustFromBig(chainID),
		Address: crypto.PubkeyToAddress(key.PublicKey),
		Nonce:   0,
	}
	signedAuth, err := types.SignSetCode(key, auth)
	require.NoError(t, err)
	return signedAuth
}

// Helper function to create EIP-7702 set-code transaction
func createSetCodeTx(t helpers.Testing, auth types.SetCodeAuthorization, user *WalletUser) *types.Transaction {
	tx := types.NewTx(&types.SetCodeTx{
		ChainID:    uint256.MustFromBig(user.ChainID),
		Nonce:      0,
		GasTipCap:  uint256.MustFromBig(big.NewInt(params.GWei)),
		GasFeeCap:  uint256.MustFromBig(big.NewInt(2 * params.GWei)),
		Gas:        100000,
		To:         user.Address,
		Value:      uint256.NewInt(0),
		Data:       nil,
		AccessList: nil,
		AuthList:   []types.SetCodeAuthorization{auth},
	})

	signer := types.LatestSignerForChainID(user.ChainID)
	signedTx, err := types.SignTx(tx, signer, user.Secret)
	require.NoError(t, err)
	return signedTx
}

// TestCrossChainWallet demonstrates how EIP-7702 enables cross-chain identity
func TestCrossChainWallet(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)
	is := dsl.SetupInterop(t)
	actors := is.CreateActors()
	actors.PrepareChainState(t)

	aliceA := setupWalletUser(t, is, actors.ChainA, 0)
	aliceB := setupWalletUser(t, is, actors.ChainB, 0)

	balanceA, err := actors.ChainA.SequencerEngine.EthClient().BalanceAt(t.Ctx(), aliceA.Address, nil)
	require.NoError(t, err)
	require.True(t, balanceA.Cmp(big.NewInt(0)) > 0, "Alice must have funds on Chain A")

	balanceB, err := actors.ChainB.SequencerEngine.EthClient().BalanceAt(t.Ctx(), aliceB.Address, nil)
	require.NoError(t, err)
	require.True(t, balanceB.Cmp(big.NewInt(0)) > 0, "Alice must have funds on Chain B")

	authA := newWalletTxOpts(t, aliceA.Secret, actors.ChainA)
	authB := newWalletTxOpts(t, aliceB.Secret, actors.ChainB)

	authA.Nonce = big.NewInt(0)
	authA.GasTipCap = big.NewInt(params.GWei)
	authB.Nonce = big.NewInt(0)
	authB.GasTipCap = big.NewInt(params.GWei)

	walletAddrA, txA, walletContractA, err := wallet.DeployWallet(authA, actors.ChainA.SequencerEngine.EthClient())
	require.NoError(t, err)
	includeTxOnChain(t, actors, actors.ChainA, txA, aliceA.Address)

	walletAddrB, txB, walletContractB, err := wallet.DeployWallet(authB, actors.ChainB.SequencerEngine.EthClient())
	require.NoError(t, err)
	includeTxOnChain(t, actors, actors.ChainB, txB, aliceB.Address)

	authA.Nonce = big.NewInt(1)
	authB.Nonce = big.NewInt(1)

	walletAddr := walletAddrA

	codeA, err := actors.ChainA.SequencerEngine.EthClient().CodeAt(t.Ctx(), walletAddrA, nil)
	require.NoError(t, err)
	require.NotEmpty(t, codeA, "Wallet code should exist on Chain A")

	codeB, err := actors.ChainB.SequencerEngine.EthClient().CodeAt(t.Ctx(), walletAddrB, nil)
	require.NoError(t, err)
	require.NotEmpty(t, codeB, "Wallet code should exist on Chain B")

	require.Equal(t, codeA, codeB, "Wallet code should be identical on both chains")

	multicallAddr := common.HexToAddress("0xcA11bde05977b3631167028862bE2a173976CA11")
	multicallABI, err := bindings.MultiCall3MetaData.GetAbi()
	require.NoError(t, err)

	getChainIDData, err := multicallABI.Pack("getChainId")
	require.NoError(t, err)

	callData := []bindings.Multicall3Call{{
		Target:   multicallAddr,
		CallData: getChainIDData,
	}}
	tokenTransferPayload, err := multicallABI.Pack("aggregate", callData)
	require.NoError(t, err)

	messengerABI, err := abi.JSON(strings.NewReader(`[{"inputs":[{"internalType":"uint256","name":"_chainid","type":"uint256"},{"internalType":"address","name":"_target","type":"address"},{"internalType":"bytes","name":"_data","type":"bytes"}],"name":"sendMessage","outputs":[],"stateMutability":"nonpayable","type":"function"}]`))
	require.NoError(t, err)

	crossChainData, err := messengerABI.Pack("sendMessage",
		actors.ChainB.ChainID.ToBig(),
		walletAddr,
		tokenTransferPayload)
	require.NoError(t, err)

	txCrossChain, err := walletContractA.Fallback(authA, crossChainData)
	require.NoError(t, err)
	includeTxOnChain(t, actors, actors.ChainA, txCrossChain, aliceA.Address)

	receiptCross, err := actors.ChainA.SequencerEngine.EthClient().TransactionReceipt(t.Ctx(), txCrossChain.Hash())
	require.NoError(t, err)
	require.Equal(t, types.ReceiptStatusSuccessful, receiptCross.Status, "Cross-chain initiation should succeed")

	actors.ChainA.Sequencer.SyncSupervisor(t)
	actors.ChainB.Sequencer.SyncSupervisor(t)
	actors.Supervisor.ProcessFull(t)
	actors.ChainA.Sequencer.ActL2PipelineFull(t)
	actors.ChainB.Sequencer.ActL2PipelineFull(t)

	actors.ChainA.Sequencer.SyncSupervisor(t)
	actors.ChainB.Sequencer.SyncSupervisor(t)
	actors.Supervisor.ProcessFull(t)
	actors.ChainA.Sequencer.ActL2PipelineFull(t)
	actors.ChainB.Sequencer.ActL2PipelineFull(t)

	statusB := actors.ChainB.Sequencer.SyncStatus()
	require.True(t, statusB.CrossUnsafeL2.Number > 0, "Chain B should have processed cross-chain message")

	code, err := actors.ChainB.SequencerEngine.EthClient().CodeAt(t.Ctx(), walletAddr, nil)
	require.NoError(t, err)
	require.NotEmpty(t, code, "Wallet code should exist on Chain B after message processing")

	calldata := []byte{0x01, 0x02, 0x03}
	_, err = walletContractB.Fallback(authB, calldata)
	require.Error(t, err, "Direct call to wallet should fail (only the wallet itself can call it)")

	assertHeads(t, actors.ChainB, statusB.UnsafeL2.Number, statusB.LocalSafeL2.Number,
		statusB.CrossUnsafeL2.Number, statusB.SafeL2.Number)
}

// // includeTxOnChain includes a transaction in a block and syncs the chain
// func includeTxOnChain(t helpers.Testing, actors *dsl.InteropActors, chain *dsl.Chain, tx *types.Transaction, sender common.Address) {
// 	chain.Sequencer.ActL2StartBlock(t)
// 	if tx != nil {
// 		_, err := chain.SequencerEngine.EngineApi.IncludeTx(tx, sender)
// 		require.NoError(t, err)
// 	}
// 	chain.Sequencer.ActL2EndBlock(t)

// 	// Sync the chain and the supervisor
// 	chain.Sequencer.SyncSupervisor(t)
// 	actors.Supervisor.ProcessFull(t)

// 	// Add to L1
// 	chain.Batcher.ActSubmitAll(t)
// 	actors.L1Miner.ActL1StartBlock(12)(t)
// 	actors.L1Miner.ActL1IncludeTx(chain.BatcherAddr)(t)
// 	actors.L1Miner.ActL1EndBlock(t)

// 	// Complete L1 data processing
// 	chain.Sequencer.ActL2EventsUntil(t, event.Is[derive.ExhaustedL1Event], 100, false)
// 	actors.Supervisor.SignalLatestL1(t)
// 	chain.Sequencer.SyncSupervisor(t)
// 	chain.Sequencer.ActL2PipelineFull(t)

// 	// Final sync of both chains
// 	actors.ChainA.Sequencer.SyncSupervisor(t)
// 	actors.ChainB.Sequencer.SyncSupervisor(t)
// 	actors.Supervisor.ProcessFull(t)
// 	actors.ChainA.Sequencer.ActL2PipelineFull(t)
// 	actors.ChainB.Sequencer.ActL2PipelineFull(t)
// }

// // assertHeads verifies the chain heads match expected values
// func assertHeads(t helpers.Testing, chain *dsl.Chain, unsafe, localSafe, crossUnsafe, safe uint64) {
// 	status := chain.Sequencer.SyncStatus()
// 	require.Equal(t, unsafe, status.UnsafeL2.ID().Number, "Unsafe")
// 	require.Equal(t, crossUnsafe, status.CrossUnsafeL2.ID().Number, "Cross Unsafe")
// 	require.Equal(t, localSafe, status.LocalSafeL2.ID().Number, "Local safe")
// 	require.Equal(t, safe, status.SafeL2.ID().Number, "Safe")
// }

// Test7702SetCode demonstrates setting code on an EOA using EIP-7702
func Test7702SetCode(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)

	is := dsl.SetupInterop(t)
	actors := is.CreateActors()
	actors.PrepareChainState(t)

	alice := setupWalletUser(t, is, actors.ChainA, 0)
	t.Log("Alice's address:", alice.Address.Hex())

	balance, err := actors.ChainA.SequencerEngine.EthClient().BalanceAt(t.Ctx(), alice.Address, nil)
	require.NoError(t, err)
	require.True(t, balance.Cmp(big.NewInt(0)) > 0, "Alice must have funds")

	code := common.FromHex("0x6042600052600a6016f3")
	auth := createAuthorization(t, alice.Secret, alice.ChainID, code)
	tx := createSetCodeTx(t, auth, alice)

	actors.ChainA.Sequencer.ActL2StartBlock(t)
	includedReceipt, err := actors.ChainA.SequencerEngine.EngineApi.IncludeTx(tx, alice.Address)
	require.NoError(t, err)
	require.NotNil(t, includedReceipt, "Transaction should be included")
	actors.ChainA.Sequencer.ActL2EndBlock(t)

	actors.ChainA.Sequencer.SyncSupervisor(t)
	actors.Supervisor.ProcessFull(t)
	actors.ChainA.Sequencer.ActL2PipelineFull(t)

	receipt, err := actors.ChainA.SequencerEngine.EthClient().TransactionReceipt(t.Ctx(), tx.Hash())
	require.NoError(t, err)
	require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status, "Transaction should succeed")

	setCode, err := actors.ChainA.SequencerEngine.EthClient().CodeAt(t.Ctx(), alice.Address, nil)
	require.NoError(t, err)
	require.Equal(t, code, setCode, "Code should be set correctly on the EOA")
}
