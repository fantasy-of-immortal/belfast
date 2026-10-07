package neweducate

import (
	"testing"

	"google.golang.org/protobuf/proto"
)

func TestDefaultEducateStateBuildsUsablePlaceholder(t *testing.T) {
	state := defaultEducateState(77, 3)

	if state.Entry == nil || state.Entry.CommanderID != 77 {
		t.Fatalf("expected placeholder entry for commander 77")
	}
	if state.Info == nil || state.Info.GetId() != 3 {
		t.Fatalf("expected tb id 3, got %d", state.Info.GetId())
	}
	if state.Permanent == nil {
		t.Fatalf("expected permanent state")
	}
	if state.Info.GetFsm() == nil || len(state.Info.GetFsm().GetCache()) == 0 {
		t.Fatalf("expected placeholder FSM cache")
	}
	if state.Info.GetDisplay() == nil {
		t.Fatalf("expected placeholder display")
	}
}

func TestAdvanceNewEducateRoundHandlesTempRoundsAndMaxRound(t *testing.T) {
	state := &educateState{
		Info:      ensureTBInfoDefaults(tbInfoPlaceholder()),
		Permanent: ensureTBPermanentDefaults(tbPermanentPlaceholder()),
	}
	state.Info.Round.Round = proto.Uint32(5)
	state.Info.Round.TempRound = proto.Uint32(2)
	state.Info.Fsm.SystemNo = proto.Uint32(newEducateSystemMap)
	state.Info.Site.Characters = []uint32{10, 11}
	state.Info.EvalFail = proto.Uint32(1)
	state.Permanent.MaxRound = proto.Uint32(4)

	advanceNewEducateRound(state)

	if state.Info.Round.GetRound() != 5 {
		t.Fatalf("expected temp round to keep round 5, got %d", state.Info.Round.GetRound())
	}
	if state.Info.Round.GetInTemp() != 1 || state.Info.Round.GetTempRound() != 1 {
		t.Fatalf("expected in_temp=1 temp_round=1, got in_temp=%d temp_round=%d", state.Info.Round.GetInTemp(), state.Info.Round.GetTempRound())
	}
	if state.Permanent.GetMaxRound() != 5 {
		t.Fatalf("expected max round 5, got %d", state.Permanent.GetMaxRound())
	}
	if state.Info.GetEvalFail() != 0 {
		t.Fatalf("expected eval_fail reset")
	}
	if state.Info.Fsm.GetSystemNo() != 0 || state.Info.Fsm.GetCurrentNode() != 0 {
		t.Fatalf("expected FSM reset, got system=%d node=%d", state.Info.Fsm.GetSystemNo(), state.Info.Fsm.GetCurrentNode())
	}
	if len(state.Info.Site.GetCharacters()) != 0 {
		t.Fatalf("expected site characters cleared, got %v", state.Info.Site.GetCharacters())
	}

	advanceNewEducateRound(state)

	if state.Info.Round.GetRound() != 5 || state.Info.Round.GetInTemp() != 1 || state.Info.Round.GetTempRound() != 0 {
		t.Fatalf("expected second temp round consumption, got round=%d in_temp=%d temp_round=%d", state.Info.Round.GetRound(), state.Info.Round.GetInTemp(), state.Info.Round.GetTempRound())
	}

	advanceNewEducateRound(state)

	if state.Info.Round.GetRound() != 6 || state.Info.Round.GetInTemp() != 0 {
		t.Fatalf("expected normal round advance to 6, got round=%d in_temp=%d", state.Info.Round.GetRound(), state.Info.Round.GetInTemp())
	}
	if state.Permanent.GetMaxRound() != 6 {
		t.Fatalf("expected max round 6, got %d", state.Permanent.GetMaxRound())
	}
}
