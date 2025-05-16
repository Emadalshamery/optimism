package loadtest

import (
	"context"
	"math"
	"math/big"
	"math/rand"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/devnet-sdk/contracts/constants"
	"github.com/ethereum-optimism/optimism/op-acceptance-tests/tests/interop"
	"github.com/ethereum-optimism/optimism/op-devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-devstack/dsl"
	"github.com/ethereum-optimism/optimism/op-devstack/presets"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/retry"
	"github.com/ethereum-optimism/optimism/op-service/txintent"
	"github.com/ethereum-optimism/optimism/op-service/txplan"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/common"
)

var rng = rand.New(rand.NewSource(1234))

func TestMain(m *testing.M) {
	presets.DoMain(m, presets.WithSimpleInterop())
}

func TestLoad(gt *testing.T) {
	if testing.Short() {
		gt.Skip("skipping load test in short mode")
	}
	t := devtest.SerialT(gt)
	sys := presets.NewSimpleInterop(t)

	SpamInteropTxs(t, sys.L2A, sys.L2B, sys.Wallet, sys.Supervisor)
	SpamInteropTxs(t, sys.L2B, sys.L2A, sys.Wallet, sys.Supervisor)
}

func SpamInteropTxs(t devtest.T, from *presets.InteropL2, to *presets.InteropL2, wallet *dsl.HDWallet, supervisor *dsl.Supervisor) {
	eventLogger := from.Funder.NewFundedEOA(eth.OneEther).DeployEventLogger()

	fromL2EL := from.L2Chain.PublicRPC()
	toL2EL := to.L2Chain.PublicRPC()

	var wg sync.WaitGroup
	defer wg.Wait()
	msgsCh := make(chan []types.Message, 1_000)
	defer close(msgsCh)

	// Spam initiating messages.
	initiators := []Initiator{
		&ManyMsgsInitiator{
			s:           newL2Spammer(wallet, from.Faucet, fromL2EL),
			eventLogger: eventLogger,
		},
		&LargeMsgInitiator{
			s:           newL2Spammer(wallet, from.Faucet, fromL2EL),
			eventLogger: eventLogger,
		},
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			for _, initiator := range initiators {
				msgsCh <- initiator.Initiate(t)
			}
		}
	}()

	// Spam executing messages.
	executors := []Executor{
		&ValidExecutor{
			s:          newL2Spammer(wallet, to.Faucet, toL2EL),
			supervisor: supervisor,
		},
		NewDelayedExecutor(&ValidExecutor{
			s:          newL2Spammer(wallet, to.Faucet, toL2EL),
			supervisor: supervisor,
		}, time.Minute, 1_000),
		&InvalidExecutor{
			s:           newL2Spammer(wallet, to.Faucet, toL2EL),
			makeInvalid: makeInvalidChainID,
		},
		&InvalidExecutor{
			s:           newL2Spammer(wallet, to.Faucet, toL2EL),
			makeInvalid: makeInvalidBlockNumber,
		},
		&InvalidExecutor{
			s:           newL2Spammer(wallet, to.Faucet, toL2EL),
			makeInvalid: makeInvalidLogIndex,
		},
		&InvalidExecutor{
			s:           newL2Spammer(wallet, to.Faucet, toL2EL),
			makeInvalid: makeInvalidOrigin,
		},
		&InvalidExecutor{
			s:           newL2Spammer(wallet, to.Faucet, toL2EL),
			makeInvalid: makeInvalidPayloadHash,
		},
		&InvalidExecutor{
			s:           newL2Spammer(wallet, to.Faucet, toL2EL),
			makeInvalid: makeInvalidTimestamp,
		},
	}
	for msgs := range msgsCh {
		for _, executor := range executors {
			wg.Add(1)
			go func() {
				defer wg.Done()
				executor.Execute(t, msgs)
			}()
		}
	}
}

func newL2Spammer(wallet *dsl.HDWallet, faucet *dsl.Faucet, l2EL *dsl.L2ELNode) *L2Spammer {
	return &L2Spammer{
		eoa: fundNewEOA(wallet, faucet, l2EL),
		el:  l2EL,
	}
}

type Executor interface {
	Execute(t devtest.T, msgs []types.Message)
}

func fundNewEOA(wallet *dsl.HDWallet, faucet *dsl.Faucet, el *dsl.L2ELNode) *dsl.EOA {
	eoa := wallet.NewEOA(el)
	faucet.Fund(eoa.Address(), eth.MillionEther)
	return eoa
}

