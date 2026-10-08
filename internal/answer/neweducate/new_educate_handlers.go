package neweducate

import (
	"fmt"
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func NewEducateSetCall(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_29009
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 29010, err
	}
	state, err := loadEducateState(client, payload.GetId())
	if err != nil {
		return 0, 29010, err
	}
	state.Info.Name = proto.String(payload.GetName())
	response := protobuf.SC_29010{Result: proto.Uint32(0)}
	if err := saveEducateState(state); err != nil {
		return 0, 29010, err
	}
	return client.SendMessage(29010, &response)
}

func NewEducateMainEvent(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_29011
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 29012, err
	}
	state, err := updateEducateState(client, payload.GetId(), startEducateMainEvent)
	if err != nil {
		logEducateFailure(client, payload.GetId(), 29011, err)
		return client.SendMessage(29012, &protobuf.SC_29012{Result: proto.Uint32(1), FirstNode: proto.Uint32(0)})
	}
	return client.SendMessage(29012, &protobuf.SC_29012{Result: proto.Uint32(0), FirstNode: proto.Uint32(state.Info.Fsm.GetCurrentNode())})
}

func NewEducateGetTopics(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_29015
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 29016, err
	}
	state, err := loadEducateState(client, payload.GetId())
	if err != nil {
		return 0, 29016, err
	}
	switch state.Info.Fsm.GetSystemNo() {
	case newEducateSystemMap, newEducateSystemTopic, newEducateSystemMind, newEducateSystemChoose:
	default:
		return client.SendMessage(29016, &protobuf.SC_29016{Result: proto.Uint32(1)})
	}
	cache := ensureEducateCache(state.Info)
	chats := cache.CacheChat[0].Chats
	if len(chats) == 0 {
		cache.CacheChat[0].Finished = proto.Uint32(1)
	}
	markEducateStage(state, newEducateSystemTopic, cache.CacheChat[0].GetFinished() != 0)
	response := protobuf.SC_29016{
		Result: proto.Uint32(0),
		Chats:  chats,
	}
	if err := saveEducateState(state); err != nil {
		return 0, 29016, err
	}
	return client.SendMessage(29016, &response)
}

func NewEducateGetTalents(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_29019
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 29020, err
	}
	state, err := updateEducateState(client, payload.GetId(), func(state *educateState) error { return loadEducateTalents(state, educateDraw) })
	if err != nil {
		logEducateFailure(client, payload.GetId(), 29019, err)
		return client.SendMessage(29020, &protobuf.SC_29020{Result: proto.Uint32(1)})
	}
	return client.SendMessage(29020, &protobuf.SC_29020{Result: proto.Uint32(0), Talents: ensureEducateCache(state.Info).CacheTalent[0].Talents})
}

// 29101-29127 priority-choice handlers. The client manages its FSM locally
// from these replies; handlers that do not mutate server state simply answer
// Result=0 (see deep/09-status.md T2 records). Missing handlers here cause the
// 6-second SC_10998 retry storm (backyard CS_19026 failure mode).

func NewEducateGetChoose(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_29126
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 29127, err
	}
	state, err := updateEducateState(client, payload.GetId(), func(state *educateState) error {
		if !educateCanChoose(state) {
			return errEducatePhase
		}
		advanceNewEducateChoose(state)
		markEducateStage(state, newEducateSystemChoose, true)
		return nil
	})
	if err != nil {
		return client.SendMessage(29127, &protobuf.SC_29127{Result: proto.Uint32(1), Fsm: tbInfoPlaceholder().Fsm})
	}
	response := protobuf.SC_29127{
		Result: proto.Uint32(0),
		Fsm:    newEducateClientInfo(state.Info, state.Lifecycle).Fsm,
	}
	return client.SendMessage(29127, &response)
}

