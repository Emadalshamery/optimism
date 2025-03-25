// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Test } from "forge-std/Test.sol";

import { BaseDeploy } from "scripts/deploy/BaseDeploy.sol";

contract BaseDeploy_Test is Test {
    event Deployed(bytes encodedOutput);

    BaseDeploy internal baseDeploy;

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
contract MyBaseDeploy is BaseDeploy { }
