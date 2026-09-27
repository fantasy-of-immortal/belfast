package neweducate

import (
	"encoding/json"
	"strconv"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const (
	newEducateRoundCategory          = "ShareCfg/child2_round.json"
	newEducateSiteNormalCategory     = "ShareCfg/child2_site_normal.json"
	newEducateSiteEventGroupCategory = "ShareCfg/child2_site_event_group.json"
	newEducateSiteCharacterCategory  = "ShareCfg/child2_site_character.json"
	newEducateShopCategory           = "ShareCfg/child2_shop.json"
	newEducateResourceCategory       = "ShareCfg/child2_resource.json"
	newEducateAttrCategory           = "ShareCfg/child2_attr.json"

	newEducateDropTypeAttr = 1
	newEducateDropTypeRes  = 2

	newEducateRoundTypeNormal = 1
)

type newEducateRoundConfig struct {
	ID            uint32          `json:"id"`
	Character     uint32          `json:"character"`
	Round         uint32          `json:"round"`
	RoundType     uint32          `json:"round_type"`
	IsHardMode    uint32          `json:"is_hard_mode"`
	BenefitSelect json.RawMessage `json:"benefit_select"`
	MapMobility   uint32          `json:"map_mobility"`
	RefreshRefill uint32          `json:"refresh_refill"`
}

type newEducateSiteNormalConfig struct {
	ID   uint32    `json:"id"`
	Cost [][]int32 `json:"cost"`
}

type newEducateSiteEventGroupConfig struct {
	ID        uint32    `json:"id"`
	EventCost [][]int32 `json:"event_cost"`
}

type newEducateSiteCharacterConfig struct {
	ID    uint32    `json:"id"`
	Group uint32    `json:"group"`
	Level uint32    `json:"level"`
	Cost  [][]int32 `json:"cost"`
}

type newEducateShopConfig struct {
	ID           uint32 `json:"id"`
	ResourceType uint32 `json:"resource_type"`
	ResourceNum  uint32 `json:"resource_num"`
}

type newEducateResourceConfig struct {
	ID           uint32 `json:"id"`
	Type         uint32 `json:"type"`
	Character    uint32 `json:"character"`
	DefaultValue uint32 `json:"default_value"`
}

type newEducateAttrConfig struct {
	ID           uint32 `json:"id"`
	Type         uint32 `json:"type"`
	Character    uint32 `json:"character"`
	DefaultValue uint32 `json:"default_value"`
}

// seedNewEducateDefaultRes fills Res.Attrs / Res.Resource with the per-character
// default values from child2_attr / child2_resource. Without the personality
// attr (child2_attr type 2) the client crashes in NewEducateChar
// GetPersonalityTag with "attempt to compare number with nil" (07-status finding).
func seedNewEducateDefaultRes(info *protobuf.TBINFO, charID uint32) {
	if info.Res == nil {
		info.Res = &protobuf.TBRES{}
	}
	if len(info.Res.Attrs) == 0 {
		if attrs, err := listNewEducateConfigs[newEducateAttrConfig](newEducateAttrCategory); err == nil {
			for _, attr := range attrs {
				if attr.Character == charID {
					info.Res.Attrs = append(info.Res.Attrs, &protobuf.KVDATA{Key: proto.Uint32(attr.ID), Value: proto.Uint32(attr.DefaultValue)})
				}
			}
		}
	}
	if len(info.Res.Resource) == 0 {
		if resources, err := listNewEducateConfigs[newEducateResourceConfig](newEducateResourceCategory); err == nil {
			for _, resource := range resources {
				if resource.Character == charID {
					info.Res.Resource = append(info.Res.Resource, &protobuf.KVDATA{Key: proto.Uint32(resource.ID), Value: proto.Uint32(resource.DefaultValue)})
				}
			}
		}
	}
}

func loadNewEducateConfigByID[T any](category string, id uint32) (*T, bool, error) {
	entry, err := orm.GetConfigEntry(category, strconv.FormatUint(uint64(id), 10))
	if err != nil {
		if db.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	var configData T
	if err := json.Unmarshal(entry.Data, &configData); err != nil {
		return nil, false, err
	}

	return &configData, true, nil
}

func listNewEducateConfigs[T any](category string) ([]T, error) {
	entries, err := orm.ListConfigEntries(category)
	if err != nil {
		return nil, err
	}

	configs := make([]T, 0, len(entries))
	for _, entry := range entries {
		var configData T
		if err := json.Unmarshal(entry.Data, &configData); err != nil {
			return nil, err
		}
		configs = append(configs, configData)
	}

	return configs, nil
}

func loadCurrentNewEducateRoundConfig(info *protobuf.TBINFO) (*newEducateRoundConfig, bool, error) {
	rounds, err := listNewEducateConfigs[newEducateRoundConfig](newEducateRoundCategory)
	if err != nil {
		return nil, false, err
	}

	for _, round := range rounds {
		if round.Character == info.GetId() && round.Round == info.GetRound().GetRound() && round.IsHardMode == info.GetDifficulty() && round.RoundType == newEducateRoundTypeNormal {
			candidate := round
			return &candidate, true, nil
		}
	}

	return nil, false, nil
}

func parseNewEducateUint32List(raw json.RawMessage) ([]uint32, error) {
	if len(raw) == 0 || string(raw) == `""` || string(raw) == "null" {
		return []uint32{}, nil
	}

	var values []uint32
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}

	return values, nil
}

func chooseNewEducateTalentCandidate(current []uint32, refreshed []uint32, available []uint32, oldTalent uint32) uint32 {
	blocked := make(map[uint32]bool, len(current)+len(refreshed))
	for _, value := range current {
		blocked[value] = true
	}
	for _, value := range refreshed {
		blocked[value] = true
	}

	for _, candidate := range available {
		if candidate != oldTalent && !blocked[candidate] {
			return candidate
		}
	}

	return oldTalent
}

func applyNewEducateConfigDrops(state *educateState, drops [][]int32, multiplier uint32) {
	if multiplier == 0 {
		return
	}

	for _, drop := range drops {
		if len(drop) < 3 {
			continue
		}

		delta := -drop[2] * int32(multiplier)
		switch uint32(drop[0]) {
		case newEducateDropTypeAttr:
			state.Info.Res.Attrs = upsertKVDATAWithDelta(state.Info.Res.Attrs, uint32(drop[1]), delta)
		case newEducateDropTypeRes:
			state.Info.Res.Resource = upsertKVDATAWithDelta(state.Info.Res.Resource, uint32(drop[1]), delta)
		}
	}
}

func upsertKVDATAWithDelta(values []*protobuf.KVDATA, key uint32, delta int32) []*protobuf.KVDATA {
	for _, entry := range values {
		if entry.GetKey() == key {
			entry.Value = proto.Uint32(applyUint32Delta(entry.GetValue(), delta))
			return values
		}
	}

	return append(values, &protobuf.KVDATA{Key: proto.Uint32(key), Value: proto.Uint32(applyUint32Delta(0, delta))})
}

func applyUint32Delta(current uint32, delta int32) uint32 {
	if delta >= 0 {
		return current + uint32(delta)
	}
	if uint32(-delta) >= current {
		return 0
	}
	return current - uint32(-delta)
}

func resolveNewEducateResourceID(state *educateState, resourceType uint32) (uint32, bool, error) {
	resources, err := listNewEducateConfigs[newEducateResourceConfig](newEducateResourceCategory)
	if err != nil {
		return 0, false, err
	}

	for _, resource := range resources {
		if resource.Type != resourceType {
			continue
		}
		for _, entry := range state.Info.Res.Resource {
			if entry.GetKey() == resource.ID {
				return resource.ID, true, nil
			}
		}
	}

	for _, resource := range resources {
		if resource.Type == resourceType {
			return resource.ID, true, nil
		}
	}

	return 0, false, nil
}

func removeUint32(values []uint32, target uint32) []uint32 {
	filtered := values[:0]
	for _, value := range values {
		if value != target {
			filtered = append(filtered, value)
		}
	}
	return filtered
}
