package orm

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/db/gen"
)

const (
	Dorm3dInstagramOpRead  uint32 = 3
	Dorm3dInstagramOpLike  uint32 = 4
	Dorm3dInstagramOpShare uint32 = 5
	Dorm3dInstagramOpExit  uint32 = 6

	dorm3dCollectionTemplateCategory = "ShareCfg/dorm3d_collection_template.json"
	dorm3dDialogueGroupCategory      = "ShareCfg/dorm3d_dialogue_group.json"
	dorm3dInsChatGroupCategory       = "ShareCfg/dorm3d_ins_chat_group.json"
	dorm3dInsUnlockCategory          = "ShareCfg/dorm3d_ins_unlock.json"
	dorm3dInsTelephoneGroupCategory  = "ShareCfg/dorm3d_ins_telephone_group.json"
	dorm3dInsTemplateCategory        = "ShareCfg/dorm3d_ins_template.json"
	dorm3dRoomsCategory              = "ShareCfg/dorm3d_rooms.json"
	dorm3dTriggerStateCategory       = "Runtime/dorm3d_ins_trigger_state"
)

var (
	ErrDorm3dShipNotFound         = errors.New("dorm3d ship not found")
	ErrDorm3dRoomNotFound         = errors.New("dorm3d room not found")
	ErrDorm3dCollectionInvalid    = errors.New("dorm3d collection invalid")
	ErrDorm3dCollectionShipGroup  = errors.New("dorm3d collection ship group mismatch")
	ErrDorm3dDialogueInvalid      = errors.New("dorm3d dialogue invalid")
	ErrDorm3dCommNotFound         = errors.New("dorm3d comm topic not found")
	ErrDorm3dInvalidCommShip      = errors.New("dorm3d comm topic does not belong to ship")
	ErrDorm3dInvalidCallName      = errors.New("dorm3d call name invalid")
	ErrDorm3dInvalidBackground    = errors.New("dorm3d background not available")
	ErrDorm3dSkinNotAvailable     = errors.New("dorm3d skin not available")
	ErrDorm3dHiddenSkinInvalid    = errors.New("dorm3d hidden skin invalid")
	ErrDorm3dCollectionRoomConfig = errors.New("dorm3d room config invalid")
	ErrDorm3dUnsupportedActType   = errors.New("dorm3d unsupported unlock act type")
	ErrDorm3dUnlockTargetMissing  = errors.New("dorm3d unlock target missing")
)

type Dorm3dApartment struct {
	CommanderID        uint32             `gorm:"primary_key" json:"commander_id"`
	DailyVigorMax      uint32             `gorm:"not_null;default:0" json:"daily_vigor_max"`
	Gifts              Dorm3dGiftList     `gorm:"type:text;not_null;default:'[]'" json:"gifts"`
	Ships              Dorm3dShipList     `gorm:"type:text;not_null;default:'[]'" json:"ships"`
	GiftDaily          Dorm3dGiftShopList `gorm:"type:text;not_null;default:'[]'" json:"gift_daily"`
	GiftPermanent      Dorm3dGiftShopList `gorm:"type:text;not_null;default:'[]'" json:"gift_permanent"`
	FurnitureDaily     Dorm3dGiftShopList `gorm:"type:text;not_null;default:'[]'" json:"furniture_daily"`
	FurniturePermanent Dorm3dGiftShopList `gorm:"type:text;not_null;default:'[]'" json:"furniture_permanent"`
	Rooms              Dorm3dRoomList     `gorm:"type:text;not_null;default:'[]'" json:"rooms"`
	Ins                Dorm3dInsList      `gorm:"type:text;not_null;default:'[]'" json:"ins"`
}

type Dorm3dGift struct {
	GiftID     uint32 `json:"gift_id"`
	Number     uint32 `json:"number"`
	UsedNumber uint32 `json:"used_number"`
}

type Dorm3dGiftList []Dorm3dGift

type Dorm3dGiftShop struct {
	GiftID uint32 `json:"gift_id"`
	Count  uint32 `json:"count"`
}

type Dorm3dGiftShopList []Dorm3dGiftShop

type Dorm3dFurniture struct {
	FurnitureID uint32 `json:"furniture_id"`
	SlotID      uint32 `json:"slot_id"`
}

type Dorm3dRoom struct {
	ID          uint32            `json:"id"`
	Furnitures  []Dorm3dFurniture `json:"furnitures"`
	Collections []uint32          `json:"collections"`
	Ships       []uint32          `json:"ships"`
}

type Dorm3dRoomList []Dorm3dRoom

type Dorm3dSkinHiddenInfo struct {
	SkinID      uint32   `json:"skin_id"`
	HiddenParts []uint32 `json:"hidden_parts"`
}

type Dorm3dShip struct {
	ShipGroup      uint32                 `json:"ship_group"`
	FavorLv        uint32                 `json:"favor_lv"`
	FavorExp       uint32                 `json:"favor_exp"`
	RegularTrigger []uint32               `json:"regular_trigger"`
	DailyFavor     uint32                 `json:"daily_favor"`
	Dialogues      []uint32               `json:"dialogues"`
	Skins          []uint32               `json:"skins"`
	CurSkin        uint32                 `json:"cur_skin"`
	Name           string                 `json:"name"`
	NameCd         uint32                 `json:"name_cd"`
	VisitTime      uint32                 `json:"visit_time"`
	HiddenInfo     []Dorm3dSkinHiddenInfo `json:"hidden_info"`
}

