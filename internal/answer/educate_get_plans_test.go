package answer

import (
	"testing"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

// Successful persisted weeks are covered by TestRecoveryL02LegacyWeekTransaction
// with actual configs, commander, protocol frames and a dedicated PostgreSQL schema.

func TestEducateGetPlansInvalidRange(t *testing.T) {
	client := &connection.Client{}
	payload := protobuf.CS_27012{
		Plans: []*protobuf.CHILD_PLAN_CELL{{
			Day:   proto.Uint32(7),
			Index: proto.Uint32(1),
			Value: []*protobuf.CHILD_PLAN_VAL{{PlanId: proto.Uint32(1001)}},
		}},
	}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	if _, _, err := EducateGetPlans(&buffer, client); err != nil {
		t.Fatalf("EducateGetPlans failed: %v", err)
	}

	var response protobuf.SC_27013
	decodePacketAt(t, client, 0, 27013, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected non-zero result for invalid range")
	}
}

func TestEducateGetPlansInvalidValueShape(t *testing.T) {
	client := &connection.Client{}
	payload := protobuf.CS_27012{
		Plans: []*protobuf.CHILD_PLAN_CELL{{
			Day:   proto.Uint32(1),
			Index: proto.Uint32(1),
			Value: []*protobuf.CHILD_PLAN_VAL{{PlanId: proto.Uint32(1001), SpecEventId: proto.Uint32(2001)}},
		}},
	}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	if _, _, err := EducateGetPlans(&buffer, client); err != nil {
		t.Fatalf("EducateGetPlans failed: %v", err)
	}

	var response protobuf.SC_27013
	decodePacketAt(t, client, 0, 27013, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected non-zero result for invalid value shape")
	}
}

func TestEducateGetPlansDecodeFailure(t *testing.T) {
	client := &connection.Client{}
	buffer := []byte{0xff, 0x00}

	_, outID, err := EducateGetPlans(&buffer, client)
	if err == nil {
		t.Fatalf("expected decode error")
	}
	if outID != 27013 {
		t.Fatalf("expected outgoing packet id 27013, got %d", outID)
	}
}