func NewEducateRefreshTalent(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_29021
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 29022, err
	}
	var chosen uint32
	_, err := updateEducateState(client, payload.GetId(), func(state *educateState) error {
		var err error
		chosen, err = refreshEducateTalent(state, payload.GetTalent(), educateDraw)
		return err
	})
	if err != nil {
		logEducateFailure(client, payload.GetId(), 29021, err)
		return client.SendMessage(29022, &protobuf.SC_29022{Result: proto.Uint32(1), Talent: proto.Uint32(0)})
	}
	return client.SendMessage(29022, &protobuf.SC_29022{Result: proto.Uint32(0), Talent: proto.Uint32(chosen)})
}
func NewEducateSelectTalent(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_29023
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 29024, err
	}
	var drop *protobuf.TBDROPS
	_, err := updateEducateState(client, payload.GetId(), func(state *educateState) error {
		var err error
		drop, err = selectEducateTalent(state, payload.GetTalent())
		return err
	})
	if err != nil {
		logEducateFailure(client, payload.GetId(), 29023, err)
		return client.SendMessage(29024, &protobuf.SC_29024{Result: proto.Uint32(1), Drop: emptyTBDrops()})
	}
	return client.SendMessage(29024, &protobuf.SC_29024{Result: proto.Uint32(0), Drop: drop})
}
func NewEducateChangePhase(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_29025
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 29026, err
	}
	drop := emptyTBDrops()
	_, err := updateEducateState(client, payload.GetId(), func(state *educateState) error {
		if !educateCanChangePhase(state) {
			return errEducatePhase
		}
		advanceNewEducateRound(state)
		var err error
		drop, err = applyEducateTalentTrigger(state, 5)
		return err
	})
	if err != nil {
		return client.SendMessage(29026, &protobuf.SC_29026{Result: proto.Uint32(1), FirstNode: proto.Uint32(0), Drop: emptyTBDrops()})
	}
	response := protobuf.SC_29026{
		Result:    proto.Uint32(0),
		FirstNode: proto.Uint32(0),
		Drop:      drop,
	}
	return client.SendMessage(29026, &response)
}

func NewEducateUpgradeFavor(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_29027
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 29028, err
	}
	var drops *protobuf.TBDROPS
	_, err := updateEducateState(client, payload.GetId(), func(state *educateState) error {
		var err error
		drops, err = claimEducateFavorLevel(state)
		return err
	})
	if err != nil {
		logEducateFailure(client, payload.GetId(), 29027, err)
		return client.SendMessage(29028, &protobuf.SC_29028{Result: proto.Uint32(1), Drop: emptyTBDrops()})
	}
	return client.SendMessage(29028, &protobuf.SC_29028{Result: proto.Uint32(0), Drop: drops})
}

// currentNewEducatePlanConfig returns the plan item currently being played
// (the slot that NextPlan advanced to) and its child2_plan config.
func currentNewEducatePlanConfig(state *educateState) (*newEducatePlanConfig, bool, error) {
	cache := ensureEducateCache(state.Info)
	planCache := cache.CachePlan[0]
	cur := planCache.GetCurIndex()
	if cur == 0 || int(cur) > len(planCache.Plans) {
		return nil, false, nil
	}
	planID := planCache.Plans[cur-1].GetValue()
	plan, ok, err := loadNewEducateConfigByID[newEducatePlanConfig](newEducatePlanCategory, planID)
	if err != nil {
		return nil, false, err
	}
	return plan, ok, nil
}

// newEducateResultDrops converts child2_plan result_display (or a generic
// [[type,id,number], ...]) into a TBDROPS worth of base drops.
func newEducateResultDrops(state *educateState, triplets [][]int32) (*protobuf.TBDROPS, error) {
	actual, err := applyEducateNumericBatch(state, triplets, 1, false)
	if err != nil {
		return nil, err
	}
	benefits, err := applyEducateTalentTrigger(state, 2)
	if err != nil {
		return nil, err
	}
	return &protobuf.TBDROPS{BaseDrop: actual, BenefitDrop: benefits.BenefitDrop, Display: emptyTBDisplay()}, nil
}

