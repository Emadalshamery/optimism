package oraclebenchmark

import (
	"fmt"
	mathrand "math/rand"

	"github.com/ethereum-optimism/optimism/op-program/client/boot"
	"github.com/ethereum-optimism/optimism/op-program/client/l2"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

func RunOracleBenchmark(
	logger log.Logger,
	bootInfo *boot.BootCanonOracle,
	oracle *l2.CachingOracle,
	db l2.KeyValueStore,
) error {
	head := oracle.BlockByHash(bootInfo.Head, bootInfo.ChainID)
	blockByHash := func(hash common.Hash) *ethtypes.Block {
		return oracle.BlockByHash(hash, bootInfo.ChainID)
	}
	fallback := l2.NewCanonicalBlockHeaderOracle(head.Header(), blockByHash)
	canonOracle := l2.NewFastCanonicalBlockHeaderOracle(
		head.Header(),
		blockByHash,
		bootInfo.ChainConfig,
		oracle,
		rawdb.NewMemoryDatabase(),
		fallback,
	)

	logger.Info("Starting oracle benchmark",
		"head", bootInfo.Head,
		"headNumber", head.Number(),
		"queryNumber", bootInfo.QueryNumber,
		"queryHash", bootInfo.QueryHash,
		"queryPattern", bootInfo.QueryPattern,
	)

	switch bootInfo.QueryPattern {
	case boot.CanonOracleQueryPatternPoint:
		return SingleQuery(logger, canonOracle, bootInfo.QueryNumber, bootInfo.QueryHash)
	case boot.CanonOracleQueryPatternForward:
		return ForwardsQueryPattern(logger, canonOracle, head, bootInfo.QueryNumber, bootInfo.QueryHash)
	case boot.CanonOracleQueryPatternBackward:
		return BackwardsQueryPattern(logger, canonOracle, head, bootInfo.QueryNumber, bootInfo.QueryHash)
	case boot.CanonOracleQueryPatternRandom:
		return RandomQueryPattern(logger, canonOracle, head, bootInfo.QueryNumber, bootInfo.QueryHash)
	default:
		panic(fmt.Sprintf("invalid query pattern: %v", bootInfo.QueryPattern))
	}
}

func SingleQuery(log log.Logger, oracle *l2.FastCanonicalBlockHeaderOracle, queryNumber uint64, queryHash common.Hash) error {
	fetchedHead := oracle.GetHeaderByNumber(queryNumber)
	if fetchedHead.Hash() != queryHash {
		return fmt.Errorf("head hash mismatch: %s != %s", fetchedHead.Hash(), queryHash)
	}
	return nil
}

// relates to the maximum number of executing messages in a single block
const maxQueries = 2000

func ForwardsQueryPattern(log log.Logger, oracle *l2.FastCanonicalBlockHeaderOracle, head *ethtypes.Block, queryNumber uint64, queryHash common.Hash) error {
	start := queryNumber
	end := head.Number().Uint64()
	if end-start > maxQueries {
		end = start + maxQueries
		log.Info("Forwards query pattern minimized", "start", start, "end", end)
	}
	for i := start; i <= end; i++ {
		log.Info("Query fetching head", "number", i)
		fetchedHead := oracle.GetHeaderByNumber(i)
		if queryNumber == i {
			if fetchedHead.Hash() != queryHash {
				return fmt.Errorf("head hash mismatch: %s != %s", fetchedHead.Hash(), queryHash)
			}
		}
	}
	return nil
}

func BackwardsQueryPattern(log log.Logger, oracle *l2.FastCanonicalBlockHeaderOracle, head *ethtypes.Block, queryNumber uint64, queryHash common.Hash) error {
	start := queryNumber
	end := head.Number().Uint64()
	if end-start > maxQueries {
		end = start + maxQueries
		log.Info("Backwards query pattern minimized", "start", start, "end", end)
	}
	for i := end; i >= start; i-- {
		log.Info("Query fetching head", "number", i)
		fetchedHead := oracle.GetHeaderByNumber(i)
		if queryNumber == i {
			if fetchedHead.Hash() != queryHash {
				return fmt.Errorf("head hash mismatch: %s != %s", fetchedHead.Hash(), queryHash)
			}
		}
	}
	return nil
}

func RandomQueryPattern(log log.Logger, oracle *l2.FastCanonicalBlockHeaderOracle, head *ethtypes.Block, queryNumber uint64, queryHash common.Hash) error {
	var accesses []uint64
	start := queryNumber
	end := head.Number().Uint64()
	for i := start; i < end; i++ {
		accesses = append(accesses, i)
	}

	src := mathrand.NewSource(int64(queryNumber))
	r := mathrand.New(src)
	r.Shuffle(len(accesses), func(i, j int) {
		accesses[i], accesses[j] = accesses[j], accesses[i]
	})

	for i, access := range accesses {
		log.Info("Query fetching head", "number", access)
		fetchedHead := oracle.GetHeaderByNumber(access)
		_ = fetchedHead
		if i >= maxQueries {
			log.Info("Random query pattern minimized", "num_accesses", len(accesses))
			break
		}
	}
	return nil
}
