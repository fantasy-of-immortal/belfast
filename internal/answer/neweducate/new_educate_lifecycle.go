package neweducate

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

var errEducatePhase = errors.New("educate action is not legal in this phase")

func logEducateFailure(client *connection.Client, id uint32, packet uint32, err error) {
	commander := uint32(0)
	if client != nil && client.Commander != nil {
		commander = client.Commander.CommanderID
	}
	logger.LogEvent("NewEducate", "Action", fmt.Sprintf("commander=%d character=%d packet=%d failed: %v", commander, id, packet, err), logger.LOG_LEVEL_ERROR)
}

func boolEducateUint(value bool) uint32 {
	if value {
		return 1
	}
	return 0
}

// Response construction happens after this returns: never publish success
// before the locked state, permanent data and lifecycle commit together.
func updateEducateState(client *connection.Client, id uint32, action func(*educateState) error) (*educateState, error) {
	if client == nil || client.Commander == nil {
		return nil, fmt.Errorf("educate commander required")
	}
	if err := orm.ValidateEducateCharacter(id); err != nil {
		return nil, err
	}
	var result *educateState
	err := orm.UpdateCommanderTB(client.Commander.CommanderID, id, func(entry *orm.CommanderTB, info *protobuf.TBINFO, permanent *protobuf.TBPERMANENT) error {
		state := &educateState{Entry: entry, Info: info, Permanent: permanent}
		if err := loadEducateLifecycle(state); err != nil {
			return err
		}
		state.Info = ensureTBInfoDefaults(info)
		state.Permanent = ensureTBPermanentDefaults(permanent)
		if err := seedNewEducateDefaultRes(state.Info, id); err != nil {
			return err
		}
		if err := action(state); err != nil {
			return err
		}
		// Keep the decoded protobuf pointer used by the transaction, including reset.
		if state.Info != info || state.Permanent != permanent {
			return fmt.Errorf("transaction action replaced protobuf pointer")
		}
		if err := storeEducateLifecycle(state); err != nil {
			return err
		}
		result = state
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Loaded empty results differ from placeholders and from completed actions.
// The protocol has no fields for these distinctions, so they live in metadata.
type educateStage struct {
	Loaded    bool `json:"loaded"`
	Completed bool `json:"completed"`
}
type educateLifecycle struct {
	Version        int                      `json:"version"`
	Round          uint32                   `json:"round"`
	TempRound      uint32                   `json:"temp_round"`
	Stages         map[uint32]educateStage  `json:"stages"`
	Schedule       *educateScheduleProgress `json:"schedule,omitempty"`
	Chain          *educateNodeChain        `json:"chain,omitempty"`
	ConditionDraws map[string]uint32        `json:"condition_draws,omitempty"`
	NumericLedger  *educateNumericLedger    `json:"numeric_ledger,omitempty"`
	RoundSites     []educateConditionSite   `json:"round_sites,omitempty"`
}

type educateCourseProgress struct {
	PlanID         uint32             `json:"plan_id"`
	Status         string             `json:"status"`
	Node           uint32             `json:"node"`
	Rewards        []*protobuf.TBDROP `json:"rewards,omitempty"`
	BenefitRewards []*protobuf.TBDROP `json:"benefit_rewards,omitempty"`
	Restarts       uint32             `json:"restarts,omitempty"`
}
type educateScheduleProgress struct {
	Version         int64                             `json:"version"`
	Slots           map[uint32]*educateCourseProgress `json:"slots"`
	SummaryComplete bool                              `json:"summary_complete"`
	ExtraComplete   bool                              `json:"extra_complete"`
	Extra           []*protobuf.TBDROP                `json:"extra,omitempty"`
	Source          string                            `json:"source,omitempty"`
}

func freshEducateLifecycle(info *protobuf.TBINFO) *educateLifecycle {
	return &educateLifecycle{Version: 1, Round: info.Round.GetRound(), TempRound: info.Round.GetTempRound(), Stages: map[uint32]educateStage{}}
}

// One-time compatibility inference for old protobuf saves. This is an explicit
// semantic map, not a comparison of numeric system IDs (CHOOSE is 10).
func legacyEducateLifecycle(info *protobuf.TBINFO) *educateLifecycle {
	life := freshEducateLifecycle(info)
	if info.Fsm == nil || len(info.Fsm.Cache) == 0 {
		return life
	}
	cache := info.Fsm.Cache[0]
	mark := func(stage uint32, completed bool) {
		life.Stages[stage] = educateStage{Loaded: true, Completed: completed}
	}
	switch info.Fsm.GetSystemNo() {
	case newEducateSystemTalent:
		if len(cache.CacheTalent) > 0 && (cache.CacheTalent[0].GetFinished() != 0 || len(cache.CacheTalent[0].Talents) > 0) {
			mark(newEducateSystemTalent, cache.CacheTalent[0].GetFinished() != 0)
		}
	case newEducateSystemChoose, newEducateSystemTopic, newEducateSystemMind, newEducateSystemMap, newEducateSystemPlan, newEducateSystemAssess, newEducateSystemPhase, newEducateSystemEnding:
		mark(newEducateSystemTalent, true)
	}
	if info.Fsm.GetSystemNo() == newEducateSystemMind {
		mark(newEducateSystemMind, true)
	}
	switch info.Fsm.GetSystemNo() {
	case newEducateSystemMap, newEducateSystemPlan, newEducateSystemAssess, newEducateSystemPhase, newEducateSystemEnding:
		mark(newEducateSystemMap, true)
	}
	if len(cache.CacheChat) > 0 && (cache.CacheChat[0].GetFinished() != 0 || len(cache.CacheChat[0].Chats) > 0) {
		mark(newEducateSystemTopic, cache.CacheChat[0].GetFinished() != 0)
	}
	if len(cache.CacheEnd) > 0 && len(cache.CacheEnd[0].Ends) > 0 {
		mark(newEducateSystemEnding, cache.CacheEnd[0].GetSelect() != 0)
	}
	if len(cache.CacheEval) > 0 {
		mark(newEducateSystemAssess, cache.CacheEval[0].GetIsFinished() != 0)
	}
	if len(cache.CachePlan) > 0 && len(cache.CachePlan[0].Plans) > 0 {
		mark(newEducateSystemPlan, false)
	}
	return life
}

func loadEducateLifecycle(state *educateState) error {
	metadata := map[string]json.RawMessage{}
	if len(state.Entry.Metadata) > 0 {
		if err := json.Unmarshal(state.Entry.Metadata, &metadata); err != nil {
			return fmt.Errorf("educate metadata: %w", err)
		}
	}
	if metadata == nil {
		return fmt.Errorf("educate metadata must be an object")
	}
	if raw, ok := metadata["lifecycle"]; ok {
		life := &educateLifecycle{}
		if err := json.Unmarshal(raw, life); err != nil {
			return fmt.Errorf("educate lifecycle: %w", err)
		}
		if life.Version != 1 || life.Stages == nil || life.Round != state.Info.Round.GetRound() || life.TempRound != state.Info.Round.GetTempRound() {
			return fmt.Errorf("educate lifecycle version/round mismatch")
		}
		state.Lifecycle = life
		// A cancelled main presentation remains the same pending instance.
		// Repair the earlier EVENT/0 response which let the client skip it.
		if chain := life.Chain; chain != nil && chain.Source == "round.main_event_node_id" && chain.Restarts > 0 && chain.Current == 0 && state.Info.Fsm != nil && state.Info.Fsm.GetCurrentNode() == 0 {
			if !chain.Completed && !life.Stages[newEducateSystemEvent].Loaded {
				state.Info.Fsm.SystemNo = proto.Uint32(0)
			} else if chain.Completed && state.Info.Fsm.GetSystemNo() == newEducateSystemEvent && life.Stages[newEducateSystemTalent].Completed && life.Stages[newEducateSystemMap].Loaded {
				// The client reuses its previously loaded future caches after
				// replay; it does not send talent/map load packets again.
				state.Info.Fsm.SystemNo = proto.Uint32(newEducateSystemMap)
			}
		}
		// The current topics command marks a successfully loaded empty list
		// finished. Repair the old server's opposite bit only when metadata
		// proves that the query succeeded; placeholders stay unloaded.
		if life.Stages[newEducateSystemTopic].Loaded && state.Info.Fsm != nil && len(state.Info.Fsm.Cache) > 0 {
			cache := state.Info.Fsm.Cache[0]
			if len(cache.CacheChat) > 0 && len(cache.CacheChat[0].Chats) == 0 {
				life.Stages[newEducateSystemTopic] = educateStage{Loaded: true, Completed: true}
				cache.CacheChat[0].Finished = proto.Uint32(1)
			}
		}
	} else {
		state.Lifecycle = legacyEducateLifecycle(state.Info)
	}
	return nil
}

func storeEducateLifecycle(state *educateState) error {
	if state.Lifecycle == nil {
		state.Lifecycle = legacyEducateLifecycle(state.Info)
	}
	metadata := map[string]json.RawMessage{}
	if len(state.Entry.Metadata) > 0 {
		if err := json.Unmarshal(state.Entry.Metadata, &metadata); err != nil {
			return err
		}
	}
	if metadata == nil {
		return fmt.Errorf("educate metadata must be an object")
	}
	raw, err := json.Marshal(state.Lifecycle)
	if err != nil {
		return err
	}
	metadata["lifecycle"] = raw
	raw, err = json.Marshal(metadata)
	if err != nil {
		return err
	}
	state.Entry.Metadata = raw
	return nil
}

func markEducateStage(state *educateState, stage uint32, complete bool) {
	if state.Lifecycle == nil {
		state.Lifecycle = legacyEducateLifecycle(state.Info)
	}
	state.Lifecycle.Stages[stage] = educateStage{Loaded: true, Completed: complete}
}

func educateHasPending(info *protobuf.TBINFO) bool {
	return info.Fsm.GetCurrentNode() != 0 || len(info.Fsm.PriorityFsm) > 0 || len(info.Fsm.TarotSelects) > 0
}

func educateCanChoose(state *educateState) bool {
	if educateHasPending(state.Info) {
		return false
	}
	switch state.Info.Fsm.GetSystemNo() {
	case newEducateSystemChoose:
		return true
	case newEducateSystemTalent:
		return state.Lifecycle.Stages[newEducateSystemTalent].Completed
	default:
		return false
	}
}

func educateCanChangePhase(state *educateState) bool {
	return state.Info.Fsm.GetSystemNo() == newEducateSystemAssess && state.Lifecycle.Stages[newEducateSystemAssess].Completed && !educateHasPending(state.Info)
}