// NewEducateTriggerNode advances the course/event playback chain:
// it loads child2_node[current], returns the follow-up node and — on the
// chain-ending node (type 102, drop_type_client 1) — applies and reports the
// current plan's result_display as drop.
func NewEducateTriggerNode(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_29030
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 29031, err
	}
	response := &protobuf.SC_29031{Result: proto.Uint32(0), NextNode: proto.Uint32(0), Drop: emptyTBDrops()}
	_, err := updateEducateState(client, payload.GetId(), func(state *educateState) error {
		current := state.Info.Fsm.GetCurrentNode()
		if state.Info.Fsm.GetSystemNo() != newEducateSystemPlan {
			if state.Lifecycle.Chain != nil && state.Lifecycle.Chain.Source == "site.normal" {
				var err error
				response.Drop, err = settleEducateNormalSite(state, payload.GetBranch())
				return err
			}
			next, err := advanceEducatePresentationChain(state, payload.GetBranch())
			if err != nil {
				return err
			}
			response.NextNode = proto.Uint32(next)
			return nil
		}
		progress, err := educateScheduleProgressFor(state)
		if err != nil {
			return err
		}
		course := progress.Slots[ensureEducateCache(state.Info).CachePlan[0].GetCurIndex()]
		if course == nil || course.Status != "playing" || course.Node != current {
			return fmt.Errorf("course/node instance mismatch")
		}
		if current == 0 || payload.GetBranch() != 0 {
			return fmt.Errorf("no active fixed course node or invalid branch")
		}
		node, ok, err := loadNewEducateConfigByID[newEducateNodeConfig](newEducateNodeCategory, current)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("missing active node %d", current)
		}
		if node.NextType != 1 || (node.Type != 1 && node.Type != 102) {
			return fmt.Errorf("node %d type %d next_type %d requires S05 interpreter", current, node.Type, node.NextType)
		}
		next, err := resolveEducateNodeNext(node, payload.GetBranch(), nil)
		if err != nil {
			return fmt.Errorf("node %d next: %w", current, err)
		}
		if next != 0 {
			if _, ok, err := loadNewEducateConfigByID[newEducateNodeConfig](newEducateNodeCategory, next); err != nil {
				return err
			} else if !ok {
				return fmt.Errorf("node %d missing successor %d", current, next)
			}
		}
		if node.Type == 102 {
			if state.Info.Fsm.GetSystemNo() != newEducateSystemPlan || node.DropTypeClient != 1 || next != 0 {
				return fmt.Errorf("node %d unsupported course settlement context", current)
			}
			if err := validateEducateNumericActives(state); err != nil {
				return err
			}
			plan, ok, err := currentNewEducatePlanConfig(state)
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("missing active course")
			}
			rows, err := parseEducateDropTriplets(plan.ResultDisplay, newEducatePlanCategory, fmt.Sprint(plan.ID), "result_display")
			if err != nil {
				return err
			}
			response.Drop, err = newEducateResultDrops(state, rows)
			if err != nil {
				return err
			}
			course.Status = "settled"
			course.Rewards = response.Drop.BaseDrop
			course.BenefitRewards = response.Drop.BenefitDrop
			cache := ensureEducateCache(state.Info).CachePlan[0]
			if cache.GetCurIndex() == uint32(len(cache.Plans)) {
				markEducateStage(state, newEducateSystemPlan, true)
			}
		} else if node.DropTypeClient != 0 || next == 0 {
			return fmt.Errorf("node %d unsupported performance settlement", current)
		}
		state.Info.Fsm.CurrentNode = proto.Uint32(next)
		course.Node = next
		response.NextNode = proto.Uint32(next)
		return nil
	})
	if err != nil {
		logEducateFailure(client, payload.GetId(), 29030, err)
		return client.SendMessage(29031, &protobuf.SC_29031{Result: proto.Uint32(1), NextNode: proto.Uint32(0), Drop: emptyTBDrops()})
	}
	return client.SendMessage(29031, response)
}

