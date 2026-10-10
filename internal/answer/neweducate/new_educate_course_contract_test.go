package neweducate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"testing"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/packets"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func s04FreshCourseState(t *testing.T, client *connection.Client, role, difficulty, round uint32) *educateState {
	t.Helper()
	if _, err := orm.DeleteCommanderTB(client.Commander.CommanderID, role); err != nil {
		t.Fatal(err)
	}
	state, err := loadEducateState(client, role)
	if err != nil {
		t.Fatal(err)
	}
	state.Info.Difficulty, state.Info.Round.Round = proto.Uint32(difficulty), proto.Uint32(round)
	state.Lifecycle = freshEducateLifecycle(state.Info)
	state.Info.Fsm.SystemNo = proto.Uint32(newEducateSystemMap)
	for _, row := range state.Info.Res.Resource {
		cfg, ok, err := loadNewEducateConfigByID[educateNumericConfig](newEducateResourceCategory, row.GetKey())
		if err != nil || !ok {
			t.Fatal(err)
		}
		row.Value = proto.Uint32(uint32(cfg.Max))
	}
	return state
}

func TestRecoveryS04ConcurrentSummaryHasOneCommit(t *testing.T) {
	client := recoveryDB(t)
	state := s04FreshCourseState(t, client, 1, 0, 1)
	state.Info.Fsm.SystemNo = proto.Uint32(newEducateSystemTalent)
	ensureEducateCache(state.Info).CacheTalent[0].Talents = []uint32{1090}
	markEducateStage(state, newEducateSystemTalent, false)
	if _, err := selectEducateTalent(state, 1090); err != nil {
		t.Fatal(err)
	}
	state.Info.Fsm.SystemNo = proto.Uint32(newEducateSystemMap)
	s04Arrange(t, client, state, s04Plans(1101, 1101, 1101, 1101, 1101))
	var skipped protobuf.SC_29047
	recoveryPacket(t, client, NewEducateScheduleSkip, &protobuf.CS_29046{Id: proto.Uint32(1)}, &skipped)
	if skipped.GetResult() != 0 {
		t.Fatal(&skipped)
	}
	before, err := orm.GetCommanderTB(client.Commander.CommanderID, 1)
	if err != nil {
		t.Fatal(err)
	}
	type result struct {
		response *protobuf.SC_29049
		err      error
	}
	start := make(chan struct{})
	results := make(chan result, 12)
	for i := 0; i < 12; i++ {
		go func() {
			peer := &connection.Client{Commander: client.Commander}
			request, _ := proto.Marshal(&protobuf.CS_29048{Id: proto.Uint32(1)})
			<-start
			_, _, err := NewEducateGetExtraDrop(&request, peer)
			response := &protobuf.SC_29049{}
			wire := peer.Buffer.Bytes()
			if err == nil {
				if len(wire) < packets.HEADER_SIZE {
					err = fmt.Errorf("missing summary response")
				} else {
					err = proto.Unmarshal(wire[packets.HEADER_SIZE:], response)
				}
			}
			results <- result{response, err}
		}()
	}
	close(start)
	var totalBonus int32
	for i := 0; i < 12; i++ {
		got := <-results
		if got.err != nil || got.response.GetResult() != 0 {
			t.Fatalf("concurrent summary: %v %v", got.err, got.response)
		}
		for _, drop := range got.response.Drop.BenefitDrop {
			totalBonus += drop.GetNumber()
		}
	}
	after, err := orm.GetCommanderTB(client.Commander.CommanderID, 1)
	if err != nil {
		t.Fatal(err)
	}
	state, err = loadEducateState(client, 1)
	if err != nil {
		t.Fatal(err)
	}
	if totalBonus != 50 || educateKVCount(state.Info.Res.Attrs, 101) != 75 || after.Revision != before.Revision+1 {
		t.Fatalf("repeated bonus/commit bonus=%d revision delta=%d", totalBonus, after.Revision-before.Revision)
	}
	// Deterministically exercise the row-lock retry path, regardless of which
	// concurrent requests used the earlier read-only fast path.
	if _, err := updateEducateState(client, 1, func(s *educateState) error {
		if !s.Lifecycle.Schedule.SummaryComplete {
			return fmt.Errorf("summary missing")
		}
		return errEducateNoChange
	}); err != nil {
		t.Fatal(err)
	}
	unchanged, err := orm.GetCommanderTB(client.Commander.CommanderID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after.State, unchanged.State) || !bytes.Equal(after.Metadata, unchanged.Metadata) || after.Revision != unchanged.Revision {
		t.Fatal("locked summary retry mutated save")
	}
}

