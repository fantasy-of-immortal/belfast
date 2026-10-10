package neweducate

import (
	"encoding/json"
	"fmt"
)

// Supplied by a server action, never copied from arbitrary request fields.
// Nil lists mean the corresponding action context has not been recovered;
// an explicitly empty list means it is known to contain no matching action.
type educateConditionContext struct {
	ExecutionID        string
	Plans              []uint32
	PlanID             uint32
	Slot               uint32
	Site               *educateConditionSite
	RoundSites         []educateConditionSite
	RemovedBuffs       []uint32
	SourceBuffID       uint32
	BuffRounds         map[uint32]educateConditionBuffRounds
	KnownBenefitRounds bool
	Draw               func(uint64) (uint64, error)
	Changes            map[string]educateNumericChange
	RoundChanges       map[string]educateNumericChange
	HeldChanges        map[string]educateNumericChange
	Number             int64
	Multiplier         uint32
	hasNumber          bool
	usesNumber         bool
	window             uint32
	draws              map[string]uint32
}

type educateConditionSite struct {
	Type uint32
	ID   uint32
}

// Acquisition/reset/temporary-round handling belongs to the benefit engine.
// Do not synthesize these counters from TBROUND or a buff's acquisition round.
type educateConditionBuffRounds struct {
	Total      uint32 `json:"total"`
	SinceReset uint32 `json:"since_reset"`
}

// Failed expressions never leave a partially recorded random decision. On
// success, draws belong to the role's lifecycle and persist with its action.
func evaluateEducateConditionWithContext(state *educateState, raw json.RawMessage, context *educateConditionContext) (bool, error) {
	if context == nil {
		return evaluateEducateConditionDepth(state, raw, 0, nil)
	}
	candidate := *context
	candidate.Number, candidate.Multiplier, candidate.hasNumber, candidate.window = 0, 1, false, 0
	candidate.usesNumber = false
	candidate.draws = make(map[string]uint32)
	if state.Lifecycle != nil {
		for key, value := range state.Lifecycle.ConditionDraws {
			candidate.draws[key] = value
		}
	}
	matched, err := evaluateEducateConditionDepth(state, raw, 0, &candidate)
	if err != nil {
		return false, err
	}
	if len(candidate.draws) > 0 {
		if state.Lifecycle == nil {
			return false, fmt.Errorf("random condition requires persisted lifecycle")
		}
		state.Lifecycle.ConditionDraws = candidate.draws
	}
	context.Number, context.Multiplier, context.hasNumber, context.usesNumber = candidate.Number, candidate.Multiplier, candidate.hasNumber, candidate.usesNumber
	context.window = candidate.window
	return matched, nil
}

func bindEducateConditionNumber(context *educateConditionContext, value int64) {
	if context != nil {
		context.Number, context.hasNumber = value, true
	}
}

