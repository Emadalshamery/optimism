package l1builder

import (
	"context"

	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/fakebeacon"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/geth"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
	"github.com/ethereum/go-ethereum/eth/catalyst"
	"github.com/ethereum/go-ethereum/log"
)

type Builder struct {
	id  seqtypes.BuilderID
	log log.Logger

	engine *catalyst.ConsensusAPI
	geth   *geth.GethInstance
	beacon *fakebeacon.FakeBeacon

	withdrawalsIndex *uint64
	registry         work.Jobs
}

var _ work.Builder = (*Builder)(nil)

func (b *Builder) Close() error {
	return nil
}

func (b *Builder) ID() seqtypes.BuilderID {
	return b.id
}

func (b *Builder) Register(jobs work.Jobs) {
	b.registry = jobs
}

func (b *Builder) NewJob(ctx context.Context, opts seqtypes.BuildOpts) (work.BuildJob, error) {
	b.log.Debug("L1Builder NewJob request", "opts", opts)

	// parentRef, err := b.ethclient.BlockRefByHash(ctx, opts.Parent)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to retrieve parent-block: %w", err)
	// }
	// b.log.Debug("L1Builder NewJob fetched parentRef", "ref", parentRef)

	id := seqtypes.RandomJobID()
	job := &Job{
		logger:           b.log,
		id:               id,
		engine:           b.engine,
		geth:             b.geth,
		withdrawalsIndex: b.withdrawalsIndex,
		beacon:           b.beacon,
		parent:           opts.Parent,
	}
	if err := b.registry.RegisterJob(job); err != nil {
		return nil, err
	}
	b.log.Info("L1Builder NewJob has registered job", "job_id", id)
	return job, nil
}

func (b *Builder) String() string {
	return "l1-builder-" + b.id.String()
}
