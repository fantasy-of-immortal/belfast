package neweducate

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"github.com/google/uuid"
	"os"
	"strings"
	"testing"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/packets"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

// Explicit opt-in, separate database and formal migrations; never InitDatabase's
// test compatibility schema (which drops player foreign keys).
func recoveryDB(t *testing.T) *connection.Client {
	t.Helper()
	dsn := os.Getenv("EDUCATE_RECOVERY_DSN")
	if dsn == "" {
		t.Skip("set EDUCATE_RECOVERY_DSN to a dedicated educate_recovery_test database")
	}
	if err := db.ValidateRecoveryTestDSN(dsn); err != nil {
		t.Fatal("recovery DSN must target a dedicated educate_recovery_test database")
	}
	schema := "recovery_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	store, err := db.InitDefaultStore(context.Background(), dsn, schema)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// Dispose only this test's new UUID schema in the dedicated test database.
		// Leaving every fixture behind makes cluster crash recovery traverse an
		// ever growing number of files; existing schemas and player data stay intact.
		if _, err := store.Pool.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Errorf("remove current recovery fixture: %v", err)
		}
		store.Pool.Close()
		db.DefaultStore = nil
	})
	file, err := os.Open(os.Getenv("EDUCATE_RECOVERY_CONFIG"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 2*1024*1024)
	for scanner.Scan() {
		var row struct {
			Category, Key string
			Data          json.RawMessage
		}
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			t.Fatal(err)
		}
		if err := orm.UpsertConfigEntry(row.Category, row.Key, row.Data); err != nil {
			t.Fatal(err)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if err := orm.CreateCommanderRoot(90001, 1, "Recovery", 0, 0); err != nil {
		t.Fatal(err)
	}
	return &connection.Client{Commander: &orm.Commander{CommanderID: 90001}}
}

func recoveryPacket(t *testing.T, client *connection.Client, handler func(*[]byte, *connection.Client) (int, int, error), request, response proto.Message) {
	t.Helper()
	client.Buffer.Reset()
	data, err := proto.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	_, id, err := handler(&data, client)
	if err != nil {
		t.Fatal(err)
	}
	wire := append([]byte(nil), client.Buffer.Bytes()...)
	if len(wire) < packets.HEADER_SIZE {
		t.Fatal("missing wire response")
	}
	size := packets.GetPacketSize(0, &wire) + 2
	if size > len(wire) {
		t.Fatalf("truncated primary frame %d > %d", size, len(wire))
	}
	for offset := size; offset < len(wire); {
		if (id != 27003 && id != 27020) || len(wire)-offset < packets.HEADER_SIZE {
			t.Fatal("unexpected additional frame")
		}
		frameSize := packets.GetPacketSize(offset, &wire) + 2
		if frameSize < packets.HEADER_SIZE || offset+frameSize > len(wire) {
			t.Fatal("truncated task notification")
		}
		frameID := packets.GetPacketId(offset, &wire)
		switch frameID {
		case 27022:
			var notification protobuf.SC_27022
			if id != 27020 {
				t.Fatal("unexpected removed tasks")
			}
			if err := proto.Unmarshal(wire[offset+packets.HEADER_SIZE:offset+frameSize], &notification); err != nil || len(notification.Ids) == 0 {
				t.Fatal("invalid removed tasks", err)
			}
		case 27021:
			var notification protobuf.SC_27021
			if err := proto.Unmarshal(wire[offset+packets.HEADER_SIZE:offset+frameSize], &notification); err != nil || len(notification.Tasks) == 0 {
				t.Fatal("invalid added tasks", err)
			}
		case 27025:
			var notification protobuf.SC_27025
			if err := proto.Unmarshal(wire[offset+packets.HEADER_SIZE:offset+frameSize], &notification); err != nil || len(notification.Tasks) == 0 {
				t.Fatal("invalid updated tasks", err)
			}
		default:
			t.Fatal("unexpected notification packet", frameID)
		}
		offset += frameSize
	}
	if err := proto.Unmarshal(wire[packets.HEADER_SIZE:size], response); err != nil {
		t.Fatal(err)
	}
	t.Logf("CS=%s payload=%s SC=%d wire=%s", request.ProtoReflect().Descriptor().Name(), hex.EncodeToString(data), id, hex.EncodeToString(wire))
}

func TestRecoveryS00FirstRoundReplay(t *testing.T) {
	client := recoveryDB(t)
	var initial protobuf.SC_29002
	recoveryPacket(t, client, NewEducateRequest, &protobuf.CS_29001{Id: proto.Uint32(2)}, &initial)
	if initial.GetTb().GetRound().GetRound() != 1 {
		t.Fatal("initial round")
	}
	recoveryPacket(t, client, NewEducateMainEvent, &protobuf.CS_29011{Id: proto.Uint32(2)}, &protobuf.SC_29012{})
	recoveryPacket(t, client, NewEducateGetTalents, &protobuf.CS_29019{Id: proto.Uint32(2)}, &protobuf.SC_29020{})
	recoveryPacket(t, client, NewEducateGetChoose, &protobuf.CS_29126{Id: proto.Uint32(2)}, &protobuf.SC_29127{})
	recoveryPacket(t, client, NewEducateGetMap, &protobuf.CS_29060{Id: proto.Uint32(2)}, &protobuf.SC_29061{})
	plans := []*protobuf.KVDATA{}
	for i, id := range []uint32{1201, 1203, 1202, 1204, 1206} {
		plans = append(plans, &protobuf.KVDATA{Key: proto.Uint32(uint32(i + 1)), Value: proto.Uint32(id)})
	}
	recoveryPacket(t, client, NewEducateSchedule, &protobuf.CS_29040{Id: proto.Uint32(2), Plans: plans}, &protobuf.SC_29041{})
	for range plans {
		var next protobuf.SC_29043
		recoveryPacket(t, client, NewEducateNextPlan, &protobuf.CS_29042{Id: proto.Uint32(2)}, &next)
		if next.GetFirstNode() == 0 {
			t.Fatal("missing course node")
		}
		before, err := orm.GetCommanderTB(client.Commander.CommanderID, 2)
		if err != nil {
			t.Fatal(err)
		}
		var repeated protobuf.SC_29043
		recoveryPacket(t, client, NewEducateNextPlan, &protobuf.CS_29042{Id: proto.Uint32(2)}, &repeated)
		if repeated.GetResult() != 1 {
			t.Fatal("next plan skipped pending course")
		}
		after, err := orm.GetCommanderTB(client.Commander.CommanderID, 2)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before.State, after.State) || before.Revision != after.Revision {
			t.Fatal("pending course rejection changed state")
		}
		node := next.GetFirstNode()
		for steps := 0; node != 0; steps++ {
			if steps >= 100 {
				t.Fatal("course chain exceeded bound")
			}
			var advance protobuf.SC_29031
			recoveryPacket(t, client, NewEducateTriggerNode, &protobuf.CS_29030{Id: proto.Uint32(2), Branch: proto.Uint32(0)}, &advance)
			if advance.GetResult() != 0 {
				t.Fatal("course node advance failed")
			}
			node = advance.GetNextNode()
		}
	}
	var exhausted protobuf.SC_29043
	recoveryPacket(t, client, NewEducateNextPlan, &protobuf.CS_29042{Id: proto.Uint32(2)}, &exhausted)
	if exhausted.GetResult() != 1 {
		t.Fatal("exhausted last course replayed")
	}
	var summary protobuf.SC_29049
	recoveryPacket(t, client, NewEducateGetExtraDrop, &protobuf.CS_29048{Id: proto.Uint32(2)}, &summary)
	var reloaded protobuf.SC_29002
	recoveryPacket(t, client, NewEducateRequest, &protobuf.CS_29001{Id: proto.Uint32(2)}, &reloaded)
	for _, kv := range reloaded.GetTb().GetRes().GetAttrs() {
		if kv.GetKey() >= 301 && kv.GetKey() <= 304 && kv.GetValue() != 5 {
			t.Fatalf("attribute %d=%d", kv.GetKey(), kv.GetValue())
		}
	}
	for _, kv := range reloaded.GetTb().GetRes().GetResource() {
		if (kv.GetKey() == 301 && kv.GetValue() != 34) || (kv.GetKey() == 302 && kv.GetValue() != 46) {
			t.Fatalf("resource %d=%d expected 34/46", kv.GetKey(), kv.GetValue())
		}
	}
	if !proto.Equal(summary.GetRes(), reloaded.GetTb().GetRes()) {
		t.Fatal("reentry changed results")
	}
	t.Logf("start=%s final=%s", initial.GetTb().String(), reloaded.GetTb().String())
}

