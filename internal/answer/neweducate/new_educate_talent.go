package neweducate

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

type educateTalentWeight struct {
	ID     uint32
	Weight uint64
}
type educateBenefitListConfig struct {
	ID          uint32   `json:"id"`
	Character   uint32   `json:"character"`
	Type        uint32   `json:"type"`
	Content     []uint32 `json:"content"`
	ShowContent []uint32 `json:"show_content"`
	Duration    int32    `json:"during_time"`
}

type educateBenefitConfig struct {
	ID        uint32              `json:"id"`
	Trigger   uint32              `json:"trigger"`
	Condition json.RawMessage     `json:"condition"`
	Effect    [][]json.RawMessage `json:"effect"`
}

// Restored numeric talent triggers: acquisition (13), course settlement (2),
// completed schedule (3), and round start (5),
// identified by real talent descriptions and benefit rows.
func educateNumericTalentBenefits(state *educateState, id uint32) ([]*educateBenefitConfig, error) {
	list, ok, err := loadNewEducateConfigByID[educateBenefitListConfig]("ShareCfg/child2_benefit_list.json", id)
	if err != nil {
		return nil, err
	}
	if !ok || list.Character != state.Info.GetId() || list.Type != 1 || list.Duration != -1 {
		return nil, fmt.Errorf("unsupported talent %d character/type/duration", id)
	}
	result := make([]*educateBenefitConfig, 0, len(list.Content))
	for _, benefitID := range list.Content {
		b, ok, err := loadNewEducateConfigByID[educateBenefitConfig]("ShareCfg/child2_benefit.json", benefitID)
		if err != nil {
			return nil, err
		}
		if !ok || (b.Trigger != 1 && b.Trigger != 5 && b.Trigger != 6 && b.Trigger != 2 && b.Trigger != 3 && b.Trigger != 13) {
			return nil, fmt.Errorf("talent %d benefit %d trigger requires recovery", id, benefitID)
		}
		var conditionContext *educateConditionContext
		if b.Trigger == 2 || b.Trigger == 3 || b.Trigger == 6 {
			// Syntax/ownership validation before the schedule exists. This is
			// not an execution snapshot; the boolean result is discarded.
			conditionContext = &educateConditionContext{Plans: []uint32{}, Slot: 1, PlanID: 1, ExecutionID: "syntax-probe", Draw: func(uint64) (uint64, error) { return 0, nil }}
		}
		probeState := *state
		if state.Lifecycle != nil {
			life := *state.Lifecycle
			probeState.Lifecycle = &life
		}
		if _, err := evaluateEducateConditionWithContext(&probeState, b.Condition, conditionContext); err != nil {
			return nil, err
		}
		for _, effect := range b.Effect {
			var kind uint32
			var row []int32
			if len(effect) == 2 && json.Unmarshal(effect[0], &kind) == nil && kind == 22 && b.Trigger == 2 {
				if _, err := validateEducatePlanDiscount(state, effect); err != nil {
					return nil, err
				}
				continue
			}
			if len(effect) != 2 || json.Unmarshal(effect[0], &kind) != nil || (kind != 1 && kind != 3 && kind != 4) || json.Unmarshal(effect[1], &row) != nil || len(row) != 3 || (row[0] != 1 && row[0] != 2) {
				return nil, fmt.Errorf("talent %d benefit %d effect requires recovery", id, benefitID)
			}
			if kind != 1 && b.Trigger != 1 && b.Trigger != 2 {
				return nil, fmt.Errorf("talent %d modifier trigger %d requires recovery", id, b.Trigger)
			}
			if kind != 1 && row[2] < -10000 {
				return nil, fmt.Errorf("talent %d negative gain factor", id)
			}
			// Validate the numeric contract without applying a future reward.
			probe := &educateState{Info: proto.Clone(state.Info).(*protobuf.TBINFO)}
			if _, err := applyEducateNumericBatch(probe, [][]int32{row}, 1, false); err != nil {
				return nil, err
			}
		}
		result = append(result, b)
	}
	return result, nil
}