func s04Plans(ids ...uint32) []*protobuf.KVDATA {
	plans := []*protobuf.KVDATA{}
	for i, id := range ids {
		plans = append(plans, &protobuf.KVDATA{Key: proto.Uint32(uint32(i + 1)), Value: proto.Uint32(id)})
	}
	return plans
}

func s04Arrange(t *testing.T, client *connection.Client, state *educateState, plans []*protobuf.KVDATA) {
	t.Helper()
	if err := saveEducateState(state); err != nil {
		t.Fatal(err)
	}
	var arranged protobuf.SC_29041
	recoveryPacket(t, client, NewEducateSchedule, &protobuf.CS_29040{Id: state.Info.Id, Plans: plans}, &arranged)
	if arranged.GetResult() != 0 {
		t.Fatalf("arrange %v: %v", plans, &arranged)
	}
}

func s04Finish(t *testing.T, client *connection.Client, role uint32, play bool) *educateState {
	t.Helper()
	if play {
		for i := 0; i < 5; i++ {
			var next protobuf.SC_29043
			recoveryPacket(t, client, NewEducateNextPlan, &protobuf.CS_29042{Id: proto.Uint32(role)}, &next)
			if next.GetResult() != 0 {
				t.Fatal(&next)
			}
			for node, steps := next.GetFirstNode(), 0; node != 0; steps++ {
				if steps >= 256 {
					t.Fatal("course step limit")
				}
				var step protobuf.SC_29031
				recoveryPacket(t, client, NewEducateTriggerNode, &protobuf.CS_29030{Id: proto.Uint32(role)}, &step)
				if step.GetResult() != 0 {
					t.Fatal(&step)
				}
				node = step.GetNextNode()
			}
		}
	} else {
		var skipped protobuf.SC_29047
		recoveryPacket(t, client, NewEducateScheduleSkip, &protobuf.CS_29046{Id: proto.Uint32(role)}, &skipped)
		if skipped.GetResult() != 0 {
			t.Fatal(&skipped)
		}
	}
	var summary protobuf.SC_29049
	recoveryPacket(t, client, NewEducateGetExtraDrop, &protobuf.CS_29048{Id: proto.Uint32(role)}, &summary)
	if summary.GetResult() != 0 {
		t.Fatal(&summary)
	}
	state, err := loadEducateState(client, role)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func TestRecoveryS04AllRealCourseContractsAndRoundSchedules(t *testing.T) {
	client := recoveryDB(t)
	plans, err := listNewEducateConfigs[newEducatePlanConfig](newEducatePlanCategory)
	if err != nil {
		t.Fatal(err)
	}
	rounds, err := listNewEducateConfigs[newEducateRoundConfig](newEducateRoundCategory)
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 70 || len(rounds) != 80 {
		t.Fatal("configuration snapshot changed; renew scope audit")
	}
	for _, plan := range plans {
		rows, err := parseEducateDropTriplets(plan.ResultDisplay, "plan", fmt.Sprint(plan.ID), "reward")
		if err != nil || len(rows) == 0 {
			t.Fatal(err)
		}
		category := newEducateAttrCategory
		if rows[0][0] == 2 {
			category = newEducateResourceCategory
		}
		owner, ok, err := loadNewEducateConfigByID[educateNumericConfig](category, uint32(rows[0][1]))
		if err != nil || !ok {
			t.Fatal(err)
		}
		state := s04FreshCourseState(t, client, owner.Character, 0, 1)
		contract, err := loadEducateCourseContract(state, &plan)
		if err != nil || len(contract.Nodes) != 2 {
			t.Fatalf("course %d: %v", plan.ID, err)
		}
	}
	sort.Slice(rounds, func(i, j int) bool { return rounds[i].ID < rounds[j].ID })
	cycles := map[string]uint32{}
	for _, round := range rounds {
		actualRound := round.Round
		if round.RoundType == 2 {
			key := fmt.Sprintf("%d/%d", round.Character, round.IsHardMode)
			cycles[key]++
			actualRound = 20 + cycles[key]
		}
		t.Run(fmt.Sprintf("round%d", round.ID), func(t *testing.T) {
			state := s04FreshCourseState(t, client, round.Character, round.IsHardMode, actualRound)
			loaded, ok, err := loadCurrentNewEducateRoundConfig(state.Info)
			if err != nil || !ok || loaded.ID != round.ID {
				t.Fatalf("round mapping: %v", err)
			}
			available, err := educateAvailablePlans(state, loaded)
			if err != nil || len(available) != len(round.PlanGroup) {
				t.Fatalf("availability: %v", err)
			}
			ids := []uint32{}
			for id := range available {
				ids = append(ids, id)
			}
			sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
			s04Arrange(t, client, state, s04Plans(ids[:5]...))
			result := s04Finish(t, client, round.Character, false)
			if !result.Lifecycle.Schedule.SummaryComplete || result.Info.Fsm.GetCurrentNode() != 0 || ensureEducateCache(result.Info).CachePlan[0].GetCurIndex() != 5 {
				t.Fatal("incomplete schedule")
			}
		})
	}
	// Each endless timeline wraps to its first configured wave.
	for _, hard := range []uint32{0, 1} {
		state := s04FreshCourseState(t, client, 2, hard, 31)
		wrapped, ok, err := loadCurrentNewEducateRoundConfig(state.Info)
		state.Info.Round.Round = proto.Uint32(21)
		first, ok2, err2 := loadCurrentNewEducateRoundConfig(state.Info)
		if err != nil || err2 != nil || !ok || !ok2 || first.ID != wrapped.ID {
			t.Fatal("endless wrap mismatch")
		}
	}
}

func TestRecoveryS04PlaybackSkipDiscountCapsAndModes(t *testing.T) {
	client := recoveryDB(t)
	for _, tc := range []struct {
		name                          string
		role, hard, round, plan, buff uint32
		cap, pending, duplicate, temp bool
		fee, moodFee                  int32
	}{
		{"flat-free", 1, 0, 1, 1101, 1078, false, false, false, false, 0, 1},
		{"half-fee", 1, 0, 6, 1107, 1080, false, false, false, false, 6, 2},
		{"ratio-and-flat", 1, 0, 6, 1107, 1080, false, false, true, false, 0, 2},
		{"ratio-and-flat-positive", 1, 0, 6, 1109, 1080, false, false, true, false, 10, 8},
		{"pending", 1, 0, 1, 1101, 1078, false, true, false, false, 0, 1},
		{"pending-ratio", 1, 0, 1, 1101, 1080, false, true, false, false, 3, 1},
		{"cap-upgraded", 1, 0, 6, 1109, 0, true, false, false, false, 40, 8},
		{"hard", 2, 1, 6, 1209, 0, false, false, false, false, 40, 8},
		{"endless", 2, 0, 21, 1209, 0, false, false, false, false, 40, 8},
		{"hard-endless-temp", 2, 1, 31, 1209, 0, true, false, false, true, 40, 8},
		{"role2-discount", 2, 0, 6, 1213, 3240012, false, false, false, false, 12, 1},
		{"role2-odd-mood", 2, 0, 1, 1203, 3240012, false, false, false, false, 6, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var played *educateState
			for _, play := range []bool{true, false} {
				state := s04FreshCourseState(t, client, tc.role, tc.hard, tc.round)
				state.Info.Plan.PlanUpgrade = []uint32{tc.plan}
				if tc.temp {
					state.Info.Round.TempRound = proto.Uint32(2)
					state.Lifecycle.TempRound = 2
				}
				if tc.buff != 0 {
					if tc.role == 1 {
						state.Info.Fsm.SystemNo = proto.Uint32(newEducateSystemTalent)
						ensureEducateCache(state.Info).CacheTalent[0].Talents = []uint32{tc.buff}
						markEducateStage(state, newEducateSystemTalent, false)
						if _, err := selectEducateTalent(state, tc.buff); err != nil {
							t.Fatal(err)
						}
					} else if _, err := deliverEducateBenefit(state, tc.buff, 1); err != nil {
						t.Fatal(err)
					}
					if tc.pending {
						state.Info.Benefit.Actives[0].IsPending = proto.Uint32(1)
					}
					if tc.duplicate {
						state.Info.Benefit.Actives = append(state.Info.Benefit.Actives, proto.Clone(state.Info.Benefit.Actives[0]).(*protobuf.TBBF))
						if _, err := deliverEducateBenefit(state, 1078, 1); err != nil {
							t.Fatal(err)
						}
					}
				}
				if tc.cap {
					for _, v := range state.Info.Res.Attrs {
						cfg, _, err := loadNewEducateConfigByID[educateNumericConfig](newEducateAttrCategory, v.GetKey())
						if err != nil {
							t.Fatal(err)
						}
						v.Value = proto.Uint32(uint32(cfg.Max - 3))
					}
				}
				state.Info.Fsm.SystemNo = proto.Uint32(newEducateSystemMap)
				s04Arrange(t, client, state, s04Plans(tc.plan, tc.plan, tc.plan, tc.plan, tc.plan))
				paid, err := loadEducateState(client, tc.role)
				if err != nil {
					t.Fatal(err)
				}
				if paid.Lifecycle.Schedule.Slots[1].PaidCosts[0][2] != tc.fee {
					t.Fatalf("fee %v wanted %d", paid.Lifecycle.Schedule.Slots[1].PaidCosts, tc.fee)
				}
				if paid.Lifecycle.Schedule.Slots[1].PaidCosts[1][2] != tc.moodFee {
					t.Fatalf("mood fee %v wanted %d", paid.Lifecycle.Schedule.Slots[1].PaidCosts, tc.moodFee)
				}
				result := s04Finish(t, client, tc.role, play)
				if play {
					played = result
				} else {
					if !proto.Equal(played.Info.Res, result.Info.Res) {
						t.Fatal("play/skip resource mismatch")
					}
					for slot := uint32(1); slot <= 5; slot++ {
						a, _ := json.Marshal(played.Lifecycle.Schedule.Slots[slot])
						b, _ := json.Marshal(result.Lifecycle.Schedule.Slots[slot])
						if !bytes.Equal(a, b) {
							t.Fatalf("slot %d settlement mismatch", slot)
						}
					}
				}
			}
		})
	}
}

