package educate

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const (
	educateResultOK     = uint32(0)
	educateResultFailed = uint32(1)

	educateFlagHomeEventBase    = uint32(270140000)
	educateFlagSpecialEventBase = uint32(270270000)
	educateFlagDiscountBase     = uint32(270271000)
	educateFlagTargetAwardBase  = uint32(270350000)
)

var educateNow = func() time.Time {
	return time.Now().UTC()
}

type educateSpecialEventConfig struct {
	Date        [][]uint32 `json:"date"`
	ID          uint32     `json:"id"`
	Show        uint32     `json:"show"`
	Type        uint32     `json:"type"`
	Result      uint32     `json:"result"`
	DropDisplay []int      `json:"drop_display"`
}

type educateEventConfig struct {
	ID uint32 `json:"id"`
}

type educateShopConfig struct {
	ID               uint32              `json:"id"`
	GoodsNum         uint32              `json:"goods_num"`
	GoodsPool        [][]json.RawMessage `json:"goods_pool"`
	GoodsRefreshTime int32               `json:"goods_refresh_time"`
}

type educateShopTemplateConfig struct {
	Time        json.RawMessage `json:"time"`
	ID          uint32          `json:"id"`
	ItemID      uint32          `json:"item_id"`
	Resource    uint32          `json:"resource"`
	ResourceNum uint32          `json:"resource_num"`
	BuyNum      uint32          `json:"buy_num"`
}

type educateTargetSetConfig struct {
	ID             uint32   `json:"id"`
	Stage          uint32   `json:"stage"`
	IDs            []uint32 `json:"ids"`
	TargetProgress uint32   `json:"target_progress"`
	DropDisplay    []int    `json:"drop_display"`
}

type educateTaskConfig struct {
	ID                 uint32 `json:"id"`
	TaskTargetProgress uint32 `json:"task_target_progress"`
}

func parseConfigEntries[T any](entries []orm.ConfigEntry) ([]T, error) {
	out := make([]T, 0, len(entries))
	for _, entry := range entries {
		trimmed := strings.TrimSpace(string(entry.Data))
		if trimmed == "" || trimmed == "null" {
			continue
		}
		var many []T
		if err := json.Unmarshal(entry.Data, &many); err == nil {
			out = append(out, many...)
			continue
		}
		var one T
		if err := json.Unmarshal(entry.Data, &one); err != nil {
			return nil, fmt.Errorf("parse %s/%s: %w", entry.Category, entry.Key, err)
		}
		out = append(out, one)
	}
	return out, nil
}

func loadEducateSpecialEvents() (map[uint32]educateSpecialEventConfig, error) {
	entries, err := orm.ListConfigEntries("ShareCfg/child_event_special.json")
	if err != nil {
		return nil, err
	}
	rows, err := parseConfigEntries[educateSpecialEventConfig](entries)
	if err != nil {
		return nil, err
	}
	out := make(map[uint32]educateSpecialEventConfig, len(rows))
	for _, row := range rows {
		if row.ID == 0 {
			continue
		}
		out[row.ID] = row
	}
	return out, nil
}

func loadEducateEvents() (map[uint32]educateEventConfig, error) {
	entries, err := orm.ListConfigEntries("ShareCfg/child_event.json")
	if err != nil {
		return nil, err
	}
	rows, err := parseConfigEntries[educateEventConfig](entries)
	if err != nil {
		return nil, err
	}
	out := make(map[uint32]educateEventConfig, len(rows))
	for _, row := range rows {
		if row.ID == 0 {
			continue
		}
		out[row.ID] = row
	}
	return out, nil
}

