package neweducate

import (
	"bytes"
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
	"net"
	"testing"
)

func TestRecoveryS04CommittedRewardSurvivesFailedDelivery(t *testing.T) {
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
	for i := uint32(1); i <= 5; i++ {
		plans = append(plans, &protobuf.KVDATA{Key: proto.Uint32(i), Value: proto.Uint32(1206)})
	}
	var arranged protobuf.SC_29041
	recoveryPacket(t, client, NewEducateSchedule, &protobuf.CS_29040{Id: proto.Uint32(2), Plans: plans}, &arranged)
	if arranged.GetResult() != 0 {
		t.Fatal("schedule failed")
	}
	var next protobuf.SC_29043
	recoveryPacket(t, client, NewEducateNextPlan, &protobuf.CS_29042{Id: proto.Uint32(2)}, &next)
	if next.GetResult() != 0 {
		t.Fatal("next failed")
	}
	var step protobuf.SC_29031
	recoveryPacket(t, client, NewEducateTriggerNode, &protobuf.CS_29030{Id: proto.Uint32(2)}, &step)
	if step.GetResult() != 0 || step.GetNextNode() == 0 {
		t.Fatal("performance failed")
	}
	// A real socket write fails after the reward transaction has committed and
	// after the handler has queued the response, exactly as in the dispatcher.
	request, err := proto.Marshal(&protobuf.CS_29030{Id: proto.Uint32(2)})
	if err != nil {
		t.Fatal(err)
	}
	client.Buffer.Reset()
	if _, _, err := NewEducateTriggerNode(&request, client); err != nil {
		t.Fatal(err)
	}
	left, right := net.Pipe()
	if err := right.Close(); err != nil {
		t.Fatal(err)
	}
	var wire net.Conn = left
	client.Connection = &wire
	if err := client.Flush(); err == nil {
		t.Fatal("socket delivery did not fail")
	}
	if !client.IsClosed() {
		t.Fatal("failed transport remained open")
	}
	resumed := &connection.Client{Commander: client.Commander}
	var snapshot protobuf.SC_29002
	recoveryPacket(t, resumed, NewEducateRequest, &protobuf.CS_29001{Id: proto.Uint32(2)}, &snapshot)
	if snapshot.GetTb().GetFsm().GetCurrentNode() != 0 || educateKVCount(snapshot.GetTb().GetRes().GetResource(), 301) != 58 {
		t.Fatal("snapshot lost committed course reward")
	}
	before, err := orm.GetCommanderTB(client.Commander.CommanderID, 2)
	if err != nil {
		t.Fatal(err)
	}
	recoveryPacket(t, resumed, NewEducateTriggerNode, &protobuf.CS_29030{Id: proto.Uint32(2)}, &step)
	after, err := orm.GetCommanderTB(client.Commander.CommanderID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if step.GetResult() != 1 || !bytes.Equal(before.State, after.State) || !bytes.Equal(before.Metadata, after.Metadata) || before.Revision != after.Revision {
		t.Fatal("retry repeated committed reward")
	}
}

func TestRecoveryS04ClearReplaysUnsettledCourse(t *testing.T) {
	for _, slot := range []uint32{1, 3, 5} {
		t.Run(string(rune('0'+slot)), func(t *testing.T) {
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
			var arranged protobuf.SC_29041
			recoveryPacket(t, client, NewEducateSchedule, &protobuf.CS_29040{Id: proto.Uint32(2), Plans: plans}, &arranged)
			if arranged.GetResult() != 0 {
				t.Fatal("schedule failed")
			}
			for i := uint32(1); i <= slot; i++ {
				var next protobuf.SC_29043
				recoveryPacket(t, client, NewEducateNextPlan, &protobuf.CS_29042{Id: proto.Uint32(2)}, &next)
				if next.GetResult() != 0 {
					t.Fatal("next failed")
				}
				if i < slot {
					for j := 0; j < 2; j++ {
						var node protobuf.SC_29031
						recoveryPacket(t, client, NewEducateTriggerNode, &protobuf.CS_29030{Id: proto.Uint32(2)}, &node)
						if node.GetResult() != 0 {
							t.Fatal("node failed")
						}
					}
				}
			}
			// Also test recovery at the drop node, before its atomic reward commit.
			var played protobuf.SC_29031
			recoveryPacket(t, client, NewEducateTriggerNode, &protobuf.CS_29030{Id: proto.Uint32(2)}, &played)
			if played.GetResult() != 0 || played.GetNextNode() == 0 {
				t.Fatal("performance failed")
			}
			before, err := loadEducateState(client, 2)
			if err != nil {
				t.Fatal(err)
			}
			var cleared protobuf.SC_29033
			recoveryPacket(t, client, NewEducateClearNodeChain, &protobuf.CS_29032{Id: proto.Uint32(2)}, &cleared)
			if cleared.GetResult() != 0 || cleared.Fsm.GetCurrentNode() != 0 {
				t.Fatal("clear failed")
			}
			after, err := loadEducateState(client, 2)
			if err != nil {
				t.Fatal(err)
			}
			if !proto.Equal(before.Info.Res, after.Info.Res) || ensureEducateCache(after.Info).CachePlan[0].GetCurIndex() != slot-1 {
				t.Fatal("clear changed resources or wrong cursor")
			}
			if after.Lifecycle.Schedule.Slots[slot].Restarts != 1 || after.Lifecycle.Schedule.Slots[slot].Status != "pending" {
				t.Fatal("restart not persisted")
			}
			rawBefore, err := orm.GetCommanderTB(client.Commander.CommanderID, 2)
			if err != nil {
				t.Fatal(err)
			}
			recoveryPacket(t, client, NewEducateClearNodeChain, &protobuf.CS_29032{Id: proto.Uint32(2)}, &cleared)
			rawAfter, err := orm.GetCommanderTB(client.Commander.CommanderID, 2)
			if err != nil {
				t.Fatal(err)
			}
			if cleared.GetResult() != 1 || !bytes.Equal(rawBefore.State, rawAfter.State) || !bytes.Equal(rawBefore.Metadata, rawAfter.Metadata) || rawBefore.Revision != rawAfter.Revision {
				t.Fatal("repeated clear changed save")
			}
			var next protobuf.SC_29043
			recoveryPacket(t, client, NewEducateNextPlan, &protobuf.CS_29042{Id: proto.Uint32(2)}, &next)
			if next.GetResult() != 0 {
				t.Fatal("replay did not restart")
			}
			for j := 0; j < 2; j++ {
				var node protobuf.SC_29031
				recoveryPacket(t, client, NewEducateTriggerNode, &protobuf.CS_29030{Id: proto.Uint32(2)}, &node)
				if node.GetResult() != 0 {
					t.Fatal("replay failed")
				}
			}
			var skip protobuf.SC_29047
			if slot < 5 {
				recoveryPacket(t, client, NewEducateScheduleSkip, &protobuf.CS_29046{Id: proto.Uint32(2)}, &skip)
				if skip.GetResult() != 0 {
					t.Fatal("remaining skip failed")
				}
			}
			final, err := loadEducateState(client, 2)
			if err != nil {
				t.Fatal(err)
			}
			for _, id := range []uint32{301, 302, 303, 304} {
				if educateKVCount(final.Info.Res.Attrs, id) != 5 {
					t.Fatal("lost/double award", id)
				}
			}
			if educateKVCount(final.Info.Res.Resource, 301) != 34 || educateKVCount(final.Info.Res.Resource, 302) != 46 {
				t.Fatal("lost/double payment or award")
			}
		})
	}
}
