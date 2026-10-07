package neweducate

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestRecoveryS05MainEntryAndReload(t *testing.T) {
	client := recoveryDB(t)
	for _, tc := range []struct{ round, personality, entry uint32 }{{1, 149, 0}, {3, 149, 3400002}, {3, 150, 3400004}, {5, 149, 3400005}} {
		state, err := loadEducateState(client, 2)
		if err != nil {
			t.Fatal(err)
		}
		state.Info.Round.Round = proto.Uint32(tc.round)
		state.Info.Fsm.SystemNo = proto.Uint32(newEducateSystemEvent)
		state.Info.Fsm.CurrentNode = proto.Uint32(0)
		state.Lifecycle = freshEducateLifecycle(state.Info)
		for _, attr := range state.Info.Res.Attrs {
			if attr.GetKey() == 305 {
				attr.Value = proto.Uint32(tc.personality)
			}
		}
		if err := saveEducateState(state); err != nil {
			t.Fatal(err)
		}
		var entry protobuf.SC_29012
		recoveryPacket(t, client, NewEducateMainEvent, &protobuf.CS_29011{Id: proto.Uint32(2)}, &entry)
		if entry.GetResult() != 0 || entry.GetFirstNode() != tc.entry {
			t.Fatalf("round %d personality %d entry %d: %+v", tc.round, tc.personality, tc.entry, &entry)
		}
		loaded, err := loadEducateState(client, 2)
		if err != nil {
			t.Fatal(err)
		}
		if loaded.Lifecycle.Chain == nil || loaded.Lifecycle.Chain.Entry != tc.entry {
			t.Fatal("lost entry instance")
		}
		version := loaded.Lifecycle.Chain.Version
		recoveryPacket(t, client, NewEducateMainEvent, &protobuf.CS_29011{Id: proto.Uint32(2)}, &entry)
		again, err := loadEducateState(client, 2)
		if err != nil {
			t.Fatal(err)
		}
		if again.Lifecycle.Chain.Version != version || entry.GetFirstNode() != tc.entry {
			t.Fatal("entry regenerated on repeat")
		}
		if tc.entry != 0 {
			before, err := orm.GetCommanderTB(client.Commander.CommanderID, 2)
			if err != nil {
				t.Fatal(err)
			}
			var bad protobuf.SC_29031
			recoveryPacket(t, client, NewEducateTriggerNode, &protobuf.CS_29030{Id: proto.Uint32(2), Branch: proto.Uint32(1)}, &bad)
			after, err := orm.GetCommanderTB(client.Commander.CommanderID, 2)
			if err != nil {
				t.Fatal(err)
			}
			if bad.GetResult() != 1 || !bytes.Equal(before.State, after.State) || !bytes.Equal(before.Metadata, after.Metadata) || before.Revision != after.Revision {
				t.Fatal("illegal branch changed main event")
			}
			var done protobuf.SC_29031
			recoveryPacket(t, client, NewEducateTriggerNode, &protobuf.CS_29030{Id: proto.Uint32(2)}, &done)
			completed, err := loadEducateState(client, 2)
			if err != nil {
				t.Fatal(err)
			}
			if done.GetResult() != 0 || done.GetNextNode() != 0 || !completed.Lifecycle.Chain.Completed || len(completed.Lifecycle.Chain.Steps) != 1 || !completed.Lifecycle.Stages[newEducateSystemEvent].Completed {
				t.Fatal("main event completion lost")
			}
			if !proto.Equal(loaded.Info.Res, completed.Info.Res) {
				t.Fatal("presentation fabricated effects")
			}
			recoveryPacket(t, client, NewEducateMainEvent, &protobuf.CS_29011{Id: proto.Uint32(2)}, &entry)
			if entry.GetResult() != 0 || entry.GetFirstNode() != 0 {
				t.Fatal("completed story replayed")
			}
		}
	}
}