func NewEducateClearNodeChain(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_29032
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 29033, err
	}
	state, err := updateEducateState(client, payload.GetId(), func(state *educateState) error {
		if state.Info.Fsm.GetSystemNo() == newEducateSystemEvent {
			return restartEducateMainPresentation(state)
		}
		return restartEducateCurrentCourse(state)
	})
	if err != nil {
		logEducateFailure(client, payload.GetId(), 29032, err)
		original, loadErr := loadEducateState(client, payload.GetId())
		if loadErr == nil {
			return client.SendMessage(29033, &protobuf.SC_29033{Result: proto.Uint32(1), Fsm: newEducateClientInfo(original.Info, original.Lifecycle).Fsm})
		}
		return client.SendMessage(29033, &protobuf.SC_29033{Result: proto.Uint32(1), Fsm: &protobuf.TBFSM{SystemNo: proto.Uint32(0), CurrentNode: proto.Uint32(0)}})
	}
	return client.SendMessage(29033, &protobuf.SC_29033{Result: proto.Uint32(0), Fsm: newEducateClientInfo(state.Info, state.Lifecycle).Fsm})
}

func NewEducateSchedule(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_29040
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 29041, err
	}
	state, err := updateEducateState(client, payload.GetId(), func(state *educateState) error { return scheduleEducatePlans(state, payload.GetPlans()) })
	if err != nil {
		logEducateFailure(client, payload.GetId(), 29040, err)
		return client.SendMessage(29041, &protobuf.SC_29041{Result: proto.Uint32(1), Drop: emptyTBDrops()})
	}
	return client.SendMessage(29041, &protobuf.SC_29041{Result: proto.Uint32(0), Plans: ensureEducateCache(state.Info).CachePlan[0].Plans, Drop: emptyTBDrops()})
}

func NewEducateNextPlan(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_29042
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 29043, err
	}
	var first uint32
	_, err := updateEducateState(client, payload.GetId(), func(state *educateState) error {
		if state.Info.Fsm.GetSystemNo() != newEducateSystemPlan || educateHasPending(state.Info) {
			return errEducatePhase
		}
		cache := ensureEducateCache(state.Info).CachePlan[0]
		if cache.GetCurIndex() >= uint32(len(cache.Plans)) {
			return fmt.Errorf("no remaining course slot")
		}
		progress, err := educateScheduleProgressFor(state)
		if err != nil {
			return err
		}
		if cache.GetCurIndex() > 0 && progress.Slots[cache.GetCurIndex()].Status != "settled" {
			return fmt.Errorf("previous course not settled")
		}
		course := progress.Slots[cache.GetCurIndex()+1]
		if course == nil || course.Status != "pending" {
			return fmt.Errorf("next course is not pending")
		}
		cache.CurIndex = proto.Uint32(cache.GetCurIndex() + 1)
		plan, ok, err := currentNewEducatePlanConfig(state)
		if err != nil {
			return err
		}
		if !ok || plan.ResultNode == 0 {
			return fmt.Errorf("missing course playback node")
		}
		if _, ok, err := loadNewEducateConfigByID[newEducateNodeConfig](newEducateNodeCategory, plan.ResultNode); err != nil {
			return err
		} else if !ok {
			return fmt.Errorf("missing first node %d", plan.ResultNode)
		}
		first = plan.ResultNode
		course.Status = "playing"
		course.Node = first
		state.Info.Fsm.CurrentNode = proto.Uint32(first)
		return nil
	})
	if err != nil {
		logEducateFailure(client, payload.GetId(), 29042, err)
		return client.SendMessage(29043, &protobuf.SC_29043{Result: proto.Uint32(1), FirstNode: proto.Uint32(0), Drop: emptyTBDrops()})
	}
	return client.SendMessage(29043, &protobuf.SC_29043{Result: proto.Uint32(0), FirstNode: proto.Uint32(first), Drop: emptyTBDrops()})
}