func validateEducateNumericActives(state *educateState) error {
	for _, active := range state.Info.Benefit.GetActives() {
		if _, _, err := loadEducateBenefitDefinition(state, active.GetId()); err != nil {
			return err
		}
	}
	return nil
}

func applyEducateTalentTrigger(state *educateState, trigger uint32) (*protobuf.TBDROPS, error) {
	return applyEducateTalentTriggerFor(state, trigger, 0)
}

func applyEducateTalentTriggerFor(state *educateState, trigger uint32, acquiredID uint32) (*protobuf.TBDROPS, error) {
	return applyEducateTalentTriggerWithContext(state, trigger, acquiredID, nil)
}

func applyEducateTalentTriggerWithContext(state *educateState, trigger uint32, acquiredID uint32, conditionContext *educateConditionContext) (*protobuf.TBDROPS, error) {
	if state.EffectDepth >= 16 {
		return nil, fmt.Errorf("benefit effect nesting exceeds 16")
	}
	candidate, err := cloneEducateDeliveryState(state)
	if err != nil {
		return nil, err
	}
	candidate.EffectDepth++
	ensureEducateBenefitMemory(&candidate)
	drops, err := applyEducateTalentTriggerForActives(&candidate, trigger, acquiredID, conditionContext, candidate.Info.Benefit.GetActives())
	if err != nil {
		return nil, err
	}
	commitEducateDeliveryState(state, &candidate)
	return drops, nil
}

func applyEducateTalentTriggerForActives(state *educateState, trigger uint32, acquiredID uint32, conditionContext *educateConditionContext, actives []*protobuf.TBBF) (*protobuf.TBDROPS, error) {
	drops := emptyTBDrops()
	seen := map[uint32]bool{}
	for _, active := range actives {
		if acquiredID != 0 && active.GetId() != acquiredID {
			continue
		}
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
		for _, b := range benefits {
			if b.Trigger != trigger {
				continue
			}
			if educateBenefitOnlyPlanDiscount(b) {
				continue
			}
			if trigger == 19 {
				imperative := false
				for _, effect := range b.Effect {
					var kind uint32
					if len(effect) == 0 || json.Unmarshal(effect[0], &kind) != nil {
						return nil, fmt.Errorf("benefit %d: invalid effect", b.ID)
					}
					if kind != 3 && kind != 4 {
						imperative = true
					}
				}
				if !imperative {
					continue
				}
			}
			context := educateBenefitConditionContext(state, conditionContext, active.GetId(), fmt.Sprintf("trigger:%d", trigger))
			executionKey := fmt.Sprintf("%d/%d/%s", active.GetId(), b.ID, context.ExecutionID)
			if state.Lifecycle.BenefitExecutions[executionKey] {
				continue
			}
			matched, err := evaluateEducateConditionWithContext(state, b.Condition, context)
			if err != nil {
				return nil, err
			}
			ensureEducateBenefitMemory(state)
			consumptionKey := fmt.Sprintf("%d/%d/%d", active.GetId(), b.ID, context.window)
			totalMultiplier := context.Multiplier
			if context.usesNumber && (context.window == 18 || context.window == 19) {
				consumed := state.Lifecycle.BenefitConsumption[consumptionKey]
				if consumed > totalMultiplier {
					return nil, fmt.Errorf("benefit %d consumption counter exceeds accumulated units", b.ID)
				}
				context.Multiplier -= consumed
				if !matched {
					state.Lifecycle.BenefitConsumption[consumptionKey] = totalMultiplier
				}
				if context.Multiplier == 0 {
					state.Lifecycle.BenefitExecutions[executionKey] = true
					continue
				}
			}
			if !matched {
				state.Lifecycle.BenefitExecutions[executionKey] = true
				continue
			}
			for _, effect := range b.Effect {
				var kind uint32
				if len(effect) != 2 {
					return nil, fmt.Errorf("benefit %d: invalid effect arity", b.ID)
				}
				if err := json.Unmarshal(effect[0], &kind); err != nil {
					return nil, err
				}
				if kind == 22 && trigger == 2 {
					if _, err := validateEducatePlanDiscount(state, effect); err != nil {
						return nil, err
					}
					continue // Already applied once at schedule payment.
				}
				if kind == 3 || kind == 4 {
					if trigger != 1 && trigger != 2 && trigger != 19 {
						return nil, fmt.Errorf("benefit %d: modifier trigger %d requires S06 execution", b.ID, trigger)
					}
					continue
				}
				if kind == 28 {
					var count int32
					if err := json.Unmarshal(effect[1], &count); err != nil {
						return nil, err
					}
					actual, err := applyEducateDropBatch(state, [][]int32{{7, 0, count}}, context.Multiplier, context)
					if err != nil {
						return nil, err
					}
					drops.BenefitDrop = append(drops.BenefitDrop, actual...)
					continue
				}
				if kind != 1 && kind != 2 {
					return nil, fmt.Errorf("ShareCfg/child2_benefit.json/%d/effect: kind %d requires S06 execution", b.ID, kind)
				}
				var row []int32
				if err := json.Unmarshal(effect[1], &row); err != nil {
					return nil, err
				}
				var actual []*protobuf.TBDROP
				var err error
				if kind == 2 {
					actual, err = applyEducateNumericSet(state, row)
				} else {
					actual, err = applyEducateDropBatch(state, [][]int32{row}, context.Multiplier, context)
				}
				if err != nil {
					return nil, err
				}
				drops.BenefitDrop = append(drops.BenefitDrop, actual...)
			}
			if context.usesNumber && (context.window == 18 || context.window == 19) {
				state.Lifecycle.BenefitConsumption[consumptionKey] = totalMultiplier
			}
			state.Lifecycle.BenefitExecutions[executionKey] = true
		}
	}
	return drops, nil
}

