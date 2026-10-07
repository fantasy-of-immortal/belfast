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
		{"NewEducateGetEndings", NewEducateGetEndings, &protobuf.CS_29003{}, &protobuf.SC_29004{}, 29004},
		{"NewEducateSelectEnding", NewEducateSelectEnding, &protobuf.CS_29005{}, &protobuf.SC_29006{}, 29006},
		{"NewEducateReset", NewEducateReset, &protobuf.CS_29007{}, &protobuf.SC_29008{}, 29008},
		{"NewEducateSetCall", NewEducateSetCall, &protobuf.CS_29009{}, &protobuf.SC_29010{}, 29010},
		{"NewEducateMainEvent", NewEducateMainEvent, &protobuf.CS_29011{}, &protobuf.SC_29012{}, 29012},
		{"NewEducateAssess", NewEducateAssess, &protobuf.CS_29013{}, &protobuf.SC_29014{}, 29014},
		{"NewEducateGetTopics", NewEducateGetTopics, &protobuf.CS_29015{}, &protobuf.SC_29016{}, 29016},
		{"NewEducateSelectTopic", NewEducateSelectTopic, &protobuf.CS_29017{}, &protobuf.SC_29018{}, 29018},
		{"NewEducateGetTalents", NewEducateGetTalents, &protobuf.CS_29019{}, &protobuf.SC_29020{}, 29020},
		{"NewEducateRefreshTalent", NewEducateRefreshTalent, &protobuf.CS_29021{}, &protobuf.SC_29022{}, 29022},
		{"NewEducateSelectTalent", NewEducateSelectTalent, &protobuf.CS_29023{}, &protobuf.SC_29024{}, 29024},
		{"NewEducateChangePhase", NewEducateChangePhase, &protobuf.CS_29025{}, &protobuf.SC_29026{}, 29026},
		{"NewEducateUpgradeFavor", NewEducateUpgradeFavor, &protobuf.CS_29027{}, &protobuf.SC_29028{}, 29028},
		{"NewEducateTriggerNode", NewEducateTriggerNode, &protobuf.CS_29030{}, &protobuf.SC_29031{}, 29031},
		{"NewEducateClearNodeChain", NewEducateClearNodeChain, &protobuf.CS_29032{}, &protobuf.SC_29033{}, 29033},
		{"NewEducateSchedule", NewEducateSchedule, &protobuf.CS_29040{}, &protobuf.SC_29041{}, 29041},
		{"NewEducateNextPlan", NewEducateNextPlan, &protobuf.CS_29042{}, &protobuf.SC_29043{}, 29043},
		{"NewEducateUpgradePlan", NewEducateUpgradePlan, &protobuf.CS_29044{}, &protobuf.SC_29045{}, 29045},
		{"NewEducateScheduleSkip", NewEducateScheduleSkip, &protobuf.CS_29046{}, &protobuf.SC_29047{}, 29047},
		{"NewEducateGetExtraDrop", NewEducateGetExtraDrop, &protobuf.CS_29048{}, &protobuf.SC_29049{}, 29049},
		{"NewEducateEnterAssess", NewEducateEnterAssess, &protobuf.CS_29050{}, &protobuf.SC_29051{}, 29051},
		{"NewEducateGetChoose", NewEducateGetChoose, &protobuf.CS_29126{}, &protobuf.SC_29127{}, 29127},
		{"NewEducateRequestChoices", NewEducateRequestChoices, &protobuf.CS_29107{}, &protobuf.SC_29108{}, 29108},
		{"NewEducateRefreshChoice", NewEducateRefreshChoice, &protobuf.CS_29105{}, &protobuf.SC_29106{}, 29106},
		{"NewEducateMakeChoice", NewEducateMakeChoice, &protobuf.CS_29103{}, &protobuf.SC_29104{}, 29104},
		{"NewEducateGiveUpChoice", NewEducateGiveUpChoice, &protobuf.CS_29101{}, &protobuf.SC_29102{}, 29102},
		{"NewEducateReplaceTarot", NewEducateReplaceTarot, &protobuf.CS_29120{}, &protobuf.SC_29121{}, 29121},
		{"NewEducateUpgradeEntry", NewEducateUpgradeEntry, &protobuf.CS_29122{}, &protobuf.SC_29123{}, 29123},
		{"NewEducateGiveUpEntryUp", NewEducateGiveUpEntryUp, &protobuf.CS_29124{}, &protobuf.SC_29125{}, 29125},
		{"NewEducateRefreshShop", NewEducateRefreshShop, &protobuf.CS_29072{}, &protobuf.SC_29073{}, 29073},
		{"NewEducateGetMap", NewEducateGetMap, &protobuf.CS_29060{}, &protobuf.SC_29061{}, 29061},
		{"NewEducateMapNormal", NewEducateMapNormal, &protobuf.CS_29062{}, &protobuf.SC_29063{}, 29063},
		{"NewEducateMapEvent", NewEducateMapEvent, &protobuf.CS_29064{}, &protobuf.SC_29065{}, 29065},
		{"NewEducateShopping", NewEducateShopping, &protobuf.CS_29066{}, &protobuf.SC_29067{}, 29067},
		{"NewEducateMapShip", NewEducateMapShip, &protobuf.CS_29068{}, &protobuf.SC_29069{}, 29069},
		{"NewEducateUpgradeNormalSite", NewEducateUpgradeNormalSite, &protobuf.CS_29070{}, &protobuf.SC_29071{}, 29071},
		{"NewEducateSelectMind", NewEducateSelectMind, &protobuf.CS_29090{}, &protobuf.SC_29091{}, 29091},
		{"NewEducateRefresh", NewEducateRefresh, &protobuf.CS_29092{}, &protobuf.SC_29093{}, 29093},
		{"NewEducateRequest", NewEducateRequest, &protobuf.CS_29001{}, &protobuf.SC_29002{}, 29002},
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
