package educate_test

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/ggmolly/belfast/internal/answer/educate"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestRecoveryL03SpecialEventsRejectUnknownResults(t *testing.T) {
	client := recoveryDB(t)
	state, err := orm.GetOrCreateLegacyEducateState(90001)
	if err != nil {
		t.Fatal(err)
	}
	state.CurTime = &orm.LegacyEducateTime{Month: 4, Week: 4, Day: 7}
	state.Resources[1], state.Resources[2], state.Resources[3] = 50, 27, 0
	state.TargetID = 12
	if err := orm.SaveLegacyEducateState(state); err != nil {
		t.Fatal(err)
	}
	before, err := orm.GetConfigEntry("Runtime/legacy_educate_state", "90001")
	if err != nil {
		t.Fatal(err)
	}
	flags, err := orm.ListCommanderCommonFlags(90001)
	if err != nil {
		t.Fatal(err)
	}
	ids := []uint32{128, 106} // Real mind result 1001 and schedule result 7.
	for _, id := range ids {
		var response protobuf.SC_27028
		recoveryPacket(t, client, educate.EducateTriggerSpecEvent, &protobuf.CS_27027{SpecEventsId: proto.Uint32(id)}, &response)
		after, err := orm.GetConfigEntry("Runtime/legacy_educate_state", "90001")
		if err != nil || response.GetResult() == 0 || len(response.Drops) != 0 || !bytes.Equal(before.Data, after.Data) {
			t.Fatal("unknown result consumed player event", id, err, &response)
		}
		afterFlags, err := orm.ListCommanderCommonFlags(90001)
		if err != nil || !reflect.DeepEqual(flags, afterFlags) {
			t.Fatal("unknown result wrote finish/discount flag", id, err)
		}
	}
	// Explicit result=0 site performances retain their existing no-reward
	// contract. This is not a guess about a nonzero private result ID.
	var response protobuf.SC_27028
	recoveryPacket(t, client, educate.EducateTriggerSpecEvent, &protobuf.CS_27027{SpecEventsId: proto.Uint32(1104)}, &response)
	if response.GetResult() != 0 || len(response.Drops) != 0 {
		t.Fatal("known presentation-only site event rejected", &response)
	}
	after, err := orm.GetConfigEntry("Runtime/legacy_educate_state", "90001")
	if err != nil || !bytes.Equal(before.Data, after.Data) {
		t.Fatal("site presentation changed legacy state", err)
	}
	recoveryPacket(t, client, educate.EducateTriggerSpecEvent, &protobuf.CS_27027{SpecEventsId: proto.Uint32(1104)}, &response)
	if response.GetResult() == 0 {
		t.Fatal("site presentation accepted duplicate")
	}
}