type Dorm3dShipList []Dorm3dShip

type Dorm3dKeyValue struct {
	Key   uint32 `json:"key"`
	Value uint32 `json:"value"`
}

type Dorm3dCommInfo struct {
	ID        uint32           `json:"id"`
	Time      uint32           `json:"time"`
	ReadFlag  uint32           `json:"read_flag"`
	ReplyList []Dorm3dKeyValue `json:"reply_list"`
}

type Dorm3dPhoneInfo struct {
	ID       uint32 `json:"id"`
	Time     uint32 `json:"time"`
	ReadFlag uint32 `json:"read_flag"`
}

type Dorm3dReplyFriend struct {
	Key   uint32 `json:"key"`
	Value uint32 `json:"value"`
	Time  uint32 `json:"time"`
}

type Dorm3dFriendCircleInfo struct {
	ID        uint32              `json:"id"`
	Time      uint32              `json:"time"`
	ReadFlag  uint32              `json:"read_flag"`
	GoodFlag  uint32              `json:"good_flag"`
	ReplyList []Dorm3dReplyFriend `json:"reply_list"`
	ExitTime  uint32              `json:"exit_time"`
}

type Dorm3dIns struct {
	ShipGroup  uint32                   `json:"ship_group"`
	CareFlag   uint32                   `json:"care_flag"`
	CurBack    uint32                   `json:"cur_back"`
	CurCommId  uint32                   `json:"cur_comm_id"`
	CommList   []Dorm3dCommInfo         `json:"comm_list"`
	PhoneList  []Dorm3dPhoneInfo        `json:"phone_list"`
	FriendList []Dorm3dFriendCircleInfo `json:"friend_list"`
}

type Dorm3dInsList []Dorm3dIns

type Dorm3dEventInfo struct {
	EventType uint32
	Value     uint32
	ShipGroup uint32
}

type Dorm3dActInfo struct {
	ShipGroup uint32
	Type      uint32
	ActID     uint32
	Time      uint32
}

type dorm3dInsUnlock struct {
	Content    uint32 `json:"content"`
	TriggerNum uint32 `json:"trigger_num"`
	TriggerTyp uint32 `json:"trigger_type"`
	Type       uint32 `json:"type"`
}

type dorm3dInsShipRef struct {
	ShipGroup uint32 `json:"ship_group"`
}

type dorm3dCollectionConfig struct {
	RoomID uint32 `json:"room_id"`
}

type dorm3dRoomConfig struct {
	Type      uint32   `json:"type"`
	Character []uint32 `json:"character"`
}

type dorm3dDialogueConfig struct {
	CharID uint32 `json:"char_id"`
}

type dorm3dTriggerState struct {
	Counters map[string]uint32 `json:"counters"`
}

func NewDorm3dApartment(commanderID uint32) Dorm3dApartment {
	return Dorm3dApartment{
		CommanderID:        commanderID,
		DailyVigorMax:      0,
		Gifts:              Dorm3dGiftList{},
		Ships:              Dorm3dShipList{},
		GiftDaily:          Dorm3dGiftShopList{},
		GiftPermanent:      Dorm3dGiftShopList{},
		FurnitureDaily:     Dorm3dGiftShopList{},
		FurniturePermanent: Dorm3dGiftShopList{},
		Rooms:              Dorm3dRoomList{},
		Ins:                Dorm3dInsList{},
	}
}

func GetDorm3dApartment(commanderID uint32) (*Dorm3dApartment, error) {
	ctx := context.Background()
	row := db.DefaultStore.Pool.QueryRow(ctx, `
SELECT commander_id,
       daily_vigor_max,
       gifts,
       ships,
       gift_daily,
       gift_permanent,
       furniture_daily,
       furniture_permanent,
       rooms,
       ins
FROM dorm3d_apartments
WHERE commander_id = $1
`, int64(commanderID))
	apartment, err := scanDorm3dApartment(row)
	err = db.MapNotFound(err)
	if err != nil {
		return nil, err
	}
	apartment.EnsureDefaults()
	return &apartment, nil
}

