package opcm

import (
	"fmt"
)

type DeployScript interface {
}

func NewDeploySuperchain() {
	return
}

func LoadDeployScript(h *Host, name string, contract string) {
	artifact, err := h.af.ReadArtifact(name, contract)
	if err != nil {
		return nil, nil, fmt.Errorf("could not load script artifact: %w", err)
	}
}
