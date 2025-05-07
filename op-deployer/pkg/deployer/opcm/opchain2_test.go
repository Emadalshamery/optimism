package opcm

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script"
	"github.com/ethereum-optimism/optimism/op-chain-ops/script/addresses"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/standard"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestNewDeployOPChainScript(t *testing.T) {
	deployDependencies := func(host *script.Host) common.Address {
		proxyAdminArtifact, err := host.Artifacts().ReadArtifact("ProxyAdmin.sol", "ProxyAdmin")
		require.NoError(t, err)

		encodedProxyAdmin, err := proxyAdminArtifact.ABI.Pack("", addresses.ScriptDeployer)
		require.NoError(t, err)

		proxyAdminAddress, err := host.Create(addresses.ScriptDeployer, append(proxyAdminArtifact.Bytecode.Object, encodedProxyAdmin...))
		require.NoError(t, err)

		// Then we get a proxy deployed
		proxyArtifact, err := host.Artifacts().ReadArtifact("Proxy.sol", "Proxy")
		require.NoError(t, err)

		encodedProxy, err := proxyArtifact.ABI.Pack("", proxyAdminAddress)
		require.NoError(t, err)

		proxyAddress, err := host.Create(addresses.ScriptDeployer, append(proxyArtifact.Bytecode.Object, encodedProxy...))
		require.NoError(t, err)

		// Then we get ProtocolVersions deployed
		protocolVersionsArtifact, err := host.Artifacts().ReadArtifact("ProtocolVersions.sol", "ProtocolVersions")
		require.NoError(t, err)

		encodedProtocolVersions, err := protocolVersionsArtifact.ABI.Pack("")
		require.NoError(t, err)

		protocolVersionsAddress, err := host.Create(addresses.ScriptDeployer, append(protocolVersionsArtifact.Bytecode.Object, encodedProtocolVersions...))
		require.NoError(t, err)

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
			SuperchainConfigProxy:           proxyAddress,
			ProtocolVersionsProxy:           protocolVersionsAddress,
			SuperchainProxyAdmin:            proxyAdminAddress,
			UpgradeController:               common.BigToAddress(big.NewInt(13)),
		})

		fmt.Printf("implementationsOutput: %+v\n", implementationsOutput)
		require.NoError(t, err)
		return implementationsOutput.Opcm
	}
	t.Run("should not fail with current version of DeployOPChain2 contract", func(t *testing.T) {
		// First we grab a test host
		host1 := createTestHost(t)

		// We'll need some contracts already deployed for this to work
		opcmImpl := deployDependencies(host1)

		// Then we load the script
		//
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
			BlobBaseFeeScalar: 100,
			L2ChainId:         big.NewInt(1105),
			Opcm:              opcmImpl,
			SaltMixer:         "0x456",
			GasLimit:          30000000,

			DisputeGameType:              1,
			DisputeAbsolutePrestate:      common.HexToHash("0x123"),
			DisputeMaxGameDepth:          big.NewInt(100),
			DisputeSplitDepth:            big.NewInt(100),
			DisputeClockExtension:        100,
			DisputeMaxClockDuration:      100,
			AllowCustomDisputeParameters: true,

			OperatorFeeScalar:   100,
			OperatorFeeConstant: 100,
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
			BlobBaseFeeScalar: 100,
			L2ChainId:         big.NewInt(1105),
			Opcm:              opcmImpl2,
			SaltMixer:         "0x456",
			GasLimit:          30000000,

			DisputeGameType:              1,
			DisputeAbsolutePrestate:      common.HexToHash("0x123"),
			DisputeMaxGameDepth:          100,
			DisputeSplitDepth:            100,
			DisputeClockExtension:        100,
			DisputeMaxClockDuration:      100,
			AllowCustomDisputeParameters: true,

			OperatorFeeScalar:   100,
			OperatorFeeConstant: 100,
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