func ListDorm3dApartments(offset int, limit int) ([]Dorm3dApartment, int64, error) {
	ctx := context.Background()

	var total int64
	if err := db.DefaultStore.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM dorm3d_apartments`).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := db.DefaultStore.Pool.Query(ctx, `
SELECT commander_id,
       daily_vigor_max,
       gifts,
       ships,
       gift_daily,
       gift_permanent,
       furniture_daily,
       furniture_permanent,
       rooms,
       ins
FROM dorm3d_apartments
ORDER BY commander_id ASC
OFFSET $1
LIMIT $2
`, int64(offset), int64(limit))
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	apartments := make([]Dorm3dApartment, 0)
	for rows.Next() {
		apartment, err := scanDorm3dApartment(rows)
		if err != nil {
			return nil, 0, err
		}
		apartment.EnsureDefaults()
		apartments = append(apartments, apartment)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return apartments, total, nil
}

func GetOrCreateDorm3dApartment(commanderID uint32) (*Dorm3dApartment, error) {
	apartment, err := GetDorm3dApartment(commanderID)
	if err == nil {
		return apartment, nil
	}
	if !errors.Is(err, db.ErrNotFound) {
		return nil, err
	}
	ctx := context.Background()
	if err := db.DefaultStore.Queries.CreateDorm3dApartment(ctx, int64(commanderID)); err != nil {
		return nil, err
	}
	return GetDorm3dApartment(commanderID)
}

func GetDorm3dApartmentTx(ctx context.Context, tx pgx.Tx, commanderID uint32) (*Dorm3dApartment, error) {
	row := tx.QueryRow(ctx, `
SELECT commander_id,
       daily_vigor_max,
       gifts,
       ships,
       gift_daily,
       gift_permanent,
       furniture_daily,
       furniture_permanent,
       rooms,
       ins
FROM dorm3d_apartments
WHERE commander_id = $1
FOR UPDATE
`, int64(commanderID))
	apartment, err := scanDorm3dApartment(row)
	err = db.MapNotFound(err)
	if err != nil {
		return nil, err
	}
	apartment.EnsureDefaults()
	return &apartment, nil
}

func GetOrCreateDorm3dApartmentTx(ctx context.Context, tx pgx.Tx, commanderID uint32) (*Dorm3dApartment, error) {
	apartment, err := GetDorm3dApartmentTx(ctx, tx, commanderID)
	if err == nil {
		return apartment, nil
	}
	if !errors.Is(err, db.ErrNotFound) {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO dorm3d_apartments (commander_id)
VALUES ($1)
ON CONFLICT (commander_id) DO NOTHING
`, int64(commanderID)); err != nil {
		return nil, err
	}
	return GetDorm3dApartmentTx(ctx, tx, commanderID)
}

func SaveDorm3dApartment(apartment *Dorm3dApartment) error {
	apartment.EnsureDefaults()
	ctx := context.Background()
	return saveDorm3dApartmentWithQueries(ctx, db.DefaultStore.Queries, apartment)
}

func SaveDorm3dApartmentTx(ctx context.Context, tx pgx.Tx, apartment *Dorm3dApartment) error {
	apartment.EnsureDefaults()
	queries := db.DefaultStore.Queries.WithTx(tx)
	return saveDorm3dApartmentWithQueries(ctx, queries, apartment)
}

type dorm3dApartmentUpserter interface {
	UpsertDorm3dApartment(context.Context, gen.UpsertDorm3dApartmentParams) error
}

func saveDorm3dApartmentWithQueries(ctx context.Context, queries dorm3dApartmentUpserter, apartment *Dorm3dApartment) error {
	gifts, err := marshalDorm3dJSONB(apartment.Gifts)
	if err != nil {
		return err
	}
	ships, err := marshalDorm3dJSONB(apartment.Ships)
	if err != nil {
		return err
	}
	giftDaily, err := marshalDorm3dJSONB(apartment.GiftDaily)
	if err != nil {
		return err
	}
	giftPermanent, err := marshalDorm3dJSONB(apartment.GiftPermanent)
	if err != nil {
		return err
	}
	furnitureDaily, err := marshalDorm3dJSONB(apartment.FurnitureDaily)
	if err != nil {
		return err
	}
	furniturePermanent, err := marshalDorm3dJSONB(apartment.FurniturePermanent)
	if err != nil {
		return err
	}
	rooms, err := marshalDorm3dJSONB(apartment.Rooms)
	if err != nil {
		return err
	}
	ins, err := marshalDorm3dJSONB(apartment.Ins)
	if err != nil {
		return err
	}
	return queries.UpsertDorm3dApartment(ctx, gen.UpsertDorm3dApartmentParams{
		CommanderID:        int64(apartment.CommanderID),
		DailyVigorMax:      int64(apartment.DailyVigorMax),
		Gifts:              gifts,
		Ships:              ships,
		GiftDaily:          giftDaily,
		GiftPermanent:      giftPermanent,
		FurnitureDaily:     furnitureDaily,
		FurniturePermanent: furniturePermanent,
		Rooms:              rooms,
		Ins:                ins,
	})
}

func (apartment *Dorm3dApartment) RoomByID(roomID uint32) *Dorm3dRoom {
	for i := range apartment.Rooms {
		if apartment.Rooms[i].ID == roomID {
			return &apartment.Rooms[i]
		}
	}
	return nil
}

func (apartment *Dorm3dApartment) AddRoom(room Dorm3dRoom) bool {
	if apartment.RoomByID(room.ID) != nil {
		return false
	}
	apartment.Rooms = append(apartment.Rooms, room)
	return true
}

func DeleteDorm3dApartment(commanderID uint32) error {
	ctx := context.Background()
	tag, err := db.DefaultStore.Pool.Exec(ctx, `DELETE FROM dorm3d_apartments WHERE commander_id = $1`, int64(commanderID))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return db.ErrNotFound
	}
	return nil
}

func CreateDorm3dApartment(apartment *Dorm3dApartment) error {
	apartment.EnsureDefaults()
	ctx := context.Background()
	gifts, err := marshalDorm3dJSONB(apartment.Gifts)
	if err != nil {
		return err
	}
	ships, err := marshalDorm3dJSONB(apartment.Ships)
	if err != nil {
		return err
	}
	giftDaily, err := marshalDorm3dJSONB(apartment.GiftDaily)
	if err != nil {
		return err
	}
	giftPermanent, err := marshalDorm3dJSONB(apartment.GiftPermanent)
	if err != nil {
		return err
	}
	furnitureDaily, err := marshalDorm3dJSONB(apartment.FurnitureDaily)
	if err != nil {
		return err
	}
	furniturePermanent, err := marshalDorm3dJSONB(apartment.FurniturePermanent)
	if err != nil {
		return err
	}
	rooms, err := marshalDorm3dJSONB(apartment.Rooms)
	if err != nil {
		return err
	}
	ins, err := marshalDorm3dJSONB(apartment.Ins)
	if err != nil {
		return err
	}
	_, err = db.DefaultStore.Pool.Exec(ctx, `
INSERT INTO dorm3d_apartments (
	commander_id,
	daily_vigor_max,
	gifts,
	ships,
	gift_daily,
	gift_permanent,
	furniture_daily,
	furniture_permanent,
	rooms,
	ins
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
`,
		int64(apartment.CommanderID),
		int64(apartment.DailyVigorMax),
		gifts,
		ships,
		giftDaily,
		giftPermanent,
		furnitureDaily,
		furniturePermanent,
		rooms,
		ins,
	)
	return err
}

