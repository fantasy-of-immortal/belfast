package dorm3d

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/consts"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const (
	dorm3dSetCategory          = "ShareCfg/dorm3d_set.json"
	dorm3dGiftCategory         = "ShareCfg/dorm3d_gift.json"
	dorm3dFavorTriggerCategory = "ShareCfg/dorm3d_favor_trigger.json"
	dorm3dFavorCategory        = "ShareCfg/dorm3d_favor.json"
	dorm3dDormTemplateCategory = "ShareCfg/dorm3d_dorm_template.json"

	dorm3dFavorResultSuccess = uint32(0)
	dorm3dFavorResultFailure = uint32(1)
)

type dorm3dGiftConfig struct {
	ID             uint32 `json:"id"`
	ShipGroupID    uint32 `json:"ship_group_id"`
	FavorTriggerID uint32 `json:"favor_trigger_id"`
}

type dorm3dFavorTriggerConfig struct {
	ID         uint32 `json:"id"`
	IsDailyMax uint32 `json:"is_daily_max"`
	IsRepeat   uint32 `json:"is_repeat"`
	Num        uint32 `json:"num"`
}

type dorm3dSetConfig struct {
	Key         string `json:"key"`
	KeyValueInt uint32 `json:"key_value_int"`
}

type dorm3dFavorLevelConfig struct {
	ID          uint32     `json:"id"`
	CharID      uint32     `json:"char_id"`
	Level       uint32     `json:"level"`
	FavorExp    uint32     `json:"favor_exp"`
	LevelupItem [][]uint32 `json:"levelup_item"`
}

type dorm3dDormTemplateConfig struct {
	ID uint32 `json:"id"`
}

type dorm3dTriggerRuntime struct {
	vigorUsed   uint32
	favorExp    uint32
	dailyFavor  uint32
	shipAtMaxLv bool
	maxFavorExp uint32
	counts      map[uint32]uint32
	newTriggers []uint32
}

func Dorm3dTriggerFavor(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_28003
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 28004, err
	}
	if client.Commander == nil {
		return 0, 28004, errors.New("missing commander")
	}
	shipGroup := payload.GetShipGroup()
	triggerID := payload.GetTriggerId()
	if shipGroup == 0 || triggerID == 0 {
		return sendDorm3dTriggerFavorResult(client, dorm3dFavorResultFailure)
	}
	apartment, err := orm.GetOrCreateDorm3dApartment(client.Commander.CommanderID)
	if err != nil {
		return 0, 28004, err
	}
	ship := apartment.FindShip(shipGroup)
	if ship == nil {
		return sendDorm3dTriggerFavorResult(client, dorm3dFavorResultFailure)
	}
	triggerCfg, err := loadDorm3dFavorTriggerConfig(triggerID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return sendDorm3dTriggerFavorResult(client, dorm3dFavorResultFailure)
		}
		return 0, 28004, err
	}
	dailyVigorMax, err := loadDorm3dSetInt("daily_vigor_max")
	if err != nil {
		return 0, 28004, err
	}
	maxLevel, err := loadDorm3dSetInt("favor_level")
	if err != nil {
		return 0, 28004, err
	}
	charID, err := resolveDorm3dCharID(shipGroup)
	if err != nil {
		return 0, 28004, err
	}
	maxFavorExp, err := loadDorm3dMaxFavorExp(charID, maxLevel)
	if err != nil {
		return 0, 28004, err
	}
	runtime := newDorm3dTriggerRuntime(apartment, ship, ship.FavorLv >= maxLevel, maxFavorExp)
	if err := runtime.apply(triggerCfg, dailyVigorMax); err != nil {
		return sendDorm3dTriggerFavorResult(client, dorm3dFavorResultFailure)
	}
	runtime.commit(apartment, ship)
	if err := orm.SaveDorm3dApartment(apartment); err != nil {
		return 0, 28004, err
	}
	return sendDorm3dTriggerFavorResult(client, dorm3dFavorResultSuccess)
}

