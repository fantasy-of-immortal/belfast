package neweducate

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
)

// All courses in the restored client use presentation nodes followed by one
// numeric settlement. Validate that entire contract before accepting payment;
// playback, skip and rewind must accept exactly the same course configuration.
type educateCourseContract struct {
	Nodes   []newEducateNodeConfig
	Costs   [][]int32
	Rewards [][]int32
	Digest  string
}

func loadEducateCourseContract(state *educateState, plan *newEducatePlanConfig) (*educateCourseContract, error) {
	contract := &educateCourseContract{}
	var err error
	contract.Costs, err = parseEducateDropTriplets(plan.Cost, newEducatePlanCategory, fmt.Sprint(plan.ID), "cost")
	if err != nil {
		return nil, err
	}
	contract.Rewards, err = parseEducateDropTriplets(plan.ResultDisplay, newEducatePlanCategory, fmt.Sprint(plan.ID), "result_display")
	if err != nil {
		return nil, err
	}
	for _, row := range contract.Costs {
		if row[2] < 0 {
			return nil, fmt.Errorf("course %d negative cost", plan.ID)
		}
	}
	// Validate numeric ownership/bounds without changing the real balance or
	// requiring sufficient funds for each separate undiscounted course.
	probe, err := cloneEducateDeliveryState(state)
	if err != nil {
		return nil, err
	}
	if _, err := applyEducateNumericBatch(&probe, append(append([][]int32{}, contract.Costs...), contract.Rewards...), 0, false); err != nil {
		return nil, err
	}
	seen := map[uint32]bool{}
	id := plan.ResultNode
	for len(contract.Nodes) < 256 {
		if id == 0 || seen[id] {
			return nil, fmt.Errorf("course %d missing/cyclic chain at %d", plan.ID, id)
		}
		seen[id] = true
		node, ok, err := loadNewEducateConfigByID[newEducateNodeConfig](newEducateNodeCategory, id)
		if err != nil {
			return nil, err
		}
		if !ok || node.NextType != 1 {
			return nil, fmt.Errorf("course %d missing/nonlinear node %d", plan.ID, id)
		}
		next, err := resolveEducateNodeNext(node, 0, nil)
		if err != nil {
			return nil, err
		}
		contract.Nodes = append(contract.Nodes, *node)
		if node.Type == 102 && node.DropTypeClient == 1 && next == 0 {
			raw, err := json.Marshal(contract)
			if err != nil {
				return nil, err
			}
			contract.Digest = fmt.Sprintf("%x", sha256.Sum256(raw))
			return contract, nil
		}
		if node.Type != 1 || node.DropTypeClient != 0 || next == 0 {
			return nil, fmt.Errorf("course %d unsupported node %d", plan.ID, id)
		}
		id = next
	}
	return nil, fmt.Errorf("course %d exceeds 256 nodes", plan.ID)
}

func educateCourseContractFor(state *educateState, course *educateCourseProgress) (*educateCourseContract, error) {
	plan, ok, err := loadNewEducateConfigByID[newEducatePlanConfig](newEducatePlanCategory, course.PlanID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("missing course %d", course.PlanID)
	}
	contract, err := loadEducateCourseContract(state, plan)
	if err != nil {
		return nil, err
	}
	if course.Contract != "" && course.Contract != contract.Digest {
		return nil, fmt.Errorf("paid course %d configuration changed", course.PlanID)
	}
	if course.Node != 0 {
		found := false
		for _, node := range contract.Nodes {
			if node.ID == course.Node {
				found = true
			}
		}
		if !found {
			return nil, fmt.Errorf("course %d current node is outside its chain", course.PlanID)
		}
	}
	// Old paid schedules acquire a contract on their first successful action.
	// Their past costs are deliberately left unknown; they are never recharged.
	course.Contract = contract.Digest
	return contract, nil
}

func validateEducatePlanDiscount(state *educateState, effect []json.RawMessage) ([]int32, error) {
	var row []int32
	if len(effect) != 2 || json.Unmarshal(effect[1], &row) != nil || len(row) != 4 || (row[0] != 1 && row[0] != 2) || row[1] <= 0 || (row[2] != 1 && row[2] != 2) {
		return nil, fmt.Errorf("invalid course discount effect")
	}
	category := newEducateAttrCategory
	if row[0] == 2 {
		category = newEducateResourceCategory
	}
	if err := validateEducateConditionNumericOwner(state, category, uint32(row[1])); err != nil {
		return nil, err
	}
	return row, nil
}

func educateBenefitOnlyPlanDiscount(benefit *educateBenefitConfig) bool {
	if benefit.Trigger != 2 || len(benefit.Effect) == 0 {
		return false
	}
	for _, effect := range benefit.Effect {
		var kind uint32
		if len(effect) != 2 || json.Unmarshal(effect[0], &kind) != nil || kind != 22 {
			return false
		}
	}
	return true
}

