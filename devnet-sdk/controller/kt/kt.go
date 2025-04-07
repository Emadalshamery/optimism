package kt

import (
	"context"

	"github.com/ethereum-optimism/optimism/devnet-sdk/controller/surface"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/api/run"
)

type KurtosisControllerSurface struct {
	runner *run.KurtosisRunner
}

func NewKurtosisControllerSurface(enclave string) (*KurtosisControllerSurface, error) {
	runner, err := run.NewKurtosisRunner(run.WithKurtosisRunnerEnclave(enclave))
	if err != nil {
		return nil, err
	}
	return &KurtosisControllerSurface{
		runner: runner,
	}, nil
}

func (s *KurtosisControllerSurface) StartService(ctx context.Context, serviceName string) error {
	script := `
def run(plan):
	plan.start_service(name="` + serviceName + `")
`
	return s.runner.RunScript(ctx, script)
}

func (s *KurtosisControllerSurface) StopService(ctx context.Context, serviceName string) error {
	script := `
def run(plan):
	plan.stop_service(name="` + serviceName + `")
`
	return s.runner.RunScript(ctx, script)
}

var _ surface.ServiceLifecycleSurface = (*KurtosisControllerSurface)(nil)
