package neweducate

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

type educateNodeStep struct {
	Revision int64  `json:"revision"`
	Node     uint32 `json:"node"`
	Branch   uint32 `json:"branch"`
	Next     uint32 `json:"next"`
}
type educateNodeChain struct {
	Version     int64              `json:"version"`
	Source      string             `json:"source"`
	ConfigID    uint32             `json:"config_id"`
	Stage       uint32             `json:"stage"`
	Entry       uint32             `json:"entry"`
	Current     uint32             `json:"current"`
	Completed   bool               `json:"completed"`
	Steps       []educateNodeStep  `json:"steps,omitempty"`
	ReplayStart int                `json:"replay_start,omitempty"`
	Restarts    uint32             `json:"restarts,omitempty"`
	RewardRows  [][]int32          `json:"reward_rows,omitempty"`
	Rewards     []*protobuf.TBDROP `json:"rewards,omitempty"`
}
type educateCharacterConfig struct {
	ID               uint32              `json:"id"`
	PersonalityType  uint32              `json:"personality_type"`
	PersonalityParam [][]json.RawMessage `json:"personality_param"`
}

func educatePersonalityTag(state *educateState) (string, error) {
	character, ok, err := loadNewEducateConfigByID[educateCharacterConfig]("ShareCfg/child2_data.json", state.Info.GetId())
	if err != nil {
		return "", err
	}
	if !ok || character.PersonalityType != 1 {
		return "", fmt.Errorf("character personality configuration missing/unsupported")
	}
	attrs, err := listNewEducateConfigs[newEducateAttrConfig](newEducateAttrCategory)
	if err != nil {
		return "", err
	}
	var id uint32
	for _, attr := range attrs {
		if attr.Character == state.Info.GetId() && attr.Type == 2 {
			if id != 0 {
				return "", fmt.Errorf("duplicate personality attribute")
			}
			id = attr.ID
		}
	}
	if id == 0 {
		return "", fmt.Errorf("missing personality attribute")
	}
	value := educateKVCount(state.Info.Res.Attrs, id)
	tag := ""
	seen := map[string]bool{}
	for _, row := range character.PersonalityParam {
		var name string
		var bounds []int64
		if len(row) != 2 || json.Unmarshal(row[0], &name) != nil || name == "" || json.Unmarshal(row[1], &bounds) != nil || len(bounds) != 2 || bounds[0] > bounds[1] || seen[name] {
			return "", fmt.Errorf("invalid character personality range")
		}
		seen[name] = true
		if value >= bounds[0] && value <= bounds[1] {
			if tag != "" {
				return "", fmt.Errorf("overlapping personality ranges")
			}
			tag = name
		}
	}
	if tag == "" {
		return "", fmt.Errorf("personality %d has no range", value)
	}
	return tag, nil
}

func educateTaggedEntry(raw json.RawMessage, tag string) (uint32, error) {
	var rows [][]json.RawMessage
	if err := json.Unmarshal(raw, &rows); err != nil || rows == nil {
		return 0, fmt.Errorf("tagged entry requires an explicit array")
	}
	var result uint32
	found := false
	seen := map[string]bool{}
	for _, row := range rows {
		var name string
		var id uint32
		if len(row) != 2 || json.Unmarshal(row[0], &name) != nil || name == "" || json.Unmarshal(row[1], &id) != nil || id == 0 || seen[name] {
			return 0, fmt.Errorf("invalid/duplicate tagged entry")
		}
		seen[name] = true
		if name == tag {
			result = id
			found = true
		}
	}
	if len(rows) > 0 && !found {
		return 0, fmt.Errorf("entry has no personality %q", tag)
	}
	return result, nil
}

func startEducateMainEvent(state *educateState) error {
	stage := state.Info.Fsm.GetSystemNo()
	if (stage != 0 && stage != newEducateSystemEvent) || len(state.Info.Fsm.PriorityFsm) > 0 || len(state.Info.Fsm.TarotSelects) > 0 {
		return errEducatePhase
	}
	if state.Lifecycle.Stages[newEducateSystemEvent].Loaded {
		if state.Info.Fsm.GetCurrentNode() != 0 {
			chain := state.Lifecycle.Chain
			if chain == nil || chain.Stage != newEducateSystemEvent || chain.Current != state.Info.Fsm.GetCurrentNode() || chain.Completed {
				return fmt.Errorf("pending main event has no matching chain instance")
			}
		}
		return nil
	}
	if state.Info.Fsm.GetCurrentNode() != 0 {
		return fmt.Errorf("legacy pending main event requires source audit")
	}
	round, ok, err := loadCurrentNewEducateRoundConfig(state.Info)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("main event missing round")
	}
	tag, err := educatePersonalityTag(state)
	if err != nil {
		return err
	}
	var first uint32
	if state.Info.Round.GetInTemp() == 0 {
		first, err = educateTaggedEntry(round.MainEventNode, tag)
		if err != nil {
			return fmt.Errorf("round %d main_event_node_id: %w", round.ID, err)
		}
	}
	if first != 0 {
		if _, ok, err := loadNewEducateConfigByID[newEducateNodeConfig](newEducateNodeCategory, first); err != nil {
			return err
		} else if !ok {
			return fmt.Errorf("main event missing entry %d", first)
		}
	}
	chain := state.Lifecycle.Chain
	if chain != nil && !chain.Completed && chain.Current == 0 && chain.Restarts > 0 {
		if chain.Source != "round.main_event_node_id" || chain.ConfigID != round.ID || chain.Entry != first {
			return fmt.Errorf("replayed main event source changed")
		}
		chain.Current = first
	} else {
		state.Lifecycle.Chain = &educateNodeChain{Version: state.Entry.Revision + 1, Source: "round.main_event_node_id", ConfigID: round.ID, Stage: newEducateSystemEvent, Entry: first, Current: first, Completed: first == 0}
	}
	state.Info.Fsm.SystemNo = proto.Uint32(newEducateSystemEvent)
	state.Info.Fsm.CurrentNode = proto.Uint32(first)
	markEducateStage(state, newEducateSystemEvent, first == 0)
	return nil
}

