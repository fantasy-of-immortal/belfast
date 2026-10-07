package answer

import (
	"testing"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestEducateSetCallAndRequestRoundTrip(t *testing.T) {
	client := setupConfigTest(t)
	setCall := protobuf.CS_27031{Name: proto.String("Commander")}
	data, err := proto.Marshal(&setCall)
	if err != nil {
		t.Fatalf("marshal set call: %v", err)
	}
	if _, _, err := EducateSetCall(&data, client); err != nil {
		t.Fatalf("set call failed: %v", err)
	}
	var setCallResp protobuf.SC_27032
	decodeResponse(t, client, &setCallResp)
	if setCallResp.GetResult() != 0 {
		t.Fatalf("expected set call success")
	}

	requestData := []byte{}
	client.Buffer.Reset()
	if _, _, err := EducateRequest(&requestData, client); err != nil {
		t.Fatalf("educate request failed: %v", err)
	}
	var requestResp protobuf.SC_27001
	decodeResponse(t, client, &requestResp)
	if requestResp.GetChild().GetUserName() != "Commander" {
		t.Fatalf("expected call name Commander, got %q", requestResp.GetChild().GetUserName())
	}

	invalid := protobuf.CS_27031{Name: proto.String("abc")}
	invalidData, _ := proto.Marshal(&invalid)
	client.Buffer.Reset()
	if _, _, err := EducateSetCall(&invalidData, client); err != nil {
		t.Fatalf("set call invalid failed: %v", err)
	}
	decodeResponse(t, client, &setCallResp)
	if setCallResp.GetResult() == 0 {
		t.Fatalf("expected invalid call name to fail")
	}
}

func TestEducateSetTargetRejectsIncompleteConfig(t *testing.T) {
	client := setupConfigTest(t)
	seedConfigEntry(t, childTargetSetCategory, "7", `{"id":7}`)

	payload := protobuf.CS_27019{Id: proto.Uint32(7)}
	data, _ := proto.Marshal(&payload)
	if _, _, err := EducateSetTarget(&data, client); err != nil {
		t.Fatalf("set target failed: %v", err)
	}
	var resp protobuf.SC_27020
	decodeResponse(t, client, &resp)
	if resp.GetResult() == 0 {
		t.Fatalf("target without stage, date or tasks must fail")
	}

	client.Buffer.Reset()
	requestData := []byte{}
	if _, _, err := EducateRequest(&requestData, client); err != nil {
		t.Fatalf("educate request failed: %v", err)
	}
	var requestResp protobuf.SC_27001
	decodeResponse(t, client, &requestResp)
	if requestResp.GetChild().GetTarget() != 0 {
		t.Fatalf("rejected target changed saved target: %d", requestResp.GetChild().GetTarget())
	}

	bad := protobuf.CS_27019{Id: proto.Uint32(99)}
	badData, _ := proto.Marshal(&bad)
	client.Buffer.Reset()
	if _, _, err := EducateSetTarget(&badData, client); err != nil {
		t.Fatalf("set target invalid failed: %v", err)
	}
	decodeResponse(t, client, &resp)
	if resp.GetResult() == 0 {
		t.Fatalf("expected invalid target to fail")
	}
}

func TestEducateMapSiteOperateRejectsMissingReward(t *testing.T) {
	client := setupConfigTest(t)
	seedConfigEntry(t, childSiteCategory, "1", `{"id":1,"option":[101]}`)
	seedConfigEntry(t, childSiteOptionCategory, "101", `{"id":101,"type":2,"result":[201],"cost":[[2,3,1]],"count_limit":[1,100]}`)
	seedConfigEntry(t, childSiteOptionBranchCategory, "201", `{"id":201}`)
	state, err := orm.GetOrCreateLegacyEducateState(client.Commander.CommanderID)
	if err != nil {
		t.Fatal(err)
	}
	initial := state.Resources[3]
	data, _ := proto.Marshal(&protobuf.CS_27004{Siteid: proto.Uint32(1), Optionid: proto.Uint32(101)})
	if _, _, err := EducateMapSiteOperate(&data, client); err != nil {
		t.Fatal(err)
	}
	var response protobuf.SC_27005
	decodeResponse(t, client, &response)
	state, _ = orm.GetOrCreateLegacyEducateState(client.Commander.CommanderID)
	if response.GetResult() == 0 || state.Resources[3] != initial || state.OptionRecords[101] != 0 {
		t.Fatal("missing reward accepted or charged")
	}
}
func TestPlayerInfoUsesChangedEducateCharacter(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	seedConfigEntry(t, childEndingCategory, "555", `{"id":555}`)
	seedConfigEntry(t, secretarySpecialShipCategory, "555", `{"id":555}`)

	state, err := orm.GetOrCreateLegacyEducateState(client.Commander.CommanderID)
	if err != nil {
		t.Fatal(err)
	}
	state.Endings = []uint32{555}
	if err := orm.SaveLegacyEducateState(state); err != nil {
		t.Fatal(err)
	}

	change := protobuf.CS_27041{EndingId: proto.Uint32(555)}
	changeData, _ := proto.Marshal(&change)
	if _, _, err := ChangeEducateCharacter(&changeData, client); err != nil {
		t.Fatalf("change educate character failed: %v", err)
	}

	client.Commander.Ships = []orm.OwnedShip{{
		OwnerID:           client.Commander.CommanderID,
		ShipID:            202124,
		IsSecretary:       true,
		SecretaryPosition: proto.Uint32(0),
	}}

	client.Buffer.Reset()
	buffer := []byte{}
	if _, _, err := PlayerInfo(&buffer, client); err != nil {
		t.Fatalf("player info failed: %v", err)
	}
	payload := decodeFirstPacketPayload(t, client.Buffer.Bytes())
	var response protobuf.SC_11003
	if err := proto.Unmarshal(payload, &response); err != nil {
		t.Fatalf("unmarshal player info: %v", err)
	}
	if response.GetChildDisplay() != 555 {
		t.Fatalf("expected child display 555, got %d", response.GetChildDisplay())
	}
}