// The client builds course discounts from show_content, extracts plan IDs
// from conditions 8/15, and sums flat amounts and basis-point ratios. Other
// conditions (including settlement-slot conditions) are not fee predicates.
func educateDiscountPlanIDs(raw json.RawMessage, depth int) (map[uint32]bool, error) {
	if depth > 64 {
		return nil, fmt.Errorf("discount condition exceeds 64 levels")
	}
	ids := map[uint32]bool{}
	var id uint32
	if json.Unmarshal(raw, &id) == nil {
		condition, ok, err := loadNewEducateConfigByID[educateConditionConfig](educateConditionCategory, id)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("discount missing condition %d", id)
		}
		if condition.Type == 8 || condition.Type == 15 {
			var plans []uint32
			if err := decodeEducateConditionParam(condition.Param, 0, &plans); err != nil {
				return nil, err
			}
			for _, plan := range plans {
				ids[plan] = true
			}
		}
		return ids, nil
	}
	var expression []json.RawMessage
	if json.Unmarshal(raw, &expression) != nil || expression == nil {
		return nil, fmt.Errorf("invalid discount condition")
	}
	if len(expression) == 0 {
		return ids, nil
	}
	var op string
	var terms []json.RawMessage
	if len(expression) != 2 || json.Unmarshal(expression[0], &op) != nil || (op != "&&" && op != "||") || json.Unmarshal(expression[1], &terms) != nil || len(terms) == 0 {
		return nil, fmt.Errorf("invalid discount expression")
	}
	for _, term := range terms {
		child, err := educateDiscountPlanIDs(term, depth+1)
		if err != nil {
			return nil, err
		}
		for plan := range child {
			ids[plan] = true
		}
	}
	return ids, nil
}

func educateCourseCosts(state *educateState, planID uint32, costs [][]int32) ([][]int32, error) {
	flat, ratio := map[string]*big.Int{}, map[string]*big.Int{}
	seen := map[uint32]bool{}
	for _, active := range state.Info.Benefit.GetActives() {
		// The current client includes pending buffs in fee display and in its
		// local debit on SC_29041. Match that fee contract; gain/trigger paths
		// still exclude pending buffs until next-round activation.
		if seen[active.GetId()] {
			continue
		}
		seen[active.GetId()] = true
		list, _, err := loadEducateBenefitDefinition(state, active.GetId())
		if err != nil {
			return nil, err
		}
		seenBenefits := map[uint32]bool{}
		for _, id := range list.ShowContent {
			if seenBenefits[id] {
				return nil, fmt.Errorf("benefit %d duplicate display content", list.ID)
			}
			seenBenefits[id] = true
			benefit, ok, err := loadNewEducateConfigByID[educateBenefitConfig]("ShareCfg/child2_benefit.json", id)
			if err != nil {
				return nil, err
			}
			if !ok {
				return nil, fmt.Errorf("benefit %d missing display content %d", list.ID, id)
			}
			for _, effect := range benefit.Effect {
				var kind uint32
				if len(effect) != 2 || json.Unmarshal(effect[0], &kind) != nil {
					return nil, fmt.Errorf("invalid benefit %d display effect", id)
				}
				if kind != 22 {
					continue
				}
				row, err := validateEducatePlanDiscount(state, effect)
				if err != nil {
					return nil, err
				}
				plans, err := educateDiscountPlanIDs(benefit.Condition, 0)
				if err != nil {
					return nil, err
				}
				if !plans[planID] {
					continue
				}
				key := educateNumericKey(uint32(row[0]), uint32(row[1]))
				target := flat
				if row[2] == 2 {
					target = ratio
				}
				if target[key] == nil {
					target[key] = new(big.Int)
				}
				target[key].Add(target[key], big.NewInt(int64(row[3])))
			}
		}
	}
	adjusted := make([][]int32, 0, len(costs))
	for _, row := range costs {
		key := educateNumericKey(uint32(row[0]), uint32(row[1]))
		factor := big.NewInt(10000)
		if ratio[key] != nil {
			factor.Add(factor, ratio[key])
		}
		value := new(big.Int).Mul(big.NewInt(int64(row[2])), factor)
		if flat[key] != nil {
			value.Add(value, new(big.Int).Mul(flat[key], big.NewInt(10000)))
		}
		if value.Sign() < 0 {
			value.SetInt64(0)
		}
		value.Quo(value, big.NewInt(10000))
		if !value.IsInt64() || value.Int64() > math.MaxInt32 {
			return nil, fmt.Errorf("course fee exceeds protocol")
		}
		adjusted = append(adjusted, []int32{row[0], row[1], int32(value.Int64())})
	}
	return adjusted, nil
}