// Only presentation nodes have enough evidence for empty effects. DROP and
// site effects must supply an explicit source-specific reward contract.
func advanceEducatePresentationChain(state *educateState, branch uint32) (uint32, error) {
	chain := state.Lifecycle.Chain
	current := state.Info.Fsm.GetCurrentNode()
	if chain == nil || chain.Completed || chain.Stage != state.Info.Fsm.GetSystemNo() || chain.Source != "round.main_event_node_id" || current == 0 || current != chain.Current {
		return 0, fmt.Errorf("no matching main event instance")
	}
	if len(chain.Steps) >= 256 {
		return 0, fmt.Errorf("node instance %d exceeded 256 steps", chain.Version)
	}
	if chain.ReplayStart < 0 || chain.ReplayStart > len(chain.Steps) {
		return 0, fmt.Errorf("invalid replay history cursor")
	}
	if err := validateEducateNumericActives(state); err != nil {
		return 0, err
	}
	node, ok, err := loadNewEducateConfigByID[newEducateNodeConfig](newEducateNodeCategory, current)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, fmt.Errorf("missing active node %d", current)
	}
	if node.DropTypeClient != 0 {
		return 0, fmt.Errorf("node %d requires source-specific drop contract", current)
	}
	switch node.Type {
	case 1, 2, 103, 104:
	default:
		return 0, fmt.Errorf("main event node type %d effects require source contract", node.Type)
	}
	next, err := resolveEducateNodeNext(node, branch, func(total uint64) (uint64, error) {
		n, err := rand.Int(rand.Reader, new(big.Int).SetUint64(total))
		if err != nil {
			return 0, err
		}
		return n.Uint64(), nil
	})
	if err != nil {
		return 0, err
	}
	if next != 0 {
		if next == current {
			return 0, fmt.Errorf("self-loop at node %d", current)
		}
		for _, step := range chain.Steps[chain.ReplayStart:] {
			if step.Node == next {
				return 0, fmt.Errorf("node instance cycle %d -> %d", current, next)
			}
		}
		target, ok, err := loadNewEducateConfigByID[newEducateNodeConfig](newEducateNodeCategory, next)
		if err != nil {
			return 0, err
		}
		if !ok {
			return 0, fmt.Errorf("missing next node %d", next)
		}
		if node.NextType == 2 {
			matched, err := evaluateEducateCondition(state, target.OptionCondition)
			if err != nil {
				return 0, err
			}
			if !matched {
				return 0, fmt.Errorf("node option %d condition unmet", next)
			}
			costs, err := parseEducateDropTriplets(target.OptionCost, newEducateNodeCategory, fmt.Sprint(next), "option_cost")
			if err != nil {
				return 0, err
			}
			if len(costs) > 0 {
				return 0, fmt.Errorf("main option %d payment presentation needs verification", next)
			}
		}
	}
	chain.Steps = append(chain.Steps, educateNodeStep{Revision: state.Entry.Revision + 1, Node: current, Branch: branch, Next: next})
	chain.Current = next
	chain.Completed = next == 0
	state.Info.Fsm.CurrentNode = proto.Uint32(next)
	if next == 0 {
		markEducateStage(state, chain.Stage, true)
	}
	return next, nil
}

// Personal-local cancel policy mirrors the course replay policy. A confirmed
// presentation-only fixed chain can restart without costs or rewards. Random,
// choice and effect chains need a delivery contract before cancellation.
func restartEducateMainPresentation(state *educateState) error {
	chain := state.Lifecycle.Chain
	current := state.Info.Fsm.GetCurrentNode()
	if chain == nil || chain.Completed || chain.Source != "round.main_event_node_id" || chain.Stage != newEducateSystemEvent || state.Info.Fsm.GetSystemNo() != chain.Stage || current == 0 || current != chain.Current {
		return errEducatePhase
	}
	seen := map[uint32]bool{}
	found := false
	id := chain.Entry
	for id != 0 {
		if seen[id] || len(seen) >= 256 {
			return fmt.Errorf("invalid main replay chain at %d", id)
		}
		seen[id] = true
		found = found || id == current
		node, ok, err := loadNewEducateConfigByID[newEducateNodeConfig](newEducateNodeCategory, id)
		if err != nil {
			return err
		}
		if !ok || node.Type != 1 || node.NextType != 1 || node.DropTypeClient != 0 {
			return fmt.Errorf("main replay node %d requires delivery recovery", id)
		}
		id, err = resolveEducateNodeNext(node, 0, nil)
		if err != nil {
			return err
		}
	}
	if !found {
		return fmt.Errorf("main replay current node outside source")
	}
	chain.Restarts++
	chain.ReplayStart = len(chain.Steps)
	chain.Current = 0
	state.Info.Fsm.CurrentNode = proto.Uint32(0)
	// EVENT with node 0 is finished in the current client. INIT routes its
	// next FSM check back to EVENT, which requests the original entry again.
	state.Info.Fsm.SystemNo = proto.Uint32(0)
	state.Lifecycle.Stages[newEducateSystemEvent] = educateStage{}
	return nil
}