func evaluateEducateContextCondition(state *educateState, c *educateConditionConfig, context *educateConditionContext) (bool, error) {
	p := c.Param
	decode := func(index int, target any) error {
		return decodeEducateConditionParam(p, index, target)
	}
	compareCount := func(count int64) (bool, error) {
		if len(p) != 3 {
			return false, fmt.Errorf("type %d requires three parameters", c.Type)
		}
		var op string
		var threshold int64
		if err := decode(1, &op); err != nil {
			return false, err
		}
		if err := decode(2, &threshold); err != nil {
			return false, err
		}
		bindEducateConditionNumber(context, count)
		return educateCompare(count, op, threshold)
	}
	switch c.Type {
	case 16:
		var divisor int64
		if len(p) != 1 {
			return false, fmt.Errorf("numeric quotient requires one parameter")
		}
		if err := decode(0, &divisor); err != nil {
			return false, err
		}
		if divisor <= 0 || context == nil || !context.hasNumber {
			return false, fmt.Errorf("numeric quotient requires a positive divisor and preceding numeric operand")
		}
		// Personal-local contract: configuration is authoritative; positive
		// units are rounded down. Description/config disagreements are recorded.
		context.Number /= divisor
		return context.Number > 0, nil
	case 17:
		return evaluateEducateChangeCondition(state, c, context)
	case 18, 19:
		if len(p) != 0 || context == nil {
			return false, fmt.Errorf("change window requires action context and empty parameters")
		}
		context.window = c.Type
		if (c.Type == 18 && context.RoundChanges == nil) || (c.Type == 19 && context.HeldChanges == nil) {
			return false, fmt.Errorf("change window %d requires recorded counters", c.Type)
		}
		return true, nil
	case 6:
		var chance uint32
		if len(p) != 1 {
			return false, fmt.Errorf("probability requires one parameter")
		}
		if err := decode(0, &chance); err != nil {
			return false, err
		}
		if chance > 10000 {
			return false, fmt.Errorf("probability exceeds 10000")
		}
		if context == nil || context.ExecutionID == "" || state.Lifecycle == nil {
			return false, fmt.Errorf("probability requires a persisted action instance")
		}
		key := fmt.Sprintf("%s/condition:%d", context.ExecutionID, c.ID)
		draw, exists := context.draws[key]
		if !exists {
			picker := context.Draw
			if picker == nil {
				picker = educateDraw
			}
			value, err := picker(10000)
			if err != nil {
				return false, err
			}
			if value >= 10000 {
				return false, fmt.Errorf("probability draw outside [0,10000)")
			}
			draw = uint32(value)
			context.draws[key] = draw
		}
		if draw >= 10000 {
			return false, fmt.Errorf("stored probability draw outside [0,10000)")
		}
		return draw < chance, nil
	case 8, 15:
		var ids []uint32
		if err := decode(0, &ids); err != nil {
			return false, err
		}
		if context == nil || (c.Type == 8 && context.Plans == nil) || (c.Type == 15 && context.PlanID == 0) {
			return false, fmt.Errorf("type %d requires course action context", c.Type)
		}
		allowed := map[uint32]bool{}
		for _, id := range ids {
			allowed[id] = true
		}
		var count int64
		if c.Type == 8 {
			for _, id := range context.Plans {
				if allowed[id] {
					count++
				}
			}
		} else if allowed[context.PlanID] {
			count = 1
		}
		return compareCount(count)
	case 12:
		var slots []uint32
		if len(p) == 0 {
			return false, fmt.Errorf("course slot condition requires slot indices")
		}
		if err := json.Unmarshal(marshalEducateConditionParams(p), &slots); err != nil {
			return false, err
		}
		if context == nil || context.Slot == 0 {
			return false, fmt.Errorf("course slot condition requires a one-based slot")
		}
		for _, slot := range slots {
			if slot == context.Slot {
				return true, nil
			}
		}
		return false, nil
	case 14:
		if context == nil || context.RemovedBuffs == nil {
			return false, fmt.Errorf("buff removal condition requires removal context")
		}
		var ids []uint32
		if err := json.Unmarshal(marshalEducateConditionParams(p), &ids); err != nil {
			return false, err
		}
		if len(ids) == 0 {
			if context.SourceBuffID == 0 {
				return false, fmt.Errorf("self removal condition requires source buff")
			}
			ids = []uint32{context.SourceBuffID}
		}
		for _, removed := range context.RemovedBuffs {
			for _, id := range ids {
				if removed == id {
					return true, nil
				}
			}
		}
		return false, nil
	case 21:
		var ids []uint32
		if err := decode(0, &ids); err != nil {
			return false, err
		}
		allowed := map[uint32]bool{}
		for _, id := range ids {
			allowed[id] = true
		}
		owned := map[uint32]bool{}
		for _, buff := range state.Info.Benefit.GetActives() {
			if allowed[buff.GetId()] {
				owned[buff.GetId()] = true
			}
		}
		return compareCount(int64(len(owned)))
	case 20:
		if len(p) != 4 {
			return false, fmt.Errorf("buff round counter requires four parameters")
		}
		var id, selector uint32
		var op string
		var threshold int64
		if err := decode(0, &id); err != nil {
			return false, err
		}
		if err := decode(1, &selector); err != nil {
			return false, err
		}
		if err := decode(2, &op); err != nil {
			return false, err
		}
		if err := decode(3, &threshold); err != nil {
			return false, err
		}
		if context == nil || context.BuffRounds == nil {
			return false, fmt.Errorf("buff round condition requires benefit counter context")
		}
		counter, exists := context.BuffRounds[id]
		if context.KnownBenefitRounds {
			if _, _, err := loadEducateBenefitDefinition(state, id); err != nil {
				return false, err
			}
			// A complete server snapshot distinguishes a buff that is absent
			// from missing caller context. An absent buff has held zero rounds.
			if !exists {
				counter, exists = educateConditionBuffRounds{}, true
			}
		}
		if !exists {
			return false, fmt.Errorf("buff %d round counters unavailable", id)
		}
		switch selector {
		case 1:
			return educateCompare(int64(counter.Total), op, threshold)
		case 2:
			return educateCompare(int64(counter.SinceReset), op, threshold)
		default:
			return false, fmt.Errorf("unsupported buff round counter selector %d", selector)
		}
	case 10, 22:
		var sites [][]uint32
		if len(p) == 0 {
			return false, fmt.Errorf("site condition requires source pairs")
		}
		if err := json.Unmarshal(marshalEducateConditionParams(p), &sites); err != nil {
			return false, err
		}
		for _, site := range sites {
			if len(site) != 2 || site[0] == 0 || site[1] == 0 {
				return false, fmt.Errorf("site condition requires positive [type,id] pairs")
			}
		}
		if context == nil || (c.Type == 10 && context.Site == nil) || (c.Type == 22 && context.RoundSites == nil) {
			return false, fmt.Errorf("type %d requires site action context", c.Type)
		}
		visited := context.RoundSites
		if c.Type == 10 {
			visited = []educateConditionSite{*context.Site}
		}
		for _, site := range sites {
			for _, visit := range visited {
				if visit.Type == site[0] && visit.ID == site[1] {
					return true, nil
				}
			}
		}
		return false, nil
	}
	return false, fmt.Errorf("unsupported condition type %d; server semantics need evidence", c.Type)
}

func marshalEducateConditionParams(params []json.RawMessage) json.RawMessage {
	raw, _ := json.Marshal(params)
	return raw
}
