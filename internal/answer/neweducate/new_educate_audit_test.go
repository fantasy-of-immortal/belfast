package neweducate

import (
	"bytes"
	"testing"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestRecoveryAuditOldMutationsCannotBypassOrBreakCourses(t *testing.T) {
	client := recoveryDB(t)
	for _, id := range []uint32{1, 2} {
		if _, err := loadEducateState(client, id); err != nil {
			t.Fatal(err)
		}
	}
	type operation struct {
		handler           func(*[]byte, *connection.Client) (int, int, error)
		request, response proto.Message
	}
	operations := []operation{
		{NewEducateAssess, &protobuf.CS_29013{Id: proto.Uint32(2), Rank: proto.Uint32(1)}, &protobuf.SC_29014{}},
		{NewEducateSelectTopic, &protobuf.CS_29017{Id: proto.Uint32(2), ChatId: proto.Uint32(999)}, &protobuf.SC_29018{}},
		{NewEducateGetEndings, &protobuf.CS_29003{Id: proto.Uint32(2)}, &protobuf.SC_29004{}},
		{NewEducateSelectEnding, &protobuf.CS_29005{Id: proto.Uint32(2), EndingId: proto.Uint32(999)}, &protobuf.SC_29006{}},
		{NewEducateReset, &protobuf.CS_29007{Id: proto.Uint32(2), Difficulty: proto.Uint32(0)}, &protobuf.SC_29008{}},
		{NewEducateMapEvent, &protobuf.CS_29064{Id: proto.Uint32(2), Event: proto.Uint32(999)}, &protobuf.SC_29065{}},
		{NewEducateMapShip, &protobuf.CS_29068{Id: proto.Uint32(2), Character: proto.Uint32(999)}, &protobuf.SC_29069{}},
	}
	for _, system := range []uint32{newEducateSystemMap, newEducateSystemPlan} {
		state, err := loadEducateState(client, 2)
		if err != nil {
			t.Fatal(err)
		}
		state.Info.Fsm.SystemNo = proto.Uint32(system)
		if system == newEducateSystemPlan {
			state.Info.Fsm.CurrentNode = proto.Uint32(120101)
		}
		if err := saveEducateState(state); err != nil {
			t.Fatal(err)
		}
		before, _ := orm.GetCommanderTB(90001, 2)
		other, _ := orm.GetCommanderTB(90001, 1)
		for _, op := range operations {
			recoveryPacket(t, client, op.handler, op.request, op.response)
			if op.response.ProtoReflect().Get(op.response.ProtoReflect().Descriptor().Fields().ByName("result")).Uint() != 1 {
				t.Fatal("placeholder operation accepted", op.request)
			}
		}
		var phase protobuf.SC_29026
		recoveryPacket(t, client, NewEducateChangePhase, &protobuf.CS_29025{Id: proto.Uint32(2)}, &phase)
		if phase.GetResult() == 0 {
			t.Fatal("rejected assessment still allowed phase advance")
		}
		after, _ := orm.GetCommanderTB(90001, 2)
		otherAfter, _ := orm.GetCommanderTB(90001, 1)
		for _, pair := range [][2]*orm.CommanderTB{{before, after}, {other, otherAfter}} {
			if pair[0].Revision != pair[1].Revision || !bytes.Equal(pair[0].State, pair[1].State) || !bytes.Equal(pair[0].Permanent, pair[1].Permanent) || !bytes.Equal(pair[0].Metadata, pair[1].Metadata) {
				t.Fatal("rejected old operation changed save")
			}
		}
	}
}
