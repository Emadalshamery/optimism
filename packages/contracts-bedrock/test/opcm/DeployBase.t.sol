// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Test, stdStorage, StdStorage } from "forge-std/Test.sol";
import { stdToml } from "forge-std/StdToml.sol";

import { ProxyAdmin } from "src/universal/ProxyAdmin.sol";
import { Proxy } from "src/universal/Proxy.sol";
import { SuperchainConfig } from "src/L1/SuperchainConfig.sol";
import { IProtocolVersions, ProtocolVersion } from "interfaces/L1/IProtocolVersions.sol";
import { DeployBase } from "scripts/deploy/DeployBase.sol";

contract DeployBase_Test is Test {
    event Deployed(bytes encodedOutput);

    using stdStorage for StdStorage;

    DeployBase deployBase;

    // Define default input variables for testing.
    address defaultProxyAdminOwner = makeAddr("defaultProxyAdminOwner");
    address defaultProtocolVersionsOwner = makeAddr("defaultProtocolVersionsOwner");
    address defaultGuardian = makeAddr("defaultGuardian");
    bool defaultPaused = false;
    ProtocolVersion defaultRequiredProtocolVersion = ProtocolVersion.wrap(1);
    ProtocolVersion defaultRecommendedProtocolVersion = ProtocolVersion.wrap(2);

    function setUp() public {
        deployBase = new MyDeployBase();
    }

    function testFuzz_emit_event_payload(bytes memory _payload) public {
        vm.expectEmit(address(deployBase));
        emit Deployed(_payload);

        deployBase.emitDeployed(_payload);
    }

    function testFuzz_store_event_payload(bytes[] memory _payloads) public {
        vm.assume(_payloads.length > 0);

        for (uint256 i = 0; i < _payloads.length; i++) {
            bytes memory _payload = _payloads[i];

            deployBase.emitDeployed(_payload);
            assertEq(deployBase.emittedDeployOutputs(i), _payload);
            assertEq(deployBase.numEmittedDeployOutputs(), i + 1);
        }

        assertEq(deployBase.numEmittedDeployOutputs(), _payloads.length);
    }
}

contract MyDeployBase is DeployBase {}
