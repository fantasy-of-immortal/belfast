package answer

import (
	"testing"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestEducateRefreshUnavailable(t *testing.T) {
	client := &connection.Client{}
	payload := protobuf.CS_27047{Type: proto.Uint32(1)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	if _, _, err := EducateRefresh(&buffer, client); err != nil {
		t.Fatalf("EducateRefresh failed: %v", err)
	}

	var response protobuf.SC_27048
	decodePacketAt(t, client, 0, 27048, &response)
	if response.GetResult() != 1 {
		t.Fatalf("expected unavailable result 1, got %d", response.GetResult())
	}
}

func TestEducateResetUnavailable(t *testing.T) {
	client := &connection.Client{}
	payload := protobuf.CS_27029{Type: proto.Uint32(1)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	if _, _, err := EducateReset(&buffer, client); err != nil {
		t.Fatalf("EducateReset failed: %v", err)
	}

	var response protobuf.SC_27030
	decodePacketAt(t, client, 0, 27030, &response)
	if response.GetResult() != 1 {
		t.Fatalf("expected unavailable result 1, got %d", response.GetResult())
	}
}

func TestEducateRefreshDecodeFailure(t *testing.T) {
	client := &connection.Client{}
	buffer := []byte{0xff, 0x00}

	_, outID, err := EducateRefresh(&buffer, client)
	if err == nil {
		t.Fatalf("expected decode error")
	}
	if outID != 27048 {
		t.Fatalf("expected outgoing packet id 27048, got %d", outID)
	}
}

func TestEducateResetDecodeFailure(t *testing.T) {
	client := &connection.Client{}
	buffer := []byte{0xff, 0x00}

	_, outID, err := EducateReset(&buffer, client)
	if err == nil {
		t.Fatalf("expected decode error")
	}
	if outID != 27030 {
		t.Fatalf("expected outgoing packet id 27030, got %d", outID)
	}
}
