package neweducate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
	"sync"
	"testing"
)

func TestRecoveryS02IllegalQueriesPreservePlayingPlan(t *testing.T) {
	client := recoveryDB(t)
	recoveryPacket(t, client, NewEducateRequest, &protobuf.CS_29001{Id: proto.Uint32(2)}, &protobuf.SC_29002{})
	state, err := loadEducateState(client, 2)
	if err != nil {
		t.Fatal(err)
	}
	state.Info.Fsm.SystemNo = proto.Uint32(newEducateSystemPlan)
	state.Info.Fsm.CurrentNode = proto.Uint32(120101)
	state.Info.Fsm.Cache[0].CachePlan[0].Plans = []*protobuf.KVDATA{{Key: proto.Uint32(1), Value: proto.Uint32(1201)}}
	if err := saveEducateState(state); err != nil {
		t.Fatal(err)
	}
	before, err := orm.GetCommanderTB(90001, 2)
	if err != nil {
		t.Fatal(err)
	}
	var choose protobuf.SC_29127
	recoveryPacket(t, client, NewEducateGetChoose, &protobuf.CS_29126{Id: proto.Uint32(2)}, &choose)
	if choose.GetResult() == 0 {
		t.Fatal("get choose illegally abandons a pending course")
	}
	recoveryPacket(t, client, NewEducateGetMap, &protobuf.CS_29060{Id: proto.Uint32(2)}, &protobuf.SC_29061{})
	recoveryPacket(t, client, NewEducateGetTopics, &protobuf.CS_29015{Id: proto.Uint32(2)}, &protobuf.SC_29016{})
	var phase protobuf.SC_29026
	recoveryPacket(t, client, NewEducateChangePhase, &protobuf.CS_29025{Id: proto.Uint32(2)}, &phase)
	if phase.GetResult() == 0 {
		t.Fatal("phase advances unfinished course")
	}
	var reset protobuf.SC_29008
	recoveryPacket(t, client, NewEducateReset, &protobuf.CS_29007{Id: proto.Uint32(2), Difficulty: proto.Uint32(0)}, &reset)
	if reset.GetResult() == 0 {
		t.Fatal("reset discards unfinished run")
	}
	after, err := orm.GetCommanderTB(90001, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before.State, after.State) || !bytes.Equal(before.Permanent, after.Permanent) || !bytes.Equal(before.Metadata, after.Metadata) || before.Revision != after.Revision {
		t.Fatal("rejected/query requests modified stored course")
	}
}

func TestDefaultsDoNotCompleteUnloadedTalent(t *testing.T) {
	info := tbInfoPlaceholder()
	info.Fsm.SystemNo = proto.Uint32(newEducateSystemChoose)
	ensureTBInfoDefaults(info)
	if info.Fsm.Cache[0].CacheTalent[0].GetFinished() != 0 {
		t.Fatal("placeholder was treated as successfully loaded empty talent")
	}
}