func scanDorm3dApartment(scanner rowScanner) (Dorm3dApartment, error) {
	var (
		apartment = Dorm3dApartment{
			Gifts:              Dorm3dGiftList{},
			Ships:              Dorm3dShipList{},
			GiftDaily:          Dorm3dGiftShopList{},
			GiftPermanent:      Dorm3dGiftShopList{},
			FurnitureDaily:     Dorm3dGiftShopList{},
			FurniturePermanent: Dorm3dGiftShopList{},
			Rooms:              Dorm3dRoomList{},
			Ins:                Dorm3dInsList{},
		}
		commanderID       int64
		dailyVigorMax     int64
		giftsPayload      []byte
		shipsPayload      []byte
		giftDailyPayload  []byte
		giftPermPayload   []byte
		furnitureDPayload []byte
		furniturePPayload []byte
		roomsPayload      []byte
		insPayload        []byte
	)
	if err := scanner.Scan(
		&commanderID,
		&dailyVigorMax,
		&giftsPayload,
		&shipsPayload,
		&giftDailyPayload,
		&giftPermPayload,
		&furnitureDPayload,
		&furniturePPayload,
		&roomsPayload,
		&insPayload,
	); err != nil {
		return Dorm3dApartment{}, err
	}
	apartment.CommanderID = uint32(commanderID)
	apartment.DailyVigorMax = uint32(dailyVigorMax)
	if err := unmarshalDorm3dJSONB(giftsPayload, &apartment.Gifts); err != nil {
		return Dorm3dApartment{}, err
	}
	if err := unmarshalDorm3dJSONB(shipsPayload, &apartment.Ships); err != nil {
		return Dorm3dApartment{}, err
	}
	if err := unmarshalDorm3dJSONB(giftDailyPayload, &apartment.GiftDaily); err != nil {
		return Dorm3dApartment{}, err
	}
	if err := unmarshalDorm3dJSONB(giftPermPayload, &apartment.GiftPermanent); err != nil {
		return Dorm3dApartment{}, err
	}
	if err := unmarshalDorm3dJSONB(furnitureDPayload, &apartment.FurnitureDaily); err != nil {
		return Dorm3dApartment{}, err
	}
	if err := unmarshalDorm3dJSONB(furniturePPayload, &apartment.FurniturePermanent); err != nil {
		return Dorm3dApartment{}, err
	}
	if err := unmarshalDorm3dJSONB(roomsPayload, &apartment.Rooms); err != nil {
		return Dorm3dApartment{}, err
	}
	if err := unmarshalDorm3dJSONB(insPayload, &apartment.Ins); err != nil {
		return Dorm3dApartment{}, err
	}
	return apartment, nil
}

func unmarshalDorm3dJSONB(value []byte, target any) error {
	if len(value) == 0 {
		return nil
	}
	return json.Unmarshal(value, target)
}

func marshalDorm3dJSONB(value any) ([]byte, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if len(payload) == 0 {
		return []byte("[]"), nil
	}
	return payload, nil
}

func UpdateDorm3dInstagramFlags(commanderID uint32, shipGroup uint32, postIDs []uint32, op uint32, now uint32) error {
	if len(postIDs) == 0 {
		return nil
	}
	apartment, err := GetOrCreateDorm3dApartment(commanderID)
	if err != nil {
		return err
	}
	ins := apartment.ensureInsEntry(shipGroup)
	for _, postID := range postIDs {
		entry := ins.ensureFriendEntry(postID, now)
		switch op {
		case Dorm3dInstagramOpRead:
			entry.ReadFlag = 1
		case Dorm3dInstagramOpLike:
			entry.GoodFlag = 1
		case Dorm3dInstagramOpExit:
			entry.ExitTime = now
		case Dorm3dInstagramOpShare:
			// No state change
		}
	}
	return SaveDorm3dApartment(apartment)
}

func AddDorm3dInstagramReply(commanderID uint32, shipGroup uint32, postID uint32, chatID uint32, value uint32, now uint32) error {
	apartment, err := GetOrCreateDorm3dApartment(commanderID)
	if err != nil {
		return err
	}
	ins := apartment.ensureInsEntry(shipGroup)
	entry := ins.ensureFriendEntry(postID, now)
	entry.ReplyList = append(entry.ReplyList, Dorm3dReplyFriend{
		Key:   chatID,
		Value: value,
		Time:  now,
	})
	return SaveDorm3dApartment(apartment)
}

