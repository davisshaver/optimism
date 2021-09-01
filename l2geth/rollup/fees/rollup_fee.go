package fees

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rollup/rcfg"
)

var (
	// errFeeTooLow represents the error case of then the user pays too little
	ErrFeeTooLow = errors.New("fee too low")
	// errFeeTooHigh represents the error case of when the user pays too much
	ErrFeeTooHigh = errors.New("fee too high")
	// errMissingInput represents the error case of missing required input to
	// PaysEnough
	errMissingInput = errors.New("missing input")
)

type Message interface {
	From() common.Address
	To() *common.Address
	GasPrice() *big.Int
	Gas() uint64
	Value() *big.Int
	Nonce() uint64
	Data() []byte
}

type StateDb interface {
	GetState(common.Address, common.Hash) common.Hash
}

type RollupOracle interface {
	SuggestL1GasPrice(ctx context.Context) (*big.Int, error)
	SuggestL2GasPrice(ctx context.Context) (*big.Int, error)
	SuggestOverhead(ctx context.Context) (*big.Int, error)
	SuggestScalar(ctx context.Context) (*big.Int, error)
}

func CalculateFee(tx *types.Transaction, gpo RollupOracle) (*big.Int, error) {
	// Read the variables from the cache
	l1GasPrice, err := gpo.SuggestL1GasPrice(context.Background())
	if err != nil {
		return nil, err
	}
	overhead, err := gpo.SuggestOverhead(context.Background())
	if err != nil {
		return nil, err
	}
	scalar, err := gpo.SuggestScalar(context.Background())
	if err != nil {
		return nil, err
	}

	raw, err := rawTransaction(tx, false)
	if err != nil {
		return nil, err
	}

	l1Fee := CalculateL1Fee(raw, overhead, l1GasPrice, scalar)
	l2GasLimit := new(big.Int).SetUint64(tx.Gas())
	l2Fee := new(big.Int).Mul(tx.GasPrice(), l2GasLimit)
	fee := new(big.Int).Add(l1Fee, l2Fee)
	return fee, nil
}

// CalculateMsgFee
func CalculateMsgFee(msg Message, state StateDb, gasUsed *big.Int) (*big.Int, error) {
	l1Fee, err := CalculateL1MsgFee(msg, state)
	if err != nil {
		return nil, err
	}
	// Multiply the gas price and the gas used to get the L2 fee
	l2Fee := new(big.Int).Mul(msg.GasPrice(), gasUsed)
	// Add the L1 cost and the L2 cost to get the total fee being paid
	fee := new(big.Int).Add(l1Fee, l2Fee)
	return fee, nil
}

// just the L1 portion of the fee
func CalculateL1MsgFee(msg Message, state StateDb) (*big.Int, error) {
	tx := asTransaction(msg)
	raw, err := rawTransaction(tx, true)
	if err != nil {
		return nil, err
	}

	l1GasPrice, overhead, scalar := ReadGPOStorageSlots(state)
	l1Fee := CalculateL1Fee(raw, overhead, l1GasPrice, scalar)
	return l1Fee, nil
}

// CalculateL1Fee computes the L1 fee
func CalculateL1Fee(data []byte, overhead, l1GasPrice, scalar *big.Int) *big.Int {
	cost := CalculateL1GasUsed(data, overhead)
	l1Fee := new(big.Int).Mul(cost, l1GasPrice)
	return new(big.Int).Mul(l1Fee, scalar)
}

// calculateL1Cost computes the L1 gas used based on the calldata and
// constant sized overhead. The overhead can be decreased as the cost of the
// batch submission goes down via contract optimizations. This will not overflow
// under standard network conditions.
func CalculateL1GasUsed(data []byte, overhead *big.Int) *big.Int {
	zeroes, ones := zeroesAndOnes(data)
	zeroesCost := zeroes * params.TxDataZeroGas
	onesCost := ones * params.TxDataNonZeroGasEIP2028
	l1Cost := new(big.Int).SetUint64(zeroesCost + onesCost)
	return new(big.Int).Add(l1Cost, overhead)
}