func Dorm3dApartmentLevelUp(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_28005
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 28006, err
	}
	if client.Commander == nil {
		return 0, 28006, errors.New("missing commander")
	}
	shipGroup := payload.GetShipGroup()
	if shipGroup == 0 {
		return sendDorm3dApartmentLevelUpResult(client, dorm3dFavorResultFailure, []*protobuf.DROPINFO{})
	}
	maxLevel, err := loadDorm3dSetInt("favor_level")
	if err != nil {
		return 0, 28006, err
	}
	charID, err := resolveDorm3dCharID(shipGroup)
	if err != nil {
		return 0, 28006, err
	}
	var drops []*protobuf.DROPINFO
	ctx := context.Background()
	if err := orm.WithPGXTx(ctx, func(tx pgx.Tx) error {
		apartment, txErr := orm.GetOrCreateDorm3dApartmentTx(ctx, tx, client.Commander.CommanderID)
		if txErr != nil {
			return txErr
		}
		ship := apartment.FindShip(shipGroup)
		if ship == nil {
			return errDorm3dBusinessFailed
		}
		if ship.FavorLv >= maxLevel {
			return errDorm3dBusinessFailed
		}
		nextLevel := ship.FavorLv + 1
		nextFavorCfg, txErr := loadDorm3dFavorLevelConfig(charID, nextLevel)
		if txErr != nil {
			if errors.Is(txErr, db.ErrNotFound) {
				return errDorm3dBusinessFailed
			}
			return txErr
		}
		if ship.FavorExp < nextFavorCfg.FavorExp {
			return errDorm3dBusinessFailed
		}
		ship.FavorLv = nextLevel
		ship.FavorExp -= nextFavorCfg.FavorExp

		drops, txErr = grantDorm3dLevelUpDropsTx(ctx, tx, client.Commander, nextFavorCfg.LevelupItem)
		if txErr != nil {
			return txErr
		}
		return orm.SaveDorm3dApartmentTx(ctx, tx, apartment)
	}); err != nil {
		if errors.Is(err, errDorm3dBusinessFailed) {
			return sendDorm3dApartmentLevelUpResult(client, dorm3dFavorResultFailure, []*protobuf.DROPINFO{})
		}
		return 0, 28006, err
	}
	return sendDorm3dApartmentLevelUpResult(client, dorm3dFavorResultSuccess, drops)
}

func HandleDorm3dGiveGift(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_28009
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 28010, err
	}
	if client.Commander == nil {
		return 0, 28010, errors.New("missing commander")
	}
	shipGroup := payload.GetShipGroup()
	giftPayloads := payload.GetGifts()
	if shipGroup == 0 || len(giftPayloads) == 0 {
		return sendDorm3dGiveGiftResult(client, dorm3dFavorResultFailure)
	}
	apartment, err := orm.GetOrCreateDorm3dApartment(client.Commander.CommanderID)
	if err != nil {
		return 0, 28010, err
	}
	ship := apartment.FindShip(shipGroup)
	if ship == nil {
		return sendDorm3dGiveGiftResult(client, dorm3dFavorResultFailure)
	}
	dailyVigorMax, err := loadDorm3dSetInt("daily_vigor_max")
	if err != nil {
		return 0, 28010, err
	}
	maxLevel, err := loadDorm3dSetInt("favor_level")
	if err != nil {
		return 0, 28010, err
	}
	charID, err := resolveDorm3dCharID(shipGroup)
	if err != nil {
		return 0, 28010, err
	}
	maxFavorExp, err := loadDorm3dMaxFavorExp(charID, maxLevel)
	if err != nil {
		return 0, 28010, err
	}

	requestedByGiftID := make(map[uint32]uint32)
	for _, giftPayload := range giftPayloads {
		if giftPayload == nil || giftPayload.GetGiftId() == 0 || giftPayload.GetNumber() == 0 {
			return sendDorm3dGiveGiftResult(client, dorm3dFavorResultFailure)
		}
		requestedByGiftID[giftPayload.GetGiftId()] += giftPayload.GetNumber()
	}

	runtime := newDorm3dTriggerRuntime(apartment, ship, ship.FavorLv >= maxLevel, maxFavorExp)
	for giftID, requested := range requestedByGiftID {
		giftState := apartment.FindGift(giftID)
		if giftState == nil || requested > giftState.Number {
			return sendDorm3dGiveGiftResult(client, dorm3dFavorResultFailure)
		}
		giftCfg, err := loadDorm3dGiftConfig(giftID)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				return sendDorm3dGiveGiftResult(client, dorm3dFavorResultFailure)
			}
			return 0, 28010, err
		}
		if giftCfg.ShipGroupID != 0 && giftCfg.ShipGroupID != shipGroup {
			return sendDorm3dGiveGiftResult(client, dorm3dFavorResultFailure)
		}
		if giftCfg.ShipGroupID != 0 && (giftState.UsedNumber > 0 || requested > 1) {
			return sendDorm3dGiveGiftResult(client, dorm3dFavorResultFailure)
		}
		if giftCfg.FavorTriggerID == 0 {
			continue
		}
		triggerCfg, err := loadDorm3dFavorTriggerConfig(giftCfg.FavorTriggerID)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				return sendDorm3dGiveGiftResult(client, dorm3dFavorResultFailure)
			}
			return 0, 28010, err
		}
		for i := uint32(0); i < requested; i++ {
			if err := runtime.apply(triggerCfg, dailyVigorMax); err != nil {
				return sendDorm3dGiveGiftResult(client, dorm3dFavorResultFailure)
			}
		}
	}

	for giftID, requested := range requestedByGiftID {
		giftState := apartment.FindGift(giftID)
		giftState.Number -= requested
		giftState.UsedNumber += requested
	}
	runtime.commit(apartment, ship)
	if err := orm.SaveDorm3dApartment(apartment); err != nil {
		return 0, 28010, err
	}
	return sendDorm3dGiveGiftResult(client, dorm3dFavorResultSuccess)
}