func SetDorm3dCallName(commanderID uint32, shipGroup uint32, name string, now uint32, cooldown uint32) error {
	if shipGroup == 0 || name == "" {
		return ErrDorm3dInvalidCallName
	}
	apartment, err := GetOrCreateDorm3dApartment(commanderID)
	if err != nil {
		return err
	}
	ship, ok := apartment.findShip(shipGroup)
	if !ok {
		return ErrDorm3dShipNotFound
	}
	if ship.NameCd > now {
		return ErrDorm3dInvalidCallName
	}
	if ship.Name == name {
		return ErrDorm3dInvalidCallName
	}
	ship.Name = name
	ship.NameCd = cooldown
	return SaveDorm3dApartment(apartment)
}

func ChangeDorm3dShipSkin(commanderID uint32, shipGroup uint32, skinID uint32) error {
	apartment, err := GetOrCreateDorm3dApartment(commanderID)
	if err != nil {
		return err
	}
	ship, ok := apartment.findShip(shipGroup)
	if !ok {
		return ErrDorm3dShipNotFound
	}
	if skinID != 0 && !containsUint32(ship.Skins, skinID) && ship.CurSkin != skinID {
		return ErrDorm3dSkinNotAvailable
	}
	ship.CurSkin = skinID
	return SaveDorm3dApartment(apartment)
}

func UpdateDorm3dSkinHiddenParts(commanderID uint32, shipGroup uint32, skinID uint32, hiddenParts []uint32) error {
	if skinID == 0 {
		return ErrDorm3dHiddenSkinInvalid
	}
	apartment, err := GetOrCreateDorm3dApartment(commanderID)
	if err != nil {
		return err
	}
	ship, ok := apartment.findShip(shipGroup)
	if !ok {
		return ErrDorm3dShipNotFound
	}
	for i := range ship.HiddenInfo {
		if ship.HiddenInfo[i].SkinID == skinID {
			ship.HiddenInfo[i].HiddenParts = append([]uint32{}, hiddenParts...)
			return SaveDorm3dApartment(apartment)
		}
	}
	ship.HiddenInfo = append(ship.HiddenInfo, Dorm3dSkinHiddenInfo{SkinID: skinID, HiddenParts: append([]uint32{}, hiddenParts...)})
	return SaveDorm3dApartment(apartment)
}

func MarkDorm3dDialogueSeen(commanderID uint32, dialogID uint32) error {
	shipGroup, err := dorm3dDialogueShipGroup(dialogID)
	if err != nil {
		return err
	}
	apartment, err := GetOrCreateDorm3dApartment(commanderID)
	if err != nil {
		return err
	}
	ship := apartment.ensureShipEntry(shipGroup)
	if containsUint32(ship.Dialogues, dialogID) {
		return nil
	}
	ship.Dialogues = append(ship.Dialogues, dialogID)
	return SaveDorm3dApartment(apartment)
}

func MarkDorm3dCollection(commanderID uint32, roomID uint32, collectionID uint32, shipGroup uint32) error {
	apartment, err := GetOrCreateDorm3dApartment(commanderID)
	if err != nil {
		return err
	}
	room := apartment.RoomByID(roomID)
	if room == nil {
		return ErrDorm3dRoomNotFound
	}
	if err := dorm3dValidateCollectionRoom(collectionID, roomID); err != nil {
		return err
	}
	if err := dorm3dValidateCollectionShipGroup(roomID, shipGroup); err != nil {
		return err
	}
	if containsUint32(room.Collections, collectionID) {
		return nil
	}
	room.Collections = append(room.Collections, collectionID)
	return SaveDorm3dApartment(apartment)
}

func UpdateDorm3dInsCareFlag(commanderID uint32, shipGroup uint32, careFlag uint32) error {
	apartment, err := GetOrCreateDorm3dApartment(commanderID)
	if err != nil {
		return err
	}
	if _, ok := apartment.findShip(shipGroup); !ok {
		return ErrDorm3dShipNotFound
	}
	ins := apartment.ensureInsEntry(shipGroup)
	ins.CareFlag = careFlag
	return SaveDorm3dApartment(apartment)
}

func UpdateDorm3dInsBackground(commanderID uint32, shipGroup uint32, backID uint32) error {
	apartment, err := GetOrCreateDorm3dApartment(commanderID)
	if err != nil {
		return err
	}
	ship, ok := apartment.findShip(shipGroup)
	if !ok {
		return ErrDorm3dShipNotFound
	}
	if backID != 0 && !containsUint32(ship.Skins, backID) && ship.CurSkin != backID {
		return ErrDorm3dInvalidBackground
	}
	ins := apartment.ensureInsEntry(shipGroup)
	ins.CurBack = backID
	return SaveDorm3dApartment(apartment)
}

func SetDorm3dCurrentCommID(commanderID uint32, shipGroup uint32, commID uint32) error {
	apartment, err := GetOrCreateDorm3dApartment(commanderID)
	if err != nil {
		return err
	}
	if _, ok := apartment.findShip(shipGroup); !ok {
		return ErrDorm3dShipNotFound
	}
	ins := apartment.ensureInsEntry(shipGroup)
	if !ins.hasComm(commID) {
		return ErrDorm3dCommNotFound
	}
	configShipGroup, err := loadDorm3dInsShipGroup(dorm3dInsChatGroupCategory, commID)
	if err != nil {
		return err
	}
	if configShipGroup != shipGroup {
		return ErrDorm3dInvalidCommShip
	}
	ins.CurCommId = commID
	return SaveDorm3dApartment(apartment)
}

