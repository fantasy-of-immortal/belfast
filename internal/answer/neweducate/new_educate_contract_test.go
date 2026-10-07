package neweducate

import (
	"bytes"
	"encoding/json"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
	"testing"
)

func TestRecoveryS03BOMIdentityContract(t *testing.T) {
	for _, raw := range []string{`{"id":3700001}`, `{"\ufeffid":3700001}`, `{"id":3700001,"\ufeffid":3700001}`} {
		input := json.RawMessage(raw)
		before := append([]byte{}, input...)
		normalized, err := normalizeEducateConfigIdentity(input)
		if err != nil {
			t.Fatal(err)
		}
		var n newEducateNodeConfig
		if err := json.Unmarshal(normalized, &n); err != nil || n.ID != 3700001 {
			t.Fatal("identity normalization failed", err)
		}
		if !bytes.Equal(input, before) {
			t.Fatal("normalization changed evidence bytes")
		}
	}
	for _, raw := range []string{`null`, `[]`, `{"id":1,"\ufeffid":2}`} {
		if _, err := normalizeEducateConfigIdentity(json.RawMessage(raw)); err == nil {
			t.Fatal("invalid/conflicting identity accepted", raw)
		}
	}
}

func TestRecoveryS03ResourceAndRoundContracts(t *testing.T) {
	client := recoveryDB(t)
	state := defaultEducateState(client.Commander.CommanderID, 2)
	state.Info.Res.Resource = append([]*protobuf.KVDATA{{Key: proto.Uint32(1), Value: proto.Uint32(1000)}}, state.Info.Res.Resource...)
	id, ok, err := resolveNewEducateResourceID(state, 1)
	if err != nil || !ok || id != 301 {
		t.Fatalf("mixed legacy fields selected other character resource: %d %v %v", id, ok, err)
	}
	round, ok, err := loadCurrentNewEducateRoundConfig(state.Info)
	if err != nil || !ok {
		t.Fatal("real round fixture", err)
	}
	clone := *round
	clone.ID = 99999
	data, err := json.Marshal(clone)
	if err != nil {
		t.Fatal(err)
	}
	if err := orm.UpsertConfigEntry(newEducateRoundCategory, "99999", data); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadCurrentNewEducateRoundConfig(state.Info); err == nil {
		t.Fatal("duplicate logical round silently selected first")
	}
}

func TestRecoveryS03NumericAtomicBounds(t *testing.T) {
	client := recoveryDB(t)
	state := defaultEducateState(client.Commander.CommanderID, 2)
	if err := seedNewEducateDefaultRes(state.Info, 2); err != nil {
		t.Fatal(err)
	}
	before := proto.Clone(state.Info.Res)
	// The first payment is affordable; the second is not. Neither may commit.
	if _, err := applyEducateNumericBatch(state, [][]int32{{2, 301, 1}, {2, 302, 999}}, 1, true); err == nil {
		t.Fatal("unaffordable batch succeeded")
	}
	if !proto.Equal(before, state.Info.Res) {
		t.Fatal("failed batch partially charged")
	}
	for _, row := range [][][]int32{{{2, 1, 1}}, {{99, 301, 1}}, {{2, 301, -1}}, {{2, 301, 1, 2}}} {
		if _, err := applyEducateNumericBatch(state, row, 1, true); err == nil {
			t.Fatalf("invalid payment accepted %v", row)
		}
		if !proto.Equal(before, state.Info.Res) {
			t.Fatal("invalid payment changed resources")
		}
	}
	var mood *protobuf.KVDATA
	for _, v := range state.Info.Res.Resource {
		if v.GetKey() == 302 {
			mood = v
		}
	}
	config, ok, err := loadNewEducateConfigByID[educateNumericConfig](newEducateResourceCategory, 302)
	if err != nil || !ok {
		t.Fatal(err)
	}
	mood.Value = proto.Uint32(uint32(config.Max - 1))
	drops, err := applyEducateNumericBatch(state, [][]int32{{2, 302, 20}}, 1, false)
	if err != nil || len(drops) != 1 || drops[0].GetNumber() != 1 {
		t.Fatalf("cap actual delta: %v %v", drops, err)
	}
	for _, v := range state.Info.Res.Resource {
		if v.GetKey() == 302 && int64(v.GetValue()) != config.Max {
			t.Fatal("reward cap not stored")
		}
	}
	// Extreme multiplication must be rejected as unaffordable, not wrap to gain.
	before = proto.Clone(state.Info.Res)
	if _, err := applyEducateNumericBatch(state, [][]int32{{2, 301, 2147483647}}, 4294967295, true); err == nil {
		t.Fatal("huge payment accepted")
	}
	if !proto.Equal(before, state.Info.Res) {
		t.Fatal("overflow payment changed state")
	}
	if _, err := parseEducateDropTriplets(json.RawMessage(`[[2,301]]`), "fixture", "1", "cost"); err == nil {
		t.Fatal("malformed drops silently became empty")
	}
}

func TestRecoveryS03ConditionBoundaries(t *testing.T) {
	client := recoveryDB(t)
	state := defaultEducateState(client.Commander.CommanderID, 1)
	if err := seedNewEducateDefaultRes(state.Info, 1); err != nil {
		t.Fatal(err)
	}
	set := func(id, value uint32) {
		for _, v := range state.Info.Res.Attrs {
			if v.GetKey() == id {
				v.Value = proto.Uint32(value)
			}
		}
	}
	for _, value := range []uint32{49, 50, 51} {
		set(101, value)
		matched, err := evaluateEducateCondition(state, json.RawMessage(`1`))
		if err != nil || matched != (value >= 50) {
			t.Fatalf("actual condition 1 boundary %d: %v %v", value, matched, err)
		}
	}
	matched, err := evaluateEducateCondition(state, json.RawMessage(`["&&",[1,["||",[1,10009]]]]`))
	if err != nil || !matched {
		t.Fatalf("nested real conditions: %v %v", matched, err)
	}
	if _, err := evaluateEducateCondition(state, json.RawMessage(`["||",[1,10002]]`)); err == nil {
		t.Fatal("unsupported probability condition hidden by true branch")
	}
	if _, err := evaluateEducateCondition(state, json.RawMessage(`999999999`)); err == nil {
		t.Fatal("missing condition treated as success")
	}
	if _, err := evaluateEducateCondition(state, json.RawMessage(`null`)); err == nil {
		t.Fatal("null condition treated as empty success")
	}
	if ok, err := evaluateEducateCondition(state, json.RawMessage(`[]`)); err != nil || !ok {
		t.Fatal("explicit empty gate")
	}
}