func loadEducateShopConfigs() (map[uint32]educateShopConfig, map[uint32]educateShopTemplateConfig, error) {
	shopEntries, err := orm.ListConfigEntries("ShareCfg/child_shop.json")
	if err != nil {
		return nil, nil, err
	}
	shopRows, err := parseConfigEntries[educateShopConfig](shopEntries)
	if err != nil {
		return nil, nil, err
	}
	templateEntries, err := orm.ListConfigEntries("ShareCfg/child_shop_template.json")
	if err != nil {
		return nil, nil, err
	}
	templateRows, err := parseConfigEntries[educateShopTemplateConfig](templateEntries)
	if err != nil {
		return nil, nil, err
	}
	shops := make(map[uint32]educateShopConfig, len(shopRows))
	for _, row := range shopRows {
		if row.ID == 0 {
			continue
		}
		shops[row.ID] = row
	}
	templates := make(map[uint32]educateShopTemplateConfig, len(templateRows))
	for _, row := range templateRows {
		if row.ID == 0 {
			continue
		}
		if row.BuyNum == 0 {
			row.BuyNum = 1
		}
		templates[row.ID] = row
	}
	return shops, templates, nil
}

func loadEducateTargetAndTaskConfigs() (map[uint32]educateTargetSetConfig, map[uint32]educateTaskConfig, error) {
	targetEntries, err := orm.ListConfigEntries("ShareCfg/child_target_set.json")
	if err != nil {
		return nil, nil, err
	}
	targetRows, err := parseConfigEntries[educateTargetSetConfig](targetEntries)
	if err != nil {
		return nil, nil, err
	}
	taskEntries, err := orm.ListConfigEntries("ShareCfg/child_task.json")
	if err != nil {
		return nil, nil, err
	}
	taskRows, err := parseConfigEntries[educateTaskConfig](taskEntries)
	if err != nil {
		return nil, nil, err
	}
	targets := make(map[uint32]educateTargetSetConfig, len(targetRows))
	for _, row := range targetRows {
		if row.ID == 0 {
			continue
		}
		targets[row.ID] = row
	}
	tasks := make(map[uint32]educateTaskConfig, len(taskRows))
	for _, row := range taskRows {
		if row.ID == 0 {
			continue
		}
		tasks[row.ID] = row
	}
	return targets, tasks, nil
}

func educateFlagID(base uint32, id uint32) uint32 {
	return base + id
}

func hasEducateFlag(commanderID uint32, flagID uint32) (bool, error) {
	flags, err := orm.ListCommanderCommonFlags(commanderID)
	if err != nil {
		return false, err
	}
	for _, flag := range flags {
		if flag == flagID {
			return true, nil
		}
	}
	return false, nil
}

func setEducateFlag(commanderID uint32, flagID uint32) error {
	if has, err := hasEducateFlag(commanderID, flagID); err != nil {
		return err
	} else if has {
		return nil
	}
	return orm.SetCommanderCommonFlag(commanderID, flagID)
}

