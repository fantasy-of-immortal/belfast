package neweducate

import (
	"fmt"
	"math"
	"strings"

	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

// These are recorded transitions, not a subtraction of acquisition round from
// the current round. Old saves begin counting when this metadata is introduced.
func ensureEducateBenefitMemory(state *educateState) {
	if state.Lifecycle.BenefitRounds == nil {
		state.Lifecycle.BenefitRounds = map[uint32]educateConditionBuffRounds{}
	}
	if state.Lifecycle.BenefitConsumption == nil {
		state.Lifecycle.BenefitConsumption = map[string]uint32{}
	}
	if state.Lifecycle.BenefitExecutions == nil {
		state.Lifecycle.BenefitExecutions = map[string]bool{}
	}
	for _, active := range state.Info.Benefit.GetActives() {
		if _, ok := state.Lifecycle.BenefitRounds[active.GetId()]; !ok {
			state.Lifecycle.BenefitRounds[active.GetId()] = educateConditionBuffRounds{}
		}
	}
}

func loadEducateBenefitDefinition(state *educateState, id uint32) (*educateBenefitListConfig, []*educateBenefitConfig, error) {
	list, found, err := loadNewEducateConfigByID[educateBenefitListConfig]("ShareCfg/child2_benefit_list.json", id)
	if err != nil {
		return nil, nil, err
	}
	if !found || list.Character != state.Info.GetId() || list.Type < 1 || list.Type > 4 || list.Duration < -1 || list.Duration == 0 {
		return nil, nil, fmt.Errorf("ShareCfg/child2_benefit_list.json/%d: invalid owner/type/duration", id)
	}
	benefits := make([]*educateBenefitConfig, 0, len(list.Content))
	seen := map[uint32]bool{}
	for _, benefitID := range list.Content {
		if seen[benefitID] {
			return nil, nil, fmt.Errorf("benefit %d: duplicate content %d", id, benefitID)
		}
		seen[benefitID] = true
		benefit, found, err := loadNewEducateConfigByID[educateBenefitConfig]("ShareCfg/child2_benefit.json", benefitID)
		if err != nil {
			return nil, nil, err
		}
		if !found || benefit.Trigger == 0 {
			return nil, nil, fmt.Errorf("benefit %d: missing/invalid content %d", id, benefitID)
		}
		benefits = append(benefits, benefit)
	}
	return list, benefits, nil
}

func resetEducateHeldBenefit(state *educateState, id uint32) {
	ensureEducateBenefitMemory(state)
	state.Lifecycle.BenefitRounds[id] = educateConditionBuffRounds{}
	if state.Lifecycle.NumericLedger != nil {
		delete(state.Lifecycle.NumericLedger.Held, id)
	}
	prefix := fmt.Sprintf("%d/", id)
	for key := range state.Lifecycle.BenefitConsumption {
		if strings.HasPrefix(key, prefix) {
			delete(state.Lifecycle.BenefitConsumption, key)
		}
	}
	for key := range state.Lifecycle.BenefitExecutions {
		if strings.HasPrefix(key, prefix) {
			delete(state.Lifecycle.BenefitExecutions, key)
		}
	}
}

// Only the reset counter changes after a periodic payout; total held rounds and
// spending remain available to cumulative conditions.
func resetEducateBenefitRoundCounter(state *educateState, id uint32) error {
	ensureEducateBenefitMemory(state)
	value, found := state.Lifecycle.BenefitRounds[id]
	if !found {
		return fmt.Errorf("benefit %d has no held counter", id)
	}
	value.SinceReset = 0
	state.Lifecycle.BenefitRounds[id] = value
	return nil
}

func deliverEducateBenefit(state *educateState, id uint32, amount int64) ([]*protobuf.TBDROP, error) {
	list, _, err := loadEducateBenefitDefinition(state, id)
	if err != nil {
		return nil, err
	}
	if amount != 1 && amount != -1 {
		return nil, fmt.Errorf("benefit %d acquisition/removal requires +/-1", id)
	}
	if amount == -1 {
		return removeEducateBenefits(state, []uint32{id})
	}
	if list.Type == 3 {
		for _, active := range state.Info.Benefit.GetActives() {
			other, _, err := loadEducateBenefitDefinition(state, active.GetId())
			if err != nil {
				return nil, err
			}
			if other.Type == 3 && other.ID != id {
				return nil, fmt.Errorf("tarot acquisition must use replacement delivery")
			}
		}
	}
	resetEducateHeldBenefit(state, id)
	pending := state.Info.Fsm.GetSystemNo() == newEducateSystemPlan || state.Info.Fsm.GetSystemNo() == newEducateSystemAssess || state.Info.Fsm.GetSystemNo() == newEducateSystemPhase
	state.Info.Benefit.Actives = upsertTBBF(state.Info.Benefit.Actives, id, state.Info.Round.GetRound(), boolEducateUint(pending))
	state.Permanent.TarotArchive = appendUniqueUint32(state.Permanent.TarotArchive, id)
	if list.Type == 1 {
		state.Info.Talent.Talents = appendUniqueUint32(state.Info.Talent.Talents, id)
	}
	if pending {
		return nil, nil
	}
	immediate, err := applyEducateTalentTriggerFor(state, 13, id)
	if err != nil {
		return nil, err
	}
	return immediate.BenefitDrop, nil
}

func removeEducateBenefits(state *educateState, ids []uint32) ([]*protobuf.TBDROP, error) {
	ensureEducateBenefitMemory(state)
	remove := map[uint32]bool{}
	for _, id := range ids {
		remove[id] = true
	}
	original := append([]*protobuf.TBBF{}, state.Info.Benefit.GetActives()...)
	removed := []*protobuf.TBBF{}
	kept := []*protobuf.TBBF{}
	for _, active := range original {
		if remove[active.GetId()] {
			removed = append(removed, active)
		} else {
			kept = append(kept, active)
		}
	}
	if len(removed) == 0 {
		return nil, fmt.Errorf("benefit removal does not reference an owned active")
	}
	state.Info.Benefit.Actives = kept
	// Removed effects can observe self-removal, but cannot modify gains or accrue
	// new held spending. Other active effects observe the same removal batch.
	context := educateChangeContext(state, educateActionID(state, "remove"), 0, nil)
	context.RemovedBuffs = append([]uint32{}, ids...)
	drops, err := applyEducateTalentTriggerForActives(state, 15, 0, context, original)
	if err != nil {
		return nil, err
	}
	for _, active := range removed {
		resetEducateHeldBenefit(state, active.GetId())
		delete(state.Lifecycle.BenefitRounds, active.GetId())
	}
	return drops.BenefitDrop, nil
}

// One completed schedule advances held counters, including a temporary round.
// Pending buffs activate at the next round boundary; they did not participate
// in the completed round. Duration follows the client's ordinary round index.
func transitionEducateBenefits(state *educateState, nextRound uint32) ([]*protobuf.TBDROP, error) {
	ensureEducateBenefitMemory(state)
	expire := []uint32{}
	activate := []uint32{}
	seen := map[uint32]bool{}
	for _, active := range state.Info.Benefit.GetActives() {
		if seen[active.GetId()] {
			continue
		}
		seen[active.GetId()] = true
		list, _, err := loadEducateBenefitDefinition(state, active.GetId())
		if err != nil {
			return nil, err
		}
		if active.GetIsPending() != 0 {
			active.Round, active.IsPending = proto.Uint32(nextRound), proto.Uint32(0)
			resetEducateHeldBenefit(state, active.GetId())
			activate = append(activate, active.GetId())
			continue
		}
		value := state.Lifecycle.BenefitRounds[active.GetId()]
		if value.Total == math.MaxUint32 || value.SinceReset == math.MaxUint32 {
			return nil, fmt.Errorf("benefit %d round counter overflow", active.GetId())
		}
		value.Total++
		value.SinceReset++
		state.Lifecycle.BenefitRounds[active.GetId()] = value
		if list.Duration > 0 && uint64(nextRound) >= uint64(active.GetRound())+uint64(list.Duration) {
			expire = appendUniqueUint32(expire, active.GetId())
		}
	}
	result := []*protobuf.TBDROP{}
	for _, id := range activate {
		drops, err := applyEducateTalentTriggerFor(state, 13, id)
		if err != nil {
			return nil, err
		}
		result = append(result, drops.BenefitDrop...)
	}
	drops, err := applyEducateTalentTrigger(state, 20)
	if err != nil {
		return nil, err
	}
	result = append(result, drops.BenefitDrop...)
	if len(expire) > 0 {
		drops, err := removeEducateBenefits(state, expire)
		if err != nil {
			return nil, err
		}
		for _, id := range expire {
			result = append(result, &protobuf.TBDROP{Type: proto.Uint32(4), Id: proto.Uint32(id), Number: proto.Int32(-1)})
		}
		result = append(result, drops...)
	}
	return result, nil
}