func selectEducateTalent(state *educateState, id uint32) (*protobuf.TBDROPS, error) {
	cache := ensureEducateCache(state.Info).CacheTalent[0]
	if state.Info.Fsm.GetSystemNo() != newEducateSystemTalent || educateHasPending(state.Info) || !state.Lifecycle.Stages[newEducateSystemTalent].Loaded || cache.GetFinished() != 0 {
		return nil, errEducatePhase
	}
	found := false
	for _, candidate := range cache.Talents {
		if candidate == id {
			found = true
		}
	}
	if !found {
		return nil, fmt.Errorf("talent %d is not offered", id)
	}
	if _, err := educateNumericTalentBenefits(state, id); err != nil {
		return nil, err
	}
	state.Info.Talent.Talents = appendUniqueUint32(state.Info.Talent.Talents, id)
	cache.Finished = proto.Uint32(1)
	markEducateStage(state, newEducateSystemTalent, true)
	drops := applyNewEducateTalentSelection(state, id)
	immediate, err := applyEducateTalentTriggerFor(state, 13, id)
	if err != nil {
		return nil, err
	}
	drops.BenefitDrop = append(drops.BenefitDrop, immediate.BenefitDrop...)
	return drops, nil
}

func educateDraw(total uint64) (uint64, error) {
	n, err := rand.Int(rand.Reader, new(big.Int).SetUint64(total))
	if err != nil {
		return 0, err
	}
	return n.Uint64(), nil
}

func educateTalentPool(raw json.RawMessage, character uint32) ([]educateTalentWeight, error) {
	if string(raw) == `""` {
		return nil, nil
	}
	var rows [][]uint32
	if err := json.Unmarshal(raw, &rows); err != nil || rows == nil {
		return nil, fmt.Errorf("benefit_select must be weighted [id,weight] pairs")
	}
	pool := make([]educateTalentWeight, 0, len(rows))
	seen := map[uint32]bool{}
	for _, row := range rows {
		if len(row) != 2 || row[0] == 0 || row[1] == 0 || seen[row[0]] {
			return nil, fmt.Errorf("invalid/duplicate talent weight %v", row)
		}
		seen[row[0]] = true
		config, ok, err := loadNewEducateConfigByID[educateBenefitListConfig]("ShareCfg/child2_benefit_list.json", row[0])
		if err != nil {
			return nil, err
		}
		if !ok || config.Character != character || config.Type != 1 {
			return nil, fmt.Errorf("talent %d missing/wrong character or type", row[0])
		}
		pool = append(pool, educateTalentWeight{row[0], uint64(row[1])})
	}
	return pool, nil
}

