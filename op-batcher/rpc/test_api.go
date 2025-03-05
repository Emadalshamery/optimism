package rpc

import (
	"context"
	"errors"

	"github.com/ethereum/go-ethereum/log"
	gethrpc "github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum-optimism/optimism/op-service/metrics"
)

var (
	ErrNotTestMode = errors.New("batcher is not in test mode")
)

// BatcherTestDriver defines the interface for the batcher test API
type BatcherTestDriver interface {
	// SetL2Scope limits the view over the L2 chain
	SetL2Scope(ctx context.Context, start, end uint64) error
	// SubmitNow forces the batcher to submit the current data, even if the buffer is not full
	SubmitNow(ctx context.Context) error
	// Cursors returns the current cursor positions (submitted L2 block)
	Cursors(ctx context.Context) (map[string]uint64, error)
}

// testAPI provides testing methods for the batcher
type testAPI struct {
	log log.Logger
	m   metrics.RPCMetricer
	b   BatcherTestDriver
}

// NewTestAPI creates a new test API
func NewTestAPI(b BatcherTestDriver, m metrics.RPCMetricer, log log.Logger) *testAPI {
	return &testAPI{
		log: log,
		m:   m,
		b:   b,
	}
}

// GetTestAPI returns the RPC API descriptor for the batcher test API
func GetTestAPI(api *testAPI) gethrpc.API {
	return gethrpc.API{
		Namespace: "batcher",
		Service:   api,
	}
}

// SetL2Scope limits the view over the L2 chain
func (a *testAPI) SetL2Scope(ctx context.Context, start, end uint64) error {
	a.m.RecordRPCServerRequest("batcher_setL2Scope")
	err := a.b.SetL2Scope(ctx, start, end)
	if err != nil {
		a.log.Error("Failed to set L2 scope", "err", err, "start", start, "end", end)
	} else {
		a.log.Info("Set L2 scope", "start", start, "end", end)
	}
	return err
}

// SubmitNow forces the batcher to submit the current data, even if the buffer is not full
func (a *testAPI) SubmitNow(ctx context.Context) error {
	a.m.RecordRPCServerRequest("batcher_submitNow")
	err := a.b.SubmitNow(ctx)
	if err != nil {
		a.log.Error("Failed to submit batch now", "err", err)
	} else {
		a.log.Info("Forced batch submission")
	}
	return err
}

// Cursors returns the current cursor positions (submitted L2 block)
func (a *testAPI) Cursors(ctx context.Context) (map[string]uint64, error) {
	a.m.RecordRPCServerRequest("batcher_cursors")
	cursors, err := a.b.Cursors(ctx)
	if err != nil {
		a.log.Error("Failed to get cursors", "err", err)
	}
	return cursors, err
}
