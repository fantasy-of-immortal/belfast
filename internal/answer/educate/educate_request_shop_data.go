package educate

import (
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func EducateRequestShopData(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_27043
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 27044, err
	}

	response := &protobuf.SC_27044{
		Result: proto.Uint32(educateResultFailed),
		ShopData: &protobuf.CHILD_SHOP_DATA{
			ShopId: proto.Uint32(payload.GetShopId()),
			Goods:  []*protobuf.CHILD_SHOP_GOODS{},
		},
	}
	if client.Commander == nil || payload.GetShopId() == 0 {
		return client.SendMessage(27044, response)
	}

	shops, templates, err := loadEducateShopConfigs()
	if err != nil {
		return 0, 27044, err
	}
	shop, ok := shops[payload.GetShopId()]
	if !ok {
		return client.SendMessage(27044, response)
	}

	var state *orm.EducateShopState
	err = orm.UpdateLegacyEducateStateTx(client.Commander.CommanderID, func(tx pgx.Tx, legacy *orm.LegacyEducateState) error {
		var txErr error
		state, txErr = ensureLegacyShopTx(tx, legacy, shop, templates)
		return txErr
	})
	if err != nil {
		if errors.Is(err, errLegacyShopUnsupported) {
			return client.SendMessage(27044, response)
		}
		return 0, 27044, err
	}

	goods := make([]*protobuf.CHILD_SHOP_GOODS, 0, len(state.Goods))
	for _, row := range state.Goods {
		goods = append(goods, &protobuf.CHILD_SHOP_GOODS{Id: proto.Uint32(row.ID), Num: proto.Uint32(row.Num)})
	}
	response.Result = proto.Uint32(educateResultOK)
	response.ShopData = &protobuf.CHILD_SHOP_DATA{ShopId: proto.Uint32(shop.ID), Goods: goods}
	return client.SendMessage(27044, response)
}
