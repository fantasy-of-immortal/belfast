package educate_test

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
)

// Foreign engine snapshots are fixtures, never dependencies on its handlers.
type auditRoleSnapshot struct {
	State, Permanent []byte
	Metadata         json.RawMessage
	Revision         int64
}

func loadEducateState(client *connection.Client, id uint32) (*auditRoleSnapshot, error) {
	ctx := context.Background()
	_, err := db.DefaultStore.Pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS commander_educate_characters (
 commander_id bigint NOT NULL, character_id bigint NOT NULL, state bytea NOT NULL,
 permanent bytea NOT NULL, metadata jsonb NOT NULL, revision bigint NOT NULL,
 PRIMARY KEY(commander_id,character_id))`)
	if err != nil {
		return nil, err
	}
	_, err = db.DefaultStore.Pool.Exec(ctx, `INSERT INTO commander_educate_characters
 (commander_id,character_id,state,permanent,metadata,revision) VALUES($1,$2,$3,$4,'{}',1)
 ON CONFLICT DO NOTHING`, client.Commander.CommanderID, id, []byte(fmt.Sprintf("role-%d", id)), []byte("permanent"))
	if err != nil {
		return nil, err
	}
	return loadAuditRole(client.Commander.CommanderID, id)
}

func loadAuditRole(commanderID, id uint32) (*auditRoleSnapshot, error) {
	role := &auditRoleSnapshot{}
	err := db.DefaultStore.Pool.QueryRow(context.Background(), `SELECT state,permanent,metadata,revision FROM commander_educate_characters WHERE commander_id=$1 AND character_id=$2`, commanderID, id).Scan(&role.State, &role.Permanent, &role.Metadata, &role.Revision)
	return role, err
}