func TestRecoveryS02LoadedEmptyAndPendingRecovery(t *testing.T) {
	client := recoveryDB(t)
	recoveryPacket(t, client, NewEducateRequest, &protobuf.CS_29001{Id: proto.Uint32(2)}, &protobuf.SC_29002{})
	state, err := loadEducateState(client, 2)
	if err != nil {
		t.Fatal(err)
	}
	state.Info.Round.Round = proto.Uint32(2)
	state.Info.Fsm.SystemNo = proto.Uint32(newEducateSystemEvent)
	state.Lifecycle = freshEducateLifecycle(state.Info)
	if err := saveEducateState(state); err != nil {
		t.Fatal(err)
	}
	var talents protobuf.SC_29020
	recoveryPacket(t, client, NewEducateGetTalents, &protobuf.CS_29019{Id: proto.Uint32(2)}, &talents)
	if talents.GetResult() != 0 || len(talents.Talents) != 0 {
		t.Fatal("round two empty talent fixture")
	}
	state, err = loadEducateState(client, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !state.Lifecycle.Stages[newEducateSystemTalent].Loaded || !state.Lifecycle.Stages[newEducateSystemTalent].Completed {
		t.Fatal("loaded empty talent lost on reload")
	}
	var choice protobuf.SC_29127
	recoveryPacket(t, client, NewEducateGetChoose, &protobuf.CS_29126{Id: proto.Uint32(2)}, &choice)
	if choice.GetResult() != 0 || choice.Fsm.GetSystemNo() != newEducateSystemChoose || len(choice.Fsm.Cache[0].CacheSite) != 0 {
		t.Fatal("choose must not advertise unloaded map")
	}
	recoveryPacket(t, client, NewEducateGetMap, &protobuf.CS_29060{Id: proto.Uint32(2)}, &protobuf.SC_29061{})
	recoveryPacket(t, client, NewEducateGetTopics, &protobuf.CS_29015{Id: proto.Uint32(2)}, &protobuf.SC_29016{})
	var parallel protobuf.SC_29002
	recoveryPacket(t, client, NewEducateRequest, &protobuf.CS_29001{Id: proto.Uint32(2)}, &parallel)
	if parallel.Tb.Fsm.GetSystemNo() != newEducateSystemMap || len(parallel.Tb.Fsm.Cache[0].CacheChat) != 1 || parallel.Tb.Fsm.Cache[0].CacheChat[0].GetFinished() != 1 {
		t.Fatal("parallel empty topics changed main phase or lost client completion")
	}
	for _, system := range []uint32{0, 1, 2, 10, 4, 5, 6, 7, 8, 9} {
		state, err = loadEducateState(client, 2)
		if err != nil {
			t.Fatal(err)
		}
		state.Info.Fsm.SystemNo = proto.Uint32(system)
		state.Info.Fsm.CurrentNode = proto.Uint32(120101)
		state.Info.Fsm.TarotSelects = []uint32{1000}
		state.Info.Fsm.PriorityFsm = []*protobuf.TBFSM{{SystemNo: proto.Uint32(100), CurrentNode: proto.Uint32(0), Cache: []*protobuf.TBFSMCACHE{{CacheNin1: []*protobuf.TBFSMCACHENIN1{{IsFromShop: proto.Uint32(0)}}}}}}
		// MIND can be loaded independently of the main stage.
		markEducateStage(state, newEducateSystemMind, true)
		if err := saveEducateState(state); err != nil {
			t.Fatal(err)
		}
		var request protobuf.SC_29002
		recoveryPacket(t, client, NewEducateRequest, &protobuf.CS_29001{Id: proto.Uint32(2)}, &request)
		var refresh protobuf.SC_29093
		recoveryPacket(t, client, NewEducateRefresh, &protobuf.CS_29092{Id: proto.Uint32(2), Difficulty: proto.Uint32(0)}, &refresh)
		if !proto.Equal(request.Tb.Fsm, refresh.Tb.Fsm) || request.Tb.Fsm.GetCurrentNode() != 120101 || len(request.Tb.Fsm.PriorityFsm) != 1 || len(request.Tb.Fsm.TarotSelects) != 1 || len(request.Tb.Fsm.Cache[0].CacheMind) != 1 {
			t.Fatalf("stage %d changed pending/projection", system)
		}
	}
	// Invalid configuration must not persist an empty completed cache, even
	// after weighted generation is implemented.
	recoveryPacket(t, client, NewEducateRequest, &protobuf.CS_29001{Id: proto.Uint32(1)}, &protobuf.SC_29002{})
	state, err = loadEducateState(client, 1)
	if err != nil {
		t.Fatal(err)
	}
	state.Info.Fsm.SystemNo = proto.Uint32(newEducateSystemEvent)
	if err := saveEducateState(state); err != nil {
		t.Fatal(err)
	}
	round, ok, err := loadCurrentNewEducateRoundConfig(state.Info)
	if err != nil || !ok {
		t.Fatal("missing round fixture", err)
	}
	round.BenefitSelect = json.RawMessage(`[[1001,0]]`)
	raw, err := json.Marshal(round)
	if err != nil {
		t.Fatal(err)
	}
	if err := orm.UpsertConfigEntry(newEducateRoundCategory, fmt.Sprint(round.ID), raw); err != nil {
		t.Fatal(err)
	}
	before, _ := orm.GetCommanderTB(90001, 1)
	recoveryPacket(t, client, NewEducateGetTalents, &protobuf.CS_29019{Id: proto.Uint32(1)}, &talents)
	after, _ := orm.GetCommanderTB(90001, 1)
	if talents.GetResult() == 0 || !bytes.Equal(before.State, after.State) || !bytes.Equal(before.Metadata, after.Metadata) || before.Revision != after.Revision {
		t.Fatal("failed generation became completed empty talent")
	}
}

func TestRecoveryS02LockedAdvanceAndRollback(t *testing.T) {
	client := recoveryDB(t)
	recoveryPacket(t, client, NewEducateRequest, &protobuf.CS_29001{Id: proto.Uint32(2)}, &protobuf.SC_29002{})
	state, err := loadEducateState(client, 2)
	if err != nil {
		t.Fatal(err)
	}
	state.Info.Fsm.SystemNo = proto.Uint32(newEducateSystemAssess)
	markEducateStage(state, newEducateSystemAssess, true)
	if err := saveEducateState(state); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan uint32, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := &connection.Client{Commander: client.Commander}
			var response protobuf.SC_29026
			data, _ := proto.Marshal(&protobuf.CS_29025{Id: proto.Uint32(2)})
			_, _, err := NewEducateChangePhase(&data, c)
			if err != nil {
				results <- 99
				return
			}
			wire := c.Buffer.Bytes()
			if err := proto.Unmarshal(wire[7:], &response); err != nil {
				results <- 99
				return
			}
			results <- response.GetResult()
		}()
	}
	wg.Wait()
	close(results)
	successes := 0
	for result := range results {
		if result == 0 {
			successes++
		}
		if result == 99 {
			t.Fatal("missing failure/success frame")
		}
	}
	state, err = loadEducateState(client, 2)
	if err != nil {
		t.Fatal(err)
	}
	if successes != 1 || state.Info.Round.GetRound() != 2 || len(state.Lifecycle.Stages) != 0 {
		t.Fatal("concurrent phase duplicated/reset stale lifecycle")
	}
	before, _ := orm.GetCommanderTB(90001, 2)
	injected := errors.New("rollback action")
	_, err = updateEducateState(client, 2, func(s *educateState) error {
		s.Info.Name = proto.String("lost")
		s.Permanent.Endings = []uint32{999}
		markEducateStage(s, 9, true)
		return injected
	})
	if !errors.Is(err, injected) {
		t.Fatal(err)
	}
	after, _ := orm.GetCommanderTB(90001, 2)
	if !bytes.Equal(before.State, after.State) || !bytes.Equal(before.Permanent, after.Permanent) || !bytes.Equal(before.Metadata, after.Metadata) || before.Revision != after.Revision {
		t.Fatal("failed action committed partial snapshot")
	}
}
