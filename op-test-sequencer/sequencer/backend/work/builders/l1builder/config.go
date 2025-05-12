package l1builder

import (
	"context"

	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/fakebeacon"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/geth"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
	"github.com/ethereum/go-ethereum/eth/catalyst"
)

type Config struct {
	Geth   *geth.GethInstance
	Beacon *fakebeacon.FakeBeacon
}

func NewL1Builder(ctx context.Context, id seqtypes.BuilderID, opts *work.ServiceOpts, config *Config) (work.Builder, error) {
	withdrawalsIndex := uint64(1001)
	return &Builder{
		id:               id,
		log:              opts.Log,
		registry:         opts.Jobs,
		engine:           catalyst.NewConsensusAPI(config.Geth.Backend),
		geth:             config.Geth,
		beacon:           config.Beacon,
		withdrawalsIndex: &withdrawalsIndex,
	}, nil
}
