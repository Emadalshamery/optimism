package boot

import preimage "github.com/ethereum-optimism/optimism/op-preimage"

const (
	L1HeadLocalIndex preimage.LocalIndexKey = iota + 1
	L2OutputRootLocalIndex
	L2ClaimLocalIndex
	L2ClaimBlockNumberLocalIndex
	L2ChainIDLocalIndex

	// These local keys are only used for custom chains
	L2ChainConfigLocalIndex
	RollupConfigLocalIndex
	DependencySetLocalIndex

	// Bootstrap Canonical Oracle keys
	CanonOracleQueryNumberLocalIndex
	CanonOracleQueryHashLocalIndex
	CanonOracleHeadLocalIndex
	CanonOracleChainIDLocalIndex
	CanonOracleChainConfigLocalIndex
	CanonOracleQueryPatternLocalIndex
)

type CanonOracleQueryPattern uint64

const (
	CanonOracleQueryPatternPoint CanonOracleQueryPattern = iota
	CanonOracleQueryPatternForward
	CanonOracleQueryPatternBackward
	CanonOracleQueryPatternRandom
)

func (p CanonOracleQueryPattern) String() string {
	switch p {
	case CanonOracleQueryPatternPoint:
		return "point"
	case CanonOracleQueryPatternForward:
		return "forward"
	case CanonOracleQueryPatternBackward:
		return "backward"
	case CanonOracleQueryPatternRandom:
		return "random"
	}
	return "unknown"
}

type oracleClient interface {
	Get(key preimage.Key) []byte
}
