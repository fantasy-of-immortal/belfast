package dorm3d

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/consts"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

const (
	dorm3dRoomsCategory   = "ShareCfg/dorm3d_rooms.json"
	shopTemplateCategory  = "ShareCfg/shop_template.json"
	dorm3dResultSuccess   = uint32(0)
	dorm3dResultFailure   = uint32(1)
	dorm3dResultNoCost    = uint32(2)
	dorm3dResultNoCommand = "missing commander"
)

type dorm3dCost struct {
	DropType uint32
	DropID   uint32
	Count    uint32
}

type dorm3dRoomConfig struct {
	ID           uint32          `json:"id"`
	Type         uint32          `json:"type"`
	Character    []uint32        `json:"character"`
	UnlockItem   json.RawMessage `json:"unlock_item"`
	CharacterPay json.RawMessage `json:"character_pay"`
	InviteCost   json.RawMessage `json:"invite_cost"`
}

// dorm3dSeedRoomShips returns the ship_groups a freshly unlocked room should
// contain. ShareCfg/dorm3d_rooms.json lists them in `character`; for personal
// rooms (type == 2) the first entry is the resident character and must be
// present so the client's ApartmentRoom.unlockCharacter map is non-empty
// (see apartmentroom.lua:3-12). Without this the room exists but shows no
// character, the client reports "invalid operation" and loading never ends.
func dorm3dSeedRoomShips(roomCfg *dorm3dRoomConfig) []uint32 {
	ships := make([]uint32, 0, len(roomCfg.Character))
	seen := make(map[uint32]bool, len(roomCfg.Character))
	for _, shipGroup := range roomCfg.Character {
		if shipGroup == 0 || seen[shipGroup] {
			continue
		}
		seen[shipGroup] = true
		ships = append(ships, shipGroup)
	}
	return ships
}

func SelectDorm3dEnter(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_28017
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 28018, err
	}
	response := protobuf.SC_28018{Result: proto.Uint32(dorm3dResultSuccess)}
	return client.SendMessage(28018, &response)
}

func Dorm3dRoomUnlock(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_28001
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 28002, err
	}
	if client.Commander == nil {
		return 0, 28002, errors.New(dorm3dResultNoCommand)
	}

	apartment, err := orm.GetOrCreateDorm3dApartment(client.Commander.CommanderID)
	if err != nil {
		return 0, 28002, err
	}

	roomID := payload.GetRoomId()
	roomCfg, err := loadDorm3dRoomConfig(roomID)
	if err != nil {
		return sendDorm3dRoomUnlockFailure(client, apartment)
	}
	if existing := apartment.RoomByID(roomID); existing != nil {
		// A room persisted before ships were seeded carries an empty ship list
		// forever: the client then has no unlockable character, never completes
		// loading, and its own "no room" check makes it re-send CS_28001 here.
		// Backfill the missing ships (no cost) and report success so the client
		// can proceed instead of looping on "invalid operation".
		if len(existing.Ships) == 0 {
			if backfilled, ok := dorm3dBackfillRoomShips(apartment, roomID, roomCfg); ok {
				return sendDorm3dRoomUnlockSuccess(client, apartment, backfilled)
			}
		}
		return sendDorm3dRoomUnlockFailure(client, apartment)
	}

	costs, err := parseDorm3dCosts(roomCfg.UnlockItem)
	if err != nil {
		return sendDorm3dRoomUnlockFailure(client, apartment)
	}
	ctx := context.Background()
	var room orm.Dorm3dRoom
	if err := orm.WithPGXTx(ctx, func(tx pgx.Tx) error {
		freshApartment, txErr := orm.GetOrCreateDorm3dApartmentTx(ctx, tx, client.Commander.CommanderID)
		if txErr != nil {
			return txErr
		}
		if freshApartment.RoomByID(roomID) != nil {
			return fmt.Errorf("room already unlocked")
		}

		room = orm.Dorm3dRoom{
			ID:          roomID,
			Furnitures:  []orm.Dorm3dFurniture{},
			Collections: []uint32{},
			Ships:       dorm3dSeedRoomShips(roomCfg),
		}
		if !freshApartment.AddRoom(room) {
			return fmt.Errorf("room already unlocked")
		}

		for _, cost := range costs {
			if txErr := consumeDorm3dCostTx(ctx, tx, client.Commander, cost); txErr != nil {
				return txErr
			}
		}

		return orm.SaveDorm3dApartmentTx(ctx, tx, freshApartment)
	}); err != nil {
		return sendDorm3dRoomUnlockFailure(client, apartment)
	}

	return sendDorm3dRoomUnlockSuccess(client, apartment, room)
}

