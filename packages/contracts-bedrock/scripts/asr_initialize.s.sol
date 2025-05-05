// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { console2 as console } from "forge-std/console2.sol";
import { Script } from "forge-std/Script.sol";
import { GnosisSafe as Safe } from "safe-contracts/GnosisSafe.sol";
import { AnchorStateRegistry } from "src/dispute/AnchorStateRegistry.sol";
import { Enum } from "safe-contracts/common/Enum.sol";
import { stdJson } from "forge-std/StdJson.sol";
import { Vm } from "forge-std/Vm.sol";
import { IDisputeGameFactory } from "interfaces/dispute/IDisputeGameFactory.sol";
import { ISystemConfig } from "interfaces/L1/ISystemConfig.sol";
import { GameType, Proposal, Hash } from "src/dispute/lib/Types.sol";

contract ASRInitialize is Script {
    function run() external view {
        string memory jsonStr = vm.readFile("asr_initialize_input.json");
        address targetAddress = vm.envAddress("TARGET_ADDRESS");
        ISystemConfig systemConfigProxy = ISystemConfig(stdJson.readAddress(jsonStr, ".systemConfigProxy"));
        IDisputeGameFactory dgfProxy = IDisputeGameFactory(stdJson.readAddress(jsonStr, ".disputeGameFactoryProxy"));
        Hash outputRoot = Hash.wrap(stdJson.readBytes32(jsonStr, ".outputRoot"));
        uint256 blockNumber = stdJson.readUint(jsonStr, ".blockNumber");
        uint32 gameType = uint32(stdJson.readUint(jsonStr, ".gameType"));
        Proposal memory proposal = Proposal({
            root: outputRoot,
            l2SequenceNumber: blockNumber
        });

        bytes memory data = abi.encodeCall(
            AnchorStateRegistry.initialize, (systemConfigProxy, dgfProxy, proposal, GameType.wrap(gameType))
        );

        bytes memory safeCalldata = abi.encodeCall(
            Safe.execTransaction,
            (
                targetAddress, // to
                0, // value
                data, // data
                Enum.Operation.DelegateCall, // operation
                0, // safeTxGas
                0, // baseGas
                0, // gasPrice
                address(0), // gasToken
                payable(0), // refundReceiver
                "" // signatures (empty for now, needs to be signed by owners)
            )
        );

        console.log("Safe transaction calldata:");
        console.logBytes(safeCalldata);
    }
}
