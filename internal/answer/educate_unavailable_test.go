package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/packets"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"testing"
)

func populateRequiredAudit(message protoreflect.Message) {
	fields := message.Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		field := fields.Get(i)
		if field.Cardinality() != protoreflect.Required || message.Has(field) {
			continue
		}
		if field.Kind() == protoreflect.MessageKind {
			populateRequiredAudit(message.Mutable(field).Message())
		} else {
			message.Set(field, field.Default())
		}
	}
}
func TestUnavailableEducateProtocolMatrix(t *testing.T) {
	cases := []struct {
		name              string
		handler           func(*[]byte, *connection.Client) (int, int, error)
		request, response proto.Message
		packet            int
	}{
		{"EducateExecutePlans", EducateExecutePlans, &protobuf.CS_27002{}, &protobuf.SC_27003{}, 27003},
		{"EducateGetEvents", EducateGetEvents, &protobuf.CS_27014{}, &protobuf.SC_27015{}, 27015},
		{"EducateGetPlans", EducateGetPlans, &protobuf.CS_27012{}, &protobuf.SC_27013{}, 27013},
		{"EducateGetTargetAward", EducateGetTargetAward, &protobuf.CS_27035{}, &protobuf.SC_27036{}, 27036},
		{"EducateUpgradeFavor", EducateUpgradeFavor, &protobuf.CS_27006{}, &protobuf.SC_27007{}, 27007},
		{"EducateTriggerEnd", EducateTriggerEnd, &protobuf.CS_27008{}, &protobuf.SC_27009{}, 27009},
		{"EducateGetEndings", EducateGetEndings, &protobuf.CS_27010{}, &protobuf.SC_10998{}, 10998},
		{"EducateSetTarget", EducateSetTarget, &protobuf.CS_27019{}, &protobuf.SC_27020{}, 27020},
		{"EducateSubmitTask", EducateSubmitTask, &protobuf.CS_27023{}, &protobuf.SC_27024{}, 27024},
		{"EducateSetCall", EducateSetCall, &protobuf.CS_27031{}, &protobuf.SC_27032{}, 27032},
		{"EducateAddTaskProgress", EducateAddTaskProgress, &protobuf.CS_27037{}, &protobuf.SC_27038{}, 27038},
		{"EducateAddExtraAttr", EducateAddExtraAttr, &protobuf.CS_27039{}, &protobuf.SC_27040{}, 27040},
		{"ChangeEducateCharacter", ChangeEducateCharacter, &protobuf.CS_27041{}, &protobuf.SC_27042{}, 27042},
		{"EducateMapSiteOperate", EducateMapSiteOperate, &protobuf.CS_27004{}, &protobuf.SC_27005{}, 27005},
		{"EducateRefresh", EducateRefresh, &protobuf.CS_27047{}, &protobuf.SC_27048{}, 27048},
		{"EducateRequest", EducateRequest, &protobuf.CS_27000{}, &protobuf.SC_27001{}, 27001},
		{"EducateRequestOption", EducateRequestOption, &protobuf.CS_27045{}, &protobuf.SC_27046{}, 27046},
		{"EducateRequestShopData", EducateRequestShopData, &protobuf.CS_27043{}, &protobuf.SC_27044{}, 27044},
		{"EducateReset", EducateReset, &protobuf.CS_27029{}, &protobuf.SC_27030{}, 27030},
		{"EducateShopping", EducateShopping, &protobuf.CS_27033{}, &protobuf.SC_27034{}, 27034},
		{"EducateTriggerEvent", EducateTriggerEvent, &protobuf.CS_27016{}, &protobuf.SC_27017{}, 27017},
		{"EducateTriggerSpecEvent", EducateTriggerSpecEvent, &protobuf.CS_27027{}, &protobuf.SC_27028{}, 27028},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			populateRequiredAudit(tc.request.ProtoReflect())
			data, err := proto.Marshal(tc.request)
			if err != nil {
				t.Fatal(err)
			}
			client := &connection.Client{}
			_, id, err := tc.handler(&data, client)
			if err != nil || id != tc.packet {
				t.Fatal(id, err)
			}
			wire := client.Buffer.Bytes()
			if len(wire) < packets.HEADER_SIZE {
				t.Fatal("missing failure reply")
			}
			if err := proto.Unmarshal(wire[packets.HEADER_SIZE:], tc.response); err != nil {
				t.Fatal(err)
			}
			response := tc.response.ProtoReflect()
			result := response.Descriptor().Fields().ByName("result")
			if result == nil || response.Get(result).Uint() != 1 {
				t.Fatal("disabled engine accepted request")
			}
			if reply, ok := tc.response.(*protobuf.SC_10998); ok && reply.GetCmd() != 27010 {
				t.Fatal("unsupported reply lost request ID")
			}
		})
	}
}
