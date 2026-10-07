package neweducate

import (
	"bytes"
	"testing"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestRecoveryS07NumericShopAndFavorTransactions(t *testing.T) {
	client := recoveryDB(t)
	for _, id := range []uint32{1, 2} {
		state, err := loadEducateState(client, id)
		if err != nil {
			t.Fatal(err)
		}
		state.Info.Fsm.SystemNo = proto.Uint32(newEducateSystemMap)
		ensureEducateCache(state.Info).CacheSite[0].Shops = []uint32{3001, 3009, 3101, 1}
		if err := saveEducateState(state); err != nil {
			t.Fatal(err)
		}
	}
	otherBefore, err := orm.GetCommanderTB(90001, 1)
	if err != nil {
		t.Fatal(err)
	}
	state, err := loadEducateState(client, 2)
	if err != nil {
		t.Fatal(err)
	}
	money := educateKVCount(state.Info.Res.Resource, 301)
	mood := educateKVCount(state.Info.Res.Resource, 302)
	body := educateKVCount(state.Info.Res.Attrs, 301)
	for _, id := range []uint32{3001, 3101} {
		var response protobuf.SC_29067
		recoveryPacket(t, client, NewEducateShopping, &protobuf.CS_29066{Id: proto.Uint32(2), Shop: proto.Uint32(id), Num: proto.Uint32(1)}, &response)
		if response.GetResult() != 0 || len(response.Drop.GetBaseDrop()) != 1 {
			t.Fatal("missing delivery", &response)
		}
	}
	state, err = loadEducateState(client, 2)
	if err != nil {
		t.Fatal(err)
	}
	if educateKVCount(state.Info.Res.Resource, 301) != money-23 || educateKVCount(state.Info.Res.Resource, 302) != mood+2 || educateKVCount(state.Info.Res.Attrs, 301) != body+10 {
		t.Fatal("payment/delivery not persisted", state.Info.Res)
	}
	site := ensureEducateCache(state.Info).CacheSite[0]
	if educateKVCount(site.Buys, 3001) != 1 || educateKVCount(site.Buys, 3101) != 1 || state.Info.Fsm.GetSystemNo() != newEducateSystemMap {
		t.Fatal("stock or phase")
	}
	otherAfter, err := orm.GetCommanderTB(90001, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(otherBefore.State, otherAfter.State) || !bytes.Equal(otherBefore.Permanent, otherAfter.Permanent) || !bytes.Equal(otherBefore.Metadata, otherAfter.Metadata) || otherBefore.Revision != otherAfter.Revision {
		t.Fatal("other character changed")
	}
	// Repeated limited stock, missing offer, insufficient funds, unsupported
	// delivery, excessive quantity, and another character's numeric good must
	// all roll back payment, delivery and revision together.
	for _, tc := range []struct{ character, good, quantity uint32 }{{2, 3001, 1}, {2, 3002, 1}, {2, 3009, 1}, {2, 1, 1}, {2, 3101, ^uint32(0)}, {1, 3001, 1}} {
		before, err := orm.GetCommanderTB(90001, tc.character)
		if err != nil {
			t.Fatal(err)
		}
		var response protobuf.SC_29067
		recoveryPacket(t, client, NewEducateShopping, &protobuf.CS_29066{Id: proto.Uint32(tc.character), Shop: proto.Uint32(tc.good), Num: proto.Uint32(tc.quantity)}, &response)
		after, err := orm.GetCommanderTB(90001, tc.character)
		if err != nil {
			t.Fatal(err)
		}
		if response.GetResult() != 1 || !bytes.Equal(before.State, after.State) || !bytes.Equal(before.Permanent, after.Permanent) || !bytes.Equal(before.Metadata, after.Metadata) || before.Revision != after.Revision {
			t.Fatal("rejected purchase mutated save", tc, &response)
		}
	}
	// Real favor thresholds are cumulative 100/150/200/300, not 100/50/50/100
	// against the unchanged cumulative resource. Repeated claims at each exact
	// boundary must fail until the next threshold is earned.
	for _, exp := range []uint32{50, 100, 150, 200, 300} {
		state, err = loadEducateState(client, 2)
		if err != nil {
			t.Fatal(err)
		}
		state.Info.Res.Resource = upsertKVDATA(state.Info.Res.Resource, 304, exp)
		if err := saveEducateState(state); err != nil {
			t.Fatal(err)
		}
		var response protobuf.SC_29028
		if exp != 50 {
			beforeMood := educateKVCount(state.Info.Res.Resource, 302)
			expected := int64(8)
			if exp >= 200 {
				expected = 16
			}
			recoveryPacket(t, client, NewEducateUpgradeFavor, &protobuf.CS_29027{Id: proto.Uint32(2)}, &response)
			state, err = loadEducateState(client, 2)
			if err != nil {
				t.Fatal(err)
			}
			if response.GetResult() != 0 || len(response.Drop.GetBaseDrop()) != 1 || educateKVCount(state.Info.Res.Resource, 302) != beforeMood+expected || educateKVCount(state.Info.Res.Resource, 304) != int64(exp) {
				t.Fatal("favor claim delivery/experience", exp, &response)
			}
		}
		before, err := orm.GetCommanderTB(90001, 2)
		if err != nil {
			t.Fatal(err)
		}
		recoveryPacket(t, client, NewEducateUpgradeFavor, &protobuf.CS_29027{Id: proto.Uint32(2)}, &response)
		after, err := orm.GetCommanderTB(90001, 2)
		if err != nil {
			t.Fatal(err)
		}
		if response.GetResult() != 1 || !bytes.Equal(before.State, after.State) || !bytes.Equal(before.Metadata, after.Metadata) || before.Revision != after.Revision {
			t.Fatal("unearned/repeated favor claim", exp)
		}
	}
	if state.Info.GetFavorLv() != 4 {
		t.Fatal("favor maximum claim count", state.Info.GetFavorLv())
	}
	otherAfter, err = orm.GetCommanderTB(90001, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(otherBefore.State, otherAfter.State) || !bytes.Equal(otherBefore.Permanent, otherAfter.Permanent) || !bytes.Equal(otherBefore.Metadata, otherAfter.Metadata) || otherBefore.Revision != otherAfter.Revision {
		t.Fatal("favor claim changed other character")
	}
}