func TestRecoveryS05MainCancelReplayRetainsInstance(t *testing.T) {
	client := recoveryDB(t)
	state, err := loadEducateState(client, 2)
	if err != nil {
		t.Fatal(err)
	}
	state.Info.Round.Round = proto.Uint32(6)
	state.Info.Fsm.SystemNo = proto.Uint32(newEducateSystemEvent)
	state.Lifecycle = freshEducateLifecycle(state.Info)
	if err := saveEducateState(state); err != nil {
		t.Fatal(err)
	}
	var entry protobuf.SC_29012
	recoveryPacket(t, client, NewEducateMainEvent, &protobuf.CS_29011{Id: proto.Uint32(2)}, &entry)
	if entry.GetResult() != 0 || entry.GetFirstNode() != 3400006 {
		t.Fatal("entry", &entry)
	}
	state, err = loadEducateState(client, 2)
	if err != nil {
		t.Fatal(err)
	}
	version := state.Lifecycle.Chain.Version
	resources := proto.Clone(state.Info.Res)
	var next protobuf.SC_29031
	recoveryPacket(t, client, NewEducateTriggerNode, &protobuf.CS_29030{Id: proto.Uint32(2)}, &next)
	if next.GetResult() != 0 || next.GetNextNode() != 3400007 {
		t.Fatal("advance", &next)
	}
	var cleared protobuf.SC_29033
	recoveryPacket(t, client, NewEducateClearNodeChain, &protobuf.CS_29032{Id: proto.Uint32(2)}, &cleared)
	if cleared.GetResult() != 0 || cleared.Fsm.GetCurrentNode() != 0 || cleared.Fsm.GetSystemNo() != 0 {
		t.Fatal("clear", &cleared)
	}
	state, err = loadEducateState(client, 2)
	if err != nil {
		t.Fatal(err)
	}
	if state.Lifecycle.Chain.Version != version || state.Lifecycle.Chain.Restarts != 1 || len(state.Lifecycle.Chain.Steps) != 1 || state.Lifecycle.Stages[newEducateSystemEvent].Loaded {
		t.Fatal("lost replay history")
	}
	// Earlier deployed cancellation returned EVENT/0; the current client
	// proceeded to MAP. Re-entry must recover this precise pending instance.
	state.Info.Fsm.SystemNo = proto.Uint32(newEducateSystemMap)
	markEducateStage(state, newEducateSystemTalent, true)
	markEducateStage(state, newEducateSystemMap, true)
	if err := saveEducateState(state); err != nil {
		t.Fatal(err)
	}
	state, err = loadEducateState(client, 2)
	if err != nil || state.Info.Fsm.GetSystemNo() != 0 {
		t.Fatal("cancelled event skipped on reload", err)
	}
	recoveryPacket(t, client, NewEducateMainEvent, &protobuf.CS_29011{Id: proto.Uint32(2)}, &entry)
	if entry.GetFirstNode() != 3400006 {
		t.Fatal("restart entry", &entry)
	}
	for _, want := range []uint32{3400007, 0} {
		recoveryPacket(t, client, NewEducateTriggerNode, &protobuf.CS_29030{Id: proto.Uint32(2)}, &next)
		if next.GetResult() != 0 || next.GetNextNode() != want {
			t.Fatal("replay", &next)
		}
	}
	state, err = loadEducateState(client, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !state.Lifecycle.Chain.Completed || state.Lifecycle.Chain.Version != version || len(state.Lifecycle.Chain.Steps) != 3 || !proto.Equal(resources, state.Info.Res) {
		t.Fatal("replay changed rewards or instance")
	}
	if !educateCanPlanInput(state) {
		t.Fatal("replayed event lost already loaded map phase")
	}
	before, err := orm.GetCommanderTB(client.Commander.CommanderID, 2)
	if err != nil {
		t.Fatal(err)
	}
	recoveryPacket(t, client, NewEducateClearNodeChain, &protobuf.CS_29032{Id: proto.Uint32(2)}, &cleared)
	after, err := orm.GetCommanderTB(client.Commander.CommanderID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if cleared.GetResult() != 1 || !bytes.Equal(before.State, after.State) || !bytes.Equal(before.Metadata, after.Metadata) || before.Revision != after.Revision {
		t.Fatal("completed event replayed")
	}
}

func TestRecoveryS05MainEventUnknownAndCyclesRollback(t *testing.T) {
	client := recoveryDB(t)
	state, err := loadEducateState(client, 2)
	if err != nil {
		t.Fatal(err)
	}
	state.Info.Round.Round = proto.Uint32(5)
	state.Lifecycle = freshEducateLifecycle(state.Info)
	state.Info.Fsm.SystemNo = proto.Uint32(newEducateSystemEvent)
	if err := saveEducateState(state); err != nil {
		t.Fatal(err)
	}
	recoveryPacket(t, client, NewEducateMainEvent, &protobuf.CS_29011{Id: proto.Uint32(2)}, &protobuf.SC_29012{})
	node, ok, err := loadNewEducateConfigByID[newEducateNodeConfig](newEducateNodeCategory, 3400005)
	if err != nil || !ok {
		t.Fatal(err)
	}
	reject := func() {
		t.Helper()
		before, err := orm.GetCommanderTB(client.Commander.CommanderID, 2)
		if err != nil {
			t.Fatal(err)
		}
		var response protobuf.SC_29031
		recoveryPacket(t, client, NewEducateTriggerNode, &protobuf.CS_29030{Id: proto.Uint32(2)}, &response)
		after, err := orm.GetCommanderTB(client.Commander.CommanderID, 2)
		if err != nil {
			t.Fatal(err)
		}
		if response.GetResult() != 1 || !bytes.Equal(before.State, after.State) || !bytes.Equal(before.Metadata, after.Metadata) || before.Revision != after.Revision {
			t.Fatal("unknown/cyclic node erased progress")
		}
	}
	for _, kind := range []uint32{102, 999, 1} {
		node.Type = kind
		if kind == 1 {
			node.Next = json.RawMessage(`"3400005"`)
		}
		raw, err := json.Marshal(node)
		if err != nil {
			t.Fatal(err)
		}
		if err := orm.UpsertConfigEntry(newEducateNodeCategory, "3400005", raw); err != nil {
			t.Fatal(err)
		}
		reject()
	}
}

func TestRecoveryS05AllConfiguredMainEvents(t *testing.T) {
	client := recoveryDB(t)
	rounds, err := listNewEducateConfigs[newEducateRoundConfig](newEducateRoundCategory)
	if err != nil {
		t.Fatal(err)
	}
	visited := map[uint32]bool{}
	for _, round := range rounds {
		if round.RoundType != newEducateRoundTypeNormal {
			continue
		}
		for _, personality := range []uint32{149, 150} {
			state, err := loadEducateState(client, round.Character)
			if err != nil {
				t.Fatal(err)
			}
			state.Info.Round.Round = proto.Uint32(round.Round)
			state.Info.Round.InTemp = proto.Uint32(0)
			state.Info.Difficulty = proto.Uint32(round.IsHardMode)
			state.Info.Fsm = ensureTBInfoDefaults(tbInfoPlaceholder()).Fsm
			state.Lifecycle = freshEducateLifecycle(state.Info)
			attrs, err := listNewEducateConfigs[newEducateAttrConfig](newEducateAttrCategory)
			if err != nil {
				t.Fatal(err)
			}
			for _, attr := range attrs {
				if attr.Character == round.Character && attr.Type == 2 {
					state.Info.Res.Attrs = upsertKVDATA(state.Info.Res.Attrs, attr.ID, personality)
				}
			}
			if err := startEducateMainEvent(state); err != nil {
				t.Fatalf("round config %d: %v", round.ID, err)
			}
			if state.Lifecycle.Chain.ConfigID != round.ID {
				t.Fatalf("wrong source config: %d != %d", state.Lifecycle.Chain.ConfigID, round.ID)
			}
			initial := proto.Clone(state.Info.Res)
			if err := saveEducateState(state); err != nil {
				t.Fatal(err)
			}
			for state.Info.Fsm.GetCurrentNode() != 0 {
				visited[state.Info.Fsm.GetCurrentNode()] = true
				state, err = loadEducateState(client, round.Character)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := advanceEducatePresentationChain(state, 0); err != nil {
					t.Fatalf("round %d node %d: %v", round.ID, state.Info.Fsm.GetCurrentNode(), err)
				}
				if err := saveEducateState(state); err != nil {
					t.Fatal(err)
				}
			}
			if !state.Lifecycle.Chain.Completed || !proto.Equal(initial, state.Info.Res) {
				t.Fatal("presentation chain changed resources or lost completion")
			}
		}
	}
	if len(visited) != 50 {
		t.Fatalf("main event configuration coverage %d/50", len(visited))
	}
}
