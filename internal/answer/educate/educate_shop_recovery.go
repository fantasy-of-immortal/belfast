package educate

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/jackc/pgx/v5"
	"sort"
)

var errLegacyShopUnsupported = errors.New("legacy shop path not restored")

// Full pools need no private weighted-selection semantics. Pool quantity is
// remaining purchases; template buy_num is items per purchase.
func legacyShopFullPool(shop educateShopConfig, templates map[uint32]educateShopTemplateConfig) ([]orm.EducateShopGoodsState, error) {
	if int(shop.GoodsNum) != len(shop.GoodsPool) || shop.GoodsNum == 0 {
		return nil, errLegacyShopUnsupported
	}
	goods := make([]orm.EducateShopGoodsState, 0, len(shop.GoodsPool))
	seen := map[uint32]bool{}
	for _, row := range shop.GoodsPool {
		var id, quantity uint32
		var conditions []json.RawMessage
		if len(row) != 4 || json.Unmarshal(row[0], &id) != nil || json.Unmarshal(row[1], &quantity) != nil || json.Unmarshal(row[3], &conditions) != nil || len(conditions) != 0 || id == 0 || quantity == 0 || seen[id] {
			return nil, errLegacyShopUnsupported
		}
		if _, exists := templates[id]; !exists {
			return nil, errLegacyShopUnsupported
		}
		seen[id] = true
		goods = append(goods, orm.EducateShopGoodsState{ID: id, Num: quantity})
	}
	sort.Slice(goods, func(i, j int) bool { return goods[i].ID < goods[j].ID })
	return goods, nil
}
func legacyShopRefreshWeek(week uint32, interval int32) uint32 {
	if interval <= 0 || week < 9 {
		return 0
	}
	if week > 60 {
		week = 60
	}
	return 9 + (week-9)/uint32(interval)*uint32(interval)
}
func ensureLegacyShopTx(tx pgx.Tx, legacy *orm.LegacyEducateState, shop educateShopConfig, templates map[uint32]educateShopTemplateConfig) (*orm.EducateShopState, error) {
	goods, err := legacyShopFullPool(shop, templates)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	stock, err := orm.GetEducateShopStateTx(ctx, tx, legacy.CommanderID, shop.ID)
	exists := err == nil
	if err != nil && !db.IsNotFound(err) {
		return nil, err
	}
	week := (legacy.CurTime.Month-1)*4 + legacy.CurTime.Week
	if legacy.ShopCalendar == nil {
		legacy.ShopCalendar = map[uint32]orm.LegacyEducateShopCalendar{}
	}
	calendar, migrated := legacy.ShopCalendar[shop.ID]
	if !exists {
		stock = &orm.EducateShopState{CommanderID: legacy.CommanderID, ShopID: shop.ID, Goods: goods}
	} else if migrated && legacyShopRefreshWeek(week, shop.GoodsRefreshTime) > legacyShopRefreshWeek(calendar.Week, shop.GoodsRefreshTime) {
		stock.Goods = goods
	}
	// Adopting a wall-clock stock row must not refill an existing player's stock.
	if !migrated && exists {
		calendar.CompatRefreshKey = stock.RefreshKey
	}
	calendar.Week = week
	legacy.ShopCalendar[shop.ID] = calendar
	stock.RefreshKey = legacyShopRefreshWeek(week, shop.GoodsRefreshTime)
	if err := orm.UpsertEducateShopStateTx(ctx, tx, stock); err != nil {
		return nil, err
	}
	return stock, nil
}
func legacyGoodInTime(template educateShopTemplateConfig, date orm.LegacyEducateTime) bool {
	if string(template.Time) == `"always"` {
		return true
	}
	var limits [][]uint32
	if json.Unmarshal(template.Time, &limits) != nil {
		return false
	}
	return legacyTaskInTime(legacyChildTask{TimeLimit: limits}, date)
}
