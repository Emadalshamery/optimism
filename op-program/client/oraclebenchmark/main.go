package oraclebenchmark

import (
	"fmt"

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

	logger.Info("Starting oracle benchmark", "head", bootInfo.Head, "headNumber", head.Number(), "queryNumber", bootInfo.QueryNumber, "queryHash", bootInfo.QueryHash)

	fetchedHead := canonOracle.GetHeaderByNumber(bootInfo.QueryNumber)
	if fetchedHead.Hash() != bootInfo.QueryHash {
		return fmt.Errorf("head hash mismatch: %s != %s", fetchedHead.Hash(), bootInfo.QueryHash)
	}
	return nil
}