func TestRecoveryS01CharactersAreIndependent(t *testing.T) {
	client := recoveryDB(t)
	for _, id := range []uint32{1, 2} {
		recoveryPacket(t, client, NewEducateRequest, &protobuf.CS_29001{Id: proto.Uint32(id)}, &protobuf.SC_29002{})
		recoveryPacket(t, client, NewEducateSetCall, &protobuf.CS_29009{Id: proto.Uint32(id), Name: proto.String(map[uint32]string{1: "Alice", 2: "Beth"}[id])}, &protobuf.SC_29010{})
		state, err := loadEducateState(client, id)
		if err != nil {
			t.Fatal(err)
		}
		state.Info.Round.Round = proto.Uint32(id + 1)
		state.Lifecycle = freshEducateLifecycle(state.Info)
		state.Info.Res.Resource[0].Value = proto.Uint32(11 * id)
		state.Permanent.Endings = []uint32{1000 + id}
		if err := saveEducateState(state); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []uint32{1, 2, 1} {
		var response protobuf.SC_29002
		recoveryPacket(t, client, NewEducateRequest, &protobuf.CS_29001{Id: proto.Uint32(id)}, &response)
		if response.GetTb().GetName() != map[uint32]string{1: "Alice", 2: "Beth"}[id] {
			t.Fatalf("character %d name was overwritten: %s", id, response.GetTb().GetName())
		}
		if response.GetTb().GetRound().GetRound() != id+1 || response.GetTb().GetRes().GetResource()[0].GetValue() != 11*id || response.GetPermanent().GetEndings()[0] != 1000+id {
			t.Fatalf("character %d round/resources/endings were overwritten", id)
		}
	}
}

func TestRecoveryS01ConfigurationFailureDoesNotCreateSave(t *testing.T) {
	client := recoveryDB(t)
	if _, err := db.DefaultStore.Pool.Exec(context.Background(), `DELETE FROM config_entries WHERE category='ShareCfg/child2_resource.json'`); err != nil {
		t.Fatal(err)
	}
	var response protobuf.SC_29002
	recoveryPacket(t, client, NewEducateRequest, &protobuf.CS_29001{Id: proto.Uint32(1)}, &response)
	if response.GetResult() == 0 {
		t.Fatal("missing resources returned success")
	}
	var count int
	if err := db.DefaultStore.Pool.QueryRow(context.Background(), `SELECT count(*) FROM commander_educate_characters`).Scan(&count); err != nil || count != 0 {
		t.Fatal("config failure created save", err)
	}
}

func TestRecoveryS01InvalidCharacterDoesNotCreateSave(t *testing.T) {
	client := recoveryDB(t)
	var response protobuf.SC_29002
	recoveryPacket(t, client, NewEducateRequest, &protobuf.CS_29001{Id: proto.Uint32(999)}, &response)
	if response.GetResult() == 0 {
		t.Fatal("invalid character returned success")
	}
}

func TestRecoveryS01MigrationAndFailureBoundaries(t *testing.T) {
	client := recoveryDB(t)
	ctx := context.Background()
	// Mixed legacy save is only assigned to Info.Id; no second history is made.
	legacy := defaultEducateState(client.Commander.CommanderID, 2)
	legacy.Info.Name = proto.String("Explorer")
	legacy.Info.Round.Round = proto.Uint32(2)
	legacy.Info.Res.Attrs = append(legacy.Info.Res.Attrs, &protobuf.KVDATA{Key: proto.Uint32(101), Value: proto.Uint32(7)})
	raw, _ := proto.Marshal(legacy.Info)
	permanent, _ := proto.Marshal(legacy.Permanent)
	if _, err := db.DefaultStore.Pool.Exec(ctx, `INSERT INTO commander_tbs(commander_id,state,permanent) VALUES($1,$2,$3)`, int64(client.Commander.CommanderID), raw, permanent); err != nil {
		t.Fatal(err)
	}
	report, err := orm.MigrateEducateLegacy(client.Commander.CommanderID, false)
	if err != nil {
		t.Fatal(err)
	}
	if report.CharacterID != 2 || len(report.ForeignAttrs) != 1 {
		t.Fatalf("wrong attribution: %+v", report)
	}
	var count int
	if err := db.DefaultStore.Pool.QueryRow(ctx, `SELECT count(*) FROM commander_educate_characters`).Scan(&count); err != nil || count != 0 {
		t.Fatal("dry-run wrote rows", err)
	}
	report, err = orm.MigrateEducateLegacy(client.Commander.CommanderID, true)
	if err != nil {
		t.Fatal(err)
	}
	migrated, err := orm.GetCommanderTB(client.Commander.CommanderID, 2)
	if err != nil {
		t.Fatal(err)
	}
	info, history, err := migrated.Decode()
	if err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(info, legacy.Info) || !proto.Equal(history, legacy.Permanent) {
		t.Fatal("migration changed bytes/semantics")
	}
	info.Name = proto.String("Continued")
	if err := orm.SaveCommanderTB(migrated, info, history); err != nil {
		t.Fatal(err)
	}
	if _, err := orm.MigrateEducateLegacy(client.Commander.CommanderID, true); err != nil {
		t.Fatal(err)
	}
	var response protobuf.SC_29002
	recoveryPacket(t, client, NewEducateRequest, &protobuf.CS_29001{Id: proto.Uint32(2)}, &response)
	if response.GetTb().GetName() != "Continued" || response.GetTb().GetRound().GetRound() != 2 {
		t.Fatal("repeat migration overwrote continued save")
	}
	recoveryPacket(t, client, NewEducateRequest, &protobuf.CS_29001{Id: proto.Uint32(1)}, &response)
	if response.GetTb().GetRound().GetRound() != 1 || response.GetTb().GetName() == "Continued" {
		t.Fatal("legacy round/name copied to other character")
	}
	// Reopen a real pool: no process-local save cache may carry the isolation.
	old := db.DefaultStore
	var schema string
	if err := old.Pool.QueryRow(ctx, `SELECT current_schema()`).Scan(&schema); err != nil {
		t.Fatal(err)
	}
	pool, err := db.OpenPostgresPool(ctx, os.Getenv("EDUCATE_RECOVERY_DSN"), schema)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	db.DefaultStore = db.NewStore(pool)
	recoveryPacket(t, client, NewEducateRequest, &protobuf.CS_29001{Id: proto.Uint32(2)}, &response)
	if response.GetTb().GetName() != "Continued" {
		t.Fatal("reopened connection lost owner save")
	}
	// Bad blob must yield failure and stay byte-identical.
	if _, err := pool.Exec(ctx, `UPDATE commander_educate_characters SET state='\x01' WHERE character_id=2`); err != nil {
		t.Fatal(err)
	}
	recoveryPacket(t, client, NewEducateRequest, &protobuf.CS_29001{Id: proto.Uint32(2)}, &response)
	if response.GetResult() == 0 {
		t.Fatal("bad blob returned success")
	}
	var broken []byte
	if err := pool.QueryRow(ctx, `SELECT state FROM commander_educate_characters WHERE character_id=2`).Scan(&broken); err != nil || hex.EncodeToString(broken) != "01" {
		t.Fatal("bad blob was overwritten")
	}
	// Database rejects writes: never send success or alter the other save.
	_, err = pool.Exec(ctx, `CREATE FUNCTION reject_educate_write() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected save failure'; END $$; CREATE TRIGGER reject_save BEFORE UPDATE ON commander_educate_characters FOR EACH ROW EXECUTE FUNCTION reject_educate_write()`)
	if err != nil {
		t.Fatal(err)
	}
	recoveryPacket(t, client, NewEducateRequest, &protobuf.CS_29001{Id: proto.Uint32(1)}, &response)
	if response.GetResult() == 0 {
		t.Fatal("failed save returned success")
	}
	db.DefaultStore = old
}

func TestRecoveryS01QuarantinesInvalidLegacy(t *testing.T) {
	client := recoveryDB(t)
	ctx := context.Background()
	for _, id := range []uint32{90002, 90003, 90004} {
		if err := orm.CreateCommanderRoot(id, 1, "Invalid legacy", 0, 0); err != nil {
			t.Fatal(err)
		}
	}
	invalid := defaultEducateState(90002, 999)
	badID, _ := proto.Marshal(invalid.Info)
	zero := defaultEducateState(90003, 0)
	zeroID, _ := proto.Marshal(zero.Info)
	permanent, _ := proto.Marshal(zero.Permanent)
	for id, raw := range map[uint32][]byte{90002: badID, 90003: zeroID, 90004: {1}} {
		if _, err := db.DefaultStore.Pool.Exec(ctx, `INSERT INTO commander_tbs(commander_id,state,permanent) VALUES($1,$2,$3)`, int64(id), raw, permanent); err != nil {
			t.Fatal(err)
		}
		report, err := orm.MigrateEducateLegacy(id, true)
		if err != nil || report.Status != "quarantined" {
			t.Fatalf("expected quarantine: %+v %v", report, err)
		}
		if _, err := orm.MigrateEducateLegacy(id, true); err != nil {
			t.Fatal(err)
		}
		client.Commander.CommanderID = id
		var response protobuf.SC_29002
		recoveryPacket(t, client, NewEducateRequest, &protobuf.CS_29001{Id: proto.Uint32(1)}, &response)
		if response.GetResult() == 0 {
			t.Fatal("quarantined legacy silently reset")
		}
		var original, archived []byte
		if err := db.DefaultStore.Pool.QueryRow(ctx, `SELECT l.state,m.original_state FROM commander_tbs l JOIN commander_educate_migrations m USING(commander_id) WHERE l.commander_id=$1`, int64(id)).Scan(&original, &archived); err != nil {
			t.Fatal(err)
		}
		if hex.EncodeToString(original) != hex.EncodeToString(raw) || hex.EncodeToString(archived) != hex.EncodeToString(raw) {
			t.Fatal("archive/source changed")
		}
	}
}