func Dorm3dReplaceFurniture(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_28007
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 28008, err
	}
	if client.Commander == nil {
		return 0, 28008, errors.New(dorm3dResultNoCommand)
	}

	apartment, err := orm.GetOrCreateDorm3dApartment(client.Commander.CommanderID)
	if err != nil {
		return 0, 28008, err
	}
	room := apartment.RoomByID(payload.GetRoomId())
	if room == nil {
		response := protobuf.SC_28008{Result: proto.Uint32(dorm3dResultFailure)}
		return client.SendMessage(28008, &response)
	}

	for _, op := range payload.GetFurnitures() {
		slotID := op.GetSlotId()
		for i := range room.Furnitures {
			if room.Furnitures[i].SlotID == slotID {
				room.Furnitures[i].SlotID = 0
			}
		}
		furnitureID := op.GetFurnitureId()
		if furnitureID == 0 {
			continue
		}
		for i := range room.Furnitures {
			if room.Furnitures[i].FurnitureID == furnitureID && room.Furnitures[i].SlotID == 0 {
				room.Furnitures[i].SlotID = slotID
				break
			}
		}
	}

	if err := orm.SaveDorm3dApartment(apartment); err != nil {
		response := protobuf.SC_28008{Result: proto.Uint32(dorm3dResultFailure)}
		return client.SendMessage(28008, &response)
	}
	response := protobuf.SC_28008{Result: proto.Uint32(dorm3dResultSuccess)}
	return client.SendMessage(28008, &response)
}

func Dorm3dRoomInviteUnlock(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_28019
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 28020, err
	}
	if client.Commander == nil {
		return 0, 28020, errors.New(dorm3dResultNoCommand)
	}

	apartment, err := orm.GetOrCreateDorm3dApartment(client.Commander.CommanderID)
	if err != nil {
		return 0, 28020, err
	}
	room := apartment.RoomByID(payload.GetRoomId())
	if room == nil {
		return sendDorm3dResultOnly(client, 28020, dorm3dResultFailure)
	}
	shipGroup := payload.GetShipGroup()
	if dorm3dRoomHasShip(room, shipGroup) {
		return sendDorm3dResultOnly(client, 28020, dorm3dResultFailure)
	}

	roomCfg, err := loadDorm3dRoomConfig(payload.GetRoomId())
	if err != nil {
		return sendDorm3dResultOnly(client, 28020, dorm3dResultFailure)
	}
	if !isInviteCharacterAllowed(roomCfg.CharacterPay, shipGroup) {
		return sendDorm3dResultOnly(client, 28020, dorm3dResultFailure)
	}
	shopID, ok := resolveInviteShopID(roomCfg.InviteCost, shipGroup)
	if !ok {
		return sendDorm3dResultOnly(client, 28020, dorm3dResultFailure)
	}
	costResourceID, costAmount, ok := loadInviteCost(shopID)
	if !ok {
		return sendDorm3dResultOnly(client, 28020, dorm3dResultFailure)
	}
	ctx := context.Background()
	if err := orm.WithPGXTx(ctx, func(tx pgx.Tx) error {
		freshApartment, txErr := orm.GetOrCreateDorm3dApartmentTx(ctx, tx, client.Commander.CommanderID)
		if txErr != nil {
			return txErr
		}
		freshRoom := freshApartment.RoomByID(payload.GetRoomId())
		if freshRoom == nil || dorm3dRoomHasShip(freshRoom, shipGroup) {
			return fmt.Errorf("invite unlock invalid state")
		}
		if txErr := client.Commander.ConsumeResourceTx(ctx, tx, costResourceID, costAmount); txErr != nil {
			return txErr
		}
		freshRoom.Ships = append(freshRoom.Ships, shipGroup)
		return orm.SaveDorm3dApartmentTx(ctx, tx, freshApartment)
	}); err != nil {
		if err.Error() == "not enough resources" {
			return sendDorm3dResultOnly(client, 28020, dorm3dResultNoCost)
		}
		return sendDorm3dResultOnly(client, 28020, dorm3dResultFailure)
	}

	return sendDorm3dResultOnly(client, 28020, dorm3dResultSuccess)
}

