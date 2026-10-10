package neweducate

import (
	"fmt"
	"math"
	"strconv"

	"github.com/ggmolly/belfast/internal/protobuf"
)

// Positive and negative totals are separate: a refund must not erase spending.
// Existing saves start collecting at deployment; no historical totals are invented.
type educateNumericChange struct {
	Positive int64 `json:"positive"`
	Negative int64 `json:"negative"`
}

type educateNumericLedger struct {
	Round map[string]educateNumericChange            `json:"round"`
	Held  map[uint32]map[string]educateNumericChange `json:"held"`
}

func educateNumericKey(kind, id uint32) string {
	return fmt.Sprintf("%d/%d", kind, id)
}

func cloneEducateChanges(source map[string]educateNumericChange) map[string]educateNumericChange {
	result := make(map[string]educateNumericChange, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func recordEducateNumericChanges(state *educateState, drops []*protobuf.TBDROP) error {
	if state.Lifecycle == nil {
		return nil
	}
	ledger := &educateNumericLedger{Round: map[string]educateNumericChange{}, Held: map[uint32]map[string]educateNumericChange{}}
	if old := state.Lifecycle.NumericLedger; old != nil {
		ledger.Round = cloneEducateChanges(old.Round)
		for id, values := range old.Held {
			ledger.Held[id] = cloneEducateChanges(values)
		}
	}
	activeIDs := map[uint32]bool{}
	for _, active := range state.Info.Benefit.GetActives() {
		if active.GetIsPending() == 0 {
			activeIDs[active.GetId()] = true
			if ledger.Held[active.GetId()] == nil {
				ledger.Held[active.GetId()] = map[string]educateNumericChange{}
			}
		}
	}
	add := func(values map[string]educateNumericChange, drop *protobuf.TBDROP) error {
		key := educateNumericKey(drop.GetType(), drop.GetId())
		value := values[key]
		amount := int64(drop.GetNumber())
		if value.Positive < 0 || value.Negative < 0 {
			return fmt.Errorf("invalid stored change counters %s", key)
		}
		if amount > 0 {
			if value.Positive > math.MaxInt64-amount {
				return fmt.Errorf("positive change counter overflow %s", key)
			}
			value.Positive += amount
		} else {
			if value.Negative > math.MaxInt64+amount {
				return fmt.Errorf("negative change counter overflow %s", key)
			}
			value.Negative -= amount
		}
		values[key] = value
		return nil
	}
	for _, drop := range drops {
		if err := add(ledger.Round, drop); err != nil {
			return err
		}
		for id := range activeIDs {
			if err := add(ledger.Held[id], drop); err != nil {
				return err
			}
		}
	}
	state.Lifecycle.NumericLedger = ledger
	return nil
}

func educateChangeContext(state *educateState, action string, sourceID uint32, drops []*protobuf.TBDROP) *educateConditionContext {
	context := &educateConditionContext{ExecutionID: action, SourceBuffID: sourceID, Changes: map[string]educateNumericChange{}, RoundChanges: map[string]educateNumericChange{}, HeldChanges: map[string]educateNumericChange{}}
	for _, drop := range drops {
		key := educateNumericKey(drop.GetType(), drop.GetId())
		value := context.Changes[key]
		if drop.GetNumber() > 0 {
			value.Positive += int64(drop.GetNumber())
		} else {
			value.Negative -= int64(drop.GetNumber())
		}
		context.Changes[key] = value
	}
	if state.Lifecycle != nil && state.Lifecycle.NumericLedger != nil {
		context.RoundChanges = state.Lifecycle.NumericLedger.Round
		if held := state.Lifecycle.NumericLedger.Held[sourceID]; held != nil {
			context.HeldChanges = held
		}
	}
	if state.Lifecycle != nil {
		context.RoundSites = append([]educateConditionSite{}, state.Lifecycle.RoundSites...)
	}
	return context
}

func educateBenefitConditionContext(state *educateState, base *educateConditionContext, id uint32, suffix string) *educateConditionContext {
	context := educateChangeContext(state, educateActionID(state, suffix), id, nil)
	if base != nil {
		context.ExecutionID, context.Draw = base.ExecutionID, base.Draw
		context.Plans, context.PlanID, context.Slot = base.Plans, base.PlanID, base.Slot
		context.Site, context.Changes = base.Site, base.Changes
		context.RemovedBuffs, context.BuffRounds = base.RemovedBuffs, base.BuffRounds
	}
	return context
}

func evaluateEducateChangeCondition(state *educateState, c *educateConditionConfig, context *educateConditionContext) (bool, error) {
	if len(c.Param) != 4 || context == nil {
		return false, fmt.Errorf("numeric change requires four parameters and action context")
	}
	var kind, id uint32
	var op string
	var threshold int64
	for index, target := range []any{&kind, &id, &op, &threshold} {
		if err := decodeEducateConditionParam(c.Param, index, target); err != nil {
			return false, err
		}
	}
	category := newEducateAttrCategory
	if kind == 2 {
		category = newEducateResourceCategory
	} else if kind != 1 {
		return false, fmt.Errorf("unsupported change drop type %d", kind)
	}
	if err := validateEducateConditionNumericOwner(state, category, id); err != nil {
		return false, err
	}
	values := context.Changes
	if context.window == 18 {
		values = context.RoundChanges
	}
	if context.window == 19 {
		values = context.HeldChanges
	}
	if values == nil {
		return false, fmt.Errorf("numeric change requires recorded window %d", context.window)
	}
	change := values[educateNumericKey(kind, id)]
	if change.Positive < 0 || change.Negative < 0 {
		return false, fmt.Errorf("negative stored change counter")
	}
	// A negative threshold or comparison against zero in the negative direction
	// selects spending. It remains signed for the comparison, absolute for num.
	value := change.Positive
	if threshold < 0 || (threshold == 0 && (op == "<" || op == "<=")) {
		value = -change.Negative
	}
	amount := value
	if amount < 0 {
		amount = -amount
	}
	bindEducateConditionNumber(context, amount)
	return educateCompare(value, op, threshold)
}

func educateActionID(state *educateState, suffix string) string {
	return "round:" + strconv.FormatUint(uint64(state.Info.Round.GetRound()), 10) + "/temp:" + strconv.FormatUint(uint64(state.Info.Round.GetInTemp()), 10) + "/revision:" + strconv.FormatInt(state.Entry.Revision+1, 10) + "/" + suffix
}
