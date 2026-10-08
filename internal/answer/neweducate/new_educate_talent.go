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
	ID        uint32   `json:"id"`
	Character uint32   `json:"character"`
	Type      uint32   `json:"type"`
	Content   []uint32 `json:"content"`
	Duration  int32    `json:"during_time"`
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
		if !ok || (b.Trigger != 5 && b.Trigger != 2 && b.Trigger != 3 && b.Trigger != 13) {
			return nil, fmt.Errorf("talent %d benefit %d trigger requires recovery", id, benefitID)
		}
		// Course condition context (e.g. ${num}) is not yet restored. Only
		// unconditional additive course talents have a verified contract.
		if (b.Trigger == 2 || b.Trigger == 13) && string(b.Condition) != "[]" {
			return nil, fmt.Errorf("talent %d course condition requires recovery", id)
		}
		var conditionContext *educateConditionContext
		if b.Trigger == 3 {
			// Syntax/ownership validation before the schedule exists. This is
			// not an execution snapshot; the boolean result is discarded.
			conditionContext = &educateConditionContext{Plans: []uint32{}}
		}
		if _, err := evaluateEducateConditionWithContext(state, b.Condition, conditionContext); err != nil {
			return nil, err
		}
		for _, effect := range b.Effect {
			var kind uint32
			var row []int32
			if len(effect) != 2 || json.Unmarshal(effect[0], &kind) != nil || kind != 1 || json.Unmarshal(effect[1], &row) != nil || len(row) != 3 || (row[0] != 1 && row[0] != 2) {
				return nil, fmt.Errorf("talent %d benefit %d effect requires recovery", id, benefitID)
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
		if active.GetIsPending() != 0 {
			return fmt.Errorf("pending benefit requires activation recovery")
		}
		if _, err := educateNumericTalentBenefits(state, active.GetId()); err != nil {
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
	drops := emptyTBDrops()
	for _, active := range state.Info.Benefit.GetActives() {
		if acquiredID != 0 && active.GetId() != acquiredID {
			continue
		}
		if active.GetIsPending() != 0 {
			return nil, fmt.Errorf("pending benefit requires activation recovery")
		}
		benefits, err := educateNumericTalentBenefits(state, active.GetId())
		if err != nil {
			return nil, err
		}
		for _, b := range benefits {
			if b.Trigger != trigger {
				continue
			}
			matched, err := evaluateEducateConditionWithContext(state, b.Condition, conditionContext)
			if err != nil {
				return nil, err
			}
			if !matched {
				continue
			}
			rows := make([][]int32, 0, len(b.Effect))
			for _, effect := range b.Effect {
				var row []int32
				if err := json.Unmarshal(effect[1], &row); err != nil {
					return nil, err
				}
				rows = append(rows, row)
			}
			actual, err := applyEducateNumericBatch(state, rows, 1, false)
			if err != nil {
				return nil, err
			}
			drops.BenefitDrop = append(drops.BenefitDrop, actual...)
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