// dorm3dBackfillRoomShips repairs a room that was persisted with an empty ship
// list. It only ever adds ships, never removes them, and is a no-op returning
// false when the room config yields nothing to add. Cost is not charged.
func dorm3dBackfillRoomShips(apartment *orm.Dorm3dApartment, roomID uint32, roomCfg *dorm3dRoomConfig) (orm.Dorm3dRoom, bool) {
	ships := dorm3dSeedRoomShips(roomCfg)
	if len(ships) == 0 {
		return orm.Dorm3dRoom{}, false
	}
	ctx := context.Background()
	var updated orm.Dorm3dRoom
	if err := orm.WithPGXTx(ctx, func(tx pgx.Tx) error {
		freshApartment, txErr := orm.GetOrCreateDorm3dApartmentTx(ctx, tx, apartment.CommanderID)
		if txErr != nil {
			return txErr
		}
		freshRoom := freshApartment.RoomByID(roomID)
		if freshRoom == nil || len(freshRoom.Ships) != 0 {
			return errDorm3dRoomNoBackfill
		}
		freshRoom.Ships = ships
		if txErr := orm.SaveDorm3dApartmentTx(ctx, tx, freshApartment); txErr != nil {
			return txErr
		}
		updated = *freshRoom
		return nil
	}); err != nil {
		return orm.Dorm3dRoom{}, false
	}
	return updated, true
}

var errDorm3dRoomNoBackfill = errors.New("dorm3d room does not need ship backfill")

func sendDorm3dRoomUnlockSuccess(client *connection.Client, apartment *orm.Dorm3dApartment, room orm.Dorm3dRoom) (int, int, error) {
	response := protobuf.SC_28002{
		Result: proto.Uint32(dorm3dResultSuccess),
		Room:   buildDorm3dRooms(orm.Dorm3dRoomList{room})[0],
		Ins:    buildDorm3dIns(apartment.Ins),
	}
	return client.SendMessage(28002, &response)
}

func sendDorm3dRoomUnlockFailure(client *connection.Client, apartment *orm.Dorm3dApartment) (int, int, error) {
	response := protobuf.SC_28002{
		Result: proto.Uint32(dorm3dResultFailure),
		Room: &protobuf.APARTMENT_ROOM{
			Id:          proto.Uint32(0),
			Furnitures:  []*protobuf.APARTMENT_FURNITURE{},
			Collections: []uint32{},
			Ships:       []uint32{},
		},
		Ins: buildDorm3dIns(apartment.Ins),
	}
	return client.SendMessage(28002, &response)
}

func sendDorm3dResultOnly(client *connection.Client, packetID int, result uint32) (int, int, error) {
	switch packetID {
	case 28008:
		response := protobuf.SC_28008{Result: proto.Uint32(result)}
		return client.SendMessage(28008, &response)
	case 28018:
		response := protobuf.SC_28018{Result: proto.Uint32(result)}
		return client.SendMessage(28018, &response)
	default:
		response := protobuf.SC_28020{Result: proto.Uint32(result)}
		return client.SendMessage(28020, &response)
	}
}

