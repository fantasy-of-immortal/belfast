package neweducate

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestRecoveryS03CourseModifierProtocol(t *testing.T) {
	client := recoveryDB(t)
	if _, err := loadEducateState(client, 2); err != nil {
		t.Fatal(err)
	}
	other, err := orm.GetCommanderTB(client.Commander.CommanderID, 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		talent    uint32
		want      int64
		cap       bool
		duplicate bool
	}{{1064, 31, false, false}, {1065, 31, false, false}, {1066, 34, false, false}, {1067, 33, false, false}, {1068, 30, false, false}, {1069, 35, false, false}, {1068, 3, true, false}, {1068, 30, false, true}} {
		t.Run(fmt.Sprintf("talent%d_cap%v_duplicate%v", tc.talent, tc.cap, tc.duplicate), func(t *testing.T) {
			if _, err := orm.DeleteCommanderTB(client.Commander.CommanderID, 1); err != nil {
				t.Fatal(err)
			}
			state, err := loadEducateState(client, 1)
			if err != nil {
				t.Fatal(err)
			}
			state.Info.Fsm.SystemNo = proto.Uint32(newEducateSystemTalent)
			ensureEducateCache(state.Info).CacheTalent[0].Talents = []uint32{tc.talent}
			markEducateStage(state, newEducateSystemTalent, false)
			if _, err := selectEducateTalent(state, tc.talent); err != nil {
				t.Fatal("real course talent rejected", err)
			}
			if tc.duplicate {
				state.Info.Benefit.Actives = append(state.Info.Benefit.Actives, proto.Clone(state.Info.Benefit.Actives[0]).(*protobuf.TBBF))
			}
			if tc.cap {
				cfg, ok, err := loadNewEducateConfigByID[educateNumericConfig](newEducateAttrCategory, 101)
				if err != nil || !ok {
					t.Fatal(err)
				}
				for _, v := range state.Info.Res.Attrs {
					if v.GetKey() == 101 {
						v.Value = proto.Uint32(uint32(cfg.Max - 3))
					}
				}
			}
			base := educateKVCount(state.Info.Res.Attrs, 101)
			state.Info.Fsm.SystemNo = proto.Uint32(newEducateSystemMap)
			if err := saveEducateState(state); err != nil {
				t.Fatal(err)
			}
			plans := []*protobuf.KVDATA{}
			for i := 1; i <= 5; i++ {
				plans = append(plans, &protobuf.KVDATA{Key: proto.Uint32(uint32(i)), Value: proto.Uint32(1101)})
			}
			var arranged protobuf.SC_29041
			recoveryPacket(t, client, NewEducateSchedule, &protobuf.CS_29040{Id: proto.Uint32(1), Plans: plans}, &arranged)
			if arranged.GetResult() != 0 {
				t.Fatal(&arranged)
			}
			var first protobuf.SC_29043
			recoveryPacket(t, client, NewEducateNextPlan, &protobuf.CS_29042{Id: proto.Uint32(1)}, &first)
			if first.GetResult() != 0 {
				t.Fatal(&first)
			}
			var paid int64
			for node, step := first.GetFirstNode(), 0; node != 0; step++ {
				if step > 20 {
					t.Fatal("course did not end")
				}
				var next protobuf.SC_29031
				recoveryPacket(t, client, NewEducateTriggerNode, &protobuf.CS_29030{Id: proto.Uint32(1), Branch: proto.Uint32(0)}, &next)
				if next.GetResult() != 0 {
					t.Fatal(&next)
				}
				for _, drop := range next.Drop.BaseDrop {
					if drop.GetId() == 101 {
						paid += int64(drop.GetNumber())
					}
				}
				node = next.GetNextNode()
			}
			var skipped protobuf.SC_29047
			recoveryPacket(t, client, NewEducateScheduleSkip, &protobuf.CS_29046{Id: proto.Uint32(1)}, &skipped)
			if skipped.GetResult() != 0 {
				t.Fatal(&skipped)
			}
			for _, drop := range skipped.Drop.BaseDrop {
				if drop.GetId() == 101 {
					paid += int64(drop.GetNumber())
				}
			}
			state, err = loadEducateState(client, 1)
			if err != nil {
				t.Fatal(err)
			}
			if paid != tc.want || educateKVCount(state.Info.Res.Attrs, 101) != base+tc.want || state.Lifecycle.NumericLedger.Round["1/101"].Positive != tc.want || state.Lifecycle.NumericLedger.Held[tc.talent]["1/101"].Positive != tc.want {
				t.Fatalf("response/storage/counter mismatch paid=%d wanted=%d", paid, tc.want)
			}
			var summary protobuf.SC_29049
			recoveryPacket(t, client, NewEducateGetExtraDrop, &protobuf.CS_29048{Id: proto.Uint32(1)}, &summary)
			if summary.GetResult() != 0 || len(summary.Drop.BenefitDrop) != 0 {
				t.Fatal("modifier paid twice at summary", &summary)
			}
			before, err := orm.GetCommanderTB(client.Commander.CommanderID, 1)
			if err != nil {
				t.Fatal(err)
			}
			recoveryPacket(t, client, NewEducateScheduleSkip, &protobuf.CS_29046{Id: proto.Uint32(1)}, &skipped)
			after, err := orm.GetCommanderTB(client.Commander.CommanderID, 1)
			if err != nil || skipped.GetResult() != 1 || !bytes.Equal(before.State, after.State) || !bytes.Equal(before.Metadata, after.Metadata) || before.Revision != after.Revision {
				t.Fatal("repeated skip paid a modifier", err)
			}
		})
	}
	after, err := orm.GetCommanderTB(client.Commander.CommanderID, 2)
	if err != nil || !bytes.Equal(other.State, after.State) || !bytes.Equal(other.Permanent, after.Permanent) || !bytes.Equal(other.Metadata, after.Metadata) || other.Revision != after.Revision {
		t.Fatal("modifier changed another role", err)
	}
}
