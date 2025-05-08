package opcm

import (
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/standard"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/require"
)

func TestNewDeployOPChainScript(t *testing.T) {
	deployDependencies := func(host *script.Host) common.Address {
		deploySuperchain, err := NewDeploySuperchainScript(host)
		require.NoError(t, err)

		superchainOutput, err := deploySuperchain.Run(DeploySuperchain2Input{
			Guardian:                   common.BigToAddress(big.NewInt(1)),
			ProtocolVersionsOwner:      common.BigToAddress(big.NewInt(2)),
			SuperchainProxyAdminOwner:  common.BigToAddress(big.NewInt(3)),
			Paused:                     true,
			RecommendedProtocolVersion: params.ProtocolVersion{1},
			RequiredProtocolVersion:    params.ProtocolVersion{2},
		})
		require.NoError(t, err)
		require.NotNil(t, superchainOutput)

		deployImplementations, err := NewDeployImplementationsScript(host)
		require.NoError(t, err)

		mipsVersion := int64(standard.MIPSVersion)
		implementationsOutput, err := deployImplementations.Run(DeployImplementations2Input{
			WithdrawalDelaySeconds:          big.NewInt(1),
			MinProposalSizeBytes:            big.NewInt(2),
			ChallengePeriodSeconds:          big.NewInt(3),
			ProofMaturityDelaySeconds:       big.NewInt(4),
			DisputeGameFinalityDelaySeconds: big.NewInt(5),
			MipsVersion:                     big.NewInt(mipsVersion),
			L1ContractsRelease:              "dev-release",
			SuperchainConfigProxy:           superchainOutput.SuperchainConfigProxy,
			ProtocolVersionsProxy:           superchainOutput.ProtocolVersionsProxy,
			SuperchainProxyAdmin:            superchainOutput.SuperchainProxyAdmin,
			UpgradeController:               common.BigToAddress(big.NewInt(13)),
		})
		require.NoError(t, err)
		require.NotNil(t, implementationsOutput)

		return implementationsOutput.Opcm
	}
	t.Run("should not fail with current version of DeployOPChain2 contract", func(t *testing.T) {
		// First we grab a test host
		host1 := createTestHost(t)

		// We need Superchain and Implementations contracts deployed for this to work
		opcmImpl := deployDependencies(host1)

		// This would raise an error if the Go types didn't match the ABI
		deployOPChain, err := NewDeployOPChainScript(host1)
		require.NoError(t, err)

		// Then we deploy
		output, err := deployOPChain.Run(DeployOPChainInput2{
			OpChainProxyAdminOwner: common.HexToAddress("0x123"),
			SystemConfigOwner:      common.HexToAddress("0x456"),
			Batcher:                common.HexToAddress("0x789"),
			UnsafeBlockSigner:      common.HexToAddress("0xabc"),
			Proposer:               common.HexToAddress("0xdef"),
			Challenger:             common.HexToAddress("0xfed"),

			BasefeeScalar:     100,
			BlobBaseFeeScalar: 200,
			L2ChainId:         big.NewInt(300),
			Opcm:              opcmImpl,
			SaltMixer:         "defaultSaltMixer",
			GasLimit:          60_000_000,

			DisputeGameType:              1,
			DisputeAbsolutePrestate:      common.HexToHash("0x038512e02c4c3f7bdaec27d00edf55b7155e0905301e1a88083e4e0a6764d54c"),
			DisputeMaxGameDepth:          big.NewInt(73),
			DisputeSplitDepth:            big.NewInt(30),
			DisputeClockExtension:        uint64(3 * 60 * 60),
			DisputeMaxClockDuration:      uint64(3.5 * 24 * 60 * 60),
			AllowCustomDisputeParameters: false,

			OperatorFeeScalar:   0,
			OperatorFeeConstant: 0,
		})

		// And do some simple asserts
		require.NoError(t, err)
		require.NotNil(t, output)

		// Now we run the old deployer
		//
		// We run it on a fresh host so that the deployer nonces are the same
		// which in turn means we should get identical output
		host2 := createTestHost(t)
		// We'll need some contracts already deployed for this to work
		opcmImpl2 := deployDependencies(host2)

		deprecatedOutput, err := DeployOPChain(host2, DeployOPChainInput{
			OpChainProxyAdminOwner: common.HexToAddress("0x123"),
			SystemConfigOwner:      common.HexToAddress("0x456"),
			Batcher:                common.HexToAddress("0x789"),
			UnsafeBlockSigner:      common.HexToAddress("0xabc"),
			Proposer:               common.HexToAddress("0xdef"),
			Challenger:             common.HexToAddress("0xfed"),

			BasefeeScalar:     100,
			BlobBaseFeeScalar: 200,
			L2ChainId:         big.NewInt(300),
			Opcm:              opcmImpl2,
			SaltMixer:         "defaultSaltMixer",
			GasLimit:          60_000_000,

			DisputeGameType:              1,
			DisputeAbsolutePrestate:      common.HexToHash("0x038512e02c4c3f7bdaec27d00edf55b7155e0905301e1a88083e4e0a6764d54c"),
			DisputeMaxGameDepth:          uint64(73),
			DisputeSplitDepth:            uint64(30),
			DisputeClockExtension:        uint64(3 * 60 * 60),
			DisputeMaxClockDuration:      uint64(3.5 * 24 * 60 * 60),
			AllowCustomDisputeParameters: false,

			OperatorFeeScalar:   0,
			OperatorFeeConstant: 0,
		})

		// Make sure it succeeded
		require.NoError(t, err)
		require.NotNil(t, deprecatedOutput)

		// Now make sure the addresses are the same
		require.Equal(t, deprecatedOutput.OpChainProxyAdmin, output.OpChainProxyAdmin)
		require.Equal(t, deprecatedOutput.AddressManager, output.AddressManager)
		require.Equal(t, deprecatedOutput.L1ERC721BridgeProxy, output.L1ERC721BridgeProxy)
		require.Equal(t, deprecatedOutput.SystemConfigProxy, output.SystemConfigProxy)
		require.Equal(t, deprecatedOutput.OptimismMintableERC20FactoryProxy, output.OptimismMintableERC20FactoryProxy)
		require.Equal(t, deprecatedOutput.L1StandardBridgeProxy, output.L1StandardBridgeProxy)
		require.Equal(t, deprecatedOutput.L1CrossDomainMessengerProxy, output.L1CrossDomainMessengerProxy)
		require.Equal(t, deprecatedOutput.OptimismPortalProxy, output.OptimismPortalProxy)
		require.Equal(t, deprecatedOutput.ETHLockboxProxy, output.EthLockboxProxy)
		require.Equal(t, deprecatedOutput.DisputeGameFactoryProxy, output.DisputeGameFactoryProxy)
		require.Equal(t, deprecatedOutput.AnchorStateRegistryProxy, output.AnchorStateRegistryProxy)
		require.Equal(t, deprecatedOutput.FaultDisputeGame, output.FaultDisputeGame)
		require.Equal(t, deprecatedOutput.PermissionedDisputeGame, output.PermissionedDisputeGame)
		require.Equal(t, deprecatedOutput.DelayedWETHPermissionedGameProxy, output.DelayedWETHPermissionedGameProxy)
		require.Equal(t, deprecatedOutput.DelayedWETHPermissionlessGameProxy, output.DelayedWETHPermissionlessGameProxy)

		// And just to be super sure we also compare the code deployed to the addresses
		require.Equal(t, host2.GetCode(deprecatedOutput.OpChainProxyAdmin), host1.GetCode(output.OpChainProxyAdmin))
		require.Equal(t, host2.GetCode(deprecatedOutput.AddressManager), host1.GetCode(output.AddressManager))
		require.Equal(t, host2.GetCode(deprecatedOutput.L1ERC721BridgeProxy), host1.GetCode(output.L1ERC721BridgeProxy))
		require.Equal(t, host2.GetCode(deprecatedOutput.SystemConfigProxy), host1.GetCode(output.SystemConfigProxy))
		require.Equal(t, host2.GetCode(deprecatedOutput.OptimismMintableERC20FactoryProxy), host1.GetCode(output.OptimismMintableERC20FactoryProxy))
		require.Equal(t, host2.GetCode(deprecatedOutput.L1StandardBridgeProxy), host1.GetCode(output.L1StandardBridgeProxy))
		require.Equal(t, host2.GetCode(deprecatedOutput.L1CrossDomainMessengerProxy), host1.GetCode(output.L1CrossDomainMessengerProxy))
		require.Equal(t, host2.GetCode(deprecatedOutput.OptimismPortalProxy), host1.GetCode(output.OptimismPortalProxy))
		require.Equal(t, host2.GetCode(deprecatedOutput.ETHLockboxProxy), host1.GetCode(output.EthLockboxProxy))
		require.Equal(t, host2.GetCode(deprecatedOutput.DisputeGameFactoryProxy), host1.GetCode(output.DisputeGameFactoryProxy))
		require.Equal(t, host2.GetCode(deprecatedOutput.AnchorStateRegistryProxy), host1.GetCode(output.AnchorStateRegistryProxy))
		require.Equal(t, host2.GetCode(deprecatedOutput.FaultDisputeGame), host1.GetCode(output.FaultDisputeGame))
		require.Equal(t, host2.GetCode(deprecatedOutput.PermissionedDisputeGame), host1.GetCode(output.PermissionedDisputeGame))
		require.Equal(t, host2.GetCode(deprecatedOutput.DelayedWETHPermissionedGameProxy), host1.GetCode(output.DelayedWETHPermissionedGameProxy))
		require.Equal(t, host2.GetCode(deprecatedOutput.DelayedWETHPermissionlessGameProxy), host1.GetCode(output.DelayedWETHPermissionlessGameProxy))
	})
}
