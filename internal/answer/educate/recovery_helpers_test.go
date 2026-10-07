package educate_test

import (
	"bufio"

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
