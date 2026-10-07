package neweducate

import (
	"bytes"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
	"testing"
)

func TestEducateWeightedTalentIntervals(t *testing.T) {
	pool := []educateTalentWeight{{1001, 2}, {1002, 3}, {1003, 1}}
	for _, tc := range []struct {
		roll uint64
		want uint32
	}{{0, 1001}, {1, 1001}, {2, 1002}, {4, 1002}, {5, 1003}} {
		got, err := drawEducateTalents(pool, nil, 1, func(total uint64) (uint64, error) {
			if total != 6 {
				t.Fatalf("weight total %d", total)
			}
			return tc.roll, nil
		})
		if err != nil || got[0] != tc.want {
			t.Fatalf("draw %d got %v %v", tc.roll, got, err)
		}
	}
	got, err := drawEducateTalents(pool, []uint32{1002}, 2, func(uint64) (uint64, error) { return 0, nil })
	if err != nil || len(got) != 2 || got[0] != 1001 || got[1] != 1003 {
		t.Fatal(got, err)
	}
	if _, err := drawEducateTalents(pool, []uint32{1001, 1002, 1003}, 1, educateDraw); err == nil {
		t.Fatal("exhausted pool accepted")
	}
}

func TestRecoveryS06ImmediateTalentOnlyAtAcquisition(t *testing.T) {
	client := recoveryDB(t)
	if _, err := loadEducateState(client, 1); err != nil {
		t.Fatal(err)
	}
	otherBefore, err := orm.GetCommanderTB(client.Commander.CommanderID, 1)
	if err != nil {
		t.Fatal(err)
	}
	state, err := loadEducateState(client, 2)
	if err != nil {
		t.Fatal(err)
	}
	state.Info.Fsm.SystemNo = proto.Uint32(newEducateSystemTalent)
	ensureEducateCache(state.Info).CacheTalent[0].Talents = []uint32{3830004}
	markEducateStage(state, newEducateSystemTalent, false)
	initial := proto.Clone(state.Info.Res).(*protobuf.TBRES)
	if err := saveEducateState(state); err != nil {
		t.Fatal(err)
	}
	var selected protobuf.SC_29024
	recoveryPacket(t, client, NewEducateSelectTalent, &protobuf.CS_29023{Id: proto.Uint32(2), Talent: proto.Uint32(3830004)}, &selected)
	state, err = loadEducateState(client, 2)
	if err != nil {
		t.Fatal(err)
	}
	if selected.GetResult() != 0 || len(selected.Drop.BenefitDrop) != 4 {
		t.Fatal("immediate talent reward", &selected)
	}
	for _, id := range []uint32{301, 302, 303, 304} {
		if educateKVCount(state.Info.Res.Attrs, id) != educateKVCount(initial.Attrs, id)+200 {
			t.Fatal("missing acquisition attribute", id)
		}
	}
	before, err := orm.GetCommanderTB(client.Commander.CommanderID, 2)
	if err != nil {
		t.Fatal(err)
	}
	recoveryPacket(t, client, NewEducateSelectTalent, &protobuf.CS_29023{Id: proto.Uint32(2), Talent: proto.Uint32(3830004)}, &selected)
	after, err := orm.GetCommanderTB(client.Commander.CommanderID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if selected.GetResult() != 1 || !bytes.Equal(before.State, after.State) || !bytes.Equal(before.Metadata, after.Metadata) || before.Revision != after.Revision {
		t.Fatal("repeat acquisition paid twice")
	}
	resources := proto.Clone(state.Info.Res)
	for _, trigger := range []uint32{2, 5} {
		if _, err := applyEducateTalentTrigger(state, trigger); err != nil {
			t.Fatal(err)
		}
	}
	if !proto.Equal(resources, state.Info.Res) {
		t.Fatal("instant reward repeated at future trigger")
	}
	otherAfter, err := orm.GetCommanderTB(client.Commander.CommanderID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(otherBefore.State, otherAfter.State) || !bytes.Equal(otherBefore.Permanent, otherAfter.Permanent) || !bytes.Equal(otherBefore.Metadata, otherAfter.Metadata) || otherBefore.Revision != otherAfter.Revision {
		t.Fatal("instant reward changed other character")
	}
}

func TestRecoveryS06TalentReloadRefreshAndRoundStart(t *testing.T) {
	client := recoveryDB(t)
	recoveryPacket(t, client, NewEducateRequest, &protobuf.CS_29001{Id: proto.Uint32(1)}, &protobuf.SC_29002{})
	recoveryPacket(t, client, NewEducateRequest, &protobuf.CS_29001{Id: proto.Uint32(2)}, &protobuf.SC_29002{})
	otherBefore, err := orm.GetCommanderTB(90001, 2)
	if err != nil {
		t.Fatal(err)
	}
	state, err := loadEducateState(client, 1)
	if err != nil {
		t.Fatal(err)
	}
	recoveryPacket(t, client, NewEducateMainEvent, &protobuf.CS_29011{Id: proto.Uint32(1)}, &protobuf.SC_29012{})
	state, err = loadEducateState(client, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := loadEducateTalents(state, func(uint64) (uint64, error) { return 0, nil }); err != nil {
		t.Fatal(err)
	}
	if err := saveEducateState(state); err != nil {
		t.Fatal(err)
	}
	expected := append([]uint32{}, state.Info.Fsm.Cache[0].CacheTalent[0].Talents...)
	if len(expected) != 3 || expected[0] != 1001 {
		t.Fatal("real weighted pool", expected)
	}
	var listed protobuf.SC_29020
	recoveryPacket(t, client, NewEducateGetTalents, &protobuf.CS_29019{Id: proto.Uint32(1)}, &listed)
	if listed.GetResult() != 0 || !proto.Equal(&protobuf.SC_29020{Result: proto.Uint32(0), Talents: expected}, &listed) {
		t.Fatal("candidates rerolled", &listed)
	}
	var changed protobuf.SC_29022
	recoveryPacket(t, client, NewEducateRefreshTalent, &protobuf.CS_29021{Id: proto.Uint32(1), Talent: proto.Uint32(expected[1])}, &changed)
	if changed.GetResult() != 0 || changed.GetTalent() == expected[1] {
		t.Fatal("refresh", &changed)
	}
	before, err := orm.GetCommanderTB(90001, 1)
	if err != nil {
		t.Fatal(err)
	}
	var repeated protobuf.SC_29022
	recoveryPacket(t, client, NewEducateRefreshTalent, &protobuf.CS_29021{Id: proto.Uint32(1), Talent: changed.Talent}, &repeated)
	var invalid protobuf.SC_29024
	recoveryPacket(t, client, NewEducateSelectTalent, &protobuf.CS_29023{Id: proto.Uint32(1), Talent: proto.Uint32(3830001)}, &invalid)
	after, err := orm.GetCommanderTB(90001, 1)
	if err != nil {
		t.Fatal(err)
	}
	if repeated.GetResult() != 1 || invalid.GetResult() != 1 || !bytes.Equal(before.State, after.State) || !bytes.Equal(before.Metadata, after.Metadata) || before.Revision != after.Revision {
		t.Fatal("failed action mutated candidate state")
	}
	var selected protobuf.SC_29024
	recoveryPacket(t, client, NewEducateSelectTalent, &protobuf.CS_29023{Id: proto.Uint32(1), Talent: proto.Uint32(1001)}, &selected)
	state, err = loadEducateState(client, 1)
	if err != nil {
		t.Fatal(err)
	}
	if selected.GetResult() != 0 || len(state.Info.Benefit.Actives) != 1 || len(state.Permanent.TarotArchive) != 1 || educateKVCount(state.Info.Res.Resource, 2) != 50 {
		t.Fatal("acquisition must not run round-start reward", &selected)
	}
	// Isolated state fixture for the round boundary; no player database writes.
	state.Info.Fsm.SystemNo = proto.Uint32(newEducateSystemAssess)
	markEducateStage(state, newEducateSystemAssess, true)
	if err := saveEducateState(state); err != nil {
		t.Fatal(err)
	}
	var advanced protobuf.SC_29026
	recoveryPacket(t, client, NewEducateChangePhase, &protobuf.CS_29025{Id: proto.Uint32(1)}, &advanced)
	state, err = loadEducateState(client, 1)
	if err != nil {
		t.Fatal(err)
	}
	if advanced.GetResult() != 0 || state.Info.Round.GetRound() != 2 || educateKVCount(state.Info.Res.Resource, 2) != 51 || len(advanced.Drop.BenefitDrop) != 1 || advanced.Drop.BenefitDrop[0].GetNumber() != 1 {
		t.Fatal("round-start actual difference", &advanced, state.Info.Res)
	}
	before, err = orm.GetCommanderTB(90001, 1)
	if err != nil {
		t.Fatal(err)
	}
	recoveryPacket(t, client, NewEducateChangePhase, &protobuf.CS_29025{Id: proto.Uint32(1)}, &advanced)
	after, err = orm.GetCommanderTB(90001, 1)
	if err != nil {
		t.Fatal(err)
	}
	if advanced.GetResult() != 1 || !bytes.Equal(before.State, after.State) || before.Revision != after.Revision {
		t.Fatal("repeat round paid twice")
	}
	otherAfter, err := orm.GetCommanderTB(90001, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(otherBefore.State, otherAfter.State) || !bytes.Equal(otherBefore.Metadata, otherAfter.Metadata) || !bytes.Equal(otherBefore.Permanent, otherAfter.Permanent) || otherBefore.Revision != otherAfter.Revision {
		t.Fatal("talent changed other character")
	}
}

func TestRecoveryS06AdditiveCourseTalentPlaybackSkip(t *testing.T) {
	client := recoveryDB(t)
	for _, playFirst := range []bool{false, true} {
		// Reset only this test's isolated schema fixture, never the player DB.
		if _, err := orm.DeleteCommanderTB(client.Commander.CommanderID, 1); err != nil {
			t.Fatal(err)
		}
		state, err := loadEducateState(client, 1)
		if err != nil {
			t.Fatal(err)
		}
		state.Info.Fsm.SystemNo = proto.Uint32(newEducateSystemTalent)
		ensureEducateCache(state.Info).CacheTalent[0].Talents = []uint32{1011}
		markEducateStage(state, newEducateSystemTalent, false)
		if _, err := selectEducateTalent(state, 1011); err != nil {
			t.Fatal(err)
		}
		if educateKVCount(state.Info.Res.Attrs, 103) != 0 {
			t.Fatal("course reward granted at acquisition")
		}
		state.Info.Fsm.SystemNo = proto.Uint32(newEducateSystemMap)
		if err := saveEducateState(state); err != nil {
			t.Fatal(err)
		}
		plans := []*protobuf.KVDATA{}
		for i, id := range []uint32{1101, 1103, 1102, 1104, 1106} {
			plans = append(plans, &protobuf.KVDATA{Key: proto.Uint32(uint32(i + 1)), Value: proto.Uint32(id)})
		}
		var scheduled protobuf.SC_29041
		recoveryPacket(t, client, NewEducateSchedule, &protobuf.CS_29040{Id: proto.Uint32(1), Plans: plans}, &scheduled)
		if scheduled.GetResult() != 0 {
			t.Fatal("schedule", &scheduled)
		}
		if playFirst {
			var first protobuf.SC_29043
			recoveryPacket(t, client, NewEducateNextPlan, &protobuf.CS_29042{Id: proto.Uint32(1)}, &first)
			node := first.GetFirstNode()
			for step := 0; node != 0; step++ {
				if step > 20 {
					t.Fatal("course did not end")
				}
				var next protobuf.SC_29031
				recoveryPacket(t, client, NewEducateTriggerNode, &protobuf.CS_29030{Id: proto.Uint32(1), Branch: proto.Uint32(0)}, &next)
				if next.GetResult() != 0 {
					t.Fatal("course", &next)
				}
				node = next.GetNextNode()
				if node == 0 && (len(next.Drop.BenefitDrop) != 1 || next.Drop.BenefitDrop[0].GetNumber() != 2) {
					t.Fatal("missing per-course bonus", &next)
				}
			}
		}
		var skipped protobuf.SC_29047
		recoveryPacket(t, client, NewEducateScheduleSkip, &protobuf.CS_29046{Id: proto.Uint32(1)}, &skipped)
		want := 5
		if playFirst {
			want = 4
		}
		if skipped.GetResult() != 0 || len(skipped.Drop.BenefitDrop) != want {
			t.Fatal("skip bonus", &skipped)
		}
		state, err = loadEducateState(client, 1)
		if err != nil {
			t.Fatal(err)
		}
		if educateKVCount(state.Info.Res.Attrs, 103) != 15 || educateKVCount(state.Info.Res.Attrs, 101) != 5 || educateKVCount(state.Info.Res.Resource, 1) != 34 {
			t.Fatal("play/skip totals differ", state.Info.Res)
		}
		for _, slot := range state.Lifecycle.Schedule.Slots {
			if len(slot.BenefitRewards) != 1 || slot.BenefitRewards[0].GetNumber() != 2 {
				t.Fatal("bonus missing after reload")
			}
		}
		before, err := orm.GetCommanderTB(client.Commander.CommanderID, 1)
		if err != nil {
			t.Fatal(err)
		}
		recoveryPacket(t, client, NewEducateScheduleSkip, &protobuf.CS_29046{Id: proto.Uint32(1)}, &skipped)
		after, err := orm.GetCommanderTB(client.Commander.CommanderID, 1)
		if err != nil {
			t.Fatal(err)
		}
		if skipped.GetResult() != 1 || !bytes.Equal(before.State, after.State) || before.Revision != after.Revision {
			t.Fatal("bonus paid twice")
		}
	}
}
