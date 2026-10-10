package neweducate

import (
	"context"
	"errors"
	"fmt"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const (
	newEducateSystemEvent  = 1
	newEducateSystemTalent = 2
	newEducateSystemTopic  = 3
	newEducateSystemMap    = 4
	newEducateSystemPlan   = 5
	newEducateSystemAssess = 6
	newEducateSystemPhase  = 7
	newEducateSystemEnding = 8
	newEducateSystemMind   = 9
	newEducateSystemChoose = 10

	newEducateSiteStateEvent  = 1
	newEducateSiteStateNormal = 2
	newEducateSiteStateShip   = 3
)

type educateState struct {
	Entry       *orm.CommanderTB
	Info        *protobuf.TBINFO
	Permanent   *protobuf.TBPERMANENT
	Lifecycle   *educateLifecycle
	EffectDepth uint32
}

func loadEducateState(client *connection.Client, tbID uint32) (*educateState, error) {
	if client == nil || client.Commander == nil {
		return nil, fmt.Errorf("educate requires a commander")
	}
	if err := orm.ValidateEducateCharacter(tbID); err != nil {
		return nil, err
	}
	entry, err := orm.GetCommanderTB(client.Commander.CommanderID, tbID)
	if err != nil {
		if !errors.Is(err, db.ErrNotFound) {
			return nil, err
		}
		var pending bool
		if err := db.DefaultStore.Pool.QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM commander_tbs l WHERE commander_id=$1 AND NOT EXISTS(SELECT 1 FROM commander_educate_migrations m WHERE m.commander_id=l.commander_id AND m.report->>'status'='migrated'))`, int64(client.Commander.CommanderID)).Scan(&pending); err != nil {
			return nil, err
		}
		if pending {
			return nil, fmt.Errorf("legacy educate save requires reviewed migration")
		}
		state := defaultEducateState(client.Commander.CommanderID, tbID)
		if err := seedNewEducateDefaultRes(state.Info, tbID); err != nil {
			return nil, err
		}
		if err := saveEducateState(state); err != nil {
			return nil, err
		}
		return state, nil
	}
	info, permanent, err := entry.Decode()
	if err != nil {
		return nil, err
	}
	state := &educateState{Entry: entry, Info: info, Permanent: permanent}
	if err := loadEducateLifecycle(state); err != nil {
		return nil, err
	}
	info = ensureTBInfoDefaults(info)
	permanent = ensureTBPermanentDefaults(permanent)
	if info.GetId() != tbID {
		return nil, fmt.Errorf("requested character differs from stored character")
	}
	if err := seedNewEducateDefaultRes(info, tbID); err != nil {
		return nil, err
	}
	state.Info, state.Permanent = info, permanent
	return state, nil
}

func saveEducateState(state *educateState) error {
	if err := storeEducateLifecycle(state); err != nil {
		return err
	}
	return orm.SaveCommanderTB(state.Entry, state.Info, state.Permanent)
}

// Internal placeholders make handlers safe to index, but their presence on
// the wire tells the client that a system has already been requested. Sending
// a future talent cache while still in EVENT skips CS_29019 and loops forever.
func newEducateClientInfo(info *protobuf.TBINFO, lifecycles ...*educateLifecycle) *protobuf.TBINFO {
	out := proto.Clone(info).(*protobuf.TBINFO)
	// Old saves can contain another character's resource IDs. Preserve them
	// internally, but never show duplicate attributes from two characters.
	if attrs, err := listNewEducateConfigs[newEducateAttrConfig](newEducateAttrCategory); err == nil && len(attrs) > 0 {
		allowed := make(map[uint32]bool)
		for _, attr := range attrs {
			if attr.Character == out.GetId() {
				allowed[attr.ID] = true
			}
		}
		if len(allowed) > 0 {
			out.Res.Attrs = filterNewEducateValues(out.Res.Attrs, allowed)
		}
	}
	if resources, err := listNewEducateConfigs[newEducateResourceConfig](newEducateResourceCategory); err == nil && len(resources) > 0 {
		allowed := make(map[uint32]bool)
		for _, resource := range resources {
			if resource.Character == out.GetId() {
				allowed[resource.ID] = true
			}
		}
		if len(allowed) > 0 {
			out.Res.Resource = filterNewEducateValues(out.Res.Resource, allowed)
		}
	}
	life := legacyEducateLifecycle(info)
	if len(lifecycles) > 0 && lifecycles[0] != nil {
		life = lifecycles[0]
	}
	cache := out.Fsm.Cache[0]
	if !life.Stages[newEducateSystemTalent].Loaded {
		cache.CacheTalent = nil
	} else if len(cache.CacheTalent) > 0 {
		cache.CacheTalent[0].Finished = proto.Uint32(boolEducateUint(life.Stages[newEducateSystemTalent].Completed))
	}
	if !life.Stages[newEducateSystemTopic].Loaded {
		cache.CacheChat = nil
	} else if len(cache.CacheChat) > 0 {
		cache.CacheChat[0].Finished = proto.Uint32(boolEducateUint(life.Stages[newEducateSystemTopic].Completed))
	}
	if !life.Stages[newEducateSystemMap].Loaded {
		cache.CacheSite = nil
	}
	if !life.Stages[newEducateSystemEnding].Loaded {
		cache.CacheEnd = nil
	}
	if !life.Stages[newEducateSystemMind].Loaded {
		cache.CacheMind = nil
	}
	if !life.Stages[newEducateSystemAssess].Loaded {
		cache.CacheEval = nil
	}
	return out
}

func advanceNewEducateChoose(state *educateState) {
	state.Info.Fsm.SystemNo = proto.Uint32(newEducateSystemChoose)
}

func finishEmptyNewEducateTalent(cache *protobuf.TBFSMCACHETALENT) {
	if len(cache.Talents) == 0 {
		cache.Finished = proto.Uint32(1)
	}
}

func ensureTBInfoDefaults(info *protobuf.TBINFO) *protobuf.TBINFO {
	defaults := tbInfoPlaceholder()
	if info == nil {
		return defaults
	}
	if info.Fsm == nil {
		info.Fsm = defaults.Fsm
	}
	if len(info.Fsm.Cache) == 0 {
		info.Fsm.Cache = defaults.Fsm.Cache
	}
	cache := info.Fsm.Cache[0]
	if len(cache.CachePlan) == 0 {
		cache.CachePlan = defaults.Fsm.Cache[0].CachePlan
	}
	if len(cache.CacheTalent) == 0 {
		cache.CacheTalent = defaults.Fsm.Cache[0].CacheTalent
	}
	if len(cache.CacheSite) == 0 {
		cache.CacheSite = defaults.Fsm.Cache[0].CacheSite
	}
	if len(cache.CacheChat) == 0 {
		cache.CacheChat = defaults.Fsm.Cache[0].CacheChat
	}
	if len(cache.CacheEnd) == 0 {
		cache.CacheEnd = defaults.Fsm.Cache[0].CacheEnd
	}
	if len(cache.CacheMind) == 0 {
		cache.CacheMind = defaults.Fsm.Cache[0].CacheMind
	}
	if cache.CacheSite[0].RefreshCount == nil {
		cache.CacheSite[0].RefreshCount = defaults.Fsm.Cache[0].CacheSite[0].RefreshCount
	}
	if info.Round == nil {
		info.Round = defaults.Round
	}
	if info.Round.InTemp == nil {
		info.Round.InTemp = defaults.Round.InTemp
	}
	if info.Round.TempRound == nil {
		info.Round.TempRound = defaults.Round.TempRound
	}
	if info.Res == nil {
		info.Res = defaults.Res
	}
	if info.Talent == nil {
		info.Talent = defaults.Talent
	}
	if info.Plan == nil {
		info.Plan = defaults.Plan
	}
	if info.Site == nil {
		info.Site = defaults.Site
	}
	if info.Benefit == nil {
		info.Benefit = defaults.Benefit
	}
	if info.Difficulty == nil {
		info.Difficulty = defaults.Difficulty
	}
	if info.EvalFail == nil {
		info.EvalFail = defaults.EvalFail
	}
	if info.Display == nil {
		info.Display = defaults.Display
	}
	if info.Evaluations == nil {
		info.Evaluations = []*protobuf.KVDATA{}
	}
	return info
}

func ensureTBPermanentDefaults(permanent *protobuf.TBPERMANENT) *protobuf.TBPERMANENT {
	if permanent == nil {
		return tbPermanentPlaceholder()
	}
	if permanent.Polaroids == nil {
		permanent.Polaroids = []uint32{}
	}
	if permanent.Endings == nil {
		permanent.Endings = []uint32{}
	}
	if permanent.ActiveEndings == nil {
		permanent.ActiveEndings = []uint32{}
	}
	if permanent.TarotArchive == nil {
		permanent.TarotArchive = []uint32{}
	}
	if permanent.MaxRound == nil {
		permanent.MaxRound = proto.Uint32(0)
	}
	return permanent
}

func emptyTBDrops() *protobuf.TBDROPS {
	return &protobuf.TBDROPS{
		BaseDrop:    []*protobuf.TBDROP{},
		BenefitDrop: []*protobuf.TBDROP{},
		Display:     emptyTBDisplay(),
	}
}

func ensureEducateCache(info *protobuf.TBINFO) *protobuf.TBFSMCACHE {
	info = ensureTBInfoDefaults(info)
	return info.Fsm.Cache[0]
}
