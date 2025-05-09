package opcm

import (
	"math/big"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script"
	"github.com/ethereum/go-ethereum/common"
)

type DeployPreimageOracleInput struct {
	MinProposalSize *big.Int
	ChallengePeriod *big.Int
}

type DeployPreimageOracleOutput struct {
	PreimageOracle common.Address
}

type DeployPreimageOracleScript script.DeployScriptWithOutput[DeployPreimageOracleInput, DeployPreimageOracleOutput]

// NewDeployPreimageOracleScript loads and validates the DeployPreimageOracle script contract
func NewDeployPreimageOracleScript(host *script.Host) (DeployPreimageOracleScript, error) {
	return script.NewDeployScriptWithOutputFromFile[DeployPreimageOracleInput, DeployPreimageOracleOutput](host, "DeployPreimageOracle.s.sol", "DeployPreimageOracle")
}