type Initiator interface {
	Initiate(t devtest.T) []types.Message
}

type ManyMsgsInitiator struct {
	s           *L2Spammer
	eventLogger common.Address
}

func (in *ManyMsgsInitiator) Initiate(t devtest.T) []types.Message {
	const numMsgs = 275 // About the max number of msgs we can create before hitting tx size limits.
	initCalls := make([]txintent.Call, 0, numMsgs)
	for range numMsgs {
		initCalls = append(initCalls, interop.RandomInitTrigger(rng, in.eventLogger, rng.Intn(5), rng.Intn(10)))
	}
	return in.s.BuildAndSendInitTx(t, initCalls)
}

type LargeMsgInitiator struct {
	s           *L2Spammer
	eventLogger common.Address
}

func (lin *LargeMsgInitiator) Initiate(t devtest.T) []types.Message {
	return lin.s.BuildAndSendInitTx(t, []txintent.Call{interop.RandomInitTrigger(rng, lin.eventLogger, 4, 100_000)})
}

type ValidExecutor struct {
	s *L2Spammer
	// supervisor is used to check if executing messages are cross-safe.
	supervisor *dsl.Supervisor
}

func retryForever(g txplan.ReceiptGetter) txplan.Option {
	return txplan.WithRetryInclusion(g, math.MaxInt, retry.Exponential())
}

func maxTimestamp(msgs []types.Message) uint64 {
	return slices.MaxFunc(msgs, func(x, y types.Message) int {
		if x.Identifier.Timestamp > y.Identifier.Timestamp {
			return 1
		} else if x.Identifier.Timestamp < y.Identifier.Timestamp {
			return -1
		}
		return 0
	}).Identifier.Timestamp
}

func (e *ValidExecutor) Execute(t devtest.T, msgs []types.Message) {
	tx := e.s.BuildExecTx(t, msgs, retryForever(e.s.el.Escape().EthClient()))
	receipt, err := tx.Included.Eval(t.Ctx())
	t.Require().NoError(err)
	t.Require().Len(receipt.Logs, len(msgs))

	t.Require().NoError(err)

	// Wait for the transaction to be cross-safe.
	includedBlock, err := tx.IncludedBlock.Eval(t.Ctx())
	t.Require().NoError(err)
	for {
		// NOTE: it may be desirable to query proxyd instead of the supervisor if/when the devstack supports it.
		crossSafeID, err := e.supervisor.Escape().QueryAPI().CrossSafe(t.Ctx(), e.s.el.ChainID())
		t.Require().NoError(err)
		if includedBlock.ID().Number <= crossSafeID.Derived.Number {
			break
		}
		e.s.el.WaitForBlock()
	}
	// Sanity check that includedBlock is still in the canonical chain.
	_, err = e.s.el.Escape().EthClient().BlockRefByHash(t.Ctx(), includedBlock.Hash)
	t.Require().NoError(err)
}

// DelayedExecutor executes messages after waiting for a specified period.
type DelayedExecutor struct {
	e       *ValidExecutor
	msgs    map[uint64][]types.Message
	delay   time.Duration
	maxMsgs int
	numMsgs int
}

func NewDelayedExecutor(e *ValidExecutor, delay time.Duration, maxMsgs int) *DelayedExecutor {
	return &DelayedExecutor{
		e:       e,
		msgs:    make(map[uint64][]types.Message, 0),
		maxMsgs: maxMsgs,
	}
}

func (de *DelayedExecutor) Execute(t devtest.T, newMsgs []types.Message) {
	// Add newMsgs to de.msgs if there is space.
	if remainingSpace := de.maxMsgs - de.numMsgs; remainingSpace > 0 {
		newMsgsToExecute := newMsgs
		if remainingSpace > len(newMsgsToExecute) {
			remainingSpace = len(newMsgsToExecute)
		}
		newMsgsToExecute = newMsgsToExecute[:remainingSpace]
		if len(newMsgsToExecute) > 0 {
			msgsAtTimestamp := de.msgs[newMsgsToExecute[0].Identifier.Timestamp]
			msgsAtTimestamp = append(msgsAtTimestamp, newMsgsToExecute...)
			de.numMsgs += len(newMsgsToExecute)
		}
	}

	// Remove and execute msgs whose delay has elapsed.
	msgsToExecute := make([]types.Message, 0)
	for t, msgs := range de.msgs {
		if time.Since(time.Unix(int64(t), 0)) >= de.delay {
			msgsToExecute = append(msgsToExecute, msgs...)
			delete(de.msgs, t)
		}
	}
	if len(msgsToExecute) > 0 {
		de.e.Execute(t, msgsToExecute)
	}
}