func NewEducateUpgradePlan(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_29044
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 29045, err
	}
	_, err := updateEducateState(client, payload.GetId(), func(state *educateState) error { return upgradeEducatePlans(state, payload.GetPlanIds()) })
	if err != nil {
		logEducateFailure(client, payload.GetId(), 29044, err)
		return client.SendMessage(29045, &protobuf.SC_29045{Result: proto.Uint32(1)})
	}
	return client.SendMessage(29045, &protobuf.SC_29045{Result: proto.Uint32(0)})
}

func NewEducateScheduleSkip(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_29046
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 29047, err
	}
	response := &protobuf.SC_29047{Result: proto.Uint32(0), Drop: emptyTBDrops()}
	_, err := updateEducateState(client, payload.GetId(), func(state *educateState) error {
		if state.Info.Fsm.GetSystemNo() != newEducateSystemPlan || len(state.Info.Fsm.PriorityFsm) > 0 || len(state.Info.Fsm.TarotSelects) > 0 {
			return errEducatePhase
		}
		cache := ensureEducateCache(state.Info).CachePlan[0]
		cur := cache.GetCurIndex()
		start := int(cur)
		if cur > uint32(len(cache.Plans)) || len(cache.Plans) == 0 {
			return fmt.Errorf("invalid course cursor")
		}
		if state.Info.Fsm.GetCurrentNode() != 0 {
			if cur == 0 {
				return fmt.Errorf("course node without active slot")
			}
			start--
		}
		if start >= len(cache.Plans) {
			return fmt.Errorf("all course slots already settled")
		}
		if err := validateEducateNumericActives(state); err != nil {
			return err
		}
		progress, err := educateScheduleProgressFor(state)
		if err != nil {
			return err
		}
		for _, kv := range cache.Plans[start:] {
			course := progress.Slots[kv.GetKey()]
			if course == nil || course.Status == "settled" {
				return fmt.Errorf("remaining slot %d already settled", kv.GetKey())
			}
			plan, ok, err := loadNewEducateConfigByID[newEducatePlanConfig](newEducatePlanCategory, kv.GetValue())
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("missing course %d", kv.GetValue())
			}
			rows, err := parseEducateDropTriplets(plan.ResultDisplay, newEducatePlanCategory, fmt.Sprint(plan.ID), "result_display")
			if err != nil {
				return err
			}
			drops, err := newEducateResultDrops(state, rows)
			if err != nil {
				return err
			}
			course.Status = "settled"
			course.Node = 0
			course.Rewards = drops.BaseDrop
			course.BenefitRewards = drops.BenefitDrop
			response.Drop.BaseDrop = append(response.Drop.BaseDrop, drops.BaseDrop...)
			response.Drop.BenefitDrop = append(response.Drop.BenefitDrop, drops.BenefitDrop...)
		}
		cache.CurIndex = proto.Uint32(uint32(len(cache.Plans)))
		state.Info.Fsm.CurrentNode = proto.Uint32(0)
		markEducateStage(state, newEducateSystemPlan, true)
		return nil
	})
	if err != nil {
		logEducateFailure(client, payload.GetId(), 29046, err)
		return client.SendMessage(29047, &protobuf.SC_29047{Result: proto.Uint32(1), Drop: emptyTBDrops()})
	}
	return client.SendMessage(29047, response)
}

