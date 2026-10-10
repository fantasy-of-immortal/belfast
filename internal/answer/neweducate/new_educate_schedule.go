package neweducate

import (
	"fmt"
	"sort"

	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

// Local recovery policy for the FSM-only clear response: replay the current
// unpaid-reward basic course without charging its already paid schedule again.
// Only an entirely known linear chain is safe to rewind. Event effects and
// random/option chains require their own persisted delivery contracts.
func restartEducateCurrentCourse(state *educateState) error {
	if err := validateEducateNumericActives(state); err != nil {
		return err
	}
	if state.Info.Fsm.GetSystemNo() != newEducateSystemPlan || state.Info.Fsm.GetCurrentNode() == 0 || len(state.Info.Fsm.PriorityFsm) > 0 || len(state.Info.Fsm.TarotSelects) > 0 {
		return errEducatePhase
	}
	progress, err := educateScheduleProgressFor(state)
	if err != nil {
		return err
	}
	cache := ensureEducateCache(state.Info).CachePlan[0]
	cur := cache.GetCurIndex()
	course := progress.Slots[cur]
	if cur == 0 || course == nil || course.Status != "playing" || course.Node != state.Info.Fsm.GetCurrentNode() || len(course.Rewards) > 0 {
		return fmt.Errorf("course restart requires an unsettled matching instance")
	}
	if _, err := educateCourseContractFor(state, course); err != nil {
		return err
	}
	if course.Restarts == ^uint32(0) {
		return fmt.Errorf("course restart counter overflow")
	}
	course.Restarts++
	course.Status = "pending"
	course.Node = 0
	cache.CurIndex = proto.Uint32(cur - 1)
	state.Info.Fsm.CurrentNode = proto.Uint32(0)
	markEducateStage(state, newEducateSystemPlan, false)
	return nil
}

func educateAvailablePlans(state *educateState, round *newEducateRoundConfig) (map[uint32]newEducatePlanConfig, error) {
	plans, err := listNewEducateConfigs[newEducatePlanConfig](newEducatePlanCategory)
	if err != nil {
		return nil, err
	}
	groups := map[uint32][]newEducatePlanConfig{}
	for _, plan := range plans {
		groups[plan.GroupID] = append(groups[plan.GroupID], plan)
	}
	upgraded := map[uint32]bool{}
	for _, id := range state.Info.Plan.PlanUpgrade {
		upgraded[id] = true
	}
	available := map[uint32]newEducatePlanConfig{}
	for _, group := range round.PlanGroup {
		items := groups[group]
		if len(items) == 0 {
			return nil, fmt.Errorf("round %d missing plan group %d", round.ID, group)
		}
		sort.Slice(items, func(i, j int) bool { return items[i].Level < items[j].Level })
		selected := items[0]
		levels := map[uint32]bool{}
		for _, item := range items {
			if levels[item.Level] {
				return nil, fmt.Errorf("plan group %d duplicate level %d", group, item.Level)
			}
			levels[item.Level] = true
			if upgraded[item.ID] {
				selected = item
			}
		}
		available[selected.ID] = selected
	}
	return available, nil
}

func scheduleEducatePlans(state *educateState, requested []*protobuf.KVDATA) error {
	if !educateCanPlanInput(state) || educateHasPending(state.Info) {
		return errEducatePhase
	}
	round, ok, err := loadCurrentNewEducateRoundConfig(state.Info)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("missing current round")
	}
	if uint32(len(requested)) != round.PlanNum {
		return fmt.Errorf("round %d requires %d plan slots, got %d", round.ID, round.PlanNum, len(requested))
	}
	if err := validateEducateNumericActives(state); err != nil {
		return err
	}
	available, err := educateAvailablePlans(state, round)
	if err != nil {
		return err
	}
	sorted := make([]*protobuf.KVDATA, 0, len(requested))
	progress := &educateScheduleProgress{Version: state.Entry.Revision + 1, Slots: map[uint32]*educateCourseProgress{}}
	seen := map[uint32]bool{}
	costs := [][]int32{}
	for _, kv := range requested {
		key := kv.GetKey()
		if key == 0 || key > round.PlanNum || seen[key] {
			return fmt.Errorf("invalid/duplicate course slot %d", key)
		}
		seen[key] = true
		plan, ok := available[kv.GetValue()]
		if !ok {
			return fmt.Errorf("course %d unavailable for character %d round %d", kv.GetValue(), state.Info.GetId(), round.ID)
		}
		contract, err := loadEducateCourseContract(state, &plan)
		if err != nil {
			return err
		}
		rows, err := educateCourseCosts(state, plan.ID, contract.Costs)
		if err != nil {
			return err
		}
		costs = append(costs, rows...)
		progress.Slots[key] = &educateCourseProgress{PlanID: plan.ID, Status: "pending", Contract: contract.Digest, PaidCosts: rows}
		sorted = append(sorted, proto.Clone(kv).(*protobuf.KVDATA))
	}
	if _, err := applyEducateNumericBatch(state, costs, 1, true); err != nil {
		return err
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].GetKey() < sorted[j].GetKey() })
	cache := ensureEducateCache(state.Info)
	cache.CachePlan[0].Plans = sorted
	cache.CachePlan[0].CurIndex = proto.Uint32(0)
	state.Lifecycle.Schedule = progress
	state.Info.Fsm.SystemNo = proto.Uint32(newEducateSystemPlan)
	markEducateStage(state, newEducateSystemPlan, false)
	return nil
}

