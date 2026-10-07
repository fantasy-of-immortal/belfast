package neweducate

import (
	"testing"

	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestClientSnapshotDoesNotAdvertiseFutureCaches(t *testing.T) {
	info := tbInfoPlaceholder()
	info.Fsm.SystemNo = proto.Uint32(newEducateSystemEvent)
	// Reproduce the persisted EVENT/empty-talent state seen on the device.
	got := newEducateClientInfo(info)
	cache := got.Fsm.Cache[0]
	if len(cache.CacheTalent) != 0 || len(cache.CacheChat) != 0 || len(cache.CacheSite) != 0 {
		t.Fatal("future caches suppress the client's requests and leave EVENT -> TALENT looping")
	}
	if len(info.Fsm.Cache[0].CacheTalent) != 1 {
		t.Fatal("snapshot projection modified the persisted state")
	}
	info.Fsm.SystemNo = proto.Uint32(newEducateSystemMap)
	info.Fsm.Cache[0].CacheTalent[0].Finished = proto.Uint32(1)
	got = newEducateClientInfo(info)
	if len(got.Fsm.Cache[0].CacheTalent) != 1 || got.Fsm.Cache[0].CacheTalent[0].GetFinished() != 1 || len(got.Fsm.Cache[0].CacheSite) != 1 {
		t.Fatal("resuming MAP must retain its loaded caches")
	}
}

func TestRepairMissingCharacterValuesPreservesEarnedAndSpentValues(t *testing.T) {
	values := []*protobuf.KVDATA{{Key: proto.Uint32(101), Value: proto.Uint32(50)}}
	values = appendMissingNewEducateValue(values, 301, 0)
	values = appendMissingNewEducateValue(values, 305, 145)
	values = appendMissingNewEducateValue(values, 301, 50)
	if len(values) != 3 || values[1].GetValue() != 0 {
		t.Fatal("repair must fill missing IDs without refilling existing zero balances")
	}
	got := filterNewEducateValues(values, map[uint32]bool{301: true, 305: true})
	if len(got) != 2 || got[0].GetKey() != 301 || got[1].GetValue() != 145 || len(values) != 3 {
		t.Fatal("wire snapshot must show only this character while retaining stored values")
	}
}

func TestEmptyTalentCandidatesAreFinishedForReconnect(t *testing.T) {
	cache := tbInfoPlaceholder().Fsm.Cache[0].CacheTalent[0]
	finishEmptyNewEducateTalent(cache)
	if cache.GetFinished() != 1 {
		t.Fatal("empty candidates must match the client's MarkFinish on CS_29019")
	}
	cache.Finished = proto.Uint32(0)
	cache.Talents = []uint32{42}
	finishEmptyNewEducateTalent(cache)
	if cache.GetFinished() != 0 {
		t.Fatal("a pending selection must remain unfinished")
	}
}

func TestChooseResponseAdvancesPastTalentWithoutPreloadingMap(t *testing.T) {
	state := &educateState{Info: tbInfoPlaceholder()}
	state.Info.Fsm.SystemNo = proto.Uint32(newEducateSystemTalent)
	state.Info.Fsm.Cache[0].CacheTalent[0].Finished = proto.Uint32(1)
	advanceNewEducateChoose(state)
	got := newEducateClientInfo(state.Info)
	if got.Fsm.GetSystemNo() != newEducateSystemChoose {
		t.Fatal("CS_29126 must not return TALENT and repeat the choice request")
	}
	if len(got.Fsm.Cache[0].CacheSite) != 0 || len(got.Fsm.Cache[0].CacheChat) != 0 {
		t.Fatal("CHOOSE precedes MAP despite having a larger numeric system id")
	}
}
