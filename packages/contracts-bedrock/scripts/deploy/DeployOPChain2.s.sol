// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Script } from "forge-std/Script.sol";
import { console2 as console } from "forge-std/console2.sol";

import { SafeCast } from "@openzeppelin/contracts/utils/math/SafeCast.sol";

import { DeployUtils } from "scripts/libraries/DeployUtils.sol";
import { Solarray } from "scripts/libraries/Solarray.sol";
import { BaseDeployIO } from "scripts/deploy/BaseDeployIO.sol";
import { LibString } from "@solady/utils/LibString.sol";

import { IResourceMetering } from "interfaces/L1/IResourceMetering.sol";
import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";
import { IBigStepper } from "interfaces/dispute/IBigStepper.sol";
import { Predeploys } from "src/libraries/Predeploys.sol";
import { Constants } from "src/libraries/Constants.sol";
import { Constants as ScriptConstants } from "scripts/libraries/Constants.sol";

import { IProxyAdmin } from "interfaces/universal/IProxyAdmin.sol";
import { IProxy } from "interfaces/universal/IProxy.sol";
import { IOPContractsManager } from "interfaces/L1/IOPContractsManager.sol";
import { IAddressManager } from "interfaces/legacy/IAddressManager.sol";
import { IDelayedWETH } from "interfaces/dispute/IDelayedWETH.sol";
import { IDisputeGameFactory } from "interfaces/dispute/IDisputeGameFactory.sol";
import { IAnchorStateRegistry } from "interfaces/dispute/IAnchorStateRegistry.sol";
import { IFaultDisputeGame } from "interfaces/dispute/IFaultDisputeGame.sol";
import { IPermissionedDisputeGame } from "interfaces/dispute/IPermissionedDisputeGame.sol";
import { Claim, Duration, GameType, GameTypes, Hash } from "src/dispute/lib/Types.sol";

import { IOptimismPortal2 as IOptimismPortal } from "interfaces/L1/IOptimismPortal2.sol";
import { ISystemConfig } from "interfaces/L1/ISystemConfig.sol";
import { IL1CrossDomainMessenger } from "interfaces/L1/IL1CrossDomainMessenger.sol";
import { IL1ERC721Bridge } from "interfaces/L1/IL1ERC721Bridge.sol";
import { IL1StandardBridge } from "interfaces/L1/IL1StandardBridge.sol";
import { IOptimismMintableERC20Factory } from "interfaces/universal/IOptimismMintableERC20Factory.sol";
import { IETHLockbox } from "interfaces/L1/IETHLockbox.sol";