func sendDorm3dTriggerFavorResult(client *connection.Client, result uint32) (int, int, error) {
	return client.SendMessage(28004, &protobuf.SC_28004{Result: proto.Uint32(result)})
}

func sendDorm3dApartmentLevelUpResult(client *connection.Client, result uint32, drops []*protobuf.DROPINFO) (int, int, error) {
	return client.SendMessage(28006, &protobuf.SC_28006{
		Result:   proto.Uint32(result),
		DropList: drops,
	})
}

func sendDorm3dGiveGiftResult(client *connection.Client, result uint32) (int, int, error) {
	return client.SendMessage(28010, &protobuf.SC_28010{Result: proto.Uint32(result)})
}

var errDorm3dBusinessFailed = errors.New("dorm3d business validation failed")

func grantDorm3dLevelUpDropsTx(ctx context.Context, tx pgx.Tx, commander *orm.Commander, tuples [][]uint32) ([]*protobuf.DROPINFO, error) {
	drops := make([]*protobuf.DROPINFO, 0, len(tuples))
	for _, tuple := range tuples {
		if len(tuple) < 3 || tuple[2] == 0 {
			continue
		}
		dropType := tuple[0]
		dropID := tuple[1]
		count := tuple[2]
		switch dropType {
		case consts.DROP_TYPE_RESOURCE:
			if err := commander.AddResourceTx(ctx, tx, dropID, count); err != nil {
				return nil, err
			}
		case consts.DROP_TYPE_ITEM, consts.DROP_TYPE_ICON_FRAME:
			if err := commander.AddItemTx(ctx, tx, dropID, count); err != nil {
				return nil, err
			}
		case consts.DROP_TYPE_DORM3D_GIFT:
			// Dorm3d gifts are not commander_items: the client counts them out of
			// SC_28000.gifts[], so they must land in the apartment record.
			// Level-up rewards use this type (e.g. dorm3d_favor 20220 lv2 grants
			// [27, 1021001, 1]); returning an error here aborted the whole
			// level-up, closed the connection and left the client retrying forever.
			apartment, apErr := orm.GetOrCreateDorm3dApartmentTx(ctx, tx, commander.CommanderID)
			if apErr != nil {
				return nil, apErr
			}
			apartment.EnsureGiftEntry(dropID).Number += count
			if saveErr := orm.SaveDorm3dApartmentTx(ctx, tx, apartment); saveErr != nil {
				return nil, saveErr
			}
		case consts.DROP_TYPE_DORM3D_FURNITURE:
			// Furniture belongs to a dorm room and is not placed anywhere yet:
			// record it as an unplaced (slot 0) entry so it shows up in the
			// apartment data the client receives.
			apartment, apErr := orm.GetOrCreateDorm3dApartmentTx(ctx, tx, commander.CommanderID)
			if apErr != nil {
				return nil, apErr
			}
			if room := apartment.RoomByID(1); room != nil {
				for i := uint32(0); i < count; i++ {
					room.Furnitures = append(room.Furnitures, orm.Dorm3dFurniture{FurnitureID: dropID})
				}
				if saveErr := orm.SaveDorm3dApartmentTx(ctx, tx, apartment); saveErr != nil {
					return nil, saveErr
				}
			}
		case consts.DROP_TYPE_LIVINGAREA_COVER:
			if _, coverErr := orm.GetCommanderLivingAreaCoverEntry(commander.CommanderID, dropID); coverErr != nil {
				if !errors.Is(coverErr, db.ErrNotFound) {
					return nil, coverErr
				}
				if upErr := orm.UpsertCommanderLivingAreaCover(orm.CommanderLivingAreaCover{
					CommanderID: commander.CommanderID,
					CoverID:     dropID,
					IsNew:       true,
				}); upErr != nil {
					return nil, upErr
				}
			}
		default:
			return nil, fmt.Errorf("unsupported dorm3d reward type %d", dropType)
		}
		drops = append(drops, newDropInfo(dropType, dropID, count))
	}
	return drops, nil
}