func loadDorm3dRoomConfig(roomID uint32) (*dorm3dRoomConfig, error) {
	entry, err := orm.GetConfigEntry(dorm3dRoomsCategory, strconv.FormatUint(uint64(roomID), 10))
	if err != nil {
		return nil, err
	}
	var cfg dorm3dRoomConfig
	if err := json.Unmarshal(entry.Data, &cfg); err != nil {
		return nil, err
	}
	if cfg.ID == 0 {
		cfg.ID = roomID
	}
	return &cfg, nil
}

func parseDorm3dCosts(raw json.RawMessage) ([]dorm3dCost, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return []dorm3dCost{}, nil
	}
	var list []any
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, err
	}
	costs := make([]dorm3dCost, 0, len(list))
	for _, entry := range list {
		triple, ok := entry.([]any)
		if !ok || len(triple) < 3 {
			continue
		}
		dropType, ok := parseJSONUint(triple[0])
		if !ok {
			continue
		}
		dropID, ok := parseJSONUint(triple[1])
		if !ok {
			continue
		}
		count, ok := parseJSONUint(triple[2])
		if !ok {
			continue
		}
		costs = append(costs, dorm3dCost{DropType: dropType, DropID: dropID, Count: count})
	}
	return costs, nil
}

func consumeDorm3dCostTx(ctx context.Context, tx pgx.Tx, commander *orm.Commander, cost dorm3dCost) error {
	switch cost.DropType {
	case consts.DROP_TYPE_RESOURCE:
		return commander.ConsumeResourceTx(ctx, tx, cost.DropID, cost.Count)
	case consts.DROP_TYPE_ITEM:
		return commander.ConsumeItemTx(ctx, tx, cost.DropID, cost.Count)
	default:
		return fmt.Errorf("unsupported cost type %d", cost.DropType)
	}
}

func dorm3dRoomHasShip(room *orm.Dorm3dRoom, shipGroup uint32) bool {
	for _, current := range room.Ships {
		if current == shipGroup {
			return true
		}
	}
	return false
}

func isInviteCharacterAllowed(raw json.RawMessage, shipGroup uint32) bool {
	if len(raw) == 0 || string(raw) == "null" {
		return false
	}
	var list []any
	if err := json.Unmarshal(raw, &list); err != nil {
		return false
	}
	for _, entry := range list {
		switch value := entry.(type) {
		case float64:
			if uint32(value) == shipGroup {
				return true
			}
		case []any:
			if len(value) == 0 {
				continue
			}
			if id, ok := parseJSONUint(value[0]); ok && id == shipGroup {
				return true
			}
		}
	}
	return false
}

func resolveInviteShopID(raw json.RawMessage, shipGroup uint32) (uint32, bool) {
	if len(raw) == 0 || string(raw) == "null" || string(raw) == `""` {
		return 0, false
	}
	var list []any
	if err := json.Unmarshal(raw, &list); err == nil {
		for _, entry := range list {
			pair, ok := entry.([]any)
			if !ok || len(pair) < 2 {
				continue
			}
			groupID, ok := parseJSONUint(pair[0])
			if !ok || groupID != shipGroup {
				continue
			}
			shopID, ok := parseJSONUint(pair[1])
			if ok {
				return shopID, true
			}
		}
	}
	var direct uint32
	if err := json.Unmarshal(raw, &direct); err == nil && direct != 0 {
		return direct, true
	}
	return 0, false
}

func loadInviteCost(shopID uint32) (uint32, uint32, bool) {
	entry, err := orm.GetConfigEntry(shopTemplateCategory, strconv.FormatUint(uint64(shopID), 10))
	if err != nil {
		return 0, 0, false
	}
	var parsed struct {
		ResourceType uint32 `json:"resource_type"`
		ResourceNum  uint32 `json:"resource_num"`
	}
	if err := json.Unmarshal(entry.Data, &parsed); err != nil {
		return 0, 0, false
	}
	if parsed.ResourceType == 0 || parsed.ResourceNum == 0 {
		return 0, 0, false
	}
	return parsed.ResourceType, parsed.ResourceNum, true
}