func UpdateDorm3dVisitTime(commanderID uint32, shipGroup uint32, visitTime uint32) error {
	apartment, err := GetOrCreateDorm3dApartment(commanderID)
	if err != nil {
		return err
	}
	ship, ok := apartment.findShip(shipGroup)
	if !ok {
		return ErrDorm3dShipNotFound
	}
	ship.VisitTime = visitTime
	return SaveDorm3dApartment(apartment)
}

func ApplyDorm3dTriggerEvents(commanderID uint32, events []Dorm3dEventInfo, now uint32) ([]Dorm3dActInfo, error) {
	apartment, err := GetOrCreateDorm3dApartment(commanderID)
	if err != nil {
		return nil, err
	}
	ownedShips := make(map[uint32]struct{}, len(apartment.Ships))
	for _, ship := range apartment.Ships {
		if ship.ShipGroup == 0 {
			continue
		}
		ownedShips[ship.ShipGroup] = struct{}{}
	}
	state, err := loadDorm3dTriggerState(commanderID)
	if err != nil {
		return nil, err
	}
	for _, event := range events {
		if event.ShipGroup == 0 || event.EventType == 0 {
			continue
		}
		if _, ok := ownedShips[event.ShipGroup]; !ok {
			continue
		}
		state.Counters[dorm3dTriggerCounterKey(event.ShipGroup, event.EventType)] = maxDorm3dCounter(
			state.Counters[dorm3dTriggerCounterKey(event.ShipGroup, event.EventType)],
			event.Value,
		)
	}
	unlocks, err := ListConfigEntries(dorm3dInsUnlockCategory)
	if err != nil {
		return nil, err
	}
	result := make([]Dorm3dActInfo, 0)
	for _, unlockEntry := range unlocks {
		var unlock dorm3dInsUnlock
		if err := json.Unmarshal(unlockEntry.Data, &unlock); err != nil {
			continue
		}
		shipGroup, err := dorm3dResolveUnlockShipGroup(unlock)
		if err != nil {
			continue
		}
		counter := state.Counters[dorm3dTriggerCounterKey(shipGroup, unlock.TriggerTyp)]
		if counter < unlock.TriggerNum {
			continue
		}
		ins := apartment.ensureInsEntry(shipGroup)
		if ins.hasAct(unlock.Type, unlock.Content) {
			continue
		}
		if err := ins.unlockAct(unlock.Type, unlock.Content, now); err != nil {
			continue
		}
		result = append(result, Dorm3dActInfo{ShipGroup: shipGroup, Type: unlock.Type, ActID: unlock.Content, Time: now})
	}
	if err := SaveDorm3dApartment(apartment); err != nil {
		return nil, err
	}
	if err := saveDorm3dTriggerState(commanderID, state); err != nil {
		return nil, err
	}
	return result, nil
}

func (apartment *Dorm3dApartment) EnsureDefaults() {
	if apartment.Gifts == nil {
		apartment.Gifts = Dorm3dGiftList{}
	}
	if apartment.Ships == nil {
		apartment.Ships = Dorm3dShipList{}
	}
	if apartment.GiftDaily == nil {
		apartment.GiftDaily = Dorm3dGiftShopList{}
	}
	if apartment.GiftPermanent == nil {
		apartment.GiftPermanent = Dorm3dGiftShopList{}
	}
	if apartment.FurnitureDaily == nil {
		apartment.FurnitureDaily = Dorm3dGiftShopList{}
	}
	if apartment.FurniturePermanent == nil {
		apartment.FurniturePermanent = Dorm3dGiftShopList{}
	}
	if apartment.Rooms == nil {
		apartment.Rooms = Dorm3dRoomList{}
	}
	if apartment.Ins == nil {
		apartment.Ins = Dorm3dInsList{}
	}
	for i := range apartment.Rooms {
		if apartment.Rooms[i].Furnitures == nil {
			apartment.Rooms[i].Furnitures = []Dorm3dFurniture{}
		}
		if apartment.Rooms[i].Collections == nil {
			apartment.Rooms[i].Collections = []uint32{}
		}
		if apartment.Rooms[i].Ships == nil {
			apartment.Rooms[i].Ships = []uint32{}
		}
	}
	for i := range apartment.Ships {
		if apartment.Ships[i].RegularTrigger == nil {
			apartment.Ships[i].RegularTrigger = []uint32{}
		}
		if apartment.Ships[i].Dialogues == nil {
			apartment.Ships[i].Dialogues = []uint32{}
		}
		if apartment.Ships[i].Skins == nil {
			apartment.Ships[i].Skins = []uint32{}
		}
		if apartment.Ships[i].HiddenInfo == nil {
			apartment.Ships[i].HiddenInfo = []Dorm3dSkinHiddenInfo{}
		}
	}
	for i := range apartment.Ins {
		if apartment.Ins[i].CommList == nil {
			apartment.Ins[i].CommList = []Dorm3dCommInfo{}
		}
		if apartment.Ins[i].PhoneList == nil {
			apartment.Ins[i].PhoneList = []Dorm3dPhoneInfo{}
		}
		if apartment.Ins[i].FriendList == nil {
			apartment.Ins[i].FriendList = []Dorm3dFriendCircleInfo{}
		}
		for j := range apartment.Ins[i].CommList {
			if apartment.Ins[i].CommList[j].ReplyList == nil {
				apartment.Ins[i].CommList[j].ReplyList = []Dorm3dKeyValue{}
			}
		}
		for j := range apartment.Ins[i].FriendList {
			if apartment.Ins[i].FriendList[j].ReplyList == nil {
				apartment.Ins[i].FriendList[j].ReplyList = []Dorm3dReplyFriend{}
			}
		}
	}
}

