// SPDX-License-Identifier: MIT
pragma solidity >0.5.0 <0.8.0;

/* Internal Imports */
import { iOVM_GasPriceOracle } from "./iOVM_GasPriceOracle.sol";

/* External Imports */
import { Ownable } from "@openzeppelin/contracts/access/Ownable.sol";
import { SafeMath } from "@openzeppelin/contracts/math/SafeMath.sol";

/**
 * @title OVM_GasPriceOracle
 * @dev This contract exposes the current l2 gas price, a measure of how congested the network
 * currently is. This measure is used by the Sequencer to determine what fee to charge for
 * transactions. When the system is more congested, the l2 gas price will increase and fees
 * will also increase as a result.
 *
 * Runtime target: OVM
 */
contract OVM_GasPriceOracle is Ownable, iOVM_GasPriceOracle {

    /*************
     * Variables *
     *************/

    // Current L2 gas price
    uint256 public gasPrice;
    // Current L1 base fee
    uint256 public l1BaseFee;
    // Amortized cost of batch submission per transaction
    uint256 public overhead = 2750;
    // Value to scale the fee up by
    uint256 public scalar = 1;

    /***************
     * Constructor *
     ***************/

    /**
     * @param _owner Address that will initially own this contract.
     */
    constructor(
        address _owner
    )
        Ownable()
    {
        transferOwnership(_owner);
    }


    /********************
     * Public Functions *
     ********************/

    /**
     * Allows the owner to modify the l2 gas price.
     * @param _gasPrice New l2 gas price.
     */
    function setGasPrice(
        uint256 _gasPrice
    )
        public
        override
        onlyOwner
    {
        gasPrice = _gasPrice;
        emit GasPriceUpdated(_gasPrice);

    }

    /**
     * Allows the owner to modify the l1 base fee.
     * @param _baseFee New l1 base fee
     */
    function setL1BaseFee(
        uint256 _baseFee
    )
        public
        override
        onlyOwner
    {
        l1BaseFee = _baseFee;
        emit L1BaseFeeUpdated(_baseFee);
    }

    /**
     * Allows the owner to modify the overhead.
     * @param _overhead New overhead
     */
    function setOverhead(
        uint256 _overhead
    )
        public
        override
        onlyOwner
    {
        overhead = _overhead;
        emit OverheadUpdated(_overhead);
    }

    /**
     * Allows the owner to modify the scalar.
     * @param _scalar New scalar
     */
    function setScalar(
        uint256 _scalar
    )
        public
        override
        onlyOwner
    {
        scalar = _scalar;
        emit ScalarUpdated(_scalar);
    }

    /**
     * Computes the L1 portion of the fee
     * based on the size of the RLP encoded tx
     * and the current l1BaseFee
     * @param _data RLP encoded tx
     */
    function getL1Fee(bytes memory _data)
        public
        view
        override
        returns (
            uint256
        )
    {
        uint256 l1Cost = getL1GasUsed(_data);
        uint256 l1Fee = SafeMath.mul(l1Cost, l1BaseFee);
        return SafeMath.mul(l1Fee, scalar);
    }

    /**
     * Computes the L1 cost of a transaction
     * 4 gas for 0 byte
     * https://github.com/ethereum/go-ethereum/blob/9ada4a2e2c415e6b0b51c50e901336872e028872/params/protocol_params.go#L33
     * 16 gas for non zero byte
     * https://github.com/ethereum/go-ethereum/blob/9ada4a2e2c415e6b0b51c50e901336872e028872/params/protocol_params.go#L87
     * This will need to be updated if calldata gas prices change
     * @param _data RLP encoded tx
     */
    function getL1GasUsed(bytes memory _data)
        public
        view
        override
        returns (
            uint256
        )
    {
        uint256 total = 0;
        for (uint256 i = 0; i < _data.length; i++) {
            if (_data[i] == 0) {
                total += 4;
            } else {
                total += 16;
            }
        }
        return SafeMath.add(total, overhead);
    }
}
