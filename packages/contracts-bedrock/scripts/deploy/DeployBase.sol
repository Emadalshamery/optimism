// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { Script } from "forge-std/Script.sol";

/// @notice Common functionality for all deploy scripts.
/// This contract should be inherited by all deploy scripts.
abstract contract DeployBase is Script {
    /// @notice Deploy scripts can use this event to communicate any deployments they have made.
    /// The encodedOutput represents a keccak encoded output struct
    event Deployed(bytes encodedOutput);

    error InvalidDeployInputProperty(string propertyName, string reason);

    /// @notice For ease of checking the outputs of the deploy script, we store all outputs in this array.
    bytes[] public emittedDeployOutputs;

    /// @notice For ease of deploy script testing, we provide a function to emit the Deployed event.
    /// This function can be "mocked" in tests to verify that the deploy script is functioning correctly.
    function emitDeployed(bytes memory encodedOutput) public virtual {
        /// We add the encoded output to the list of deployed outputs.
        emittedDeployOutputs.push(encodedOutput);

        /// And emit the Deployed event.
        emit Deployed(encodedOutput);
    }

    function numEmittedDeployOutputs() public view returns (uint256) {
        return emittedDeployOutputs.length;
    }
}