func loadDorm3dGiftConfig(giftID uint32) (*dorm3dGiftConfig, error) {
	entry, err := orm.GetConfigEntry(dorm3dGiftCategory, strconv.FormatUint(uint64(giftID), 10))
	if err != nil {
		return nil, err
	}
	var cfg dorm3dGiftConfig
	if err := json.Unmarshal(entry.Data, &cfg); err != nil {
		return nil, err
	}
	if cfg.ID == 0 {
		return nil, fmt.Errorf("invalid dorm3d gift config id %d", giftID)
	}
	return &cfg, nil
}

func loadDorm3dFavorTriggerConfig(triggerID uint32) (*dorm3dFavorTriggerConfig, error) {
	entry, err := orm.GetConfigEntry(dorm3dFavorTriggerCategory, strconv.FormatUint(uint64(triggerID), 10))
	if err != nil {
		return nil, err
	}
	var cfg dorm3dFavorTriggerConfig
	if err := json.Unmarshal(entry.Data, &cfg); err != nil {
		return nil, err
	}
	if cfg.ID == 0 {
		return nil, fmt.Errorf("invalid dorm3d favor trigger config id %d", triggerID)
	}
	return &cfg, nil
}

func loadDorm3dSetInt(key string) (uint32, error) {
	entry, err := orm.GetConfigEntry(dorm3dSetCategory, key)
	if err != nil {
		return 0, err
	}
	var cfg dorm3dSetConfig
	if err := json.Unmarshal(entry.Data, &cfg); err != nil {
		return 0, err
	}
	return cfg.KeyValueInt, nil
}

// dorm3dStarterGiftCount is how many of each applicable gift a commander is
// granted when the inventory is first seeded.
const dorm3dStarterGiftCount = 3

// SeedDorm3dGifts fills an empty gift inventory from ShareCfg/dorm3d_gift.json.
//
// The client refuses to send CS_28009 at all while its giftBag is empty
// (apartmentgivegiftcommand.lua:7 -> getGiftCount -> apartmentproxy.lua:25-27,
// whose only source is SC_28000.gifts), so an unseeded apartment can never give
// a gift. Gift ids are read from the config table -- never guessed -- and are
// limited to gifts usable by a ship the commander actually has in the dorm.
func SeedDorm3dGifts(apartment *orm.Dorm3dApartment) (bool, error) {
	if len(apartment.Gifts) > 0 || len(apartment.Ships) == 0 {
		return false, nil
	}
	entries, err := orm.ListConfigEntries(dorm3dGiftCategory)
	if err != nil {
		return false, err
	}
	owned := make(map[uint32]bool, len(apartment.Ships))
	for _, ship := range apartment.Ships {
		owned[ship.ShipGroup] = true
	}
	seeded := false
	for _, entry := range entries {
		var cfg dorm3dGiftConfig
		if err := json.Unmarshal(entry.Data, &cfg); err != nil || cfg.ID == 0 {
			continue
		}
		// ShipGroupID == 0 marks a universal gift; otherwise it is restricted to
		// that ship_group and the dorm must actually contain that ship.
		if cfg.ShipGroupID != 0 && !owned[cfg.ShipGroupID] {
			continue
		}
		gift := apartment.EnsureGiftEntry(cfg.ID)
		gift.Number += dorm3dStarterGiftCount
		seeded = true
	}
	return seeded, nil
}

