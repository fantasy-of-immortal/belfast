package educate

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	"math"
)

func EducateShopping(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_27033
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 27034, err
	}
	response := &protobuf.SC_27034{Result: proto.Uint32(educateResultFailed)}
	if client.Commander == nil || payload.GetShopId() == 0 || len(payload.GetGoods()) == 0 {
		return client.SendMessage(27034, response)
	}
	shops, templates, err := loadEducateShopConfigs()
	if err != nil {
		return 0, 27034, err
	}
	shop, ok := shops[payload.GetShopId()]
	if !ok {
		return client.SendMessage(27034, response)
	}
	flags, err := orm.ListCommanderCommonFlags(client.Commander.CommanderID)
	if err != nil {
		return 0, 27034, err
	}
	events, err := loadEducateSpecialEvents()
	if err != nil {
		return 0, 27034, err
	}
	err = orm.UpdateLegacyEducateStateTx(client.Commander.CommanderID, func(tx pgx.Tx, legacy *orm.LegacyEducateState) error {
		// Historical flag ranges overlap. Resolve real event IDs/types rather
		// than treating every site event above ID 1000 as a discount.
		for _, event := range events {
			if event.Type == 4 && legacyTaskInTime(legacyChildTask{TimeLimit: event.Date}, *legacy.CurTime) {
				for _, flag := range flags {
					if flag == educateFlagSpecialEventBase+event.ID || flag == educateFlagDiscountBase+event.ID {
						return errLegacyShopUnsupported
					}
				}
			}
		}
		stock, err := ensureLegacyShopTx(tx, legacy, shop, templates)
		if err != nil {
			return err
		}
		index := map[uint32]int{}
		for i, row := range stock.Goods {
			index[row.ID] = i
		}
		member := map[uint32]bool{}
		for _, row := range shop.GoodsPool {
			var id uint32
			if len(row) > 0 && json.Unmarshal(row[0], &id) == nil {
				member[id] = true
			}
		}
		costs := map[uint32]uint64{}
		drops := []*protobuf.CHILD_DROP{}
		for _, row := range payload.GetGoods() {
			if row == nil || row.GetNum() == 0 {
				return errLegacyShopUnsupported
			}
			i, exists := index[row.GetId()]
			tpl := templates[row.GetId()]
			if !exists || !member[row.GetId()] || stock.Goods[i].Num < row.GetNum() || !legacyGoodInTime(tpl, *legacy.CurTime) || tpl.Resource < 1 || tpl.Resource > 3 || tpl.BuyNum == 0 {
				return errLegacyShopUnsupported
			}
			costs[tpl.Resource] += uint64(tpl.ResourceNum) * uint64(row.GetNum())
			amount := uint64(tpl.BuyNum) * uint64(row.GetNum())
			if amount > math.MaxInt32 {
				return errLegacyShopUnsupported
			}
			stock.Goods[i].Num -= row.GetNum()
			drops = append(drops, &protobuf.CHILD_DROP{Type: proto.Uint32(3), Id: proto.Uint32(tpl.ItemID), Number: proto.Int32(int32(amount))})
		}
		for id, cost := range costs {
			if legacy.Resources[id] < 0 || cost > uint64(legacy.Resources[id]) {
				return errLegacyShopUnsupported
			}
			legacy.Resources[id] -= int32(cost)
		}
		for _, drop := range drops {
			if err := applyLegacyTaskDrop(legacy, []uint32{3, drop.GetId(), uint32(drop.GetNumber())}); err != nil {
				return errLegacyShopUnsupported
			}
		}
		if err := orm.UpsertEducateShopStateTx(context.Background(), tx, stock); err != nil {
			return err
		}
		response.Drops = drops
		return nil
	})
	if err != nil && !errors.Is(err, errLegacyShopUnsupported) {
		return 0, 27034, err
	}
	if err == nil {
		response.Result = proto.Uint32(educateResultOK)
	}
	return client.SendMessage(27034, response)
}
