package neweducate

import (
	"bytes"
	"fmt"
	"net"
	"testing"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestRecoveryS03ScheduledCountBonusProtocol(t *testing.T) {
	client := recoveryDB(t)
	if _, err := loadEducateState(client, 2); err != nil {
		t.Fatal(err)
	}
	other, err := orm.GetCommanderTB(client.Commander.CommanderID, 2)
	if err != nil {
		t.Fatal(err)
	}
	getRole := func(t *testing.T) *orm.CommanderTB {
		t.Helper()
		row, err := orm.GetCommanderTB(client.Commander.CommanderID, 1)
		if err != nil {
			t.Fatal(err)
		}
		return row
	}
	for _, tc := range []struct {
		count     int
		play, cap bool
	}{{0, false, false}, {1, false, false}, {2, true, false}, {3, false, false}, {4, false, false}, {5, false, false}, {1, false, true}} {
		t.Run(fmt.Sprintf("count%d_play%v_cap%v", tc.count, tc.play, tc.cap), func(t *testing.T) {
			// Only replace this test's generated commander/role fixture.
			if _, err := orm.DeleteCommanderTB(client.Commander.CommanderID, 1); err != nil {
				t.Fatal(err)
			}
			state, err := loadEducateState(client, 1)
			if err != nil {
				t.Fatal(err)
			}
			state.Info.Fsm.SystemNo = proto.Uint32(newEducateSystemTalent)
			ensureEducateCache(state.Info).CacheTalent[0].Talents = []uint32{1090}
			markEducateStage(state, newEducateSystemTalent, false)
			if _, err := selectEducateTalent(state, 1090); err != nil {
				t.Fatal("real whole-schedule count talent rejected", err)
			}
			if tc.cap {
				cfg, found, err := loadNewEducateConfigByID[educateNumericConfig](newEducateAttrCategory, 101)
				if err != nil || !found {
					t.Fatal(err)
				}
				for _, attr := range state.Info.Res.Attrs {
					if attr.GetKey() == 101 {
						// Course +5 leaves room for exactly +3 of the +10 bonus.
						attr.Value = proto.Uint32(uint32(cfg.Max - 8))
					}
				}
			}
			state.Info.Fsm.SystemNo = proto.Uint32(newEducateSystemMap)
			if err := saveEducateState(state); err != nil {
				t.Fatal(err)
			}
			var plans []*protobuf.KVDATA
			for i := 0; i < 5; i++ {
				id := uint32(1106)
				if i < tc.count {
					id = 1101
				}
				plans = append(plans, &protobuf.KVDATA{Key: proto.Uint32(uint32(i + 1)), Value: proto.Uint32(id)})
			}
			var scheduled protobuf.SC_29041
			recoveryPacket(t, client, NewEducateSchedule, &protobuf.CS_29040{Id: proto.Uint32(1), Plans: plans}, &scheduled)
			if scheduled.GetResult() != 0 {
				t.Fatal("schedule", &scheduled)
			}
			// A summary cannot settle unfinished courses or award the count bonus.
			before := getRole(t)
			var summary protobuf.SC_29049
			recoveryPacket(t, client, NewEducateGetExtraDrop, &protobuf.CS_29048{Id: proto.Uint32(1)}, &summary)
			after := getRole(t)
			if summary.GetResult() != 1 || !bytes.Equal(before.State, after.State) || !bytes.Equal(before.Metadata, after.Metadata) || before.Revision != after.Revision {
				t.Fatal("early summary changed course state")
			}
			if tc.play {
				var first protobuf.SC_29043
				recoveryPacket(t, client, NewEducateNextPlan, &protobuf.CS_29042{Id: proto.Uint32(1)}, &first)
				if first.GetResult() != 0 {
					t.Fatal(&first)
				}
				for node, steps := first.GetFirstNode(), 0; node != 0; steps++ {
					if steps > 20 {
						t.Fatal("course failed to end")
					}
					var next protobuf.SC_29031
					recoveryPacket(t, client, NewEducateTriggerNode, &protobuf.CS_29030{Id: proto.Uint32(1), Branch: proto.Uint32(0)}, &next)
					if next.GetResult() != 0 {
						t.Fatal(&next)
					}
					node = next.GetNextNode()
				}
			}
			var skipped protobuf.SC_29047
			recoveryPacket(t, client, NewEducateScheduleSkip, &protobuf.CS_29046{Id: proto.Uint32(1)}, &skipped)
			if skipped.GetResult() != 0 {
				t.Fatal("skip", &skipped)
			}
			state, err = loadEducateState(client, 1)
			if err != nil {
				t.Fatal(err)
			}
			base := educateKVCount(state.Info.Res.Attrs, 101)
			recoveryPacket(t, client, NewEducateGetExtraDrop, &protobuf.CS_29048{Id: proto.Uint32(1)}, &summary)
			if summary.GetResult() != 0 {
				t.Fatal("summary", &summary)
			}
			if tc.play {
				// Representative new boundary: the counted reward committed but
				// the real socket failed before the client received its summary.
				left, right := net.Pipe()
				if err := right.Close(); err != nil {
					t.Fatal(err)
				}
				var wire net.Conn = left
				client.Connection = &wire
				if err := client.Flush(); err == nil {
					t.Fatal("summary delivery did not fail")
				}
				if err := left.Close(); err != nil {
					t.Fatal(err)
				}
				client = &connection.Client{Commander: client.Commander}
			}
			want := int32(tc.count * 10)
			if tc.cap {
				want = 3
			}
			if tc.count == 0 {
				if len(summary.Drop.BenefitDrop) != 0 {
					t.Fatal("zero matching courses got a bonus")
				}
			} else if len(summary.Drop.BenefitDrop) != 1 || summary.Drop.BenefitDrop[0].GetId() != 101 || summary.Drop.BenefitDrop[0].GetNumber() != want {
				t.Fatal("wrong counted/capped bonus", want, &summary)
			}
			state, err = loadEducateState(client, 1)
			if err != nil {
				t.Fatal(err)
			}
			if educateKVCount(state.Info.Res.Attrs, 101) != base+int64(want) || !proto.Equal(summary.Res, state.Info.Res) || !state.Lifecycle.Schedule.SummaryComplete || !state.Lifecycle.Schedule.ExtraComplete {
				t.Fatal("summary resources/progress disagree with committed bonus")
			}
			if len(state.Lifecycle.Schedule.Extra) != len(summary.Drop.BenefitDrop) {
				t.Fatal("extra reward not persisted")
			}
			before = getRole(t)
			recoveryPacket(t, client, NewEducateGetExtraDrop, &protobuf.CS_29048{Id: proto.Uint32(1)}, &summary)
			after = getRole(t)
			if summary.GetResult() != 0 || len(summary.Drop.BenefitDrop) != 0 || !bytes.Equal(before.State, after.State) || !bytes.Equal(before.Metadata, after.Metadata) || before.Revision != after.Revision {
				t.Fatal("summary retry re-awarded bonus")
			}
		})
	}
	after, err := orm.GetCommanderTB(client.Commander.CommanderID, 2)
	if err != nil || !bytes.Equal(other.State, after.State) || !bytes.Equal(other.Permanent, after.Permanent) || !bytes.Equal(other.Metadata, after.Metadata) || other.Revision != after.Revision {
		t.Fatal("course condition changed another role", err)
	}
}