func NewEducateGetExtraDrop(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_29048
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 29049, err
	}
	state, err := loadEducateState(client, payload.GetId())
	if err != nil {
		logEducateFailure(client, payload.GetId(), 29048, err)
		return client.SendMessage(29049, &protobuf.SC_29049{Result: proto.Uint32(1), Drop: emptyTBDrops(), Res: &protobuf.TBRES{}})
	}
	if state.Lifecycle.Schedule != nil && state.Lifecycle.Schedule.SummaryComplete {
		return client.SendMessage(29049, &protobuf.SC_29049{Result: proto.Uint32(0), Drop: emptyTBDrops(), Res: state.Info.Res})
	}
	drops := emptyTBDrops()
	state, err = updateEducateState(client, payload.GetId(), func(state *educateState) error {
		// A concurrent retry may have completed the summary since the load.
		if state.Lifecycle.Schedule != nil && state.Lifecycle.Schedule.SummaryComplete {
			return nil
		}
		if state.Info.Fsm.GetSystemNo() != newEducateSystemPlan || educateHasPending(state.Info) {
			return errEducatePhase
		}
		progress, err := educateScheduleProgressFor(state)
		if err != nil {
			return err
		}
		if len(progress.Slots) == 0 {
			return fmt.Errorf("missing course schedule")
		}
		for key, slot := range progress.Slots {
			if slot.Status != "settled" {
				return fmt.Errorf("course slot %d not settled", key)
			}
		}
		if err := validateEducateNumericActives(state); err != nil {
			return err
		}
		plans := make([]uint32, 0, len(ensureEducateCache(state.Info).CachePlan[0].Plans))
		for _, plan := range ensureEducateCache(state.Info).CachePlan[0].Plans {
			plans = append(plans, plan.GetValue())
		}
		drops, err = applyEducateTalentTriggerWithContext(state, 3, 0, &educateConditionContext{Plans: plans})
		if err != nil {
			return err
		}
		progress.Extra = drops.BenefitDrop
		progress.ExtraComplete = true
		progress.SummaryComplete = true
		markEducateStage(state, newEducateSystemPlan, true)
		return nil
	})
	if err != nil {
		logEducateFailure(client, payload.GetId(), 29048, err)
		return client.SendMessage(29049, &protobuf.SC_29049{Result: proto.Uint32(1), Drop: emptyTBDrops(), Res: &protobuf.TBRES{}})
	}
	return client.SendMessage(29049, &protobuf.SC_29049{Result: proto.Uint32(0), Drop: drops, Res: state.Info.Res})
}

func NewEducateGetMap(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_29060
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 29061, err
	}
	state, err := loadEducateState(client, payload.GetId())
	if err != nil {
		return 0, 29061, err
	}
	cache := ensureEducateCache(state.Info)
	// Reopening a map during a pending course is a query of the existing
	// snapshot, not a transition back to MAP or a reason to regenerate it.
	if state.Info.Fsm.GetSystemNo() != newEducateSystemChoose && state.Info.Fsm.GetSystemNo() != newEducateSystemMap && state.Info.Fsm.GetSystemNo() != newEducateSystemTopic && state.Info.Fsm.GetSystemNo() != newEducateSystemMind {
		return client.SendMessage(29061, &protobuf.SC_29061{Result: proto.Uint32(1), FsmSite: cache.CacheSite[0], Characters: state.Info.Site.Characters, Drop: emptyTBDrops()})
	}
	if educateHasPending(state.Info) {
		return client.SendMessage(29061, &protobuf.SC_29061{Result: proto.Uint32(1), FsmSite: cache.CacheSite[0], Drop: emptyTBDrops()})
	}
	if state.Info.Fsm.GetSystemNo() == newEducateSystemChoose {
		state.Info.Fsm.SystemNo = proto.Uint32(newEducateSystemMap)
	}
	markEducateStage(state, newEducateSystemMap, true)
	response := protobuf.SC_29061{
		Result:     proto.Uint32(0),
		FsmSite:    cache.CacheSite[0],
		Characters: state.Info.Site.Characters,
		Drop:       emptyTBDrops(),
	}
	if err := saveEducateState(state); err != nil {
		return 0, 29061, err
	}
	return client.SendMessage(29061, &response)
}

