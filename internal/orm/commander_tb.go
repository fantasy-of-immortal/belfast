package orm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

type CommanderTB struct {
	CommanderID uint32 `gorm:"primary_key"`
	CharacterID uint32
	Metadata    json.RawMessage
	Revision    int64
	State       []byte `gorm:"not_null"`
	Permanent   []byte `gorm:"not_null"`
}

func (CommanderTB) TableName() string { return "commander_educate_characters" }

func GetCommanderTB(commanderID uint32, characterID uint32) (*CommanderTB, error) {
	ctx := context.Background()
	entry := CommanderTB{}
	err := db.DefaultStore.Pool.QueryRow(ctx, `
SELECT commander_id, character_id, state, permanent, metadata, revision
FROM commander_educate_characters
WHERE commander_id = $1 AND character_id = $2
`, int64(commanderID), int64(characterID)).Scan(&entry.CommanderID, &entry.CharacterID, &entry.State, &entry.Permanent, &entry.Metadata, &entry.Revision)
	err = db.MapNotFound(err)
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

func NewCommanderTB(commanderID uint32, info *protobuf.TBINFO, permanent *protobuf.TBPERMANENT) (*CommanderTB, error) {
	if info == nil || info.GetId() == 0 || permanent == nil {
		return nil, fmt.Errorf("character id and state/permanent required")
	}
	stateBytes, err := proto.Marshal(info)
	if err != nil {
		return nil, fmt.Errorf("failed to encode tb state: %w", err)
	}
	permanentBytes, err := proto.Marshal(permanent)
	if err != nil {
		return nil, fmt.Errorf("failed to encode tb permanent state: %w", err)
	}
	return &CommanderTB{
		CommanderID: commanderID,
		CharacterID: info.GetId(),
		Metadata:    json.RawMessage(`{}`),
		State:       stateBytes,
		Permanent:   permanentBytes,
	}, nil
}

func (entry *CommanderTB) Decode() (*protobuf.TBINFO, *protobuf.TBPERMANENT, error) {
	state := &protobuf.TBINFO{}
	if err := proto.Unmarshal(entry.State, state); err != nil {
		return nil, nil, fmt.Errorf("failed to decode tb state: %w", err)
	}
	permanent := &protobuf.TBPERMANENT{}
	if err := proto.Unmarshal(entry.Permanent, permanent); err != nil {
		return nil, nil, fmt.Errorf("failed to decode tb permanent state: %w", err)
	}
	if entry.CharacterID != 0 && state.GetId() != entry.CharacterID {
		return nil, nil, fmt.Errorf("character key %d differs from state id %d", entry.CharacterID, state.GetId())
	}
	return state, permanent, nil
}

func (entry *CommanderTB) Encode(info *protobuf.TBINFO, permanent *protobuf.TBPERMANENT) error {
	if info == nil || permanent == nil || info.GetId() == 0 || info.GetId() != entry.CharacterID {
		return fmt.Errorf("save character key does not match state")
	}
	stateBytes, err := proto.Marshal(info)
	if err != nil {
		return fmt.Errorf("failed to encode tb state: %w", err)
	}
	permanentBytes, err := proto.Marshal(permanent)
	if err != nil {
		return fmt.Errorf("failed to encode tb permanent state: %w", err)
	}
	entry.State = stateBytes
	entry.Permanent = permanentBytes
	return nil
}

func SaveCommanderTB(entry *CommanderTB, info *protobuf.TBINFO, permanent *protobuf.TBPERMANENT) error {
	if err := entry.Encode(info, permanent); err != nil {
		return err
	}
	ctx := context.Background()
	if entry.Metadata == nil {
		entry.Metadata = json.RawMessage(`{}`)
	}
	if entry.Revision == 0 {
		err := db.DefaultStore.Pool.QueryRow(ctx, `INSERT INTO commander_educate_characters
 (commander_id,character_id,state,permanent,metadata) VALUES ($1,$2,$3,$4,$5) RETURNING revision`,
			int64(entry.CommanderID), int64(entry.CharacterID), entry.State, entry.Permanent, entry.Metadata).Scan(&entry.Revision)
		return err
	}
	var revision int64
	err := db.DefaultStore.Pool.QueryRow(ctx, `UPDATE commander_educate_characters SET state=$3,permanent=$4,metadata=$5,revision=revision+1
 WHERE commander_id=$1 AND character_id=$2 AND revision=$6 RETURNING revision`,
		int64(entry.CommanderID), int64(entry.CharacterID), entry.State, entry.Permanent, entry.Metadata, entry.Revision).Scan(&revision)
	if err != nil {
		return fmt.Errorf("save character %d revision %d: %w", entry.CharacterID, entry.Revision, err)
	}
	entry.Revision = revision
	return nil
}

func DeleteCommanderTB(commanderID uint32, characterID uint32) (bool, error) {
	ctx := context.Background()
	tag, err := db.DefaultStore.Pool.Exec(ctx, `
DELETE FROM commander_educate_characters
WHERE commander_id = $1 AND character_id = $2
`, int64(commanderID), int64(characterID))
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// UpdateCommanderTB reads and commits an action under the same row lock. The
// callback must only mutate this snapshot; returning an error rolls it back.
func UpdateCommanderTB(commanderID, characterID uint32, action func(*CommanderTB, *protobuf.TBINFO, *protobuf.TBPERMANENT) error) error {
	ctx := context.Background()
	tx, err := db.DefaultStore.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	entry := &CommanderTB{}
	err = tx.QueryRow(ctx, `SELECT commander_id,character_id,state,permanent,metadata,revision FROM commander_educate_characters WHERE commander_id=$1 AND character_id=$2 FOR UPDATE`, int64(commanderID), int64(characterID)).Scan(&entry.CommanderID, &entry.CharacterID, &entry.State, &entry.Permanent, &entry.Metadata, &entry.Revision)
	if err != nil {
		return db.MapNotFound(err)
	}
	info, permanent, err := entry.Decode()
	if err != nil {
		return err
	}
	if err = action(entry, info, permanent); err != nil {
		return err
	}
	if err = entry.Encode(info, permanent); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE commander_educate_characters SET state=$3,permanent=$4,metadata=$5,revision=revision+1 WHERE commander_id=$1 AND character_id=$2`, int64(commanderID), int64(characterID), entry.State, entry.Permanent, entry.Metadata)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