contract DeployOPChain2 is Script {
    struct Input {
        address opChainProxyAdminOwner;
        address systemConfigOwner;
        address batcher;
        address unsafeBlockSigner;
        address proposer;
        address challenger;
        // TODO Add fault proofs inputs in a future PR.
        uint32 basefeeScalar;
        uint32 blobBaseFeeScalar;
        uint256 l2ChainId;
        IOPContractsManager opcm;
        string saltMixer;
        uint64 gasLimit;
        // Configurable dispute game inputs
        GameType disputeGameType;
        Claim disputeAbsolutePrestate;
        uint256 disputeMaxGameDepth;
        uint256 disputeSplitDepth;
        Duration disputeClockExtension;
        Duration disputeMaxClockDuration;
        bool allowCustomDisputeParameters;
        uint32 operatorFeeScalar;
        uint64 operatorFeeConstant;
    }

    struct Output {
        IProxyAdmin opChainProxyAdmin;
        IAddressManager addressManager;
        IL1ERC721Bridge l1ERC721BridgeProxy;
        ISystemConfig systemConfigProxy;
        IOptimismMintableERC20Factory optimismMintableERC20FactoryProxy;
        IL1StandardBridge l1StandardBridgeProxy;
        IL1CrossDomainMessenger l1CrossDomainMessengerProxy;
        IOptimismPortal optimismPortalProxy;
        IETHLockbox ethLockboxProxy;
        IDisputeGameFactory disputeGameFactoryProxy;
        IAnchorStateRegistry anchorStateRegistryProxy;
        IFaultDisputeGame faultDisputeGame;
        IPermissionedDisputeGame permissionedDisputeGame;
        IDelayedWETH delayedWETHPermissionedGameProxy;
        IDelayedWETH delayedWETHPermissionlessGameProxy;
    }

    function run(Input memory _input) public returns (Output memory output_) {
        console.log("DeployOPChain2.run initiated");
        bytes memory startingAnchorRoot = abi.encode(ScriptConstants.DEFAULT_OUTPUT_ROOT());

        assertValidInput(_input);
        console.log("Confirmed valid input");

        IOPContractsManager opcm = _input.opcm;

        IOPContractsManager.Roles memory roles = IOPContractsManager.Roles({
            opChainProxyAdminOwner: _input.opChainProxyAdminOwner,
            systemConfigOwner: _input.systemConfigOwner,
            batcher: _input.batcher,
            unsafeBlockSigner: _input.unsafeBlockSigner,
            proposer: _input.proposer,
            challenger: _input.challenger
        });
        IOPContractsManager.DeployInput memory deployInput = IOPContractsManager.DeployInput({
            roles: roles,
            basefeeScalar: _input.basefeeScalar,
            blobBasefeeScalar: _input.blobBaseFeeScalar,
            l2ChainId: _input.l2ChainId,
            startingAnchorRoot: startingAnchorRoot,
            saltMixer: _input.saltMixer,
            gasLimit: _input.gasLimit,
            disputeGameType: _input.disputeGameType,
            disputeAbsolutePrestate: _input.disputeAbsolutePrestate,
            disputeMaxGameDepth: _input.disputeMaxGameDepth,
            disputeSplitDepth: _input.disputeSplitDepth,
            disputeClockExtension: _input.disputeClockExtension,
            disputeMaxClockDuration: _input.disputeMaxClockDuration
        });

        vm.broadcast(msg.sender);
        IOPContractsManager.DeployOutput memory deployOutput = opcm.deploy(deployInput);
        console.log("opcm.deploy complete");

        output_ = Output({
            opChainProxyAdmin: deployOutput.opChainProxyAdmin,
            addressManager: deployOutput.addressManager,
            l1ERC721BridgeProxy: deployOutput.l1ERC721BridgeProxy,
            systemConfigProxy: deployOutput.systemConfigProxy,
            optimismMintableERC20FactoryProxy: deployOutput.optimismMintableERC20FactoryProxy,
            l1StandardBridgeProxy: deployOutput.l1StandardBridgeProxy,
            l1CrossDomainMessengerProxy: deployOutput.l1CrossDomainMessengerProxy,
            optimismPortalProxy: deployOutput.optimismPortalProxy,
            ethLockboxProxy: deployOutput.ethLockboxProxy,
            disputeGameFactoryProxy: deployOutput.disputeGameFactoryProxy,
            anchorStateRegistryProxy: deployOutput.anchorStateRegistryProxy,
            faultDisputeGame: deployOutput.faultDisputeGame,
            permissionedDisputeGame: deployOutput.permissionedDisputeGame,
            delayedWETHPermissionedGameProxy: deployOutput.delayedWETHPermissionedGameProxy,
            delayedWETHPermissionlessGameProxy: deployOutput.delayedWETHPermissionlessGameProxy
        });

        vm.label(address(output_.opChainProxyAdmin), "opChainProxyAdmin");
        vm.label(address(output_.addressManager), "addressManager");
        vm.label(address(output_.l1ERC721BridgeProxy), "l1ERC721BridgeProxy");
        vm.label(address(output_.systemConfigProxy), "systemConfigProxy");
        vm.label(address(output_.optimismMintableERC20FactoryProxy), "optimismMintableERC20FactoryProxy");
        vm.label(address(output_.l1StandardBridgeProxy), "l1StandardBridgeProxy");
        vm.label(address(output_.l1CrossDomainMessengerProxy), "l1CrossDomainMessengerProxy");
        vm.label(address(output_.optimismPortalProxy), "optimismPortalProxy");
        vm.label(address(output_.ethLockboxProxy), "ethLockboxProxy");
        vm.label(address(output_.disputeGameFactoryProxy), "disputeGameFactoryProxy");
        vm.label(address(output_.anchorStateRegistryProxy), "anchorStateRegistryProxy");
        // vm.label(address(output_.faultDisputeGame), "faultDisputeGame");
        vm.label(address(output_.permissionedDisputeGame), "permissionedDisputeGame");
        vm.label(address(output_.delayedWETHPermissionedGameProxy), "delayedWETHPermissionedGameProxy");
        // TODO: Eventually switch from Permissioned to Permissionless.
        // vm.label(address(output_.delayedWETHPermissionlessGameProxy), "delayedWETHPermissionlessGameProxy");

        assertValidOutput(_input, output_);
    }

    function assertValidInput(Input memory _input) private pure {
        require(_input.opChainProxyAdminOwner != address(0), "DeployOPChain: opChainProxyAdminOwner not set");
        require(_input.systemConfigOwner != address(0), "DeployOPChain: systemConfigOwner not set");
        require(_input.batcher != address(0), "DeployOPChain: batcher not set");
        require(_input.unsafeBlockSigner != address(0), "DeployOPChain: unsafeBlockSigner not set");
        require(_input.proposer != address(0), "DeployOPChain: proposer not set");
        require(_input.challenger != address(0), "DeployOPChain: challenger not set");
        require(_input.basefeeScalar != 0, "DeployOPChain: basefeeScalar not set");
        require(_input.blobBaseFeeScalar != 0, "DeployOPChain: blobBaseFeeScalar not set");
        require(_input.l2ChainId != 0, "DeployOPChain: l2ChainId not set");
        require(address(_input.opcm) != address(0), "DeployOPChain: opcm not set");
        require(!LibString.eq(_input.saltMixer, ""), "DeployOPChain: saltMixer not set");
        require(_input.gasLimit != 0, "DeployOPChain: gasLimit not set");

        require(_input.disputeMaxGameDepth != 0, "DeployOPChain: disputeMaxGameDepth not set");
        require(_input.disputeSplitDepth != 0, "DeployOPChain: disputeSplitDepth not set");
        require(Duration.unwrap(_input.disputeClockExtension) != 0, "DeployOPChain: disputeClockExtension not set");
        require(Duration.unwrap(_input.disputeMaxClockDuration) != 0, "DeployOPChain: disputeMaxClockDuration not set");
    }

    function assertValidOutput(Input memory _input, Output memory _output) private {
        // With 16 addresses, we'd get a stack too deep error if we tried to do this inline as a
        // single call to `Solarray.addresses`. So we split it into two calls.
        address[] memory addrs1 = Solarray.addresses(
            address(_output.opChainProxyAdmin),
            address(_output.addressManager),
            address(_output.l1ERC721BridgeProxy),
            address(_output.systemConfigProxy),
            address(_output.optimismMintableERC20FactoryProxy),
            address(_output.l1StandardBridgeProxy),
            address(_output.l1CrossDomainMessengerProxy)
        );
        address[] memory addrs2 = Solarray.addresses(
            address(_output.optimismPortalProxy),
            address(_output.disputeGameFactoryProxy),
            address(_output.anchorStateRegistryProxy),
            address(_output.permissionedDisputeGame),
            //address(_output.faultDisputeGame),
            address(_output.delayedWETHPermissionedGameProxy),
            address(_output.ethLockboxProxy)
        );
        // TODO: Eventually switch from Permissioned to Permissionless. Add this address back in.
        // address(_delayedWETHPermissionlessGameProxy)

        DeployUtils.assertValidContractAddresses(Solarray.extend(addrs1, addrs2));
        assertValidDeploy(_input, _output);
    }

    // -------- Deployment Assertions --------
    function assertValidDeploy(Input memory _input, Output memory _output) private {
        assertValidAnchorStateRegistryProxy(_output);
        assertValidDelayedWETH(_input, _output);
        assertValidDisputeGameFactory(_input, _output);
        assertValidL1CrossDomainMessenger(_output);
        assertValidL1ERC721Bridge(_output);
        assertValidL1StandardBridge(_output);
        assertValidOptimismMintableERC20Factory(_output);
        assertValidOptimismPortal(_input, _output);
        assertValidETHLockbox(_input, _output);
        assertValidPermissionedDisputeGame(_input, _output);
        assertValidSystemConfig(_input, _output);
        assertValidAddressManager(_output);
        assertValidOPChainProxyAdmin(_input, _output);
    }

    function assertValidPermissionedDisputeGame(Input memory _input, Output memory _output) private view {
        IPermissionedDisputeGame game = _output.permissionedDisputeGame;

        require(GameType.unwrap(game.gameType()) == GameType.unwrap(GameTypes.PERMISSIONED_CANNON), "DPG-10");

        if (_input.allowCustomDisputeParameters) {
            return;
        }

        // This hex string is the absolutePrestate of the latest op-program release, see where the
        // `EXPECTED_PRESTATE_HASH` is defined in `config.yml`.
        require(
            Claim.unwrap(game.absolutePrestate())
                == bytes32(hex"038512e02c4c3f7bdaec27d00edf55b7155e0905301e1a88083e4e0a6764d54c"),
            "DPG-20"
        );

        IOPContractsManager opcm = _input.opcm;
        address mipsImpl = opcm.implementations().mipsImpl;
        require(game.vm() == IBigStepper(mipsImpl), "DPG-30");

        require(address(game.weth()) == address(_output.delayedWETHPermissionedGameProxy), "DPG-40");
        require(address(game.anchorStateRegistry()) == address(_output.anchorStateRegistryProxy), "DPG-50");
        require(game.l2ChainId() == _input.l2ChainId, "DPG-60");
        require(game.l2BlockNumber() == 0, "DPG-70");
        require(Duration.unwrap(game.clockExtension()) == 10800, "DPG-80");
        require(Duration.unwrap(game.maxClockDuration()) == 302400, "DPG-110");
        require(game.splitDepth() == 30, "DPG-90");
        require(game.maxGameDepth() == 73, "DPG-100");
    }

    function assertValidAnchorStateRegistryProxy(Output memory _output) private {
        // First we check the proxy as itself.
        IProxy proxy = IProxy(payable(address(_output.anchorStateRegistryProxy)));
        vm.prank(address(0));
        address admin = proxy.admin();
        require(admin == address(_output.opChainProxyAdmin), "ANCHORP-10");

        // Then we check the proxy as ASR.
        DeployUtils.assertInitialized({
            _contractAddress: address(_output.anchorStateRegistryProxy),
            _isProxy: true,
            _slot: 0,
            _offset: 0
        });

        require(
            address(_output.anchorStateRegistryProxy.disputeGameFactory()) == address(_output.disputeGameFactoryProxy),
            "ANCHORP-30"
        );

        (Hash actualRoot,) = _output.anchorStateRegistryProxy.anchors(GameTypes.PERMISSIONED_CANNON);
        bytes32 expectedRoot = 0xdead000000000000000000000000000000000000000000000000000000000000;
        require(Hash.unwrap(actualRoot) == expectedRoot, "ANCHORP-40");
    }

    function assertValidSystemConfig(Input memory _input, Output memory _output) private view {
        ISystemConfig systemConfig = _output.systemConfigProxy;

        DeployUtils.assertInitialized({ _contractAddress: address(systemConfig), _isProxy: true, _slot: 0, _offset: 0 });

        require(systemConfig.owner() == _input.systemConfigOwner, "SYSCON-10");
        require(systemConfig.basefeeScalar() == _input.basefeeScalar, "SYSCON-20");
        require(systemConfig.blobbasefeeScalar() == _input.blobBaseFeeScalar, "SYSCON-30");
        require(systemConfig.batcherHash() == bytes32(uint256(uint160(_input.batcher))), "SYSCON-40");
        require(systemConfig.gasLimit() == uint64(60_000_000), "SYSCON-50");
        require(systemConfig.unsafeBlockSigner() == _input.unsafeBlockSigner, "SYSCON-60");
        require(systemConfig.scalar() >> 248 == 1, "SYSCON-70");

        IResourceMetering.ResourceConfig memory rConfig = Constants.DEFAULT_RESOURCE_CONFIG();
        IResourceMetering.ResourceConfig memory outputConfig = systemConfig.resourceConfig();
        require(outputConfig.maxResourceLimit == rConfig.maxResourceLimit, "SYSCON-80");
        require(outputConfig.elasticityMultiplier == rConfig.elasticityMultiplier, "SYSCON-90");
        require(outputConfig.baseFeeMaxChangeDenominator == rConfig.baseFeeMaxChangeDenominator, "SYSCON-100");
        require(outputConfig.systemTxMaxGas == rConfig.systemTxMaxGas, "SYSCON-110");
        require(outputConfig.minimumBaseFee == rConfig.minimumBaseFee, "SYSCON-120");
        require(outputConfig.maximumBaseFee == rConfig.maximumBaseFee, "SYSCON-130");

        require(systemConfig.startBlock() == block.number, "SYSCON-140");
        require(systemConfig.batchInbox() == _input.opcm.chainIdToBatchInboxAddress(_input.l2ChainId), "SYSCON-150");

        require(systemConfig.l1CrossDomainMessenger() == address(_output.l1CrossDomainMessengerProxy), "SYSCON-160");
        require(systemConfig.l1ERC721Bridge() == address(_output.l1ERC721BridgeProxy), "SYSCON-170");
        require(systemConfig.l1StandardBridge() == address(_output.l1StandardBridgeProxy), "SYSCON-180");
        require(systemConfig.optimismPortal() == address(_output.optimismPortalProxy), "SYSCON-190");
        require(
            systemConfig.optimismMintableERC20Factory() == address(_output.optimismMintableERC20FactoryProxy),
            "SYSCON-200"
        );
    }

    function assertValidL1CrossDomainMessenger(Output memory _output) private view {
        IL1CrossDomainMessenger messenger = _output.l1CrossDomainMessengerProxy;

        DeployUtils.assertInitialized({ _contractAddress: address(messenger), _isProxy: true, _slot: 0, _offset: 20 });

        require(address(messenger.OTHER_MESSENGER()) == Predeploys.L2_CROSS_DOMAIN_MESSENGER, "L1xDM-10");
        require(address(messenger.otherMessenger()) == Predeploys.L2_CROSS_DOMAIN_MESSENGER, "L1xDM-20");

        require(address(messenger.PORTAL()) == address(_output.optimismPortalProxy), "L1xDM-30");
        require(address(messenger.portal()) == address(_output.optimismPortalProxy), "L1xDM-40");
        require(address(messenger.systemConfig()) == address(_output.systemConfigProxy), "L1xDM-50");

        bytes32 xdmSenderSlot = vm.load(address(messenger), bytes32(uint256(204)));
        require(address(uint160(uint256(xdmSenderSlot))) == Constants.DEFAULT_L2_SENDER, "L1xDM-60");
    }

    function assertValidL1StandardBridge(Output memory _output) private view {
        IL1StandardBridge bridge = _output.l1StandardBridgeProxy;
        IL1CrossDomainMessenger messenger = _output.l1CrossDomainMessengerProxy;

        DeployUtils.assertInitialized({ _contractAddress: address(bridge), _isProxy: true, _slot: 0, _offset: 0 });

        require(address(bridge.MESSENGER()) == address(messenger), "L1SB-10");
        require(address(bridge.messenger()) == address(messenger), "L1SB-20");
        require(address(bridge.OTHER_BRIDGE()) == Predeploys.L2_STANDARD_BRIDGE, "L1SB-30");
        require(address(bridge.otherBridge()) == Predeploys.L2_STANDARD_BRIDGE, "L1SB-40");
        require(address(bridge.systemConfig()) == address(_output.systemConfigProxy), "L1SB-50");
    }

    function assertValidOptimismMintableERC20Factory(Output memory _output) private view {
        IOptimismMintableERC20Factory factory = _output.optimismMintableERC20FactoryProxy;

        DeployUtils.assertInitialized({ _contractAddress: address(factory), _isProxy: true, _slot: 0, _offset: 0 });

        require(factory.BRIDGE() == address(_output.l1StandardBridgeProxy), "MERC20F-10");
        require(factory.bridge() == address(_output.l1StandardBridgeProxy), "MERC20F-20");
    }

    function assertValidL1ERC721Bridge(Output memory _output) private view {
        IL1ERC721Bridge bridge = _output.l1ERC721BridgeProxy;

        DeployUtils.assertInitialized({ _contractAddress: address(bridge), _isProxy: true, _slot: 0, _offset: 0 });

        require(address(bridge.OTHER_BRIDGE()) == Predeploys.L2_ERC721_BRIDGE, "L721B-10");
        require(address(bridge.otherBridge()) == Predeploys.L2_ERC721_BRIDGE, "L721B-20");

        require(address(bridge.MESSENGER()) == address(_output.l1CrossDomainMessengerProxy), "L721B-30");
        require(address(bridge.messenger()) == address(_output.l1CrossDomainMessengerProxy), "L721B-40");
        require(address(bridge.systemConfig()) == address(_output.systemConfigProxy), "L721B-50");
    }

    function assertValidOptimismPortal(Input memory _input, Output memory _output) private view {
        IOptimismPortal portal = _output.optimismPortalProxy;
        ISuperchainConfig superchainConfig = ISuperchainConfig(address(_input.opcm.superchainConfig()));

        require(address(portal.anchorStateRegistry()) == address(_output.anchorStateRegistryProxy), "PORTAL-10");
        require(address(portal.disputeGameFactory()) == address(_output.disputeGameFactoryProxy), "PORTAL-20");
        require(address(portal.systemConfig()) == address(_output.systemConfigProxy), "PORTAL-30");
        require(address(portal.superchainConfig()) == address(superchainConfig), "PORTAL-40");
        require(portal.guardian() == superchainConfig.guardian(), "PORTAL-50");
        require(portal.paused() == portal.systemConfig().paused(), "PORTAL-60");
        require(portal.l2Sender() == Constants.DEFAULT_L2_SENDER, "PORTAL-70");

        // This slot is the custom gas token _balance and this check ensures
        // that it stays unset for forwards compatibility with custom gas token.
        require(vm.load(address(portal), bytes32(uint256(61))) == bytes32(0), "PORTAL-80");

        // Check once the portal is updated to use the new lockbox.
        require(address(portal.ethLockbox()) == address(_output.ethLockboxProxy), "PORTAL-90");
        require(portal.proxyAdminOwner() == _input.opChainProxyAdminOwner, "PORTAL-100");
    }

    function assertValidETHLockbox(Input memory _input, Output memory _output) private view {
        IETHLockbox lockbox = _output.ethLockboxProxy;

        require(address(lockbox.systemConfig()) == address(_output.systemConfigProxy), "ETHLOCKBOX-10");
        require(lockbox.authorizedPortals(_output.optimismPortalProxy), "ETHLOCKBOX-20");
        require(lockbox.proxyAdminOwner() == _input.opChainProxyAdminOwner, "ETHLOCKBOX-30");
    }

    function assertValidDisputeGameFactory(Input memory _input, Output memory _output) private view {
        IDisputeGameFactory factory = _output.disputeGameFactoryProxy;

        DeployUtils.assertInitialized({ _contractAddress: address(factory), _isProxy: true, _slot: 0, _offset: 0 });

        require(
            address(factory.gameImpls(GameTypes.PERMISSIONED_CANNON)) == address(_output.permissionedDisputeGame),
            "DF-10"
        );
        require(factory.owner() == _input.opChainProxyAdminOwner, "DF-20");
    }

    function assertValidDelayedWETH(Input memory _input, Output memory _output) private {
        IDelayedWETH permissioned = _output.delayedWETHPermissionedGameProxy;

        require(permissioned.proxyAdminOwner() == _input.opChainProxyAdminOwner, "DWETH-10");

        IProxy proxy = IProxy(payable(address(permissioned)));
        vm.prank(address(0));
        address admin = proxy.admin();
        require(admin == address(_output.opChainProxyAdmin), "DWETH-20");
    }

    function assertValidAddressManager(Output memory _output) private view {
        require(_output.addressManager.owner() == address(_output.opChainProxyAdmin), "AM-10");
    }

    function assertValidOPChainProxyAdmin(Input memory _input, Output memory _output) private {
        IProxyAdmin admin = _output.opChainProxyAdmin;
        require(admin.owner() == _input.opChainProxyAdminOwner, "OPCPA-10");
        require(
            admin.getProxyImplementation(address(_output.l1CrossDomainMessengerProxy))
                == DeployUtils.assertResolvedDelegateProxyImplementationSet(
                    "OVM_L1CrossDomainMessenger", _output.addressManager
                ),
            "OPCPA-20"
        );
        require(address(admin.addressManager()) == address(_output.addressManager), "OPCPA-30");
        require(
            admin.getProxyImplementation(address(_output.l1StandardBridgeProxy))
                == DeployUtils.assertL1ChugSplashImplementationSet(address(_output.l1StandardBridgeProxy)),
            "OPCPA-40"
        );
        require(
            admin.getProxyImplementation(address(_output.l1ERC721BridgeProxy))
                == DeployUtils.assertERC1967ImplementationSet(address(_output.l1ERC721BridgeProxy)),
            "OPCPA-50"
        );
        require(
            admin.getProxyImplementation(address(_output.optimismPortalProxy))
                == DeployUtils.assertERC1967ImplementationSet(address(_output.optimismPortalProxy)),
            "OPCPA-60"
        );
        require(
            admin.getProxyImplementation(address(_output.systemConfigProxy))
                == DeployUtils.assertERC1967ImplementationSet(address(_output.systemConfigProxy)),
            "OPCPA-70"
        );
        require(
            admin.getProxyImplementation(address(_output.optimismMintableERC20FactoryProxy))
                == DeployUtils.assertERC1967ImplementationSet(address(_output.optimismMintableERC20FactoryProxy)),
            "OPCPA-80"
        );
        require(
            admin.getProxyImplementation(address(_output.disputeGameFactoryProxy))
                == DeployUtils.assertERC1967ImplementationSet(address(_output.disputeGameFactoryProxy)),
            "OPCPA-90"
        );
        require(
            admin.getProxyImplementation(address(_output.delayedWETHPermissionedGameProxy))
                == DeployUtils.assertERC1967ImplementationSet(address(_output.delayedWETHPermissionedGameProxy)),
            "OPCPA-100"
        );
        require(
            admin.getProxyImplementation(address(_output.anchorStateRegistryProxy))
                == DeployUtils.assertERC1967ImplementationSet(address(_output.anchorStateRegistryProxy)),
            "OPCPA-110"
        );
        require(
            admin.getProxyImplementation(address(_output.ethLockboxProxy))
                == DeployUtils.assertERC1967ImplementationSet(address(_output.ethLockboxProxy)),
            "OPCPA-120"
        );
    }
}
