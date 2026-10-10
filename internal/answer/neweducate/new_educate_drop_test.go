package neweducate

import (
	"encoding/json"
	"testing"

	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestRecoveryS03MixedDropDeliveryAtomic(t *testing.T) {
	client := recoveryDB(t)
	state, err := loadEducateState(client, 1)
	if err != nil {
		t.Fatal(err)
	}
	res := proto.Clone(state.Info.Res)
	permanent := proto.Clone(state.Permanent)
	metadata, _ := json.Marshal(state.Lifecycle)
	for _, rows := range [][][]int32{
		{{2, 1, 5}, {3, 1, 1}, {5, 1, 1}},
		{{2, 1, 5}, {4, 99999, 1}},
		{{3, 1, 1}, {7, 0, -1}},
		{{2, 1, 5}, {10000, 1, 1}},
	} {
		if _, err := applyEducateDropBatch(state, rows, 1, nil); err == nil {
			t.Fatal("invalid/unsupported mixed batch accepted", rows)
		}
		after, _ := json.Marshal(state.Lifecycle)
		if !proto.Equal(res, state.Info.Res) || !proto.Equal(permanent, state.Permanent) || string(metadata) != string(after) {
			t.Fatal("failed delivery left payment/photo/counter changes")
		}
	}
	rows := [][]int32{{2, 1, 5}, {3, 1, 1}, {7, 0, 1}}
	drops, err := applyEducateDropBatch(state, rows, 1, nil)
	if err != nil || len(drops) != 3 || len(state.Permanent.Polaroids) != len(permanent.(*protobuf.TBPERMANENT).Polaroids)+1 || state.Info.Round.GetTempRound() != 1 {
		t.Fatal("mixed drop lost delivery", drops, err)
	}
	if err := saveEducateState(state); err != nil {
		t.Fatal(err)
	}
	state, err = loadEducateState(client, 1)
	if err != nil || state.Info.Round.GetTempRound() != 1 || state.Lifecycle.TempRound != 1 || state.Permanent.Polaroids[len(state.Permanent.Polaroids)-1] != 1 {
		t.Fatal("mixed delivery did not reload", err)
	}
	photos, err := listNewEducateConfigs[educatePolaroidConfig]("ShareCfg/child2_polaroid.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, photo := range photos {
		if photo.Character == 2 {
			if _, err := applyEducateDropBatch(state, [][]int32{{3, int32(photo.ID), 1}}, 1, nil); err == nil {
				t.Fatal("foreign role photo accepted")
			}
			break
		}
	}
}

func TestRecoveryS03SiteProbabilityDelivery(t *testing.T) {
	client := recoveryDB(t)
	state, err := loadEducateState(client, 1)
	if err != nil {
		t.Fatal(err)
	}
	state.Info.Round.Round = proto.Uint32(2)
	state.Lifecycle = freshEducateLifecycle(state.Info)
	state.Info.Fsm.SystemNo = proto.Uint32(newEducateSystemMap)
	state.Info.Benefit.Actives = []*protobuf.TBBF{{Id: proto.Uint32(1052), Round: proto.Uint32(2), IsPending: proto.Uint32(0)}}
	for _, value := range state.Info.Res.Resource {
		if value.GetKey() == 3 {
			value.Value = proto.Uint32(1)
		}
	}
	if err := saveEducateState(state); err != nil {
		t.Fatal(err)
	}
	var response protobuf.SC_29063
	recoveryPacket(t, client, NewEducateMapNormal, &protobuf.CS_29062{Id: proto.Uint32(1), WorkId: proto.Uint32(1)}, &response)
	if response.GetResult() != 0 {
		t.Fatal("probability talent blocked a real outing", &response)
	}
	state, err = loadEducateState(client, 1)
	if err != nil {
		t.Fatal(err)
	}
	var refund int64
	for _, drop := range response.Drop.BenefitDrop {
		if drop.GetType() != 2 || drop.GetId() != 3 {
			t.Fatal("unexpected probability reward", drop)
		}
		refund += int64(drop.GetNumber())
	}
	if refund < 0 || refund > 1 || educateKVCount(state.Info.Res.Resource, 3) != refund || len(state.Lifecycle.ConditionDraws) != 1 || len(state.Lifecycle.RoundSites) != 1 || state.Lifecycle.NumericLedger.Round["2/3"].Negative != 1 || state.Lifecycle.NumericLedger.Round["2/3"].Positive != refund {
		t.Fatal("probability response/store/counter mismatch")
	}
	before, _ := json.Marshal(state.Lifecycle)
	recoveryPacket(t, client, NewEducateMapNormal, &protobuf.CS_29062{Id: proto.Uint32(1), WorkId: proto.Uint32(1)}, &response)
	state, err = loadEducateState(client, 1)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(state.Lifecycle)
	if response.GetResult() != 1 || string(before) != string(after) || educateKVCount(state.Info.Res.Resource, 3) != refund {
		t.Fatal("pending outing recharged/rerolled probability")
	}
}
