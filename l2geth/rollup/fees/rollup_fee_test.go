package fees

import (
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestPaysEnough(t *testing.T) {
	tests := map[string]struct {
		opts *PaysEnoughOpts
		err  error
	}{
		"missing-gas-price": {
			opts: &PaysEnoughOpts{
				UserFee:       nil,
				ExpectedFee:   new(big.Int),
				ThresholdUp:   nil,
				ThresholdDown: nil,
			},
			err: errMissingInput,
		},
		"missing-fee": {
			opts: &PaysEnoughOpts{
				UserFee:       nil,
				ExpectedFee:   nil,
				ThresholdUp:   nil,
				ThresholdDown: nil,
			},
			err: errMissingInput,
		},
		"equal-fee": {
			opts: &PaysEnoughOpts{
				UserFee:       common.Big1,
				ExpectedFee:   common.Big1,
				ThresholdUp:   nil,
				ThresholdDown: nil,
			},
			err: nil,
		},
		"fee-too-low": {
			opts: &PaysEnoughOpts{
				UserFee:       common.Big1,
				ExpectedFee:   common.Big2,
				ThresholdUp:   nil,
				ThresholdDown: nil,
			},
			err: ErrFeeTooLow,
		},
		"fee-threshold-down": {
			opts: &PaysEnoughOpts{
				UserFee:       common.Big1,
				ExpectedFee:   common.Big2,
				ThresholdUp:   nil,
				ThresholdDown: new(big.Float).SetFloat64(0.5),
			},
			err: nil,
		},
		"fee-threshold-up": {
			opts: &PaysEnoughOpts{
				UserFee:       common.Big256,
				ExpectedFee:   common.Big1,
				ThresholdUp:   new(big.Float).SetFloat64(1.5),
				ThresholdDown: nil,
			},
			err: ErrFeeTooHigh,
		},
		"fee-too-low-high": {
			opts: &PaysEnoughOpts{
				UserFee:       new(big.Int).SetUint64(10_000),
				ExpectedFee:   new(big.Int).SetUint64(1),
				ThresholdUp:   new(big.Float).SetFloat64(3),
				ThresholdDown: new(big.Float).SetFloat64(0.8),
			},
			err: ErrFeeTooHigh,
		},
		"fee-too-low-down": {
			opts: &PaysEnoughOpts{
				UserFee:       new(big.Int).SetUint64(1),
				ExpectedFee:   new(big.Int).SetUint64(10_000),
				ThresholdUp:   new(big.Float).SetFloat64(3),
				ThresholdDown: new(big.Float).SetFloat64(0.8),
			},
			err: ErrFeeTooLow,
		},
		"fee-too-low-down-2": {
			opts: &PaysEnoughOpts{
				UserFee:       new(big.Int).SetUint64(0),
				ExpectedFee:   new(big.Int).SetUint64(10_000),
				ThresholdUp:   new(big.Float).SetFloat64(3),
				ThresholdDown: new(big.Float).SetFloat64(0.8),
			},
			err: ErrFeeTooLow,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := PaysEnough(tt.opts)
			if !errors.Is(err, tt.err) {
				t.Fatalf("%s: got %s, expected %s", name, err, tt.err)
			}
		})
	}
}
