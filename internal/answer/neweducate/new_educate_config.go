package neweducate

import (
	"encoding/json"
	"fmt"
	"sort"
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
	newEducatePlanCategory           = "ShareCfg/child2_plan.json"
	newEducateNodeCategory           = "ShareCfg/child2_node.json"

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
	TargetID      uint32          `json:"target_id"`
	PlanNum       uint32          `json:"plan_num"`
	PlanGroup     []uint32        `json:"plan_group"`
	MainEventNode json.RawMessage `json:"main_event_node_id"`
	MainChatNode  json.RawMessage `json:"main_event_chat_node_id"`
}

type newEducateSiteNormalConfig struct {
	ID        uint32    `json:"id"`
	Character uint32    `json:"character"`
	Type      uint32    `json:"type"`
	Level     uint32    `json:"site_lv"`
	Node      uint32    `json:"node_id"`
	Cost      []int32   `json:"cost"`
	Drops     [][]int32 `json:"drop_display"`
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
	GoodsType    uint32 `json:"goods_type"`
	GoodsID      uint32 `json:"goods_id"`
	GoodsNum     uint32 `json:"goods_num"`
	LimitNum     int32  `json:"limit_num"`
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

// child2_plan: one scheduled activity per round. result_node is the first
// node of the course's playback chain; result_display holds the gain applied
// when the chain reaches its end node (drop_type_client 1, node type 102).
type newEducatePlanConfig struct {
	ID             uint32          `json:"id"`
	ResultNode     uint32          `json:"result_node"`
	Cost           json.RawMessage `json:"cost"`
	ResultDisplay  json.RawMessage `json:"result_display"`
	GroupID        uint32          `json:"group_id"`
	Level          uint32          `json:"level"`
	LevelCondition json.RawMessage `json:"level_condition"`
}

// child2_node: one step of the playback chain. "next" may be a number, a
// number-as-string, an array of ids, or an array of [node, weight] pairs.
type newEducateNodeConfig struct {
	ID              uint32          `json:"id"`
	Type            uint32          `json:"type"`
	NextType        uint32          `json:"next_type"`
	Next            json.RawMessage `json:"next"`
	DropTypeClient  uint32          `json:"drop_type_client"`
	OptionCondition json.RawMessage `json:"option_condition"`
	OptionCost      json.RawMessage `json:"option_cost"`
}

// seedNewEducateDefaultRes fills Res.Attrs / Res.Resource with the per-character
// default values from child2_attr / child2_resource. Without the personality
// attr (child2_attr type 2) the client crashes in NewEducateChar
// GetPersonalityTag with "attempt to compare number with nil" (07-status finding).
func seedNewEducateDefaultRes(info *protobuf.TBINFO, charID uint32) error {
	if info.Res == nil {
		info.Res = &protobuf.TBRES{}
	}
	attrs, err := listNewEducateConfigs[newEducateAttrConfig](newEducateAttrCategory)
	if err != nil {
		return fmt.Errorf("character %d attribute configuration: %w", charID, err)
	}
	resources, err := listNewEducateConfigs[newEducateResourceConfig](newEducateResourceCategory)
	if err != nil {
		return fmt.Errorf("character %d resource configuration: %w", charID, err)
	}
	attrCount, resCount := 0, 0
	{
		for _, attr := range attrs {
			if attr.Character == charID {
				attrCount++
				info.Res.Attrs = appendMissingNewEducateValue(info.Res.Attrs, attr.ID, attr.DefaultValue)
			}
		}
	}
	{
		for _, resource := range resources {
			if resource.Character == charID {
				resCount++
				info.Res.Resource = appendMissingNewEducateValue(info.Res.Resource, resource.ID, resource.DefaultValue)
			}
		}
	}
	if attrCount == 0 || resCount == 0 {
		return fmt.Errorf("character %d has incomplete attr/resource configuration", charID)
	}
	return nil
}

func appendMissingNewEducateValue(values []*protobuf.KVDATA, id, value uint32) []*protobuf.KVDATA {
	for _, existing := range values {
		if existing.GetKey() == id {
			return values
		}
	}
	return append(values, &protobuf.KVDATA{Key: proto.Uint32(id), Value: proto.Uint32(value)})
}

func filterNewEducateValues(values []*protobuf.KVDATA, allowed map[uint32]bool) []*protobuf.KVDATA {
	out := make([]*protobuf.KVDATA, 0, len(values))
	for _, value := range values {
		if allowed[value.GetKey()] {
			out = append(out, value)
		}
	}
	return out
}

func loadNewEducateConfigByID[T any](category string, id uint32) (*T, bool, error) {
	entry, err := orm.GetConfigEntry(category, strconv.FormatUint(uint64(id), 10))
	if err != nil {
		if db.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	data, err := normalizeEducateConfigIdentity(entry.Data)
	if err != nil {
		return nil, false, fmt.Errorf("%s/%s: %w", category, entry.Key, err)
	}
	var configData T
	if err := json.Unmarshal(data, &configData); err != nil {
		return nil, false, fmt.Errorf("%s/%s: %w", category, entry.Key, err)
	}
	var identity struct {
		ID uint32 `json:"id"`
	}
	if err := json.Unmarshal(data, &identity); err != nil || identity.ID != id {
		return nil, false, fmt.Errorf("%s/%s: id does not match key", category, entry.Key)
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
		data, err := normalizeEducateConfigIdentity(entry.Data)
		if err != nil {
			return nil, fmt.Errorf("%s/%s: %w", category, entry.Key, err)
		}
		var configData T
		if err := json.Unmarshal(data, &configData); err != nil {
			return nil, fmt.Errorf("%s/%s: %w", category, entry.Key, err)
		}
		var identity struct {
			ID uint32 `json:"id"`
		}
		key, keyErr := strconv.ParseUint(entry.Key, 10, 32)
		if err := json.Unmarshal(data, &identity); err != nil || keyErr != nil || key == 0 || uint32(key) != identity.ID {
			return nil, fmt.Errorf("%s/%s: id does not match positive numeric key", category, entry.Key)
		}
		configs = append(configs, configData)
	}

	return configs, nil
}

// Some imported CSV-derived configurations preserve a BOM on the first
// column name. Accept that known alias without modifying the evidence rows.
func normalizeEducateConfigIdentity(data json.RawMessage) (json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	if fields == nil {
		return nil, fmt.Errorf("configuration must be an object")
	}
	if bom, ok := fields["\ufeffid"]; ok {
		if ordinary, exists := fields["id"]; exists && string(ordinary) != string(bom) {
			return nil, fmt.Errorf("conflicting id and BOM id")
		}
		fields["id"] = bom
		delete(fields, "\ufeffid")
		return json.Marshal(fields)
	}
	return data, nil
}

func loadCurrentNewEducateRoundConfig(info *protobuf.TBINFO) (*newEducateRoundConfig, bool, error) {
	rounds, err := listNewEducateConfigs[newEducateRoundConfig](newEducateRoundCategory)
	if err != nil {
		return nil, false, err
	}

	normal := map[uint32]newEducateRoundConfig{}
	cycles := []newEducateRoundConfig{}
	ids := map[uint32]bool{}
	for _, round := range rounds {
		if round.Character != info.GetId() || round.IsHardMode != info.GetDifficulty() {
			continue
		}
		if round.ID == 0 || round.Round == 0 || ids[round.ID] {
			return nil, false, fmt.Errorf("%s/%d: invalid/duplicate id or round", newEducateRoundCategory, round.ID)
		}
		ids[round.ID] = true
		switch round.RoundType {
		case newEducateRoundTypeNormal:
			if _, duplicate := normal[round.Round]; duplicate {
				return nil, false, fmt.Errorf("%s: character %d difficulty %d duplicate round %d", newEducateRoundCategory, info.GetId(), info.GetDifficulty(), round.Round)
			}
			normal[round.Round] = round
		case 2:
			cycles = append(cycles, round)
		default:
			return nil, false, fmt.Errorf("%s/%d: unknown round_type %d", newEducateRoundCategory, round.ID, round.RoundType)
		}
	}
	for i := 1; i <= len(normal); i++ {
		if _, ok := normal[uint32(i)]; !ok {
			return nil, false, fmt.Errorf("character %d difficulty %d missing round %d", info.GetId(), info.GetDifficulty(), i)
		}
	}
	current := info.Round.GetRound()
	if current == 0 || len(normal) == 0 {
		return nil, false, fmt.Errorf("character %d difficulty %d has no valid timeline", info.GetId(), info.GetDifficulty())
	}
	if round, ok := normal[current]; ok {
		return &round, true, nil
	}
	if current > uint32(len(normal)) && len(cycles) > 0 {
		sort.Slice(cycles, func(i, j int) bool { return cycles[i].ID < cycles[j].ID })
		// NewEducateRound.InitEndlessRoundId cycles sorted configuration IDs.
		wave := current - uint32(len(normal))
		round := cycles[(wave-1)%uint32(len(cycles))]
		return &round, true, nil
	}
	return nil, false, fmt.Errorf("character %d difficulty %d round %d has no configuration", info.GetId(), info.GetDifficulty(), current)
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

func resolveNewEducateResourceID(state *educateState, resourceType uint32) (uint32, bool, error) {
	resources, err := listNewEducateConfigs[newEducateResourceConfig](newEducateResourceCategory)
	if err != nil {
		return 0, false, err
	}

	var foundID uint32
	for _, resource := range resources {
		if resource.Type == resourceType && resource.Character == state.Info.GetId() {
			if foundID != 0 {
				return 0, false, fmt.Errorf("character %d duplicate resource type %d", state.Info.GetId(), resourceType)
			}
			foundID = resource.ID
		}
	}
	if foundID == 0 {
		return 0, false, fmt.Errorf("character %d missing resource type %d", state.Info.GetId(), resourceType)
	}
	return foundID, true, nil
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

// parseChild2NodeNext resolves a child2_node "next" field to the follow-up
// node id. Shapes observed in the 9.7.393 dump:
//
//	`"3629107"` (string number)  |  3629206 (number)
//	[3629204,3629207]            (branch list -> take first, deterministic)
//	[[0,50],[3700807,50]]        (weighted pairs -> take first pair's node)
func parseNewEducateDropTriplets(raw json.RawMessage) [][]int32 {
	if len(raw) == 0 {
		return nil
	}
	var triplets [][]int32
	if err := json.Unmarshal(raw, &triplets); err != nil {
		return nil
	}
	return triplets
}

func parseEducateFixedNodeNext(raw json.RawMessage) (uint32, error) {
	var number uint32
	if err := json.Unmarshal(raw, &number); err == nil && string(raw) != "null" {
		return number, nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0, fmt.Errorf("fixed successor must be uint32 or numeric string")
	}
	if value == "" {
		return 0, nil
	} // Configured terminal marker, not a parse failure.
	parsed, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		return 0, err
	}
	return uint32(parsed), nil
}
