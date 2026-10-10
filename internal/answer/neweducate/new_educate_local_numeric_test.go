package neweducate

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestRecoveryS03LocalNumericBindingsAndWindows(t *testing.T) {
	client := recoveryDB(t)
	state, err := loadEducateState(client, 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, expression     string
		current, round, held int64
		want                 uint32
		matched              bool
	}{
		{"current spending rounds down", `["&&",[38103212,38103213,"${num}"]]`, 99, 500, 900, 1, true},
		{"current below unit", `["&&",[38103212,38103213,"${num}"]]`, 49, 500, 900, 0, false},
		{"round excludes refund", `["&&",[38103311,38103312,38103313,"${num}"]]`, 1, 250, 900, 2, true},
		{"held configuration divisor", `["&&",[38102411,38102412,38102413,"${num}"]]`, 1, 250, 650, 2, true},
		{"gain round quotient", `["&&",[38100111,38100112,38100113,"${num}"]]`, 1, 9, 99, 3, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			context := &educateConditionContext{Changes: map[string]educateNumericChange{"2/301": {Negative: tc.current}}, RoundChanges: map[string]educateNumericChange{"2/301": {Positive: 600, Negative: tc.round}, "2/302": {Positive: tc.round}}, HeldChanges: map[string]educateNumericChange{"2/301": {Negative: tc.held}}}
			matched, err := evaluateEducateConditionWithContext(state, json.RawMessage(tc.expression), context)
			if err != nil || matched != tc.matched || context.Multiplier != tc.want {
				t.Fatalf("matched=%v multiplier=%d err=%v", matched, context.Multiplier, err)
			}
		})
	}
	for _, raw := range []string{
		`["||",[["&&",[38103212,38103213,"${num}"]],"${num}"]]`,
		`["||",[["&&",[38103212,38103213,"${num}"]],["&&",[38103212,38103313,"${num}"]]]]`,
		`["||",[[],10010]]`,
	} {
		context := &educateConditionContext{Changes: map[string]educateNumericChange{"2/301": {Negative: 200}}}
		if _, err := evaluateEducateConditionWithContext(state, json.RawMessage(raw), context); err == nil {
			t.Fatal("leaking/ambiguous/unknown expression accepted", raw)
		}
		if context.hasNumber || context.Number != 0 {
			t.Fatal("failed expression leaked numeric binding")
		}
	}
	// Even a retained foreign field is not a valid change source for this role.
	context := &educateConditionContext{Changes: map[string]educateNumericChange{"2/1": {Positive: 100}}}
	foreign := &educateConditionConfig{ID: 1, Type: 17, Param: []json.RawMessage{json.RawMessage("2"), json.RawMessage("1"), json.RawMessage(`">"`), json.RawMessage("0")}}
	if _, err := evaluateEducateChangeCondition(state, foreign, context); err == nil {
		t.Fatal("foreign change field accepted")
	}
}

func TestRecoveryS03NumericLedgerAtomicPersistence(t *testing.T) {
	client := recoveryDB(t)
	state, err := loadEducateState(client, 1)
	if err != nil {
		t.Fatal(err)
	}
	state.Info.Benefit.Actives = []*protobuf.TBBF{{Id: proto.Uint32(1001), Round: proto.Uint32(1), IsPending: proto.Uint32(0)}}
	if _, err := applyEducateNumericBatch(state, [][]int32{{2, 1, 10}}, 1, true); err != nil {
		t.Fatal(err)
	}
	if _, err := applyEducateNumericBatch(state, [][]int32{{2, 1, 7}}, 1, false); err != nil {
		t.Fatal(err)
	}
	if err := saveEducateState(state); err != nil {
		t.Fatal(err)
	}
	state, err = loadEducateState(client, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, counter := range []educateNumericChange{state.Lifecycle.NumericLedger.Round["2/1"], state.Lifecycle.NumericLedger.Held[1001]["2/1"]} {
		if counter.Positive != 7 || counter.Negative != 10 {
			t.Fatal("refund erased spend", counter)
		}
	}
	before := proto.Clone(state.Info.Res)
	ledgerBefore, _ := json.Marshal(state.Lifecycle.NumericLedger)
	if _, err := applyEducateNumericBatch(state, [][]int32{{2, 1, 1}, {2, 1, 99999}}, 1, true); err == nil {
		t.Fatal("bad batch accepted")
	}
	ledgerAfter, _ := json.Marshal(state.Lifecycle.NumericLedger)
	if !proto.Equal(before, state.Info.Res) || string(ledgerBefore) != string(ledgerAfter) {
		t.Fatal("failed batch left resource/counter changes")
	}
	advanceNewEducateRound(state)
	if len(state.Lifecycle.NumericLedger.Round) != 0 || state.Lifecycle.NumericLedger.Held[1001]["2/1"].Negative != 10 {
		t.Fatal("round/held windows reset together")
	}
	state.Lifecycle.NumericLedger.Round["2/1"] = educateNumericChange{Positive: math.MaxInt64}
	before = proto.Clone(state.Info.Res)
	if _, err := applyEducateNumericBatch(state, [][]int32{{2, 1, 1}}, 1, false); err == nil {
		t.Fatal("counter overflow accepted")
	}
	if !proto.Equal(before, state.Info.Res) || state.Lifecycle.NumericLedger.Round["2/1"].Positive != math.MaxInt64 {
		t.Fatal("overflow partially committed")
	}
}