type L2Spammer struct {
	eoa   *dsl.EOA
	el    *dsl.L2ELNode
	mu    sync.Mutex
	nonce uint64
}

func (s *L2Spammer) loadAndIncrementNonce() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	nonce := s.nonce
	s.nonce++
	return nonce
}

func (s *L2Spammer) BuildAndSendInitTx(t devtest.T, initCalls []txintent.Call) []types.Message {
	initMsgsTx := txintent.NewIntent[*txintent.MultiTrigger, *txintent.InteropOutput](s.eoa.Plan(), retryForever(s.el.Escape().EthClient()))
	initMsgsTx.Content.Set(&txintent.MultiTrigger{
		Emitter: constants.MultiCall3,
		Calls:   initCalls,
	})
	initMsgsTx.PlannedTx.Nonce.Fn(func(_ context.Context) (uint64, error) {
		return s.loadAndIncrementNonce(), nil
	})
	_, err := initMsgsTx.PlannedTx.Success.Eval(t.Ctx())
	t.Require().NoError(err)
	out, err := initMsgsTx.Result.Eval(t.Ctx())
	t.Require().NoError(err)
	return out.Entries
}

func (s *L2Spammer) BuildExecTx(t devtest.T, msgs []types.Message, opts ...txplan.Option) *txplan.PlannedTx {
	execCalls := make([]txintent.Call, 0, len(msgs))
	for _, msg := range msgs {
		execCalls = append(execCalls, &txintent.ExecTrigger{
			Executor: constants.CrossL2Inbox,
			Msg:      msg,
		})
	}
	tx := txintent.NewIntent[*txintent.MultiTrigger, txintent.Result](s.eoa.Plan(), txplan.Combine(opts...))
	tx.Content.Set(&txintent.MultiTrigger{
		Emitter: constants.MultiCall3,
		Calls:   execCalls,
	})
	tx.PlannedTx.Nonce.Fn(func(_ context.Context) (uint64, error) {
		return s.loadAndIncrementNonce(), nil
	})

	// The exec tx is invalid until we know it will be included at a higher timestamp than any of the initiating messages, modulo reorgs.
	// NOTE: this should be `<`, but the mempool filtering in op-geth currently uses the unsafe head's timestamp instead of
	// the pending timestamp. See https://github.com/ethereum-optimism/op-geth/issues/603.
	for t := maxTimestamp(msgs); s.el.BlockRefByLabel(eth.Unsafe).Time <= t; {
		s.el.WaitForBlock()
	}
	return tx.PlannedTx
}

type InvalidExecutor struct {
	s           *L2Spammer
	makeInvalid func(types.Message) types.Message
}

func (ie *InvalidExecutor) Execute(t devtest.T, msgs []types.Message) {
	invalidMsg := ie.makeInvalid(msgs[len(msgs)-1])
	tx := ie.s.BuildExecTx(t, append(msgs[:len(msgs)-1], invalidMsg))
	_, err := tx.Submitted.Eval(t.Ctx())
	t.Require().NoError(err)
	_, err = tx.Included.Eval(t.Ctx())
	t.Require().Error(err)
}

func makeInvalidChainID(msg types.Message) types.Message {
	bigChainID := msg.Identifier.ChainID.ToBig()
	bigChainID.Add(bigChainID, big.NewInt(1))
	msg.Identifier.ChainID = eth.ChainIDFromBig(bigChainID)
	return msg
}

func makeInvalidOrigin(msg types.Message) types.Message {
	bigOrigin := msg.Identifier.Origin.Big()
	bigOrigin.Add(bigOrigin, big.NewInt(1))
	msg.Identifier.Origin = common.BigToAddress(bigOrigin)
	return msg
}

func makeInvalidBlockNumber(msg types.Message) types.Message {
	msg.Identifier.BlockNumber++
	return msg
}

func makeInvalidLogIndex(msg types.Message) types.Message {
	msg.Identifier.LogIndex++
	return msg
}

func makeInvalidTimestamp(msg types.Message) types.Message {
	msg.Identifier.Timestamp++
	return msg
}

func makeInvalidPayloadHash(msg types.Message) types.Message {
	bigPayloadHash := msg.PayloadHash.Big()
	bigPayloadHash.Add(bigPayloadHash, big.NewInt(1))
	msg.PayloadHash = common.BigToHash(bigPayloadHash)
	return msg
}
