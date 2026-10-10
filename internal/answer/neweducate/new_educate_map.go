package neweducate

import (
	"encoding/json"
	"fmt"

	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

type educateSiteUnlockConfig struct {
	Unlock [][]json.RawMessage `json:"unlock"`
}

// Recover only the fixed terminal normal-site reward contract. The site's
// explicit title and drop_display give the local base award; random extra
// chains and map-trigger talents still require separate evidence.
func startEducateNormalSite(state *educateState, id uint32) error {
	if !educateCanPlanInput(state) || educateHasPending(state.Info) {
		return errEducatePhase
	}
	if err := validateEducateNumericActives(state); err != nil {
		return err
	}
	character, ok, err := loadNewEducateConfigByID[educateSiteUnlockConfig]("ShareCfg/child2_data.json", state.Info.GetId())
	if err != nil {
		return err
	}
	unlocked := false
	if ok {
		for _, row := range character.Unlock {
			var name string
			var round uint32
			if len(row) == 2 && json.Unmarshal(row[0], &name) == nil && name == "out" && json.Unmarshal(row[1], &round) == nil {
				unlocked = state.Info.Round.GetRound() >= round
			}
		}
	}
	if !unlocked {
		return fmt.Errorf("normal sites are not unlocked")
	}
	config, ok, err := loadNewEducateConfigByID[newEducateSiteNormalConfig](newEducateSiteNormalCategory, id)
	if err != nil {
		return err
	}
	if !ok || config.Character != state.Info.GetId() {
		return fmt.Errorf("normal site does not belong to character")
	}
	current := uint32(0)
	for _, stored := range state.Info.Site.Works {
		row, ok, err := loadNewEducateConfigByID[newEducateSiteNormalConfig](newEducateSiteNormalCategory, stored)
		if err != nil {
			return err
		}
		if ok && row.Character == state.Info.GetId() && row.Type == config.Type {
			current = stored
		}
	}
	if (current != 0 && current != id) || (current == 0 && config.Level != 1) {
		return fmt.Errorf("normal site level is not available")
	}
	node, ok, err := loadNewEducateConfigByID[newEducateNodeConfig](newEducateNodeCategory, config.Node)
	if err != nil {
		return err
	}
	if !ok || node.Type != 102 || node.DropTypeClient != 1 || node.NextType != 1 {
		return fmt.Errorf("normal site chain needs recovery")
	}
	next, err := resolveEducateNodeNext(node, 0, nil)
	if err != nil {
		return err
	}
	if next != 0 {
		return fmt.Errorf("normal site additional chain needs recovery")
	}
	if len(config.Drops) == 0 {
		return fmt.Errorf("normal site has no numeric award contract")
	}
	probe := &educateState{Info: proto.Clone(state.Info).(*protobuf.TBINFO)}
	if _, err := applyEducateNumericBatch(probe, config.Drops, 1, false); err != nil {
		return err
	}
	costs, err := applyEducateNumericBatch(state, [][]int32{config.Cost}, 1, true)
	if err != nil {
		return err
	}
	state.Lifecycle.Chain = &educateNodeChain{Version: state.Entry.Revision + 1, Source: "site.normal", ConfigID: id, Stage: newEducateSystemMap, Entry: config.Node, Current: config.Node, RewardRows: config.Drops}
	state.Info.Fsm.SystemNo = proto.Uint32(newEducateSystemMap)
	state.Info.Fsm.CurrentNode = proto.Uint32(config.Node)
	ensureEducateCache(state.Info).CacheSite[0].State = &protobuf.KVDATA{Key: proto.Uint32(newEducateSiteStateNormal), Value: proto.Uint32(id)}
	state.Info.Site.WorkCounter = upsertKVDATACount(state.Info.Site.WorkCounter, id, 1)
	context := educateChangeContext(state, educateActionID(state, fmt.Sprintf("site:%d", id)), 0, costs)
	context.Site = &educateConditionSite{Type: 1, ID: id}
	state.Lifecycle.RoundSites = append(state.Lifecycle.RoundSites, *context.Site)
	context.RoundSites = state.Lifecycle.RoundSites
	drops, err := applyEducateTalentTriggerWithContext(state, 6, 0, context)
	if err != nil {
		return err
	}
	state.Lifecycle.Chain.StartRewards = drops.BenefitDrop
	return nil
}

func settleEducateNormalSite(state *educateState, branch uint32) (*protobuf.TBDROPS, error) {
	chain := state.Lifecycle.Chain
	if chain == nil || chain.Source != "site.normal" || chain.Completed || chain.Stage != state.Info.Fsm.GetSystemNo() || chain.Current == 0 || chain.Current != state.Info.Fsm.GetCurrentNode() || branch != 0 {
		return nil, errEducatePhase
	}
	if err := validateEducateNumericActives(state); err != nil {
		return nil, err
	}
	context := educateChangeContext(state, educateActionID(state, fmt.Sprintf("site.reward:%d", chain.Version)), 0, nil)
	context.Site = &educateConditionSite{Type: 1, ID: chain.ConfigID}
	delivery, err := applyEducateDropBatch(state, chain.RewardRows, 1, context)
	if err != nil {
		return nil, err
	}
	chain.Steps = append(chain.Steps, educateNodeStep{Revision: state.Entry.Revision + 1, Node: chain.Current, Branch: branch, Next: 0})
	chain.Rewards = delivery
	chain.Current = 0
	chain.Completed = true
	state.Info.Fsm.CurrentNode = proto.Uint32(0)
	drops := emptyTBDrops()
	drops.BaseDrop = delivery
	return drops, nil
}
