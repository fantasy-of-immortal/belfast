package neweducate

import (
	"bytes"
	"testing"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestRecoveryS07FixedNormalSite(t *testing.T) {
	client := recoveryDB(t)
	if _, err := loadEducateState(client, 1); err != nil {
		t.Fatal(err)
	}
	otherBefore, err := orm.GetCommanderTB(90001, 1)
	if err != nil {
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
	var entered protobuf.SC_29063
	recoveryPacket(t, client, NewEducateMapNormal, &protobuf.CS_29062{Id: proto.Uint32(2), WorkId: proto.Uint32(7)}, &entered)
	if entered.GetResult() != 1 {
		t.Fatal("locked outing accepted")
	}
	state.Info.Round.Round = proto.Uint32(2)
	state.Lifecycle = freshEducateLifecycle(state.Info)
	if err := saveEducateState(state); err != nil {
		t.Fatal(err)
	}
	// Unsupported random travel, a higher level and another character's site
	// must fail before action payment.
	for _, id := range []uint32{10, 8, 1} {
		before, err := orm.GetCommanderTB(90001, 2)
		if err != nil {
			t.Fatal(err)
		}
		recoveryPacket(t, client, NewEducateMapNormal, &protobuf.CS_29062{Id: proto.Uint32(2), WorkId: proto.Uint32(id)}, &entered)
		after, err := orm.GetCommanderTB(90001, 2)
		if err != nil {
			t.Fatal(err)
		}
		if entered.GetResult() != 1 || !bytes.Equal(before.State, after.State) || !bytes.Equal(before.Metadata, after.Metadata) || before.Revision != after.Revision {
			t.Fatal("rejected outing changed save", id)
		}
	}
	money := educateKVCount(state.Info.Res.Resource, 301)
	action := educateKVCount(state.Info.Res.Resource, 303)
	var version int64
	for instance := int64(1); instance <= 2; instance++ {
		recoveryPacket(t, client, NewEducateMapNormal, &protobuf.CS_29062{Id: proto.Uint32(2), WorkId: proto.Uint32(7)}, &entered)
		state, err = loadEducateState(client, 2)
		if err != nil {
			t.Fatal(err)
		}
		if entered.GetResult() != 0 || entered.GetFirstNode() != 3700801 || educateKVCount(state.Info.Res.Resource, 303) != action-instance || educateKVCount(state.Info.Res.Resource, 301) != money+(instance-1)*80 || state.Lifecycle.Chain.Version <= version {
			t.Fatal("entry/payment/instance", &entered)
		}
		version = state.Lifecycle.Chain.Version
		before, err := orm.GetCommanderTB(90001, 2)
		if err != nil {
			t.Fatal(err)
		}
		recoveryPacket(t, client, NewEducateMapNormal, &protobuf.CS_29062{Id: proto.Uint32(2), WorkId: proto.Uint32(7)}, &entered)
		after, err := orm.GetCommanderTB(90001, 2)
		if err != nil {
			t.Fatal(err)
		}
		if entered.GetResult() != 1 || !bytes.Equal(before.State, after.State) || before.Revision != after.Revision {
			t.Fatal("pending outing paid again")
		}
		var completed protobuf.SC_29031
		recoveryPacket(t, client, NewEducateTriggerNode, &protobuf.CS_29030{Id: proto.Uint32(2), Branch: proto.Uint32(0)}, &completed)
		state, err = loadEducateState(client, 2)
		if err != nil {
			t.Fatal(err)
		}
		if completed.GetResult() != 0 || completed.GetNextNode() != 0 || len(completed.Drop.GetBaseDrop()) != 1 || educateKVCount(state.Info.Res.Resource, 301) != money+instance*80 || !state.Lifecycle.Chain.Completed || state.Lifecycle.Chain.Version != version || educateKVCount(state.Info.Site.WorkCounter, 7) != instance {
			t.Fatal("outing settlement", &completed)
		}
		before, err = orm.GetCommanderTB(90001, 2)
		if err != nil {
			t.Fatal(err)
		}
		recoveryPacket(t, client, NewEducateTriggerNode, &protobuf.CS_29030{Id: proto.Uint32(2), Branch: proto.Uint32(0)}, &completed)
		after, err = orm.GetCommanderTB(90001, 2)
		if err != nil {
			t.Fatal(err)
		}
		if completed.GetResult() != 1 || !bytes.Equal(before.State, after.State) || !bytes.Equal(before.Metadata, after.Metadata) || before.Revision != after.Revision {
			t.Fatal("outing awarded again")
		}
	}
	otherAfter, err := orm.GetCommanderTB(90001, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(otherBefore.State, otherAfter.State) || !bytes.Equal(otherBefore.Permanent, otherAfter.Permanent) || !bytes.Equal(otherBefore.Metadata, otherAfter.Metadata) || otherBefore.Revision != otherAfter.Revision {
		t.Fatal("outing changed other character")
	}
}