func (apartment *Dorm3dApartment) ensureInsEntry(shipGroup uint32) *Dorm3dIns {
	for i := range apartment.Ins {
		if apartment.Ins[i].ShipGroup == shipGroup {
			return &apartment.Ins[i]
		}
	}
	newEntry := Dorm3dIns{
		ShipGroup:  shipGroup,
		CommList:   []Dorm3dCommInfo{},
		PhoneList:  []Dorm3dPhoneInfo{},
		FriendList: []Dorm3dFriendCircleInfo{},
	}
	apartment.Ins = append(apartment.Ins, newEntry)
	return &apartment.Ins[len(apartment.Ins)-1]
}

func (apartment *Dorm3dApartment) FindShip(shipGroup uint32) *Dorm3dShip {
	for i := range apartment.Ships {
		if apartment.Ships[i].ShipGroup == shipGroup {
			return &apartment.Ships[i]
		}
	}
	return nil
}

func (apartment *Dorm3dApartment) FindGift(giftID uint32) *Dorm3dGift {
	for i := range apartment.Gifts {
		if apartment.Gifts[i].GiftID == giftID {
			return &apartment.Gifts[i]
		}
	}
	return nil
}

// EnsureGiftEntry returns the inventory slot for giftID, creating it when the
// commander does not own that gift yet.
func (apartment *Dorm3dApartment) EnsureGiftEntry(giftID uint32) *Dorm3dGift {
	if gift := apartment.FindGift(giftID); gift != nil {
		return gift
	}
	apartment.Gifts = append(apartment.Gifts, Dorm3dGift{GiftID: giftID})
	return &apartment.Gifts[len(apartment.Gifts)-1]
}

func (apartment *Dorm3dApartment) findShip(shipGroup uint32) (*Dorm3dShip, bool) {
	for i := range apartment.Ships {
		if apartment.Ships[i].ShipGroup == shipGroup {
			return &apartment.Ships[i], true
		}
	}
	return nil, false
}

func (apartment *Dorm3dApartment) ensureShipEntry(shipGroup uint32) *Dorm3dShip {
	if ship, ok := apartment.findShip(shipGroup); ok {
		return ship
	}
	entry := Dorm3dShip{
		ShipGroup:      shipGroup,
		RegularTrigger: []uint32{},
		Dialogues:      []uint32{},
		Skins:          []uint32{},
		HiddenInfo:     []Dorm3dSkinHiddenInfo{},
	}
	apartment.Ships = append(apartment.Ships, entry)
	return &apartment.Ships[len(apartment.Ships)-1]
}

func (ins *Dorm3dIns) hasComm(commID uint32) bool {
	for i := range ins.CommList {
		if ins.CommList[i].ID == commID {
			return true
		}
	}
	return false
}