func drawEducateTalents(pool []educateTalentWeight, excluded []uint32, count int, draw func(uint64) (uint64, error)) ([]uint32, error) {
	blocked := map[uint32]bool{}
	for _, id := range excluded {
		blocked[id] = true
	}
	result := make([]uint32, 0, count)
	for len(result) < count {
		var total uint64
		for _, p := range pool {
			if !blocked[p.ID] {
				total += p.Weight
			}
		}
		if total == 0 {
			return nil, fmt.Errorf("talent pool exhausted after %d/%d candidates", len(result), count)
		}
		value, err := draw(total)
		if err != nil {
			return nil, err
		}
		if value >= total {
			return nil, fmt.Errorf("talent draw outside weight interval")
		}
		for _, p := range pool {
			if blocked[p.ID] {
				continue
			}
			if value < p.Weight {
				result = append(result, p.ID)
				blocked[p.ID] = true
				break
			}
			value -= p.Weight
		}
	}
	return result, nil
}

func loadEducateTalents(state *educateState, draw func(uint64) (uint64, error)) error {
	stage := state.Info.Fsm.GetSystemNo()
	if (stage != newEducateSystemEvent && stage != newEducateSystemTalent) || educateHasPending(state.Info) {
		return errEducatePhase
	}
	cache := ensureEducateCache(state.Info).CacheTalent[0]
	if !state.Lifecycle.Stages[newEducateSystemTalent].Loaded {
		round, ok, err := loadCurrentNewEducateRoundConfig(state.Info)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("missing talent round")
		}
		pool, err := educateTalentPool(round.BenefitSelect, state.Info.GetId())
		if err != nil {
			return fmt.Errorf("round %d benefit_select: %w", round.ID, err)
		}
		cache.Talents = nil
		cache.Retalents = nil
		if len(pool) > 0 {
			// The client presents a row of candidates; the server-only draw count
			// is absent from this snapshot. Personal-local policy: three choices.
			cache.Talents, err = drawEducateTalents(pool, state.Info.Talent.Talents, 3, draw)
			if err != nil {
				return err
			}
		}
		cache.Finished = proto.Uint32(boolEducateUint(len(cache.Talents) == 0))
		markEducateStage(state, newEducateSystemTalent, cache.GetFinished() != 0)
	}
	state.Info.Fsm.SystemNo = proto.Uint32(newEducateSystemTalent)
	return nil
}

func refreshEducateTalent(state *educateState, id uint32, draw func(uint64) (uint64, error)) (uint32, error) {
	cache := ensureEducateCache(state.Info).CacheTalent[0]
	if state.Info.Fsm.GetSystemNo() != newEducateSystemTalent || educateHasPending(state.Info) || !state.Lifecycle.Stages[newEducateSystemTalent].Loaded || cache.GetFinished() != 0 {
		return 0, errEducatePhase
	}
	index := -1
	for i, candidate := range cache.Talents {
		if candidate == id {
			index = i
		}
	}
	if index < 0 {
		return 0, fmt.Errorf("talent %d is not offered", id)
	}
	for _, refreshed := range cache.Retalents {
		if refreshed == id {
			return 0, fmt.Errorf("talent slot already refreshed")
		}
	}
	round, ok, err := loadCurrentNewEducateRoundConfig(state.Info)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, fmt.Errorf("missing talent round")
	}
	pool, err := educateTalentPool(round.BenefitSelect, state.Info.GetId())
	if err != nil {
		return 0, err
	}
	excluded := append(append(append([]uint32{}, state.Info.Talent.Talents...), cache.Talents...), cache.Retalents...)
	chosen, err := drawEducateTalents(pool, excluded, 1, draw)
	if err != nil {
		return 0, err
	}
	cache.Talents[index] = chosen[0]
	// Current Lua inserts the replacement ID, so the replacement's button is
	// disabled. Preserve that projection and remember both IDs for exclusion.
	cache.Retalents = appendUniqueUint32(cache.Retalents, id)
	cache.Retalents = appendUniqueUint32(cache.Retalents, chosen[0])
	return chosen[0], nil
}
