// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import { IL2ToL2CrossDomainMessenger } from "interfaces/L2/IL2ToL2CrossDomainMessenger.sol";
import { ICrossL2Inbox, Identifier } from "interfaces/L2/ICrossL2Inbox.sol";
import { Predeploys } from "src/libraries/Predeploys.sol";

/// @notice Error for when a callback has already been completed.
error CallbackCompleted();

/// @notice Error for when the origin is not the L2CrossDomainMessenger.
error InvalidOrigin();

/// @notice Error for when the message is not a RelayedMessage.
error InvalidEvent();

/// @notice Error for when the message hash is not the same as the original message hash.
error CrossDomainMessageHashMismatch();

contract PromiseCallack {
    /// @notice The hash of the xdomain message
    bytes32 public messageHash;

    /// @notice The target contract to invoke the callback
    address public target;

    /// @notice The selector of the callback
    bytes4 public selector;

    /// @notice Any additional data to pass to the callback
    bytes public context;

    /// @notice Whether the callback has been completed
    bool public completed;

    constructor(bytes32 _messageHash, address _target, bytes4 _selector, bytes memory _context) {
        messageHash = _messageHash;
        selector = _selector;
        target = _target;
        context = _context;
    }

    /// @dev continue chain of execution with the RelayedMessage event receipt
    function callback(Identifier calldata _id, bytes calldata _payload) external {
        if (completed) revert CallbackCompleted();
        if (_id.origin != Predeploys.L2_TO_L2_CROSS_DOMAIN_MESSENGER) revert InvalidOrigin();

        ICrossL2Inbox(Predeploys.CROSS_L2_INBOX).validateMessage(_id, keccak256(_payload));

        bytes32 eventSel = abi.decode(_payload[:32], (bytes32));
        if (eventSel != IL2ToL2CrossDomainMessenger.RelayedMessage.selector) revert InvalidEvent();

        (,, bytes32 relayedMessageHash, bytes memory returnData) =
            abi.decode(_payload[32:], (uint256, uint256, bytes32, bytes));

        // Assert this callback is for the original message
        if (relayedMessageHash != messageHash) revert CrossDomainMessageHashMismatch();

        // Invoke the callback with the return data and optionally the context if present
        bytes memory data =
            context.length > 0 ? abi.encode(selector, returnData, context) : abi.encode(selector, returnData);
        (completed,) = target.call(data);
    }
}

library Promise {
    event OptimismCallback(PromiseCallack callback);

    /// @notice Create a new callback dependent only on the return data of the message
    function then(bytes32 _messageHash, bytes4 _selector) internal returns (PromiseCallack) {
        PromiseCallack callback = new PromiseCallack(_messageHash, msg.sender, _selector, "");
        emit OptimismCallback(callback);
        return callback;
    }

    /// @notice Create a new callback dependent on the return data of the message and some additional context
    function then(bytes32 _messageHash, bytes4 _selector, bytes memory _context) internal returns (PromiseCallack) {
        PromiseCallack callback = new PromiseCallack(_messageHash, msg.sender, _selector, _context);
        emit OptimismCallback(callback);
        return callback;
    }
}