func NewEducateMapNormal(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_29062
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 29063, err
	}
	state, err := updateEducateState(client, payload.GetId(), func(state *educateState) error {
		return startEducateNormalSite(state, payload.GetWorkId())
	})
	if err != nil {
		logEducateFailure(client, payload.GetId(), 29062, err)
		return client.SendMessage(29063, &protobuf.SC_29063{Result: proto.Uint32(1), FirstNode: proto.Uint32(0), Drop: emptyTBDrops()})
	}
	return client.SendMessage(29063, &protobuf.SC_29063{Result: proto.Uint32(0), FirstNode: proto.Uint32(state.Info.Fsm.GetCurrentNode()), Drop: emptyTBDrops()})
}
func NewEducateShopping(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_29066
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 29067, err
	}
	var drops *protobuf.TBDROPS
	_, err := updateEducateState(client, payload.GetId(), func(state *educateState) error {
		var err error
		drops, err = purchaseEducateNumericGood(state, payload.GetShop(), payload.GetNum())
		return err
	})
	if err != nil {
		logEducateFailure(client, payload.GetId(), 29066, err)
		return client.SendMessage(29067, &protobuf.SC_29067{Result: proto.Uint32(1), Drop: emptyTBDrops()})
	}
	return client.SendMessage(29067, &protobuf.SC_29067{Result: proto.Uint32(0), Drop: drops})
}
func NewEducateRefresh(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_29092
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 29093, err
	}
	state, err := loadEducateState(client, payload.GetId())
	if err != nil {
		return 0, 29093, err
	}
	if payload.GetDifficulty() != state.Info.GetDifficulty() {
		return client.SendMessage(29093, &protobuf.SC_29093{Result: proto.Uint32(1), Tb: newEducateClientInfo(state.Info, state.Lifecycle)})
	}
	response := protobuf.SC_29093{
		Result: proto.Uint32(0),
		Tb:     newEducateClientInfo(state.Info, state.Lifecycle),
	}
	return client.SendMessage(29093, &response)
}

func appendUniqueUint32(values []uint32, value uint32) []uint32 {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func upsertKVDATA(values []*protobuf.KVDATA, key uint32, value uint32) []*protobuf.KVDATA {
	for _, entry := range values {
		if entry.GetKey() == key {
			entry.Value = proto.Uint32(value)
			return values
		}
	}
	return append(values, &protobuf.KVDATA{Key: proto.Uint32(key), Value: proto.Uint32(value)})
}

func upsertKVDATACount(values []*protobuf.KVDATA, key uint32, increment uint32) []*protobuf.KVDATA {
	for _, entry := range values {
		if entry.GetKey() == key {
			entry.Value = proto.Uint32(entry.GetValue() + increment)
			return values
		}
	}
	return append(values, &protobuf.KVDATA{Key: proto.Uint32(key), Value: proto.Uint32(increment)})
}

func advanceNewEducateRound(state *educateState) {
	tempRounds := state.Info.Round.GetTempRound()
	if tempRounds > 0 {
		state.Info.Round.InTemp = proto.Uint32(1)
		state.Info.Round.TempRound = proto.Uint32(tempRounds - 1)
	} else {
		state.Info.Round.InTemp = proto.Uint32(0)
		state.Info.Round.Round = proto.Uint32(state.Info.Round.GetRound() + 1)
	}
	state.Info.EvalFail = proto.Uint32(0)
	state.Permanent.MaxRound = proto.Uint32(maxUint32(state.Permanent.GetMaxRound(), state.Info.Round.GetRound()))
	state.Info.Fsm = ensureTBInfoDefaults(tbInfoPlaceholder()).Fsm
	state.Lifecycle = freshEducateLifecycle(state.Info)
	state.Info.Site.Characters = []uint32{}
}

func maxUint32(left uint32, right uint32) uint32 {
	if left > right {
		return left
	}
	return right
}