func TestRecoveryS04CourseContractRejectionIsAtomic(t *testing.T) {
	client := recoveryDB(t)
	state := s04FreshCourseState(t, client, 2, 0, 1)
	if err := saveEducateState(state); err != nil {
		t.Fatal(err)
	}
	plans := s04Plans(1201, 1201, 1201, 1201, 1201)
	plan, _, err := loadNewEducateConfigByID[newEducatePlanConfig](newEducatePlanCategory, 1201)
	if err != nil {
		t.Fatal(err)
	}
	node, _, err := loadNewEducateConfigByID[newEducateNodeConfig](newEducateNodeCategory, plan.ResultNode)
	if err != nil {
		t.Fatal(err)
	}
	original, _ := json.Marshal(node)
	for _, invalid := range []json.RawMessage{json.RawMessage(fmt.Sprint(node.ID)), json.RawMessage("null"), json.RawMessage("99999999")} {
		node.Next = invalid
		raw, _ := json.Marshal(node)
		if err := orm.UpsertConfigEntry(newEducateNodeCategory, fmt.Sprint(node.ID), raw); err != nil {
			t.Fatal(err)
		}
		before, _ := orm.GetCommanderTB(client.Commander.CommanderID, 2)
		var arranged protobuf.SC_29041
		recoveryPacket(t, client, NewEducateSchedule, &protobuf.CS_29040{Id: proto.Uint32(2), Plans: plans}, &arranged)
		after, _ := orm.GetCommanderTB(client.Commander.CommanderID, 2)
		if arranged.GetResult() != 1 || !bytes.Equal(before.State, after.State) || !bytes.Equal(before.Metadata, after.Metadata) || before.Revision != after.Revision {
			t.Fatal("invalid chain charged/mutated schedule")
		}
	}
	if err := orm.UpsertConfigEntry(newEducateNodeCategory, fmt.Sprint(node.ID), original); err != nil {
		t.Fatal(err)
	}
	s04Arrange(t, client, state, plans)
	var next protobuf.SC_29043
	recoveryPacket(t, client, NewEducateNextPlan, &protobuf.CS_29042{Id: proto.Uint32(2)}, &next)
	if next.GetResult() != 0 {
		t.Fatal(&next)
	}
	// Alter an otherwise valid reward contract after payment. Every route must
	// reject it without awarding or changing the paid schedule.
	plan.ResultDisplay = json.RawMessage("[[1,301,6]]")
	raw, _ := json.Marshal(plan)
	if err := orm.UpsertConfigEntry(newEducatePlanCategory, "1201", raw); err != nil {
		t.Fatal(err)
	}
	before, _ := orm.GetCommanderTB(client.Commander.CommanderID, 2)
	var step protobuf.SC_29031
	recoveryPacket(t, client, NewEducateTriggerNode, &protobuf.CS_29030{Id: proto.Uint32(2)}, &step)
	var skip protobuf.SC_29047
	recoveryPacket(t, client, NewEducateScheduleSkip, &protobuf.CS_29046{Id: proto.Uint32(2)}, &skip)
	var clear protobuf.SC_29033
	recoveryPacket(t, client, NewEducateClearNodeChain, &protobuf.CS_29032{Id: proto.Uint32(2)}, &clear)
	after, _ := orm.GetCommanderTB(client.Commander.CommanderID, 2)
	if step.GetResult() != 1 || skip.GetResult() != 1 || clear.GetResult() != 1 || !bytes.Equal(before.State, after.State) || !bytes.Equal(before.Metadata, after.Metadata) || before.Revision != after.Revision {
		t.Fatal("changed paid contract accepted")
	}
	plan.ResultDisplay = json.RawMessage("[[1,301,5]]")
	raw, _ = json.Marshal(plan)
	if err := orm.UpsertConfigEntry(newEducatePlanCategory, "1201", raw); err != nil {
		t.Fatal(err)
	}
	// A forged current node, future settlement or noncontiguous cache must not
	// let skip turn a broken instance into a successful payout.
	for _, mutate := range []func(*educateState){
		func(s *educateState) {
			s.Info.Fsm.CurrentNode = proto.Uint32(3700003)
			s.Lifecycle.Schedule.Slots[1].Node = 3700003
		},
		func(s *educateState) { s.Lifecycle.Schedule.Slots[2].Status = "settled" },
		func(s *educateState) { ensureEducateCache(s.Info).CachePlan[0].Plans[1].Key = proto.Uint32(1) },
	} {
		state, err := loadEducateState(client, 2)
		if err != nil {
			t.Fatal(err)
		}
		mutate(state)
		if err := saveEducateState(state); err != nil {
			t.Fatal(err)
		}
		before, _ := orm.GetCommanderTB(client.Commander.CommanderID, 2)
		recoveryPacket(t, client, NewEducateScheduleSkip, &protobuf.CS_29046{Id: proto.Uint32(2)}, &skip)
		after, _ := orm.GetCommanderTB(client.Commander.CommanderID, 2)
		if skip.GetResult() != 1 || !bytes.Equal(before.State, after.State) || !bytes.Equal(before.Metadata, after.Metadata) || before.Revision != after.Revision {
			t.Fatal("invalid course instance settled")
		}
		// Restore the same known paid state for the next independent corruption.
		info, permanent, err := before.Decode()
		if err != nil {
			t.Fatal(err)
		}
		info.Fsm.CurrentNode = next.FirstNode
		ensureEducateCache(info).CachePlan[0].Plans = plans
		state.Info, state.Permanent = info, permanent
		state.Lifecycle.Schedule.Slots[1].Node = next.GetFirstNode()
		state.Lifecycle.Schedule.Slots[2].Status = "pending"
		if err := saveEducateState(state); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRecoveryS04ChoiceCannotBeSkippedAcross(t *testing.T) {
	client := recoveryDB(t)
	// Isolated schema fixture for a course-triggered choice, reusing the real
	// role/slot condition and the restored pool 1000 delivery contract.
	benefit, ok, err := loadNewEducateConfigByID[educateBenefitConfig]("ShareCfg/child2_benefit.json", 32400031)
	if err != nil || !ok {
		t.Fatal(err)
	}
	benefit.Effect = [][]json.RawMessage{{json.RawMessage("1"), json.RawMessage("[5,1000,1]")}}
	raw, _ := json.Marshal(benefit)
	if err := orm.UpsertConfigEntry("ShareCfg/child2_benefit.json", "32400031", raw); err != nil {
		t.Fatal(err)
	}
	state := s04FreshCourseState(t, client, 2, 0, 1)
	if _, err := deliverEducateBenefit(state, 3240012, 1); err != nil {
		t.Fatal(err)
	}
	s04Arrange(t, client, state, s04Plans(1203, 1203, 1203, 1203, 1203))
	before, _ := orm.GetCommanderTB(client.Commander.CommanderID, 2)
	var skip protobuf.SC_29047
	recoveryPacket(t, client, NewEducateScheduleSkip, &protobuf.CS_29046{Id: proto.Uint32(2)}, &skip)
	after, _ := orm.GetCommanderTB(client.Commander.CommanderID, 2)
	if skip.GetResult() != 1 || !bytes.Equal(before.State, after.State) || !bytes.Equal(before.Metadata, after.Metadata) || before.Revision != after.Revision {
		t.Fatal("skip crossed choice or leaked first reward")
	}
	var next protobuf.SC_29043
	recoveryPacket(t, client, NewEducateNextPlan, &protobuf.CS_29042{Id: proto.Uint32(2)}, &next)
	if next.GetResult() != 0 {
		t.Fatal(&next)
	}
	for i := 0; i < 2; i++ {
		var step protobuf.SC_29031
		recoveryPacket(t, client, NewEducateTriggerNode, &protobuf.CS_29030{Id: proto.Uint32(2)}, &step)
		if step.GetResult() != 0 {
			t.Fatal(&step)
		}
	}
	state, err = loadEducateState(client, 2)
	if err != nil {
		t.Fatal(err)
	}
	if educateKVCount(state.Info.Res.Attrs, 302) != 5 || len(state.Info.Fsm.PriorityFsm) != 1 || state.Lifecycle.Schedule.Slots[1].Status != "settled" || state.Lifecycle.Schedule.Slots[2].Status != "pending" {
		t.Fatal("playback lost choice boundary")
	}
	before, _ = orm.GetCommanderTB(client.Commander.CommanderID, 2)
	recoveryPacket(t, client, NewEducateScheduleSkip, &protobuf.CS_29046{Id: proto.Uint32(2)}, &skip)
	recoveryPacket(t, client, NewEducateNextPlan, &protobuf.CS_29042{Id: proto.Uint32(2)}, &next)
	after, _ = orm.GetCommanderTB(client.Commander.CommanderID, 2)
	if skip.GetResult() != 1 || next.GetResult() != 1 || !bytes.Equal(before.State, after.State) || !bytes.Equal(before.Metadata, after.Metadata) || before.Revision != after.Revision {
		t.Fatal("pending choice advanced a course")
	}
}