// readGPOStorageSlots returns the
// l1GasPrice, overhead and scalar from a statedb
func ReadGPOStorageSlots(state StateDb) (*big.Int, *big.Int, *big.Int) {
	l1GasPrice := state.GetState(rcfg.L2GasPriceOracleAddress, rcfg.L1GasPriceSlot)
	overhead := state.GetState(rcfg.L2GasPriceOracleAddress, rcfg.OverheadSlot)
	scalar := state.GetState(rcfg.L2GasPriceOracleAddress, rcfg.ScalarSlot)
	return l1GasPrice.Big(), overhead.Big(), scalar.Big()
}

// rawTransaction RLP encodes the transaction into bytes
// When a signature is not included, set pad to true to
// fill in a dummy signature full on non 0 bytes
func rawTransaction(tx *types.Transaction, pad bool) ([]byte, error) {
	raw := new(bytes.Buffer)
	if err := tx.EncodeRLP(raw); err != nil {
		return nil, err
	}
	if pad {
		// Account for the signature
		// TODO: double check this is the correct padding
		raw.Write(bytes.Repeat([]byte("ff"), 68))
	}
	return raw.Bytes(), nil
}

// asTransaction turns a Message into a types.Transaction
func asTransaction(msg Message) *types.Transaction {
	if msg.To() == nil {
		return types.NewContractCreation(
			msg.Nonce(),
			msg.Value(),
			msg.Gas(),
			msg.GasPrice(),
			msg.Data(),
		)
	}
	return types.NewTransaction(
		msg.Nonce(),
		*msg.To(),
		msg.Value(),
		msg.Gas(),
		msg.GasPrice(),
		msg.Data(),
	)
}

// PaysEnoughOpts represent the options to PaysEnough
type PaysEnoughOpts struct {
	UserFee, ExpectedFee       *big.Int
	ThresholdUp, ThresholdDown *big.Float
}

// PaysEnough returns an error if the fee is not large enough
// `GasPrice` and `Fee` are required arguments.
func PaysEnough(opts *PaysEnoughOpts) error {
	if opts.UserFee == nil {
		return fmt.Errorf("%w: no user fee", errMissingInput)
	}
	if opts.ExpectedFee == nil {
		return fmt.Errorf("%w: no expected fee", errMissingInput)
	}

	fee := new(big.Int).Set(opts.ExpectedFee)
	// Allow for a downward buffer to protect against L1 gas price volatility
	if opts.ThresholdDown != nil {
		fee = mulByFloat(fee, opts.ThresholdDown)
	}
	// Protect the sequencer from being underpaid
	// if user fee < expected fee, return error
	if opts.UserFee.Cmp(fee) == -1 {
		return ErrFeeTooLow
	}
	// Protect users from overpaying by too much
	if opts.ThresholdUp != nil {
		// overpaying = user fee - expected fee
		overpaying := new(big.Int).Sub(opts.UserFee, opts.ExpectedFee)
		threshold := mulByFloat(opts.ExpectedFee, opts.ThresholdUp)
		// if overpaying > threshold, return error
		if overpaying.Cmp(threshold) == 1 {
			return ErrFeeTooHigh
		}
	}
	return nil
}

// zeroesAndOnes counts the number of 0 bytes and non 0 bytes in a byte slice
func zeroesAndOnes(data []byte) (uint64, uint64) {
	var zeroes uint64
	var ones uint64
	for _, byt := range data {
		if byt == 0 {
			zeroes++
		} else {
			ones++
		}
	}
	return zeroes, ones
}

// mulByFloat multiplies a big.Int by a float and returns the
// big.Int rounded upwards
func mulByFloat(num *big.Int, float *big.Float) *big.Int {
	n := new(big.Float).SetUint64(num.Uint64())
	product := n.Mul(n, float)
	pfloat, _ := product.Float64()
	rounded := math.Ceil(pfloat)
	return new(big.Int).SetUint64(uint64(rounded))
}
