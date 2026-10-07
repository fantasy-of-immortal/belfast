package answer

import (
	"testing"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

// Successful persisted weeks are covered by TestRecoveryL02LegacyWeekTransaction
// with actual configs, commander, protocol frames and a dedicated PostgreSQL schema.

func TestEducateExecutePlansUnsupportedType(t *testing.T) {
	client := &connection.Client{}
	payload := protobuf.CS_27002{Type: proto.Uint32(2)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	if _, _, err := EducateExecutePlans(&buffer, client); err != nil {
		t.Fatalf("EducateExecutePlans failed: %v", err)
	}

	var response protobuf.SC_27003
	decodePacketAt(t, client, 0, 27003, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected non-zero result for unsupported type")
	}
}

func TestEducateExecutePlansDecodeFailure(t *testing.T) {
	client := &connection.Client{}
	buffer := []byte{0xff, 0x00}

	_, outID, err := EducateExecutePlans(&buffer, client)
	if err == nil {
		t.Fatalf("expected decode error")
	}
	if outID != 27003 {
		t.Fatalf("expected outgoing packet id 27003, got %d", outID)
	}
}
