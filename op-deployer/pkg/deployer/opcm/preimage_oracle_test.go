package opcm

import (
	"github.com/ethereum/go-ethereum/common"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewDeployPreimageOracleScript(t *testing.T) {
	t.Run("should not fail with current version of DeployPreimageOracle contract", func(t *testing.T) {
		// First we grab a test host
		host1 := createTestHost(t)

		// Then we load the script
		//
		// This would raise an error if the Go types didn't match the ABI
		deployPreimageOracle, err := NewDeployPreimageOracleScript(host1)
		require.NoError(t, err)

		// Then we deploy
		output, err := deployPreimageOracle.Run(DeployPreimageOracleInput{
			MinProposalSize: big.NewInt(1),
			ChallengePeriod: big.NewInt(2),
		})

		// And do some simple asserts
		require.NoError(t, err)
		require.NotNil(t, output)
		require.NotEqual(t, output.PreimageOracle, common.Address{})
	})
}
