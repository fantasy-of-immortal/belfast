package neweducate

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"

	"github.com/ggmolly/belfast/internal/protobuf"
)

// Local gain policy: add percentage bonuses, then multiply each final factor;
// round positive awards down once, before the configured resource cap.
func applyEducateGainBatch(state *educateState, rows [][]int32, multiplier uint32, context *educateConditionContext) ([]*protobuf.TBDROP, error) {
	if len(rows) == 0 {
		return []*protobuf.TBDROP{}, nil
	}
	additive := map[string]*big.Int{}
	factors := map[string][]int64{}
	seen := map[uint32]bool{}
	for _, active := range state.Info.Benefit.GetActives() {
		if active.GetIsPending() != 0 {
			return nil, fmt.Errorf("pending benefit requires activation recovery")
		}
		if seen[active.GetId()] {
			continue
		}
		seen[active.GetId()] = true
		benefits, err := educateNumericTalentBenefits(state, active.GetId())
		if err != nil {
			return nil, err
		}
		for _, benefit := range benefits {
			if benefit.Trigger != 1 && (benefit.Trigger != 2 || context == nil || context.Slot == 0) {
				continue
			}
			benefitContext := educateBenefitConditionContext(state, context, active.GetId(), "gain")
			matched, err := evaluateEducateConditionWithContext(state, benefit.Condition, benefitContext)
			if err != nil {
				return nil, err
			}
			if !matched {
				continue
			}
			for _, effect := range benefit.Effect {
				var kind uint32
				var row []int32
				if err := json.Unmarshal(effect[0], &kind); err != nil {
					return nil, err
				}
				if kind == 1 {
					continue
				}
				if err := json.Unmarshal(effect[1], &row); err != nil {
					return nil, err
				}
				key := educateNumericKey(uint32(row[0]), uint32(row[1]))
				if kind == 3 {
					if additive[key] == nil {
						additive[key] = new(big.Int)
					}
					additive[key].Add(additive[key], big.NewInt(int64(row[2])*int64(benefitContext.Multiplier)))
				} else {
					factors[key] = append(factors[key], 10000+int64(row[2]))
				}
			}
		}
	}
	adjusted := make([][]int32, 0, len(rows))
	for index, row := range rows {
		if len(row) != 3 {
			return nil, fmt.Errorf("gain[%d]: expected drop triplet", index)
		}
		amount := int64(row[2]) * int64(multiplier)
		if amount > 0 {
			key := educateNumericKey(uint32(row[0]), uint32(row[1]))
			factor := big.NewInt(10000)
			if additive[key] != nil {
				factor.Add(factor, additive[key])
			}
			if factor.Sign() < 0 {
				factor.SetInt64(0)
			}
			value := new(big.Rat).SetInt64(amount)
			value.Mul(value, new(big.Rat).SetFrac(factor, big.NewInt(10000)))
			for _, f := range factors[key] {
				value.Mul(value, new(big.Rat).SetFrac64(f, 10000))
			}
			result := new(big.Int).Quo(value.Num(), value.Denom())
			if !result.IsInt64() {
				return nil, fmt.Errorf("gain[%d]: overflow", index)
			}
			amount = result.Int64()
		}
		if amount < math.MinInt32 || amount > math.MaxInt32 {
			return nil, fmt.Errorf("gain[%d]: exceeds protocol int32", index)
		}
		adjusted = append(adjusted, []int32{row[0], row[1], int32(amount)})
	}
	return applyEducateNumericBatch(state, adjusted, 1, false)
}
