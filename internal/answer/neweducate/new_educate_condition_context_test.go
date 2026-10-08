package neweducate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestRecoveryS03ActionConditionContexts(t *testing.T) {
	client := recoveryDB(t)
	state := defaultEducateState(client.Commander.CommanderID, 2)
	cases := []struct {
		name, expression string
		context          *educateConditionContext
		want             bool
	}{
		{"odd course slot", "1030", &educateConditionContext{Slot: 3}, true},
		{"odd excludes even slot", "1030", &educateConditionContext{Slot: 2}, false},
		{"even course slot", "1031", &educateConditionContext{Slot: 2}, true},
		{"current physical course", "1061", &educateConditionContext{PlanID: 1101}, true},
		{"other current course", "1061", &educateConditionContext{PlanID: 1102}, false},
		{"one matching plan occurrence", "1033", &educateConditionContext{Plans: []uint32{1101, 1102}}, true},
		{"literal plan count boundary", "1033", &educateConditionContext{Plans: []uint32{1101, 1101}}, false},
		{"specific removed buff", "28", &educateConditionContext{RemovedBuffs: []uint32{1001}}, true},
		{"unrelated removed buff", "28", &educateConditionContext{RemovedBuffs: []uint32{1002}}, false},
		{"self removed buff", "60", &educateConditionContext{RemovedBuffs: []uint32{2041}, SourceBuffID: 2041}, true},
		{"current travel source", "38102111", &educateConditionContext{Site: &educateConditionSite{Type: 1, ID: 10}}, true},
		{"current work source", "38102111", &educateConditionContext{Site: &educateConditionSite{Type: 1, ID: 7}}, false},
		{"both activities in same round", `["&&",[38108311,38108312]]`, &educateConditionContext{RoundSites: []educateConditionSite{{Type: 1, ID: 10}, {Type: 1, ID: 7}}}, true},
		{"travel alone not both activities", `["&&",[38108311,38108312]]`, &educateConditionContext{RoundSites: []educateConditionSite{{Type: 1, ID: 10}}}, false},
		{"known empty round actions", "38108311", &educateConditionContext{RoundSites: []educateConditionSite{}}, false},
		{"three-round recurring buff at threshold", "38101011", &educateConditionContext{BuffRounds: map[uint32]educateConditionBuffRounds{3810101: {Total: 6, SinceReset: 3}}}, true},
		{"recurring buff uses reset counter", "38101011", &educateConditionContext{BuffRounds: map[uint32]educateConditionBuffRounds{3810101: {Total: 6, SinceReset: 2}}}, false},
		{"six-round cumulative buff at threshold", "38104911", &educateConditionContext{BuffRounds: map[uint32]educateConditionBuffRounds{3810481: {Total: 6, SinceReset: 2}}}, true},
		{"cumulative buff ignores reset counter", "38104911", &educateConditionContext{BuffRounds: map[uint32]educateConditionBuffRounds{3810481: {Total: 5, SinceReset: 6}}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := proto.Clone(state.Info)
			matched, err := evaluateEducateConditionWithContext(state, json.RawMessage(tc.expression), tc.context)
			if err != nil || matched != tc.want {
				t.Fatalf("%s: %v %v", tc.expression, matched, err)
			}
			if !proto.Equal(before, state.Info) {
				t.Fatal("predicate changed stored state")
			}
		})
	}
	for _, id := range []uint32{1028, 1030, 1033, 1061, 28, 60, 38102111, 38108311, 38101011} {
		if _, err := evaluateEducateCondition(state, json.RawMessage(fmt.Sprint(id))); err == nil {
			t.Fatalf("missing action context accepted for %d", id)
		}
	}
	if _, err := evaluateEducateConditionWithContext(state, json.RawMessage(`38104911`), &educateConditionContext{BuffRounds: map[uint32]educateConditionBuffRounds{}}); err == nil {
		t.Fatal("missing buff counter silently treated as zero")
	}
}

func TestRecoveryS03UnverifiedConditionsRollbackTransaction(t *testing.T) {
	client := recoveryDB(t)
	for _, id := range []uint32{1, 2} {
		if _, err := loadEducateState(client, id); err != nil {
			t.Fatal(err)
		}
	}
	before, err := orm.GetCommanderTB(client.Commander.CommanderID, 2)
	if err != nil {
		t.Fatal(err)
	}
	other, err := orm.GetCommanderTB(client.Commander.CommanderID, 1)
	if err != nil {
		t.Fatal(err)
	}
	// These are actual configuration IDs, not invented condition enums.
	// Keep unsupported windows and the ${num} placeholder observable even
	// when an earlier probability branch already returned true.
	for _, raw := range []string{"10010", "10012", "38100113", "38100112", "38103311", "38102212", `"${num}"`} {
		t.Run(raw, func(t *testing.T) {
			_, err := updateEducateState(client, 2, func(state *educateState) error {
				if _, err := applyEducateNumericBatch(state, [][]int32{{2, 301, 1}}, 1, true); err != nil {
					return err
				}
				_, err := evaluateEducateConditionWithContext(state, json.RawMessage(`["||",[1028,`+raw+`]]`), &educateConditionContext{ExecutionID: "unsupported-window", Draw: func(uint64) (uint64, error) { return 0, nil }})
				return err
			})
			if err == nil || (!strings.Contains(err.Error(), "unsupported condition type") && !strings.Contains(err.Error(), "invalid condition expression")) {
				t.Fatalf("unverified semantics silently accepted or wrong failure: %v", err)
			}
			after, err := orm.GetCommanderTB(client.Commander.CommanderID, 2)
			if err != nil || !bytes.Equal(before.State, after.State) || !bytes.Equal(before.Permanent, after.Permanent) || !bytes.Equal(before.Metadata, after.Metadata) || before.Revision != after.Revision {
				t.Fatal("unsupported condition committed payment/draw/revision", err)
			}
		})
	}
	after, err := orm.GetCommanderTB(client.Commander.CommanderID, 1)
	if err != nil || !bytes.Equal(other.State, after.State) || !bytes.Equal(other.Permanent, after.Permanent) || !bytes.Equal(other.Metadata, after.Metadata) || other.Revision != after.Revision {
		t.Fatal("failed transactions changed another role", err)
	}
}