func educateScheduleProgressFor(state *educateState) (*educateScheduleProgress, error) {
	cache := ensureEducateCache(state.Info).CachePlan[0]
	if len(cache.Plans) == 0 || cache.GetCurIndex() > uint32(len(cache.Plans)) {
		return nil, fmt.Errorf("invalid schedule size/cursor")
	}
	for i, kv := range cache.Plans {
		if kv.GetKey() != uint32(i+1) {
			return nil, fmt.Errorf("schedule slots must be contiguous and ordered")
		}
	}
	progress := state.Lifecycle.Schedule
	if progress == nil {
		// Compatibility is limited to the old linear course convention: completed
		// slots precede the active cursor; a nonzero node identifies the playing slot.
		progress = &educateScheduleProgress{Version: state.Entry.Revision, Slots: map[uint32]*educateCourseProgress{}, Source: "legacy-linear-course"}
		for _, kv := range cache.Plans {
			status := "pending"
			node := uint32(0)
			if kv.GetKey() <= cache.GetCurIndex() {
				status = "settled"
			}
			if kv.GetKey() == cache.GetCurIndex() && state.Info.Fsm.GetCurrentNode() != 0 {
				status = "playing"
				node = state.Info.Fsm.GetCurrentNode()
			}
			progress.Slots[kv.GetKey()] = &educateCourseProgress{PlanID: kv.GetValue(), Status: status, Node: node}
		}
		state.Lifecycle.Schedule = progress
	}
	if progress.Slots == nil || len(progress.Slots) != len(cache.Plans) {
		return nil, fmt.Errorf("course progress/cache mismatch")
	}
	for _, kv := range cache.Plans {
		slot := progress.Slots[kv.GetKey()]
		if slot == nil || slot.PlanID != kv.GetValue() || (slot.Status != "pending" && slot.Status != "playing" && slot.Status != "settled") {
			return nil, fmt.Errorf("invalid course progress slot %d", kv.GetKey())
		}
		key, cur, current := kv.GetKey(), cache.GetCurIndex(), state.Info.Fsm.GetCurrentNode()
		if key > cur && (slot.Status != "pending" || slot.Node != 0 || len(slot.Rewards) != 0 || len(slot.BenefitRewards) != 0) {
			return nil, fmt.Errorf("future course %d already started", key)
		}
		if key < cur || (key == cur && current == 0) {
			if slot.Status != "settled" || slot.Node != 0 {
				return nil, fmt.Errorf("past course %d unsettled", key)
			}
		}
		if key == cur && current != 0 && (slot.Status != "playing" || slot.Node != current || len(slot.Rewards) != 0 || len(slot.BenefitRewards) != 0) {
			return nil, fmt.Errorf("active course %d node/reward mismatch", key)
		}
	}
	if cache.GetCurIndex() == 0 && state.Info.Fsm.GetCurrentNode() != 0 {
		return nil, fmt.Errorf("course node without cursor")
	}
	return progress, nil
}

func upgradeEducatePlans(state *educateState, ids []uint32) error {
	if !educateCanPlanInput(state) || educateHasPending(state.Info) {
		return errEducatePhase
	}
	if len(ids) == 0 {
		return fmt.Errorf("no courses requested for upgrade")
	}
	round, ok, err := loadCurrentNewEducateRoundConfig(state.Info)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("missing round")
	}
	available, err := educateAvailablePlans(state, round)
	if err != nil {
		return err
	}
	all, err := listNewEducateConfigs[newEducatePlanConfig](newEducatePlanCategory)
	if err != nil {
		return err
	}
	seen := map[uint32]bool{}
	for _, id := range ids {
		current, ok := available[id]
		if !ok || seen[id] {
			return fmt.Errorf("upgrade course %d is unavailable/duplicate", id)
		}
		seen[id] = true
		original := current.ID
		for steps := 0; steps <= len(all); steps++ {
			var next *newEducatePlanConfig
			for i := range all {
				p := &all[i]
				if p.GroupID == current.GroupID && p.Level == current.Level+1 {
					if next != nil {
						return fmt.Errorf("duplicate course level in group %d", current.GroupID)
					}
					next = p
				}
			}
			if next == nil {
				break
			}
			match, err := evaluateEducateCondition(state, current.LevelCondition)
			if err != nil {
				return err
			}
			if !match {
				break
			}
			current = *next
			if steps == len(all) {
				return fmt.Errorf("course upgrade cycle")
			}
		}
		if current.ID == original {
			return fmt.Errorf("course %d upgrade requirements not met or maximal", id)
		}
		retained := []uint32{}
		for _, stored := range state.Info.Plan.PlanUpgrade {
			sameGroup := false
			for _, p := range all {
				if p.ID == stored && p.GroupID == current.GroupID {
					sameGroup = true
					break
				}
			}
			if !sameGroup {
				retained = append(retained, stored)
			}
		}
		state.Info.Plan.PlanUpgrade = append(retained, current.ID)
	}
	return nil
}

func educateCanPlanInput(state *educateState) bool {
	switch state.Info.Fsm.GetSystemNo() {
	case newEducateSystemMap, newEducateSystemMind:
		return true
	case newEducateSystemTopic:
		// Current client's completed TOPIC handler returns to the already loaded
		// map. Its local SystemNo transition is not a separate server request.
		return state.Lifecycle.Stages[newEducateSystemTopic].Completed && state.Lifecycle.Stages[newEducateSystemMap].Loaded
	default:
		return false
	}
}
