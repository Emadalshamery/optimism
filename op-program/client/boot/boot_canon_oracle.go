package boot

import (
	"encoding/binary"
	"encoding/json"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
)

type BootCanonOracle struct {
	QueryNumber  uint64
	QueryHash    common.Hash
	Head         common.Hash
	ChainID      eth.ChainID
	ChainConfig  *params.ChainConfig
	QueryPattern CanonOracleQueryPattern
}

func BootstrapCanonOracle(r oracleClient) *BootCanonOracle {
	queryNumber := binary.BigEndian.Uint64(r.Get(CanonOracleQueryNumberLocalIndex))
	queryHash := common.BytesToHash(r.Get(CanonOracleQueryHashLocalIndex))
	head := common.BytesToHash(r.Get(CanonOracleHeadLocalIndex))
	chainID := eth.ChainIDFromUInt64(binary.BigEndian.Uint64(r.Get(CanonOracleChainIDLocalIndex)))

	chainConfig := new(params.ChainConfig)
	err := json.Unmarshal(r.Get(CanonOracleChainConfigLocalIndex), &chainConfig)
	if err != nil {
		panic("failed to bootstrap chain config")
	}
	queryPattern := binary.BigEndian.Uint64(r.Get(CanonOracleQueryPatternLocalIndex))

	return &BootCanonOracle{
		QueryNumber:  queryNumber,
		QueryHash:    queryHash,
		Head:         head,
		ChainID:      chainID,
		ChainConfig:  chainConfig,
		QueryPattern: CanonOracleQueryPattern(queryPattern),
	}
}
