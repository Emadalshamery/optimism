package activation

import (
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
)

// Check handles activation checks against a given dependency set.
type Check struct {
	depSet depset.DependencySet
	logger log.Logger
}

// NewCheck creates a new Check object with the provided dependency set.
func NewCheck(depSet depset.DependencySet, logger log.Logger) *Check {
	return &Check{
		depSet: depSet,
		logger: logger,
	}
}

// Check checks if interop is active for a given chain and timestamp.
func (c *Check) Check(chain eth.ChainID, timestamp uint64) bool {
	// If we have a nil pointer or no dependency set then interop is never active
	if c == nil || c.depSet == nil {
		return false
	}

	// Interop is active if the chain can initiate at the given timestamp
	canInitiate, err := c.depSet.CanInitiateAt(chain, timestamp)
	if err != nil {
		c.logger.Debug("Error checking interop activation", "chain", chain, "timestamp", timestamp, "err", err)
		return false
	}
	return canInitiate
}
