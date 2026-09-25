package answer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/consts"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/orm"

	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

// Drop List type :
// 0 = upgrades
// 1 = resource
// 2 = items
// 3 = does not exist
// 4 = ship
// 5 = does not exist
// 6 = skins
// 7 = not an ownable
// 8 = not an ownable
// 9 = equip skin
// 10 = not an ownable
// 12 = operation siren stuff
// 20 = ?

func HandleShopPurchase(buffer *[]byte, client *connection.Client) (int, int, error) {
	var boughtOffer protobuf.CS_16001
	err := proto.Unmarshal(*buffer, &boughtOffer)
	if err != nil {
		return 0, 16002, err
	}
	// A purchase can arrive before the commander finished loading (e.g. while the
	// session is re-authenticating). Without this guard the deref below panics,
	// which kills the whole server process instead of answering the client.
	if client.Commander == nil {
		response := protobuf.SC_16002{Result: proto.Uint32(1)}
		return client.SendMessage(16002, &response)
	}

	shopOffer, err := loadShopOfferByID(boughtOffer.GetId())
	if err != nil {
		response := protobuf.SC_16002{Result: proto.Uint32(2)}
		return client.SendMessage(16002, &response)
	}
	logger.LogEvent("Shop", "Purchase", fmt.Sprintf("uid=%d is buying #%d", client.Commander.CommanderID, shopOffer.ID), logger.LOG_LEVEL_INFO)

	response := protobuf.SC_16002{
		Result: proto.Uint32(0),
	}

	if !client.Commander.HasEnoughResource(shopOffer.ResourceID, uint32(shopOffer.ResourceNumber)) {
		logger.LogEvent("Shop", "Purchase", fmt.Sprintf("uid=%d does not have enough resources", client.Commander.CommanderID), logger.LOG_LEVEL_INFO)
		response.Result = proto.Uint32(1)
		return 0, 16002, nil
	}
	if shopOffer.Genre == "shopping_street" {
		ctx := context.Background()
		var buyCount int64
		err := db.DefaultStore.Pool.QueryRow(ctx, `
SELECT buy_count
FROM shopping_street_goods
WHERE commander_id = $1 AND goods_id = $2
`, int64(client.Commander.CommanderID), int64(shopOffer.ID)).Scan(&buyCount)
		if err != nil || buyCount <= 0 {
			response.Result = proto.Uint32(1)
			return client.SendMessage(16002, &response)
		}
	}

	response.DropList = make([]*protobuf.DROPINFO, len(shopOffer.Effects))

	switch shopOffer.Type {
	case 1: // bought resources
		for i, resourceId := range shopOffer.Effects {
			client.Commander.AddResource(uint32(resourceId), uint32(shopOffer.Number))
			response.DropList[i] = &protobuf.DROPINFO{
				Type:   proto.Uint32(shopOffer.Type), // ressource
				Id:     proto.Uint32(uint32(resourceId)),
				Number: proto.Uint32(uint32(shopOffer.Number)),
			}
		}
	case 2: // packs
		for i, packId := range shopOffer.Effects {
			client.Commander.AddItem(uint32(packId), uint32(shopOffer.Number))
			response.DropList[i] = &protobuf.DROPINFO{
				Type:   proto.Uint32(shopOffer.Type), // item
				Id:     proto.Uint32(uint32(packId)),
				Number: proto.Uint32(uint32(shopOffer.Number)),
			}
		}
	case 4: // merit shop, to implement
		response.Result = proto.Uint32(3)
	case 6: // skins
		for i, skinId := range shopOffer.Effects {
			client.Commander.GiveSkin(uint32(skinId))
			response.DropList[i] = &protobuf.DROPINFO{
				Type:   proto.Uint32(shopOffer.Type), // skin
				Id:     proto.Uint32(uint32(skinId)),
				Number: proto.Uint32(uint32(shopOffer.Number)),
			}
		}
	case 12: // operation siren, to implement
	case 20:
		response.Result = proto.Uint32(3)
	case consts.DROP_TYPE_DORM3D_GIFT: // dorm3d gifts.
		// These are not commander_items: the client counts them out of
		// SC_28000.gifts[], so they have to land in the apartment record.
		apartment, apErr := orm.GetOrCreateDorm3dApartment(client.Commander.CommanderID)
		if apErr != nil {
			response.Result = proto.Uint32(2)
			break
		}
		for i, giftID := range shopOffer.Effects {
			gift := apartment.EnsureGiftEntry(uint32(giftID))
			gift.Number += uint32(shopOffer.Number)
			response.DropList[i] = &protobuf.DROPINFO{
				Type:   proto.Uint32(shopOffer.Type),
				Id:     proto.Uint32(uint32(giftID)),
				Number: proto.Uint32(uint32(shopOffer.Number)),
			}
		}
		if saveErr := orm.SaveDorm3dApartment(apartment); saveErr != nil {
			response.Result = proto.Uint32(2)
		}
	case consts.DROP_TYPE_DORM3D_FURNITURE: // dorm3d furniture.
		// Same story as gifts: furniture lives in the apartment record, not in
		// commander_items. It is recorded unplaced (slot 0) until the player
		// arranges it, which is what SC_28000.rooms[].furnitures carries.
		apartment, apErr := orm.GetOrCreateDorm3dApartment(client.Commander.CommanderID)
		if apErr != nil {
			response.Result = proto.Uint32(2)
			break
		}
		room := apartment.RoomByID(1)
		if room == nil {
			response.Result = proto.Uint32(2)
			break
		}
		for i, furnitureID := range shopOffer.Effects {
			bought := uint32(shopOffer.Number)
			for n := uint32(0); n < bought; n++ {
				room.Furnitures = append(room.Furnitures, orm.Dorm3dFurniture{FurnitureID: uint32(furnitureID)})
			}
			response.DropList[i] = &protobuf.DROPINFO{
				Type:   proto.Uint32(shopOffer.Type),
				Id:     proto.Uint32(uint32(furnitureID)),
				Number: proto.Uint32(bought),
			}
		}
		if saveErr := orm.SaveDorm3dApartment(apartment); saveErr != nil {
			response.Result = proto.Uint32(2)
		}
	default:
		response.Result = proto.Uint32(2)
	}

	if response.GetResult() == 0 {
		if shopOffer.Genre == "shopping_street" {
			ctx := context.Background()
			_, _ = db.DefaultStore.Pool.Exec(ctx, `
UPDATE shopping_street_goods
SET buy_count = buy_count - 1
WHERE commander_id = $1 AND goods_id = $2 AND buy_count > 0
`, int64(client.Commander.CommanderID), int64(shopOffer.ID))
		}
		client.Commander.ConsumeResource(shopOffer.ResourceID, uint32(shopOffer.ResourceNumber))
		logger.LogEvent("Shop", "Purchase", fmt.Sprintf("uid=%d bought #%d successfully!", client.Commander.CommanderID, shopOffer.ID), logger.LOG_LEVEL_INFO)
	}

	return client.SendMessage(16002, &response)
}

func loadShopOfferByID(offerID uint32) (*orm.ShopOffer, error) {
	ctx := context.Background()
	row := orm.ShopOffer{}
	var rawEffects []byte
	err := db.DefaultStore.Pool.QueryRow(ctx, `
SELECT id, effects, effect_args, number, resource_number, resource_id, type, genre, discount
FROM shop_offers
WHERE id = $1
`, int64(offerID)).Scan(
		&row.ID,
		&rawEffects,
		&row.EffectArgs,
		&row.Number,
		&row.ResourceNumber,
		&row.ResourceID,
		&row.Type,
		&row.Genre,
		&row.Discount,
	)
	err = db.MapNotFound(err)
	if err != nil {
		return nil, err
	}
	if len(rawEffects) > 0 {
		if err := json.Unmarshal(rawEffects, &row.Effects); err != nil {
			return nil, err
		}
	}
	return &row, nil
}
