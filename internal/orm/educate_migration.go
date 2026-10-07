package orm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
	"github.com/jackc/pgx/v5"
)

func ValidateEducateCharacter(id uint32) error {
	if id == 0 {
		return fmt.Errorf("invalid educate character 0")
	}
	entry, err := GetConfigEntry("ShareCfg/child2_data.json", strconv.FormatUint(uint64(id), 10))
	if err != nil {
		return fmt.Errorf("character %d configuration: %w", id, err)
	}
	var config struct {
		ID uint32 `json:"id"`
	}
	if err := json.Unmarshal(entry.Data, &config); err != nil {
		return fmt.Errorf("character %d configuration: %w", id, err)
	}
	if config.ID != id {
		return fmt.Errorf("character configuration key/id mismatch")
	}
	return nil
}

type EducateMigrationReport struct {
	CommanderID      uint32   `json:"commander_id"`
	CharacterID      uint32   `json:"character_id"`
	Status           string   `json:"status"`
	StateSHA256      string   `json:"state_sha256"`
	PermanentSHA256  string   `json:"permanent_sha256"`
	ForeignAttrs     []uint32 `json:"foreign_attrs"`
	ForeignResources []uint32 `json:"foreign_resources"`
	Notes            []string `json:"notes"`
}

func educateDigest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

// No protobuf byte offsets or SQL decoding. The raw source is archived even
// when invalid. Foreign values remain in the raw owner save and archive; they
// do not imply a second recoverable round or permanent history.
func inspectEducateLegacy(entry *CommanderTB) (*EducateMigrationReport, *protobuf.TBINFO, error) {
	r := &EducateMigrationReport{CommanderID: entry.CommanderID, Status: "quarantined", StateSHA256: educateDigest(entry.State), PermanentSHA256: educateDigest(entry.Permanent)}
	info, _, err := entry.Decode()
	if err != nil {
		r.Notes = append(r.Notes, err.Error())
		return r, nil, nil
	}
	r.CharacterID = info.GetId()
	if info.GetId() == 0 {
		r.Notes = append(r.Notes, "invalid character id 0")
		return r, info, nil
	}
	if _, err := GetConfigEntry("ShareCfg/child2_data.json", strconv.FormatUint(uint64(info.GetId()), 10)); err != nil {
		if !errors.Is(err, db.ErrNotFound) {
			return nil, nil, err
		}
		r.Notes = append(r.Notes, "unknown character id")
		return r, info, nil
	}
	if err := ValidateEducateCharacter(info.GetId()); err != nil {
		return nil, nil, err
	}
	for _, spec := range []struct {
		category string
		values   []*protobuf.KVDATA
		foreign  *[]uint32
	}{
		{"ShareCfg/child2_attr.json", info.GetRes().GetAttrs(), &r.ForeignAttrs},
		{"ShareCfg/child2_resource.json", info.GetRes().GetResource(), &r.ForeignResources},
	} {
		for _, value := range spec.values {
			config, err := GetConfigEntry(spec.category, strconv.FormatUint(uint64(value.GetKey()), 10))
			if err != nil {
				return nil, nil, fmt.Errorf("migration field %s/%d: %w", spec.category, value.GetKey(), err)
			}
			var owner struct {
				Character uint32 `json:"character"`
			}
			if err := json.Unmarshal(config.Data, &owner); err != nil {
				return nil, nil, err
			}
			if owner.Character != info.GetId() {
				*spec.foreign = append(*spec.foreign, value.GetKey())
			}
		}
	}
	r.Status = "migrated"
	r.Notes = append(r.Notes, "round, plans and permanent blob attributed only to Info.Id; permanent ID provenance is not fully reconstructable; raw bytes retained; no history copied to other characters")
	if len(r.ForeignAttrs) > 0 || len(r.ForeignResources) > 0 {
		r.Notes = append(r.Notes, "mixed values retained in owner blob and archive, filtered from wire; other character's prior round is unknown")
	}
	return r, info, nil
}

// Dry-run performs no writes. Apply archives and creates the owner row in one
// transaction, never overwriting a row that has continued playing.
func MigrateEducateLegacy(commanderID uint32, apply bool) (*EducateMigrationReport, error) {
	ctx := context.Background()
	tx, err := db.DefaultStore.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var entry CommanderTB
	entry.CommanderID = commanderID
	err = tx.QueryRow(ctx, `SELECT state,permanent FROM commander_tbs WHERE commander_id=$1 FOR UPDATE`, int64(commanderID)).Scan(&entry.State, &entry.Permanent)
	if err != nil {
		return nil, db.MapNotFound(err)
	}
	var archived []byte
	err = tx.QueryRow(ctx, `SELECT report FROM commander_educate_migrations WHERE commander_id=$1`, int64(commanderID)).Scan(&archived)
	if err == nil {
		var r EducateMigrationReport
		if err := json.Unmarshal(archived, &r); err != nil {
			return nil, err
		}
		return &r, nil
	}
	if err != pgx.ErrNoRows {
		return nil, err
	}
	report, _, err := inspectEducateLegacy(&entry)
	if err != nil {
		return nil, err
	}
	if !apply {
		return report, nil
	}
	data, err := json.Marshal(report)
	if err != nil {
		return nil, err
	}
	if report.Status == "migrated" {
		_, err = tx.Exec(ctx, `INSERT INTO commander_educate_characters(commander_id,character_id,state,permanent,metadata)
 VALUES($1,$2,$3,$4,'{"version":1,"source":"legacy"}') ON CONFLICT(commander_id,character_id) DO NOTHING`, int64(commanderID), int64(report.CharacterID), entry.State, entry.Permanent)
		if err != nil {
			return nil, err
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO commander_educate_migrations(commander_id,original_state,original_permanent,report) VALUES($1,$2,$3,$4)`, int64(commanderID), entry.State, entry.Permanent, data)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return report, nil
}
