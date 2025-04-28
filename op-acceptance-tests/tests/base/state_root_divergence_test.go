package base

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/devnet-sdk/system"
	"github.com/ethereum-optimism/optimism/devnet-sdk/testing/systest"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
)

// TestConsensusStateRoot checks that all nodes in the network have the same state root,
// regardless of which Ethereum client implementation they're running.
func TestConsensusStateRoot(t *testing.T) {
	systest.SystemTest(t, divergenceTestScenario())
}

func divergenceTestScenario() systest.SystemTestFunc {
	const (
		// testTimeout is the maximum time allowed for the test to run
		testTimeout = 60 * time.Second
	)

	return func(t systest.T, sys system.System) {
		logger := testlog.Logger(t, log.LevelInfo)
		logger.Info("Started consensus state root test")

		// Check all L2 chains
		for i, chain := range sys.L2s() {
			chainIndex := i
			currentChain := chain
			t.Run(fmt.Sprintf("Chain_%d", chainIndex), func(t systest.T) {
				t.Parallel()
				checkChainForDivergence(t, currentChain, logger.New("chain", chainIndex), testTimeout)
			})
		}
	}
}

// checkChainForDivergence checks for state root divergence across all nodes in a chain.
// We do this by checking that the state root of the latest block is the same for all nodes.
// In summary:
// - get the latest block for each node
// - get the state root for each node
// - check that the state root is the same for all nodes
// - if not, fail the test
// - if yes, log that there is no divergence
func checkChainForDivergence(
	t systest.T,
	chain system.L2Chain,
	logger log.Logger,
	testTimeout time.Duration,
) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(t.Context(), testTimeout)
	defer cancel()

	nodes := chain.Nodes()
	if len(nodes) < 2 {
		logger.Info("Not enough nodes to check for divergence", "node_count", len(nodes))
		return
	}

	type nodeInfo struct {
		node        system.Node
		client      *ethclient.Client
		latestBlock uint64
	}

	nodeInfos := []nodeInfo{}
	minLatestBlock := uint64(0)

	// Get clients and latest blocks for all nodes
	for _, node := range nodes {
		client, err := node.GethClient() // should be client-agnostic, despite the name
		if err != nil {
			logger.Warn("Failed to get client", "node", node.Name(), "error", err)
			continue
		}

		latestBlock, err := client.BlockNumber(ctx)
		if err != nil {
			logger.Warn("Failed to get latest block", "node", node.Name(), "error", err)
			continue
		}

		info := nodeInfo{
			node:        node,
			client:      client,
			latestBlock: latestBlock,
		}
		nodeInfos = append(nodeInfos, info)

		// Find the minimum latest block
		if minLatestBlock == 0 || latestBlock < minLatestBlock {
			minLatestBlock = latestBlock
		}
	}

	// Check that there are at least two nodes to compare
	if len(nodeInfos) < 2 {
		logger.Info("Not enough accessible nodes to check for divergence", "node_count", len(nodeInfos))
		return
	}

	logger.Info("Checking state root divergence", "num_nodes", len(nodeInfos), "min_latest_block", minLatestBlock)

	// Get the block header and state root for each node
	stateRoots := make(map[string][]string)

	for _, info := range nodeInfos {
		header, err := info.client.HeaderByNumber(ctx, big.NewInt(int64(minLatestBlock)))
		if err != nil {
			logger.Warn("Failed to get header", "node", info.node.Name(), "block", minLatestBlock, "error", err)
			continue
		}

		stateRoot := header.Root.Hex()
		nodeName := info.node.Name()

		logger.Debug("Got state root",
			"node", nodeName,
			"block", minLatestBlock,
			"state_root", stateRoot)

		stateRoots[stateRoot] = append(stateRoots[stateRoot], nodeName)
	}

	// If there's more than one state root, we have divergence
	if len(stateRoots) > 1 {
		errMsg := fmt.Sprintf("State root divergence detected at block %d:\n", minLatestBlock)
		for root, nodes := range stateRoots {
			errMsg += fmt.Sprintf("  State root %s: %v\n", root, nodes)
		}
		t.Fatalf(errMsg)
	}

	logger.Info("No state root divergence detected", "block", minLatestBlock)
}
