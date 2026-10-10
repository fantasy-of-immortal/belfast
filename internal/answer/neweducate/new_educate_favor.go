package neweducate

import (
	"fmt"

	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

type educateFavorConfig struct {
	ID         uint32    `json:"id"`
	Level      uint32    `json:"favor_level"`
	Experience []uint32  `json:"favor_exp"`
	Rewards    [][]int32 `json:"favor_result_display"`
}

// favor_lv counts claimed upgrades, while the FAVOR resource is cumulative
// experience. The client subtracts earlier thresholds for display only.
func claimEducateFavorLevel(state *educateState) (*protobuf.TBDROPS, error) {
	if !educateCanPlanInput(state) || educateHasPending(state.Info) {
		return nil, errEducatePhase
	}
	if err := validateEducateNumericActives(state); err != nil {
		return nil, err
	}
	config, ok, err := loadNewEducateConfigByID[educateFavorConfig]("ShareCfg/child2_data.json", state.Info.GetId())
	if err != nil {
		return nil, err
	}
	claimed := state.Info.GetFavorLv()
	if !ok || config.Level < 2 || len(config.Experience) != int(config.Level-1) || len(config.Rewards) != len(config.Experience) || claimed >= config.Level-1 {
		return nil, fmt.Errorf("favor level has no available upgrade")
	}
	id, ok, err := resolveNewEducateResourceID(state, 4)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("favor resource missing")
	}
	var threshold uint64
	for _, exp := range config.Experience[:claimed+1] {
		threshold += uint64(exp)
	}
	if uint64(educateKVCount(state.Info.Res.Resource, id)) < threshold {
		return nil, fmt.Errorf("favor experience below threshold %d", threshold)
	}
	delivery, err := applyEducateDropBatch(state, [][]int32{config.Rewards[claimed]}, 1, educateChangeContext(state, educateActionID(state, "favor"), 0, nil))
	if err != nil {
		return nil, err
	}
	state.Info.FavorLv = proto.Uint32(claimed + 1)
	drops := emptyTBDrops()
	drops.BaseDrop = delivery
	return drops, nil
}
