package neweducate

import (
	"bytes"
	"testing"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestRecoveryS04ScheduleGuards(t *testing.T) {
	client := recoveryDB(t)
	state, err := loadEducateState(client, 2)
	if err != nil {
		t.Fatal(err)
	}
	state.Info.Fsm.SystemNo = proto.Uint32(newEducateSystemMap)
	if err := saveEducateState(state); err != nil {
		t.Fatal(err)
	}
	plans := []*protobuf.KVDATA{}
	for i, id := range []uint32{1201, 1203, 1202, 1204, 1206} {
		plans = append(plans, &protobuf.KVDATA{Key: proto.Uint32(uint32(i + 1)), Value: proto.Uint32(id)})
	}
	unchanged := func(before, after *orm.CommanderTB) {
		t.Helper()
		if !bytes.Equal(before.State, after.State) || !bytes.Equal(before.Permanent, after.Permanent) || !bytes.Equal(before.Metadata, after.Metadata) || before.Revision != after.Revision {
			t.Fatal("failed schedule mutated save")
		}
	}
	reject := func(rows []*protobuf.KVDATA) {
		t.Helper()
		before, err := orm.GetCommanderTB(client.Commander.CommanderID, 2)
		if err != nil {
			t.Fatal(err)
		}
		var response protobuf.SC_29041
		recoveryPacket(t, client, NewEducateSchedule, &protobuf.CS_29040{Id: proto.Uint32(2), Plans: rows}, &response)
		if response.GetResult() != 1 {
			t.Fatal("invalid schedule accepted")
		}
		after, err := orm.GetCommanderTB(client.Commander.CommanderID, 2)
		if err != nil {
			t.Fatal(err)
		}
		unchanged(before, after)
	}
	reject(plans[:4])
	duplicate := make([]*protobuf.KVDATA, len(plans))
	copy(duplicate, plans)
	duplicate[1] = &protobuf.KVDATA{Key: proto.Uint32(1), Value: proto.Uint32(1203)}
	reject(duplicate)
	foreign := make([]*protobuf.KVDATA, len(plans))
	copy(foreign, plans)
	foreign[0] = &protobuf.KVDATA{Key: proto.Uint32(1), Value: proto.Uint32(1101)}
	reject(foreign)
	// Insufficient funds: metadata and revision must be unchanged too.
	state, err = loadEducateState(client, 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range state.Info.Res.Resource {
		if v.GetKey() == 301 {
			v.Value = proto.Uint32(0)
		}
	}
	if err := saveEducateState(state); err != nil {
		t.Fatal(err)
	}
	reject(plans)
	for _, v := range state.Info.Res.Resource {
		if v.GetKey() == 301 {
			v.Value = proto.Uint32(50)
		}
	}
	state.Info.Fsm.SystemNo = proto.Uint32(newEducateSystemTopic)
	markEducateStage(state, newEducateSystemTopic, true)
	markEducateStage(state, newEducateSystemMap, true)
	if err := saveEducateState(state); err != nil {
		t.Fatal(err)
	}
	var response protobuf.SC_29041
	recoveryPacket(t, client, NewEducateSchedule, &protobuf.CS_29040{Id: proto.Uint32(2), Plans: plans}, &response)
	if response.GetResult() != 0 {
		t.Fatal("valid schedule rejected")
	}
	reject(plans) // resend cannot charge again.
	after, err := loadEducateState(client, 2)
	if err != nil {
		t.Fatal(err)
	}
	if educateKVCount(after.Info.Res.Resource, 301) != 26 || educateKVCount(after.Info.Res.Resource, 302) != 46 {
		t.Fatal("real course cost mismatch")
	}
}

func TestRecoveryS04SkipBoundaries(t *testing.T) {
	client := recoveryDB(t)
	plans := []*protobuf.KVDATA{}
	for i, id := range []uint32{1201, 1203, 1202, 1204, 1206} {
		plans = append(plans, &protobuf.KVDATA{Key: proto.Uint32(uint32(i + 1)), Value: proto.Uint32(id)})
	}
	for _, mode := range []string{"unstarted", "playing", "first-settled"} {
		t.Run(mode, func(t *testing.T) {
			if _, err := orm.DeleteCommanderTB(client.Commander.CommanderID, 2); err != nil {
				t.Fatal(err)
			}
			state, err := loadEducateState(client, 2)
			if err != nil {
				t.Fatal(err)
			}
			state.Info.Fsm.SystemNo = proto.Uint32(newEducateSystemMap)
			if err := saveEducateState(state); err != nil {
				t.Fatal(err)
			}
			var schedule protobuf.SC_29041
			recoveryPacket(t, client, NewEducateSchedule, &protobuf.CS_29040{Id: proto.Uint32(2), Plans: plans}, &schedule)
			if schedule.GetResult() != 0 {
				t.Fatal("schedule")
			}
			if mode != "unstarted" {
				var next protobuf.SC_29043
				recoveryPacket(t, client, NewEducateNextPlan, &protobuf.CS_29042{Id: proto.Uint32(2)}, &next)
				if next.GetResult() != 0 {
					t.Fatal("next")
				}
				if mode == "first-settled" {
					node := next.GetFirstNode()
					for step := 0; node != 0; step++ {
						if step > 20 {
							t.Fatal("chain bound")
						}
						var response protobuf.SC_29031
						recoveryPacket(t, client, NewEducateTriggerNode, &protobuf.CS_29030{Id: proto.Uint32(2), Branch: proto.Uint32(0)}, &response)
						if response.GetResult() != 0 {
							t.Fatal("node")
						}
						node = response.GetNextNode()
					}
				}
			}
			var skip protobuf.SC_29047
			recoveryPacket(t, client, NewEducateScheduleSkip, &protobuf.CS_29046{Id: proto.Uint32(2)}, &skip)
			if skip.GetResult() != 0 {
				t.Fatal("skip")
			}
			state, err = loadEducateState(client, 2)
			if err != nil {
				t.Fatal(err)
			}
			for _, id := range []uint32{301, 302, 303, 304} {
				if educateKVCount(state.Info.Res.Attrs, id) != 5 {
					t.Fatalf("mode %s attribute %d repeated/missing", mode, id)
				}
			}
			if educateKVCount(state.Info.Res.Resource, 301) != 34 || educateKVCount(state.Info.Res.Resource, 302) != 46 || state.Info.Fsm.GetCurrentNode() != 0 {
				t.Fatal("skip results")
			}
			before, err := orm.GetCommanderTB(client.Commander.CommanderID, 2)
			if err != nil {
				t.Fatal(err)
			}
			recoveryPacket(t, client, NewEducateScheduleSkip, &protobuf.CS_29046{Id: proto.Uint32(2)}, &skip)
			if skip.GetResult() != 1 {
				t.Fatal("repeat skip accepted")
			}
			after, err := orm.GetCommanderTB(client.Commander.CommanderID, 2)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before.State, after.State) || !bytes.Equal(before.Permanent, after.Permanent) || !bytes.Equal(before.Metadata, after.Metadata) || before.Revision != after.Revision {
				t.Fatal("repeat skip changed save")
			}
			var summary protobuf.SC_29049
			recoveryPacket(t, client, NewEducateGetExtraDrop, &protobuf.CS_29048{Id: proto.Uint32(2)}, &summary)
			if summary.GetResult() != 0 {
				t.Fatal("summary")
			}
			before, err = orm.GetCommanderTB(client.Commander.CommanderID, 2)
			if err != nil {
				t.Fatal(err)
			}
			recoveryPacket(t, client, NewEducateGetExtraDrop, &protobuf.CS_29048{Id: proto.Uint32(2)}, &summary)
			after, err = orm.GetCommanderTB(client.Commander.CommanderID, 2)
			if err != nil {
				t.Fatal(err)
			}
			if summary.GetResult() != 0 || len(summary.GetDrop().GetBaseDrop()) != 0 || !bytes.Equal(before.State, after.State) || !bytes.Equal(before.Metadata, after.Metadata) || before.Revision != after.Revision {
				t.Fatal("repeated summary changed save or delivered extra twice")
			}
			state, err = loadEducateState(client, 2)
			if err != nil {
				t.Fatal(err)
			}
			if state.Lifecycle.Schedule == nil || !state.Lifecycle.Schedule.SummaryComplete || !state.Lifecycle.Schedule.ExtraComplete {
				t.Fatal("summary marker missing on reload")
			}
			for _, slot := range state.Lifecycle.Schedule.Slots {
				if slot.Status != "settled" || slot.Node != 0 {
					t.Fatal("slot marker not persisted")
				}
			}
		})
	}
}

func TestRecoveryS04UpgradeThreshold(t *testing.T) {
	client := recoveryDB(t)
	state, err := loadEducateState(client, 2)
	if err != nil {
		t.Fatal(err)
	}
	state.Info.Round.Round = proto.Uint32(6)
	state.Lifecycle = freshEducateLifecycle(state.Info)
	state.Info.Fsm.SystemNo = proto.Uint32(newEducateSystemMap)
	for _, v := range state.Info.Res.Attrs {
		switch v.GetKey() {
		case 301:
			v.Value = proto.Uint32(300)
		case 302:
			v.Value = proto.Uint32(299)
		}
	}
	if err := saveEducateState(state); err != nil {
		t.Fatal(err)
	}
	before, err := orm.GetCommanderTB(client.Commander.CommanderID, 2)
	if err != nil {
		t.Fatal(err)
	}
	var response protobuf.SC_29045
	recoveryPacket(t, client, NewEducateUpgradePlan, &protobuf.CS_29044{Id: proto.Uint32(2), PlanIds: []uint32{1207}}, &response)
	if response.GetResult() != 1 {
		t.Fatal("upgrade below actual 600 sum gate")
	}
	after, err := orm.GetCommanderTB(client.Commander.CommanderID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before.State, after.State) || !bytes.Equal(before.Metadata, after.Metadata) || before.Revision != after.Revision {
		t.Fatal("failed upgrade mutated save")
	}
	for _, v := range state.Info.Res.Attrs {
		if v.GetKey() == 302 {
			v.Value = proto.Uint32(300)
		}
	}
	if err := saveEducateState(state); err != nil {
		t.Fatal(err)
	}
	recoveryPacket(t, client, NewEducateUpgradePlan, &protobuf.CS_29044{Id: proto.Uint32(2), PlanIds: []uint32{1207}}, &response)
	if response.GetResult() != 0 {
		t.Fatal("upgrade at real gate")
	}
	state, err = loadEducateState(client, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Info.Plan.PlanUpgrade) != 1 {
		t.Fatal("missing unlocked course")
	}
	plan, ok, err := loadNewEducateConfigByID[newEducatePlanConfig](newEducatePlanCategory, state.Info.Plan.PlanUpgrade[0])
	if err != nil || !ok || plan.GroupID != 107 || plan.Level != 2 {
		t.Fatalf("stored wrong upgraded level: %v %v", plan, err)
	}
	recoveryPacket(t, client, NewEducateUpgradePlan, &protobuf.CS_29044{Id: proto.Uint32(2), PlanIds: []uint32{1207}}, &response)
	if response.GetResult() != 1 {
		t.Fatal("old-level repeat upgraded again")
	}
}