func TestRecoveryS03OwnedBuffCountBoundary(t *testing.T) {
	client := recoveryDB(t)
	state := defaultEducateState(client.Commander.CommanderID, 2)
	config, ok, err := loadNewEducateConfigByID[educateConditionConfig](educateConditionCategory, 38100911)
	if err != nil || !ok {
		t.Fatal(err)
	}
	var ids []uint32
	if err := json.Unmarshal(config.Param[0], &ids); err != nil {
		t.Fatal(err)
	}
	for _, count := range []int{15, 16} {
		state.Info.Benefit.Actives = nil
		for _, id := range ids[:count] {
			state.Info.Benefit.Actives = append(state.Info.Benefit.Actives, &protobuf.TBBF{Id: proto.Uint32(id), Round: proto.Uint32(1), IsPending: proto.Uint32(0)})
		}
		// Client buff ownership is an ID-keyed dictionary, not a slice count.
		state.Info.Benefit.Actives = append(state.Info.Benefit.Actives, proto.Clone(state.Info.Benefit.Actives[0]).(*protobuf.TBBF))
		matched, err := evaluateEducateCondition(state, json.RawMessage(`38100911`))
		if err != nil || matched != (count > 15) {
			t.Fatal(count, matched, err)
		}
	}
	other := defaultEducateState(client.Commander.CommanderID, 1)
	if matched, err := evaluateEducateCondition(other, json.RawMessage(`38100911`)); err != nil || matched {
		t.Fatal("another role supplied buffs", matched, err)
	}
}

func TestRecoveryS03ProbabilityPersistsPerActionAndRole(t *testing.T) {
	client := recoveryDB(t)
	if _, err := loadEducateState(client, 2); err != nil {
		t.Fatal(err)
	}
	other, err := orm.GetCommanderTB(client.Commander.CommanderID, 2)
	if err != nil {
		t.Fatal(err)
	}
	state, err := loadEducateState(client, 1)
	if err != nil {
		t.Fatal(err)
	}
	stateBefore := proto.Clone(state.Info)
	for _, tc := range []struct {
		roll uint64
		want bool
	}{{1999, true}, {2000, false}} {
		calls := 0
		context := &educateConditionContext{ExecutionID: fmt.Sprintf("round:1/test-action:%d", tc.roll), Draw: func(total uint64) (uint64, error) {
			calls++
			if total != 10000 {
				t.Fatal(total)
			}
			return tc.roll, nil
		}}
		got, err := evaluateEducateConditionWithContext(state, json.RawMessage(`1028`), context)
		if err != nil || got != tc.want {
			t.Fatal(got, err)
		}
		if err := saveEducateState(state); err != nil {
			t.Fatal(err)
		}
		state, err = loadEducateState(client, 1)
		if err != nil {
			t.Fatal(err)
		}
		got, err = evaluateEducateConditionWithContext(state, json.RawMessage(`1028`), context)
		if err != nil || got != tc.want || calls != 1 {
			t.Fatal("same instance re-rolled", got, err, calls)
		}
	}
	if !proto.Equal(stateBefore, state.Info) {
		t.Fatal("probability changed numeric/node snapshot")
	}
	metadataBefore, _ := json.Marshal(state.Lifecycle)
	_, err = evaluateEducateConditionWithContext(state, json.RawMessage(`["||",[1028,999999999]]`), &educateConditionContext{ExecutionID: "failed-new-action", Draw: func(uint64) (uint64, error) { return 0, nil }})
	metadataAfter, _ := json.Marshal(state.Lifecycle)
	if err == nil || !bytes.Equal(metadataBefore, metadataAfter) {
		t.Fatal("failed expression committed random state", err)
	}
	_, err = evaluateEducateConditionWithContext(state, json.RawMessage(`1028`), &educateConditionContext{ExecutionID: "invalid-draw", Draw: func(uint64) (uint64, error) { return 10000, nil }})
	if err == nil {
		t.Fatal("out-of-range draw accepted")
	}
	after, err := orm.GetCommanderTB(client.Commander.CommanderID, 2)
	if err != nil || !bytes.Equal(other.State, after.State) || !bytes.Equal(other.Permanent, after.Permanent) || !bytes.Equal(other.Metadata, after.Metadata) || other.Revision != after.Revision {
		t.Fatal("random state changed other role", err)
	}
}
