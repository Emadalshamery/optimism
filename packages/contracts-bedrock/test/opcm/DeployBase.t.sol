// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Test, stdStorage, StdStorage } from "forge-std/Test.sol";
import { stdToml } from "forge-std/StdToml.sol";

import { ProxyAdmin } from "src/universal/ProxyAdmin.sol";
import { Proxy } from "src/universal/Proxy.sol";
import { SuperchainConfig } from "src/L1/SuperchainConfig.sol";
import { IProtocolVersions, ProtocolVersion } from "interfaces/L1/IProtocolVersions.sol";
import { BaseDeploy } from "scripts/deploy/BaseDeploy.sol";

contract DeployBase_Test is Test {
    event Deployed(bytes encodedOutput);

    using stdStorage for StdStorage;

    BaseDeploy baseDeploy;

    // Define default input variables for testing.
    address defaultProxyAdminOwner = makeAddr("defaultProxyAdminOwner");
    address defaultProtocolVersionsOwner = makeAddr("defaultProtocolVersionsOwner");
    address defaultGuardian = makeAddr("defaultGuardian");
    bool defaultPaused = false;
    ProtocolVersion defaultRequiredProtocolVersion = ProtocolVersion.wrap(1);
    ProtocolVersion defaultRecommendedProtocolVersion = ProtocolVersion.wrap(2);

    function setUp() public {
        baseDeploy = new MyBaseDeploy();
    }

    function testFuzz_emit_event_payload(bytes memory _payload) public {
        vm.expectEmit(address(baseDeploy));
        emit Deployed(_payload);

        baseDeploy.emitDeployed(_payload);
    }

    function testFuzz_store_event_payload(bytes[] memory _payloads) public {
        vm.assume(_payloads.length > 0);

        for (uint256 i = 0; i < _payloads.length; i++) {
            bytes memory _payload = _payloads[i];

            baseDeploy.emitDeployed(_payload);
            assertEq(baseDeploy.emittedDeployOutputs(i), _payload);
            assertEq(baseDeploy.numEmittedDeployOutputs(), i + 1);
        }

        assertEq(baseDeploy.numEmittedDeployOutputs(), _payloads.length);
    }
}

/// @notice We need to create a new contract to test the abstract BaseDeploy
contract MyBaseDeploy is BaseDeploy {}