func resolveDorm3dCharID(shipGroup uint32) (uint32, error) {
	entry, err := orm.GetConfigEntry(dorm3dDormTemplateCategory, strconv.FormatUint(uint64(shipGroup), 10))
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return shipGroup, nil
		}
		return 0, err
	}
	var cfg dorm3dDormTemplateConfig
	if err := json.Unmarshal(entry.Data, &cfg); err != nil {
		return 0, err
	}
	if cfg.ID == 0 {
		return shipGroup, nil
	}
	return cfg.ID, nil
}

func loadDorm3dMaxFavorExp(charID uint32, maxLevel uint32) (uint32, error) {
	favorCfg, err := loadDorm3dFavorLevelConfig(charID, maxLevel)
	if err != nil {
		return 0, err
	}
	return favorCfg.FavorExp, nil
}

func loadDorm3dFavorLevelConfig(charID uint32, level uint32) (*dorm3dFavorLevelConfig, error) {
	entries, err := orm.ListConfigEntries(dorm3dFavorCategory)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		var cfg dorm3dFavorLevelConfig
		if err := json.Unmarshal(entry.Data, &cfg); err != nil {
			continue
		}
		if cfg.CharID == charID && cfg.Level == level {
			return &cfg, nil
		}
	}
	return nil, db.ErrNotFound
}

func newDorm3dTriggerRuntime(apartment *orm.Dorm3dApartment, ship *orm.Dorm3dShip, shipAtMaxLv bool, maxFavorExp uint32) dorm3dTriggerRuntime {
	counts := make(map[uint32]uint32, len(ship.RegularTrigger))
	for _, triggerID := range ship.RegularTrigger {
		counts[triggerID]++
	}
	return dorm3dTriggerRuntime{
		vigorUsed:   apartment.DailyVigorMax,
		favorExp:    ship.FavorExp,
		dailyFavor:  ship.DailyFavor,
		shipAtMaxLv: shipAtMaxLv,
		maxFavorExp: maxFavorExp,
		counts:      counts,
		newTriggers: []uint32{},
	}
}

func (runtime *dorm3dTriggerRuntime) apply(triggerCfg *dorm3dFavorTriggerConfig, dailyVigorMax uint32) error {
	if triggerCfg.IsRepeat == 0 && runtime.counts[triggerCfg.ID] > 0 {
		return fmt.Errorf("trigger %d already consumed", triggerCfg.ID)
	}
	if runtime.vigorUsed+triggerCfg.IsDailyMax > dailyVigorMax {
		return fmt.Errorf("insufficient vigor")
	}
	runtime.vigorUsed += triggerCfg.IsDailyMax
	gain := triggerCfg.Num
	if runtime.shipAtMaxLv {
		if runtime.favorExp >= runtime.maxFavorExp {
			gain = 0
		} else if runtime.favorExp+gain > runtime.maxFavorExp {
			gain = runtime.maxFavorExp - runtime.favorExp
		}
	}
	runtime.favorExp += gain
	runtime.dailyFavor += gain
	runtime.counts[triggerCfg.ID]++
	runtime.newTriggers = append(runtime.newTriggers, triggerCfg.ID)
	return nil
}

func (runtime *dorm3dTriggerRuntime) commit(apartment *orm.Dorm3dApartment, ship *orm.Dorm3dShip) {
	apartment.DailyVigorMax = runtime.vigorUsed
	ship.FavorExp = runtime.favorExp
	ship.DailyFavor = runtime.dailyFavor
	ship.RegularTrigger = append(ship.RegularTrigger, runtime.newTriggers...)
}
