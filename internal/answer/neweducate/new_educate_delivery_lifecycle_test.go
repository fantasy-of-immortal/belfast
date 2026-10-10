package neweducate

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/ggmolly/belfast/internal/orm"
	"google.golang.org/protobuf/proto"
)

func TestRecoveryS03PriorityDeliveryReloadAndRollback(t *testing.T) {
	client := recoveryDB(t)
	state, err := loadEducateState(client, 2)
	if err != nil {
		t.Fatal(err)
	}
	state.Info.Fsm.SystemNo = proto.Uint32(newEducateSystemMap)
	rows := [][]int32{{2, 301, 5}, {5, 1000, 2}, {6, 0, 1}, {10000, 3810011, 1}}
	drops, err := applyEducateDropBatch(state, rows, 1, educateChangeContext(state, "fixture/priority", 0, nil))
	if err != nil || len(drops) != 4 {
		t.Fatal(drops, err)
	}
	if state.Info.Fsm.GetSystemNo() != newEducateSystemMap || len(state.Info.Fsm.PriorityFsm) != 3 || state.Info.Fsm.PriorityFsm[0].GetSystemNo() != 101 || len(state.Info.Fsm.TarotSelects) != 1 || len(state.Lifecycle.PrioritySources) != 3 || state.Lifecycle.PrioritySources[1].ID != 1000 {
		t.Fatal("priority delivery lost order, source, count or continuation")
	}
	expected := proto.Clone(state.Info)
	metadata, _ := json.Marshal(state.Lifecycle)
	if err := saveEducateState(state); err != nil {
		t.Fatal(err)
	}
	state, err = loadEducateState(client, 2)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(state.Lifecycle)
	if !proto.Equal(expected, state.Info) || string(metadata) != string(after) || !educateHasPending(state.Info) {
		t.Fatal("priority state did not reload or block unrelated actions")
	}
	if err := scheduleEducatePlans(state, nil); err != errEducatePhase {
		t.Fatal("pending priority did not block schedule input", err)
	}
	for _, invalid := range [][][]int32{
		{{2, 301, 1}, {5, 1, 1}}, {{6, 0, 1}, {10000, 1001, 1}}, {{5, 1000, 129}}, {{2, 301, 1}, {6, 1, 1}}, {{5, 1000, 1}, {99, 1, 1}},
	} {
		if _, err := applyEducateDropBatch(state, invalid, 1, nil); err == nil {
			t.Fatal("bad priority batch accepted", invalid)
		}
		after, _ = json.Marshal(state.Lifecycle)
		if !proto.Equal(expected, state.Info) || string(metadata) != string(after) {
			t.Fatal("failed priority batch partially committed")
		}
	}
	other, err := loadEducateState(client, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := applyEducateDropBatch(other, [][]int32{{5, 1000, 1}}, 1, nil); err == nil {
		t.Fatal("foreign character private pool accepted")
	}
}

func TestRecoveryS03PendingBenefitActivationExpiryAndReacquisition(t *testing.T) {
	client := recoveryDB(t)
	state, err := loadEducateState(client, 1)
	if err != nil {
		t.Fatal(err)
	}
	state.Info.Fsm.SystemNo = proto.Uint32(newEducateSystemPlan)
	if _, err := applyEducateDropBatch(state, [][]int32{{4, 2001, 1}}, 1, nil); err != nil {
		t.Fatal(err)
	}
	if state.Info.Benefit.Actives[0].GetIsPending() != 1 {
		t.Fatal("plan award must remain pending")
	}
	context := &educateConditionContext{Slot: 1, PlanID: 1101}
	drops, err := applyEducateGainBatch(state, [][]int32{{1, 101, 5}}, 1, context)
	if err != nil || drops[0].GetNumber() != 5 || len(state.Lifecycle.NumericLedger.Held[2001]) != 0 {
		t.Fatal("pending modifier or ledger activated early", drops, err)
	}
	if _, err := advanceNewEducateRound(state); err != nil {
		t.Fatal(err)
	}
	if state.Info.Benefit.Actives[0].GetIsPending() != 0 || state.Info.Benefit.Actives[0].GetRound() != 2 || state.Lifecycle.BenefitRounds[2001].Total != 0 {
		t.Fatal("pending activation counted previous round")
	}
	drops, err = applyEducateGainBatch(state, [][]int32{{1, 101, 5}}, 1, context)
	if err != nil || drops[0].GetNumber() != 6 {
		t.Fatal("active real status modifier", drops, err)
	}
	if _, err := applyEducateDropBatch(state, [][]int32{{7, 0, 1}}, 1, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := advanceNewEducateRound(state); err != nil {
		t.Fatal(err)
	}
	if state.Info.Round.GetRound() != 2 || state.Info.Round.GetInTemp() != 1 || state.Lifecycle.BenefitRounds[2001].Total != 1 {
		t.Fatal("temporary round transition was not recorded")
	}
	if err := resetEducateBenefitRoundCounter(state, 2001); err != nil {
		t.Fatal(err)
	}
	if state.Lifecycle.BenefitRounds[2001].Total != 1 || state.Lifecycle.BenefitRounds[2001].SinceReset != 0 || state.Lifecycle.NumericLedger.Held[2001]["1/101"].Positive != 6 {
		t.Fatal("periodic reset erased cumulative held data")
	}
	if err := saveEducateState(state); err != nil {
		t.Fatal(err)
	}
	state, err = loadEducateState(client, 1)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if _, err := advanceNewEducateRound(state); err != nil {
			t.Fatal(err)
		}
	}
	if state.Info.Round.GetRound() != 5 || len(state.Info.Benefit.Actives) != 0 || state.Lifecycle.NumericLedger.Held[2001] != nil {
		t.Fatal("duration expiry did not remove held state")
	}
	if _, err := applyEducateDropBatch(state, [][]int32{{4, 2001, 1}}, 1, nil); err != nil {
		t.Fatal(err)
	}
	if state.Lifecycle.BenefitRounds[2001].Total != 0 || state.Lifecycle.NumericLedger.Held[2001] != nil {
		t.Fatal("reacquisition reused previous held counters")
	}
}

func TestRecoveryS03AbsoluteSetAndRemovalContext(t *testing.T) {
	client := recoveryDB(t)
	state, err := loadEducateState(client, 1)
	if err != nil {
		t.Fatal(err)
	}
	state.Info.Fsm.SystemNo = proto.Uint32(newEducateSystemMap)
	before := educateKVCount(state.Info.Res.Attrs, 104)
	drops, err := applyEducateDropBatch(state, [][]int32{{4, 2035, 1}}, 1, nil)
	if err != nil || educateKVCount(state.Info.Res.Attrs, 104) != 40 || drops[0].GetNumber() != int32(40-before) {
		t.Fatal("effect 2 must set absolute value", drops, err)
	}
	if _, err := applyEducateDropBatch(state, [][]int32{{4, 1001, 1}}, 1, nil); err != nil {
		t.Fatal(err)
	}
	// Synthetic observer in the isolated test schema uses real removal condition
	// 28. It tests dispatch, not an invented official effect implementation.
	if err := orm.UpsertConfigEntry("ShareCfg/child2_benefit_list.json", "900010", json.RawMessage(`{"id":900010,"character":1,"type":2,"during_time":-1,"content":[900010]}`)); err != nil {
		t.Fatal(err)
	}
	if err := orm.UpsertConfigEntry("ShareCfg/child2_benefit.json", "900010", json.RawMessage(`{"id":900010,"trigger":15,"condition":["&&",[28]],"effect":[[1,[1,101,10]]]}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := applyEducateDropBatch(state, [][]int32{{4, 900010, 1}}, 1, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := applyEducateNumericBatch(state, [][]int32{{2, 1, 10}}, 1, true); err != nil {
		t.Fatal(err)
	}
	attr := educateKVCount(state.Info.Res.Attrs, 101)
	drops, err = applyEducateDropBatch(state, [][]int32{{4, 1001, -1}}, 1, nil)
	if err != nil || educateKVCount(state.Info.Res.Attrs, 101) != attr+10 || state.Lifecycle.NumericLedger.Held[1001] != nil {
		t.Fatal("actual removal context did not dispatch observer", drops, err)
	}
	info := proto.Clone(state.Info)
	life, _ := json.Marshal(state.Lifecycle)
	if _, err := applyEducateDropBatch(state, [][]int32{{2, 1, 1}, {4, 1001, -1}}, 1, nil); err == nil {
		t.Fatal("removing unowned buff accepted")
	}
	after, _ := json.Marshal(state.Lifecycle)
	if !proto.Equal(info, state.Info) || string(life) != string(after) {
		t.Fatal("failed removal paid partially")
	}
}

func TestRecoveryS03AccumulatedConsumptionAndProbabilityNoRetry(t *testing.T) {
	client := recoveryDB(t)
	state, err := loadEducateState(client, 2)
	if err != nil {
		t.Fatal(err)
	}
	state.Info.Fsm.SystemNo = proto.Uint32(newEducateSystemMap)
	if _, err := applyEducateDropBatch(state, [][]int32{{4, 3810331, 1}}, 1, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := applyEducateNumericSet(state, []int32{2, 301, 900}); err != nil {
		t.Fatal(err)
	}
	pay := func(amount int32) {
		t.Helper()
		if _, err := applyEducateNumericBatch(state, [][]int32{{2, 301, amount}}, 1, true); err != nil {
			t.Fatal(err)
		}
	}
	trigger := func(action string, roll uint64) int32 {
		t.Helper()
		context := educateChangeContext(state, action, 0, nil)
		context.Draw = func(uint64) (uint64, error) { return roll, nil }
		drops, err := applyEducateTalentTriggerWithContext(state, 19, 0, context)
		if err != nil {
			t.Fatal(err)
		}
		var result int32
		for _, drop := range drops.BenefitDrop {
			result += drop.GetNumber()
		}
		return result
	}
	pay(250)
	if got := trigger("spend/1", 0); got != 40 {
		t.Fatal("cumulative quotient payout", got)
	}
	if err := saveEducateState(state); err != nil {
		t.Fatal(err)
	}
	state, err = loadEducateState(client, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got := trigger("spend/1", 0); got != 0 {
		t.Fatal("replayed action paid twice", got)
	}
	if got := trigger("spend/2", 0); got != 0 {
		t.Fatal("unchanged cumulative units paid twice", got)
	}
	pay(50)
	if got := trigger("spend/3", 9999); got != 0 {
		t.Fatal("probability failure paid", got)
	}
	if got := trigger("spend/4", 0); got != 0 {
		t.Fatal("failed cumulative probability rerolled old units", got)
	}
	pay(100)
	if got := trigger("spend/5", 0); got != 20 {
		t.Fatal("new units not paid", got)
	}
}

func TestRecoveryS03RoundFailureAtomicAndNestedWindow(t *testing.T) {
	client := recoveryDB(t)
	state, err := loadEducateState(client, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := applyEducateDropBatch(state, [][]int32{{4, 1001, 1}}, 1, nil); err != nil {
		t.Fatal(err)
	}
	state.Lifecycle.BenefitRounds[1001] = educateConditionBuffRounds{Total: math.MaxUint32, SinceReset: 1}
	info := proto.Clone(state.Info)
	life, _ := json.Marshal(state.Lifecycle)
	if _, err := advanceNewEducateRound(state); err == nil {
		t.Fatal("round counter overflow accepted")
	}
	after, _ := json.Marshal(state.Lifecycle)
	if !proto.Equal(info, state.Info) || string(life) != string(after) {
		t.Fatal("failed boundary changed round/FSM/counters")
	}
	state.Lifecycle.BenefitRounds[1001] = educateConditionBuffRounds{}
	if _, err := applyEducateDropBatch(state, [][]int32{{4, 2041, 1}}, 1, nil); err != nil {
		t.Fatal(err)
	}
	info = proto.Clone(state.Info)
	life, _ = json.Marshal(state.Lifecycle)
	if _, err := advanceNewEducateRound(state); err == nil {
		t.Fatal("unrecovered private removal effect accepted")
	}
	after, _ = json.Marshal(state.Lifecycle)
	if !proto.Equal(info, state.Info) || string(life) != string(after) {
		t.Fatal("private effect failure partially advanced round")
	}
	state, err = loadEducateState(client, 2)
	if err != nil {
		t.Fatal(err)
	}
	absent := educateChangeContext(state, "known/absent", 0, nil)
	if matched, err := evaluateEducateConditionWithContext(state, json.RawMessage(`38104911`), absent); err != nil || matched {
		t.Fatal("known absent buff was treated as missing context", matched, err)
	}
	context := &educateConditionContext{Changes: map[string]educateNumericChange{"2/301": {Negative: 1}}, RoundChanges: map[string]educateNumericChange{"2/301": {Negative: 250}}}
	matched, err := evaluateEducateConditionWithContext(state, json.RawMessage(`["&&",[["||",[38103311]],38103312,38103313,"${num}"]]`), context)
	if err != nil || !matched || context.Multiplier != 2 || context.window != 18 {
		t.Fatal("OR lost numeric window", matched, context.Multiplier, err)
	}
}
