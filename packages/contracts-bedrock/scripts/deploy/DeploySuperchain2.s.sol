// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Script } from "forge-std/Script.sol";

import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";
import { IProtocolVersions, ProtocolVersion } from "interfaces/L1/IProtocolVersions.sol";
import { IProxyAdmin } from "interfaces/universal/IProxyAdmin.sol";
import { IProxy } from "interfaces/universal/IProxy.sol";

import { DeployUtils } from "scripts/libraries/DeployUtils.sol";
import { Solarray } from "scripts/libraries/Solarray.sol";

// For all broadcasts in this script we explicitly specify the deployer as `msg.sender` because for
// testing we deploy this script from a test contract. If we provide no argument, the foundry
// default sender would be the broadcaster during test, but the broadcaster needs to be the deployer
// since they are set to the initial proxy admin owner.
contract DeploySuperchain2 is Script {
    struct Input {
        // Role inputs.
        address guardian;
        address protocolVersionsOwner;
        address superchainProxyAdminOwner;
        // Other inputs.
        bool paused;
        ProtocolVersion recommendedProtocolVersion;
        ProtocolVersion requiredProtocolVersion;
    }

    struct Output {
        IProtocolVersions protocolVersionsImpl;
        IProtocolVersions protocolVersionsProxy;
        ISuperchainConfig superchainConfigImpl;
        ISuperchainConfig superchainConfigProxy;
        IProxyAdmin superchainProxyAdmin;
    }

    bytes32 internal _salt = DeployUtils.DEFAULT_SALT;

    // -------- Core Deployment Methods --------

    function run(Input memory _input) public returns (Output memory output_) {
        assertValidInput(_input);

        // Notice that we do not do any explicit verification here that inputs are set. This is because
        // the verification happens elsewhere:
        //   - Getter methods on the input contract provide sanity checks that values are set, when applicable.
        //   - The individual methods below that we use to compose the deployment are responsible for handling
        //     their own verification.
        // This pattern ensures that other deploy scripts that might compose these contracts and
        // methods in different ways are still protected from invalid inputs without need to implement
        // additional verification logic.

        // Deploy the proxy admin, with the owner set to the deployer.
        deploySuperchainProxyAdmin(_input, output_);

        // Deploy and initialize the superchain contracts.
        deploySuperchainImplementationContracts(_input, output_);
        deployAndInitializeSuperchainConfig(_input, output_);
        deployAndInitializeProtocolVersions(_input, output_);

        // Transfer ownership of the ProxyAdmin from the deployer to the specified owner.
        transferProxyAdminOwnership(_input, output_);

        // Output assertions, to make sure outputs were assigned correctly.
        assertValidOutput(_input, output_);
    }

    // -------- Deployment Steps --------

    function deploySuperchainProxyAdmin(Input memory, Output memory _output) internal {
        // Deploy the proxy admin, with the owner set to the deployer.
        // We explicitly specify the deployer as `msg.sender` because for testing we deploy this script from a test
        // contract. If we provide no argument, the foundry default sender would be the broadcaster during test, but the
        // broadcaster needs to be the deployer since they are set to the initial proxy admin owner.
        vm.broadcast(msg.sender);
        IProxyAdmin superchainProxyAdmin = IProxyAdmin(
            DeployUtils.create1({
                _name: "ProxyAdmin",
                _args: DeployUtils.encodeConstructor(abi.encodeCall(IProxyAdmin.__constructor__, (msg.sender)))
            })
        );

        vm.label(address(superchainProxyAdmin), "SuperchainProxyAdmin");
        _output.superchainProxyAdmin = superchainProxyAdmin;
    }

    function deploySuperchainImplementationContracts(Input memory, Output memory _output) internal {
        // Deploy implementation contracts.
        ISuperchainConfig superchainConfigImpl = ISuperchainConfig(
            DeployUtils.createDeterministic({
                _name: "SuperchainConfig",
                _args: DeployUtils.encodeConstructor(abi.encodeCall(ISuperchainConfig.__constructor__, ())),
                _salt: _salt
            })
        );
        IProtocolVersions protocolVersionsImpl = IProtocolVersions(
            DeployUtils.createDeterministic({
                _name: "ProtocolVersions",
                _args: DeployUtils.encodeConstructor(abi.encodeCall(IProtocolVersions.__constructor__, ())),
                _salt: _salt
            })
        );

        vm.label(address(superchainConfigImpl), "SuperchainConfigImpl");
        vm.label(address(protocolVersionsImpl), "ProtocolVersionsImpl");

        _output.superchainConfigImpl = superchainConfigImpl;
        _output.protocolVersionsImpl = protocolVersionsImpl;
    }

    function deployAndInitializeSuperchainConfig(Input memory _input, Output memory _output) internal {
        assertValidGuardianInput(_input);

        address guardian = _input.guardian;
        bool paused = _input.paused;

        IProxyAdmin superchainProxyAdmin = _output.superchainProxyAdmin;
        ISuperchainConfig superchainConfigImpl = _output.superchainConfigImpl;

        vm.startBroadcast(msg.sender);
        ISuperchainConfig superchainConfigProxy = ISuperchainConfig(
            DeployUtils.create1({
                _name: "Proxy",
                _args: DeployUtils.encodeConstructor(
                    abi.encodeCall(IProxy.__constructor__, (address(superchainProxyAdmin)))
                )
            })
        );
        superchainProxyAdmin.upgradeAndCall(
            payable(address(superchainConfigProxy)),
            address(superchainConfigImpl),
            abi.encodeCall(ISuperchainConfig.initialize, (guardian, paused))
        );
        vm.stopBroadcast();

        vm.label(address(superchainConfigProxy), "SuperchainConfigProxy");
        _output.superchainConfigProxy = superchainConfigProxy;
    }

    function deployAndInitializeProtocolVersions(Input memory _input, Output memory _output) internal {
        assertValidProtocolInput(_input);

        address protocolVersionsOwner = _input.protocolVersionsOwner;
        ProtocolVersion requiredProtocolVersion = _input.requiredProtocolVersion;
        ProtocolVersion recommendedProtocolVersion = _input.recommendedProtocolVersion;

        IProxyAdmin superchainProxyAdmin = _output.superchainProxyAdmin;
        IProtocolVersions protocolVersionsImpl = _output.protocolVersionsImpl;

        vm.startBroadcast(msg.sender);
        IProtocolVersions protocolVersionsProxy = IProtocolVersions(
            DeployUtils.create1({
                _name: "Proxy",
                _args: DeployUtils.encodeConstructor(
                    abi.encodeCall(IProxy.__constructor__, (address(superchainProxyAdmin)))
                )
            })
        );
        superchainProxyAdmin.upgradeAndCall(
            payable(address(protocolVersionsProxy)),
            address(protocolVersionsImpl),
            abi.encodeCall(
                IProtocolVersions.initialize,
                (protocolVersionsOwner, requiredProtocolVersion, recommendedProtocolVersion)
            )
        );
        vm.stopBroadcast();

        vm.label(address(protocolVersionsProxy), "ProtocolVersionsProxy");
        _output.protocolVersionsProxy = protocolVersionsProxy;
    }

    function transferProxyAdminOwnership(Input memory _input, Output memory _output) internal {
        assertValidProxyInput(_input);

        address superchainProxyAdminOwner = _input.superchainProxyAdminOwner;

        IProxyAdmin superchainProxyAdmin = _output.superchainProxyAdmin;
        DeployUtils.assertValidContractAddress(address(superchainProxyAdmin));

        vm.broadcast(msg.sender);
        superchainProxyAdmin.transferOwnership(superchainProxyAdminOwner);
    }

    function assertValidInput(Input memory _input) internal pure {
        assertValidGuardianInput(_input);
        assertValidProxyInput(_input);
        assertValidProtocolInput(_input);
    }

    function assertValidGuardianInput(Input memory _input) internal pure {
        require(_input.guardian != address(0), "DeploySuperchain: guardian not set");
    }

    function assertValidProtocolInput(Input memory _input) internal pure {
        require(_input.protocolVersionsOwner != address(0), "DeploySuperchain: protocolVersionsOwner not set");
        require(
            ProtocolVersion.unwrap(_input.requiredProtocolVersion) != 0,
            "DeploySuperchain: requiredProtocolVersion not set"
        );
        require(
            ProtocolVersion.unwrap(_input.recommendedProtocolVersion) != 0,
            "DeploySuperchain: recommendedProtocolVersion not set"
        );
    }

    function assertValidProxyInput(Input memory _input) internal pure {
        require(_input.superchainProxyAdminOwner != address(0), "DeploySuperchain: superchainProxyAdminOwner not set");
    }

    function assertValidOutput(Input memory _input, Output memory _output) public {
        assertValidContractAddresses(_input, _output);
        assertValidSuperchainProxyAdmin(_input, _output);
        assertValidSuperchainConfig(_input, _output);
        assertValidProtocolVersions(_input, _output);
    }

    function assertValidContractAddresses(Input memory, Output memory _output) internal {
        address[] memory addrs = Solarray.addresses(
            address(_output.superchainProxyAdmin),
            address(_output.superchainConfigImpl),
            address(_output.superchainConfigProxy),
            address(_output.protocolVersionsImpl),
            address(_output.protocolVersionsProxy)
        );
        DeployUtils.assertValidContractAddresses(addrs);

        // To read the implementations we prank as the zero address due to the proxyCallIfNotAdmin modifier.
        vm.startPrank(address(0));
        address actualSuperchainConfigImpl = IProxy(payable(address(_output.superchainConfigProxy))).implementation();
        address actualProtocolVersionsImpl = IProxy(payable(address(_output.protocolVersionsProxy))).implementation();
        vm.stopPrank();

        require(actualSuperchainConfigImpl == address(_output.superchainConfigImpl), "100"); // nosemgrep:
            // sol-style-malformed-require
        require(actualProtocolVersionsImpl == address(_output.protocolVersionsImpl), "200"); // nosemgrep:
            // sol-style-malformed-require
    }

    function assertValidSuperchainProxyAdmin(Input memory _input, Output memory _output) internal view {
        require(_output.superchainProxyAdmin.owner() == _input.superchainProxyAdminOwner, "SPA-10");
    }

    function assertValidSuperchainConfig(Input memory _input, Output memory _output) internal {
        // Proxy checks.
        ISuperchainConfig superchainConfig = _output.superchainConfigProxy;
        DeployUtils.assertInitialized({
            _contractAddress: address(superchainConfig),
            _isProxy: true,
            _slot: 0,
            _offset: 0
        });
        require(superchainConfig.guardian() == _input.guardian, "SUPCON-10");
        require(superchainConfig.paused() == _input.paused, "SUPCON-20");

        vm.startPrank(address(0));
        require(
            IProxy(payable(address(superchainConfig))).implementation() == address(_output.superchainConfigImpl),
            "SUPCON-30"
        );
        require(
            IProxy(payable(address(superchainConfig))).admin() == address(_output.superchainProxyAdmin), "SUPCON-40"
        );
        vm.stopPrank();

        // Implementation checks
        superchainConfig = _output.superchainConfigImpl;
        require(superchainConfig.guardian() == address(0), "SUPCON-50");
        require(superchainConfig.paused() == false, "SUPCON-60");
    }

    function assertValidProtocolVersions(Input memory _input, Output memory _output) internal {
        // Proxy checks.
        IProtocolVersions pv = _output.protocolVersionsProxy;
        DeployUtils.assertInitialized({ _contractAddress: address(pv), _isProxy: true, _slot: 0, _offset: 0 });
        require(pv.owner() == _input.protocolVersionsOwner, "PV-10");
        require(
            ProtocolVersion.unwrap(pv.required()) == ProtocolVersion.unwrap(_input.requiredProtocolVersion), "PV-20"
        );
        require(
            ProtocolVersion.unwrap(pv.recommended()) == ProtocolVersion.unwrap(_input.recommendedProtocolVersion),
            "PV-30"
        );

        vm.startPrank(address(0));
        require(IProxy(payable(address(pv))).implementation() == address(_output.protocolVersionsImpl), "PV-40");
        require(IProxy(payable(address(pv))).admin() == address(_output.superchainProxyAdmin), "PV-50");
        vm.stopPrank();

        // Implementation checks.
        pv = _output.protocolVersionsImpl;
        require(pv.owner() == address(0), "PV-60");
        require(ProtocolVersion.unwrap(pv.required()) == 0, "PV-70");
        require(ProtocolVersion.unwrap(pv.recommended()) == 0, "PV-80");
    }
}
