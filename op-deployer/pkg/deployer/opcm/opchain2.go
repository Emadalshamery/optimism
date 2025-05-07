package opcm

import (
	_ "embed"
	"math/big"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script"
	"github.com/ethereum/go-ethereum/common"
)

type DeployOPChainInput2 struct {
	OpChainProxyAdminOwner common.Address
	SystemConfigOwner      common.Address
	Batcher                common.Address
	UnsafeBlockSigner      common.Address
	Proposer               common.Address
	Challenger             common.Address

	BasefeeScalar     uint32
	BlobBaseFeeScalar uint32
	L2ChainId         *big.Int
	Opcm              common.Address
	SaltMixer         string
	GasLimit          uint64

	DisputeGameType              uint32
	DisputeAbsolutePrestate      common.Hash
	DisputeMaxGameDepth          *big.Int
	DisputeSplitDepth            *big.Int
	DisputeClockExtension        uint64
	DisputeMaxClockDuration      uint64
	AllowCustomDisputeParameters bool

	OperatorFeeScalar   uint32
	OperatorFeeConstant uint64
}

func (input *DeployOPChainInput2) InputSet() bool {
	return true
}

func (input *DeployOPChainInput2) StartingAnchorRoot() []byte {
	return PermissionedGameStartingAnchorRoot
}

type DeployOPChainOutput2 struct {
	OpChainProxyAdmin                 common.Address
	AddressManager                    common.Address
	L1ERC721BridgeProxy               common.Address
	SystemConfigProxy                 common.Address
	OptimismMintableERC20FactoryProxy common.Address
	L1StandardBridgeProxy             common.Address
	L1CrossDomainMessengerProxy       common.Address
	// Fault proof contracts below.
	OptimismPortalProxy                common.Address
	EthLockboxProxy                    common.Address
	DisputeGameFactoryProxy            common.Address
	AnchorStateRegistryProxy           common.Address
	FaultDisputeGame                   common.Address
	PermissionedDisputeGame            common.Address
	DelayedWETHPermissionedGameProxy   common.Address
	DelayedWETHPermissionlessGameProxy common.Address
}

func (output *DeployOPChainOutput2) CheckOutput(input common.Address) error {
	return nil
}

type DeployOPChainScript2 script.DeployScriptWithOutput[DeployOPChainInput2, DeployOPChainOutput2]

// NewDeployOPChainScript loads and validates the DeployOPChain2 script contract
func NewDeployOPChainScript(host *script.Host) (DeployOPChainScript2, error) {
	return script.NewDeployScriptWithOutputFromFile[DeployOPChainInput2, DeployOPChainOutput2](host, "DeployOPChain2.s.sol", "DeployOPChain2")
}

type ReadImplementationAddressesInput2 struct {
	DeployOPChainOutput2
	Opcm    common.Address
	Release string
}

type ReadImplementationAddressesOutput2 struct {
	DelayedWETH                  common.Address
	OptimismPortal               common.Address
	ETHLockbox                   common.Address `evm:"ethLockbox"`
	SystemConfig                 common.Address
	L1CrossDomainMessenger       common.Address
	L1ERC721Bridge               common.Address
	L1StandardBridge             common.Address
	OptimismMintableERC20Factory common.Address
	DisputeGameFactory           common.Address
	MipsSingleton                common.Address
	PreimageOracleSingleton      common.Address
}

type ReadImplementationAddressesScript2 script.DeployScriptWithOutput[ReadImplementationAddressesInput2, ReadImplementationAddressesOutput2]

// NewReadImplementationAddressesScript loads and validates the ReadImplementationAddresses2 script contract
func NewReadImplementationAddressesScript(host *script.Host) (ReadImplementationAddressesScript2, error) {
	return script.NewDeployScriptWithOutputFromFile[ReadImplementationAddressesInput2, ReadImplementationAddressesOutput2](host, "ReadImplementationAddresses2.s.sol", "ReadImplementationAddresses2")
}
