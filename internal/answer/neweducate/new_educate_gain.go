package neweducate

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"

	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
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
			continue
		}
		if seen[active.GetId()] {
			continue
		}
		seen[active.GetId()] = true
		_, benefits, err := loadEducateBenefitDefinition(state, active.GetId())
		if err != nil {
			return nil, err
		}
		for _, benefit := range benefits {
			if benefit.Trigger != 1 && benefit.Trigger != 19 && (benefit.Trigger != 2 || context == nil || context.Slot == 0) {
				continue
			}
			// Payment discounts are a passive fee contract, not gain modifiers.
			// Skip fee-only rows before evaluating their course-slot conditions.
			if educateBenefitOnlyPlanDiscount(benefit) {
				continue
			}
			// Numeric-change passive modifiers are evaluated against their held
			// ledger at award time. One-off change rewards use trigger 19 instead.
			if benefit.Trigger == 19 {
				modifier := false
				for _, effect := range benefit.Effect {
					var kind uint32
					if len(effect) > 0 && json.Unmarshal(effect[0], &kind) == nil && (kind == 3 || kind == 4) {
						modifier = true
					}
				}
				if !modifier {
					continue
				}
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
				if len(effect) != 2 {
					return nil, fmt.Errorf("benefit %d: invalid modifier effect", benefit.ID)
				}
				if err := json.Unmarshal(effect[0], &kind); err != nil {
					return nil, err
				}
				if kind == 1 || kind == 2 || kind == 28 || (kind == 22 && benefit.Trigger == 2) {
					continue
				}
				if kind != 3 && kind != 4 {
					return nil, fmt.Errorf("benefit %d modifier kind %d requires S06 execution", benefit.ID, kind)
				}
				if err := json.Unmarshal(effect[1], &row); err != nil {
					return nil, err
				}
				if len(row) != 3 || (row[0] != 1 && row[0] != 2) || row[1] <= 0 || row[2] < -10000 {
					return nil, fmt.Errorf("benefit %d: invalid numeric modifier", benefit.ID)
				}
				probe := &educateState{Info: proto.Clone(state.Info).(*protobuf.TBINFO)}
				if _, err := applyEducateNumericBatch(probe, [][]int32{row}, 1, false); err != nil {
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
