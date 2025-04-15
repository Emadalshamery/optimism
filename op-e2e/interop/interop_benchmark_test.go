package interop

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-chain-ops/interopgen"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/rand"
)

// setupAndRun is a helper function that sets up a SuperSystem
// which contains two L2 Chains, and two users on each chain.
func setup(t testing.TB, config SuperSystemConfig) SuperSystem {
	recipe := interopgen.InteropDevRecipe{
		L1ChainID:        900100,
		L2s:              []interopgen.InteropDevL2Recipe{{ChainID: 900200}, {ChainID: 900201}},
		GenesisTimestamp: uint64(time.Now().Unix() + 3), // start chain 3 seconds from now
	}
	worldResources := WorldResourcePaths{
		FoundryArtifacts: "../../packages/contracts-bedrock/forge-artifacts",
		SourceMap:        "../../packages/contracts-bedrock",
	}

	// create a super system from the recipe
	// and get the L2 IDs for use in the test
	s2 := NewSuperSystem(t, &recipe, worldResources, config)

	// create two users on all L2 chains
	s2.AddUser("Alice")
	s2.AddUser("Bob")
	s2.AddUser("Charlie")
	s2.AddUser("Dennis")
	s2.AddUser("Eve")
	s2.AddUser("Frank")

	return s2
}

func BenchmarkCheckMessages(b *testing.B) {
	// Skip this benchmark
	b.Skip("Run benchmarks on demand only")

	// Set up the SuperSystem once before the benchmark loop
	s2 := setup(b, SuperSystemConfig{})

	// Set up chains with Emitter Contracts
	chains := s2.L2IDs()
	chainA := chains[0]
	chainB := chains[1]
	clientA := s2.L2GethClient(chainA, "sequencer")
	clientB := s2.L2GethClient(chainB, "sequencer")
	// Deploy emitter to chain A
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	EmitterA := s2.DeployEmitterContract(ctx, chainA, "Alice")

	// Deploy emitter to chain B
	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	EmitterB := s2.DeployEmitterContract(ctx, chainB, "Alice")

	// Set up chains with lots of initiating messages
	numEmits := 100
	// emit logs on both chains in parallel
	var emitParallel sync.WaitGroup
	emitOn := func(chainID string, address string) {
		for i := 0; i < numEmits; i++ {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			payload := fmt.Sprintf("%s-%d", address, i)
			s2.EmitData(ctx, chainID, "sequencer", address, payload)
			cancel()
		}
		emitParallel.Done()
	}
	// 6 users on chain A, 6 users on chain B
	// all will submit 100 messages
	emitParallel.Add(6 * 2)
	go emitOn(chainA, "Alice")
	go emitOn(chainB, "Alice")
	go emitOn(chainA, "Bob")
	go emitOn(chainB, "Bob")
	go emitOn(chainA, "Charlie")
	go emitOn(chainB, "Charlie")
	go emitOn(chainA, "Dennis")
	go emitOn(chainB, "Dennis")
	go emitOn(chainA, "Eve")
	go emitOn(chainB, "Eve")
	go emitOn(chainA, "Frank")
	go emitOn(chainB, "Frank")
	emitParallel.Wait()

	// Get every possible log from both chains, as potential queries
	// check that the logs are emitted on chain A
	qA := ethereum.FilterQuery{
		Addresses: []common.Address{EmitterA},
	}
	logsA, err := clientA.FilterLogs(context.Background(), qA)
	require.NoError(b, err)
	require.Len(b, logsA, numEmits*6)

	// check that the logs are emitted on chain B
	qB := ethereum.FilterQuery{
		Addresses: []common.Address{EmitterB},
	}
	logsB, err := clientB.FilterLogs(context.Background(), qB)
	require.NoError(b, err)
	require.Len(b, logsB, numEmits*6)

	// helper function to turn a log into an access-list object
	logToAccessListHashes := func(chainID string, log gethTypes.Log) []common.Hash {
		client := s2.L2GethClient(chainID, "sequencer")
		// construct the expected hash of the log's payload
		// (topics concatenated with data)
		msgPayload := make([]byte, 0)
		for _, topic := range log.Topics {
			msgPayload = append(msgPayload, topic.Bytes()...)
		}
		msgPayload = append(msgPayload, log.Data...)
		msgHash := crypto.Keccak256Hash(msgPayload)

		// get block for the log (for timestamp)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		block, err := client.BlockByHash(ctx, log.BlockHash)
		require.NoError(b, err)

		args := types.ChecksumArgs{
			BlockNumber: log.BlockNumber,
			Timestamp:   block.Time(),
			LogIndex:    uint32(log.Index),
			ChainID:     eth.ChainIDFromBig(s2.ChainID(chainID)),
			LogHash:     types.PayloadHashToLogHash(msgHash, log.Address),
		}
		accessList := types.EncodeAccessList([]types.Access{args.Access()})
		return accessList
	}
	// get all access lists from both chains
	allAccessLists := make([][]common.Hash, 0)
	for _, log := range logsA {
		allAccessLists = append(allAccessLists, logToAccessListHashes(chainA, log))
	}
	for _, log := range logsB {
		allAccessLists = append(allAccessLists, logToAccessListHashes(chainB, log))
	}

	super := s2.SupervisorClient()
	executingDescriptor := types.ExecutingDescriptor{Timestamp: uint64(time.Now().Unix() + 1000)}

	runParallel := func(b *testing.B, num int) {
		b.Run(fmt.Sprintf("CheckAccessListParallel%d", num), func(b *testing.B) {
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				var wg sync.WaitGroup
				wg.Add(num)

				for j := 0; j < num; j++ {
					go func() {
						defer wg.Done()
						randomAccessList := allAccessLists[rand.Intn(len(allAccessLists))]
						err := super.CheckAccessList(context.Background(), randomAccessList, types.CrossUnsafe, executingDescriptor)
						require.NoError(b, err)
					}()
				}

				wg.Wait()
			}
		})
	}

	runParallel(b, 1)
	runParallel(b, 10)
	runParallel(b, 100)
}