func (ins *Dorm3dIns) hasAct(actType uint32, actID uint32) bool {
	switch actType {
	case 1:
		return ins.hasComm(actID)
	case 2:
		for i := range ins.PhoneList {
			if ins.PhoneList[i].ID == actID {
				return true
			}
		}
		return false
	case 3:
		for i := range ins.FriendList {
			if ins.FriendList[i].ID == actID {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func (ins *Dorm3dIns) unlockAct(actType uint32, actID uint32, now uint32) error {
	switch actType {
	case 1:
		ins.CommList = append(ins.CommList, Dorm3dCommInfo{ID: actID, Time: now, ReadFlag: 0, ReplyList: []Dorm3dKeyValue{}})
		if ins.CurCommId == 0 {
			ins.CurCommId = actID
		}
		return nil
	case 2:
		ins.PhoneList = append(ins.PhoneList, Dorm3dPhoneInfo{ID: actID, Time: now, ReadFlag: 0})
		return nil
	case 3:
		ins.FriendList = append(ins.FriendList, Dorm3dFriendCircleInfo{
			ID:        actID,
			Time:      now,
			ReadFlag:  0,
			GoodFlag:  0,
			ReplyList: []Dorm3dReplyFriend{},
			ExitTime:  0,
		})
		return nil
	default:
		return ErrDorm3dUnsupportedActType
	}
}

func (ins *Dorm3dIns) ensureFriendEntry(postID uint32, now uint32) *Dorm3dFriendCircleInfo {
	for i := range ins.FriendList {
		if ins.FriendList[i].ID == postID {
			return &ins.FriendList[i]
		}
	}
	entry := Dorm3dFriendCircleInfo{
		ID:        postID,
		Time:      now,
		ReadFlag:  0,
		GoodFlag:  0,
		ReplyList: []Dorm3dReplyFriend{},
		ExitTime:  0,
	}
	ins.FriendList = append(ins.FriendList, entry)
	return &ins.FriendList[len(ins.FriendList)-1]
}

func dorm3dDialogueShipGroup(dialogID uint32) (uint32, error) {
	entry, err := GetConfigEntry(dorm3dDialogueGroupCategory, strconv.FormatUint(uint64(dialogID), 10))
	if err != nil {
		return 0, ErrDorm3dDialogueInvalid
	}
	var data dorm3dDialogueConfig
	if err := json.Unmarshal(entry.Data, &data); err != nil {
		return 0, ErrDorm3dDialogueInvalid
	}
	if data.CharID == 0 {
		return 0, ErrDorm3dDialogueInvalid
	}
	return data.CharID, nil
}

func dorm3dValidateCollectionRoom(collectionID uint32, roomID uint32) error {
	entry, err := GetConfigEntry(dorm3dCollectionTemplateCategory, strconv.FormatUint(uint64(collectionID), 10))
	if err != nil {
		return ErrDorm3dCollectionInvalid
	}
	var data dorm3dCollectionConfig
	if err := json.Unmarshal(entry.Data, &data); err != nil {
		return ErrDorm3dCollectionInvalid
	}
	if data.RoomID != roomID {
		return ErrDorm3dCollectionInvalid
	}
	return nil
}

func dorm3dValidateCollectionShipGroup(roomID uint32, shipGroup uint32) error {
	entry, err := GetConfigEntry(dorm3dRoomsCategory, strconv.FormatUint(uint64(roomID), 10))
	if err != nil {
		return ErrDorm3dCollectionRoomConfig
	}
	var room dorm3dRoomConfig
	if err := json.Unmarshal(entry.Data, &room); err != nil {
		return ErrDorm3dCollectionRoomConfig
	}
	if room.Type != 2 {
		return nil
	}
	if len(room.Character) == 0 || room.Character[0] == 0 {
		return ErrDorm3dCollectionRoomConfig
	}
	if shipGroup == 0 || shipGroup != room.Character[0] {
		return ErrDorm3dCollectionShipGroup
	}
	return nil
}

func loadDorm3dInsShipGroup(category string, id uint32) (uint32, error) {
	entry, err := GetConfigEntry(category, strconv.FormatUint(uint64(id), 10))
	if err != nil {
		return 0, err
	}
	var data dorm3dInsShipRef
	if err := json.Unmarshal(entry.Data, &data); err != nil {
		return 0, err
	}
	if data.ShipGroup == 0 {
		return 0, ErrDorm3dUnlockTargetMissing
	}
	return data.ShipGroup, nil
}

func dorm3dResolveUnlockShipGroup(unlock dorm3dInsUnlock) (uint32, error) {
	if unlock.Content == 0 {
		return 0, ErrDorm3dUnlockTargetMissing
	}
	switch unlock.Type {
	case 1:
		return loadDorm3dInsShipGroup(dorm3dInsChatGroupCategory, unlock.Content)
	case 2:
		return loadDorm3dInsShipGroup(dorm3dInsTelephoneGroupCategory, unlock.Content)
	case 3:
		return loadDorm3dInsShipGroup(dorm3dInsTemplateCategory, unlock.Content)
	default:
		return 0, ErrDorm3dUnsupportedActType
	}
}

func loadDorm3dTriggerState(commanderID uint32) (*dorm3dTriggerState, error) {
	entry, err := GetConfigEntry(dorm3dTriggerStateCategory, strconv.FormatUint(uint64(commanderID), 10))
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return &dorm3dTriggerState{Counters: map[string]uint32{}}, nil
		}
		return nil, err
	}
	var state dorm3dTriggerState
	if err := json.Unmarshal(entry.Data, &state); err != nil {
		return nil, err
	}
	if state.Counters == nil {
		state.Counters = map[string]uint32{}
	}
	return &state, nil
}

func saveDorm3dTriggerState(commanderID uint32, state *dorm3dTriggerState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return UpsertConfigEntry(dorm3dTriggerStateCategory, strconv.FormatUint(uint64(commanderID), 10), data)
}

func dorm3dTriggerCounterKey(shipGroup uint32, triggerType uint32) string {
	return strconv.FormatUint(uint64(shipGroup), 10) + ":" + strconv.FormatUint(uint64(triggerType), 10)
}

func maxDorm3dCounter(current uint32, value uint32) uint32 {
	if value > current {
		return value
	}
	return current
}

func containsUint32(values []uint32, target uint32) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (list Dorm3dGiftList) Value() (driver.Value, error) {
	return marshalDorm3dJSON(list)
}

func (list *Dorm3dGiftList) Scan(value any) error {
	return scanDorm3dJSON(value, list)
}

func (list Dorm3dGiftShopList) Value() (driver.Value, error) {
	return marshalDorm3dJSON(list)
}

func (list *Dorm3dGiftShopList) Scan(value any) error {
	return scanDorm3dJSON(value, list)
}

func (list Dorm3dRoomList) Value() (driver.Value, error) {
	return marshalDorm3dJSON(list)
}

func (list *Dorm3dRoomList) Scan(value any) error {
	return scanDorm3dJSON(value, list)
}

func (list Dorm3dShipList) Value() (driver.Value, error) {
	return marshalDorm3dJSON(list)
}

func (list *Dorm3dShipList) Scan(value any) error {
	return scanDorm3dJSON(value, list)
}

func (list Dorm3dInsList) Value() (driver.Value, error) {
	return marshalDorm3dJSON(list)
}

func (list *Dorm3dInsList) Scan(value any) error {
	return scanDorm3dJSON(value, list)
}

func marshalDorm3dJSON(value any) (driver.Value, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return string(payload), nil
}

func scanDorm3dJSON(value any, target any) error {
	if value == nil {
		return nil
	}
	switch v := value.(type) {
	case string:
		return json.Unmarshal([]byte(v), target)
	case []byte:
		return json.Unmarshal(v, target)
	default:
		return fmt.Errorf("unsupported Dorm3d type: %T", value)
	}
}