func chooseEducateTargetID(targets map[uint32]educateTargetSetConfig) uint32 {
	ids := make([]uint32, 0, len(targets))
	for id, row := range targets {
		if row.Stage == 1 {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		for id := range targets {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return 0
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids[0]
}

func shopGoodsFromConfig(shop educateShopConfig, templates map[uint32]educateShopTemplateConfig) []orm.EducateShopGoodsState {
	goodIDs := make([]uint32, 0, len(shop.GoodsPool))
	for _, entry := range shop.GoodsPool {
		if len(entry) == 0 {
			continue
		}
		var id uint32
		if err := json.Unmarshal(entry[0], &id); err != nil || id == 0 {
			continue
		}
		if _, ok := templates[id]; !ok {
			continue
		}
		goodIDs = append(goodIDs, id)
	}
	sort.Slice(goodIDs, func(i, j int) bool { return goodIDs[i] < goodIDs[j] })
	limit := int(shop.GoodsNum)
	if limit <= 0 || limit > len(goodIDs) {
		limit = len(goodIDs)
	}
	out := make([]orm.EducateShopGoodsState, 0, limit)
	for _, goodID := range goodIDs[:limit] {
		tpl := templates[goodID]
		num := tpl.BuyNum
		if num == 0 {
			num = 1
		}
		out = append(out, orm.EducateShopGoodsState{ID: goodID, Num: num})
	}
	return out
}

func shopRefreshKey(now time.Time, refreshDays int32) uint32 {
	if refreshDays <= 0 {
		return 0
	}
	seconds := int64(refreshDays) * 24 * 60 * 60
	if seconds <= 0 {
		return 0
	}
	return uint32(now.Unix() / seconds)
}

func ensureEducateShopStateTx(ctx context.Context, tx pgx.Tx, commanderID uint32, shop educateShopConfig, templates map[uint32]educateShopTemplateConfig, now time.Time) (*orm.EducateShopState, error) {
	state, err := orm.GetEducateShopStateTx(ctx, tx, commanderID, shop.ID)
	if err != nil {
		if !db.IsNotFound(err) {
			return nil, err
		}
		state = &orm.EducateShopState{
			CommanderID: commanderID,
			ShopID:      shop.ID,
			RefreshKey:  shopRefreshKey(now, shop.GoodsRefreshTime),
			Goods:       shopGoodsFromConfig(shop, templates),
		}
		if err := orm.UpsertEducateShopStateTx(ctx, tx, state); err != nil {
			return nil, err
		}
		state, err = orm.GetEducateShopStateTx(ctx, tx, commanderID, shop.ID)
		if err != nil {
			return nil, err
		}
	}
	if shop.GoodsRefreshTime > 0 {
		key := shopRefreshKey(now, shop.GoodsRefreshTime)
		if state.RefreshKey != key {
			state.RefreshKey = key
			state.Goods = shopGoodsFromConfig(shop, templates)
			if err := orm.UpsertEducateShopStateTx(ctx, tx, state); err != nil {
				return nil, err
			}
		}
	}
	return state, nil
}

func populateEducateSnapshot(commanderID uint32, child *protobuf.CHILD_INFO) error {
	if child == nil {
		return nil
	}
	flags, err := orm.ListCommanderCommonFlags(commanderID)
	if err != nil {
		return err
	}
	specEvents := make([]uint32, 0)
	discountEvents := make([]uint32, 0)
	hadTargetAward := uint32(0)
	for _, flag := range flags {
		switch {
		case flag >= educateFlagSpecialEventBase && flag < educateFlagSpecialEventBase+100000:
			specEvents = append(specEvents, flag-educateFlagSpecialEventBase)
		case flag >= educateFlagDiscountBase && flag < educateFlagDiscountBase+100000:
			discountEvents = append(discountEvents, flag-educateFlagDiscountBase)
		case flag >= educateFlagTargetAwardBase && flag < educateFlagTargetAwardBase+100000:
			hadTargetAward = 1
		}
	}
	sort.Slice(specEvents, func(i, j int) bool { return specEvents[i] < specEvents[j] })
	sort.Slice(discountEvents, func(i, j int) bool { return discountEvents[i] < discountEvents[j] })
	child.SpecEvents = specEvents
	child.DiscountEventId = discountEvents
	child.HadTargetStageAward = proto.Uint32(hadTargetAward)

	shopStates, err := orm.ListEducateShopStates(commanderID)
	if err != nil {
		return err
	}
	shops := make([]*protobuf.CHILD_SHOP_DATA, 0, len(shopStates))
	for _, state := range shopStates {
		goods := make([]*protobuf.CHILD_SHOP_GOODS, 0, len(state.Goods))
		for _, row := range state.Goods {
			goods = append(goods, &protobuf.CHILD_SHOP_GOODS{Id: proto.Uint32(row.ID), Num: proto.Uint32(row.Num)})
		}
		shops = append(shops, &protobuf.CHILD_SHOP_DATA{ShopId: proto.Uint32(state.ShopID), Goods: goods})
	}
	child.Shop = shops
	return nil
}
