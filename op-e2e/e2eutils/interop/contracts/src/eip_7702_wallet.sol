// SPDX-License-Identifier: MIT
pragma solidity 0.8.25;

/// @title L2ToL2CrossDomainMessenger
/// @notice Gives replay protection and domain binding to cross chain calls.
interface L2ToL2CrossDomainMessenger {
    function crossDomainMessageSender() external view returns (address sender_);
    function sendMessage(uint256 _chainid, address _target, bytes memory _data) external;
}

/// @title Wallet
/// @notice A simple multicall3-based smart contract wallet meant to be used with EIP-7702.
///         Implements cosmos-inspired multichain accounts.
///         - Install at same address across all chains
///         - Send multicall3 data to yourself to make multiple calls successively
///         - Any L2ToL2CrossDomainMessenger calls to yourself on another chain will execute on the remote chain as you
contract Wallet {
    L2ToL2CrossDomainMessenger internal constant messenger = L2ToL2CrossDomainMessenger(0x4200000000000000000000000000000000000023);
    address internal constant multicall = 0xcA11bde05977b3631167028862bE2a173976CA11;

    error OnlyOwner();
    error Revert();

    fallback() external payable {
        address sender = msg.sender;
        try messenger.crossDomainMessageSender() returns (address _sender) {
            sender = _sender;
        } catch {} {}

        if (sender != address(this)) revert OnlyOwner();

        (bool success,) = address(multicall).delegatecall(msg.data);

        if (success == false) revert Revert();
    }
}