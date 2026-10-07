package answer

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func setupEducateHandlerTest(t *testing.T, commanderID uint32) *connection.Client {
	t.Helper()
	os.Setenv("MODE", "test")
	orm.InitDatabase()
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.CommanderCommonFlag{})
	clearTable(t, &orm.EducateShopState{})
	clearTable(t, &orm.OwnedResource{})
	clearTable(t, &orm.Commander{})
	if err := orm.CreateCommanderRoot(commanderID, commanderID, "Educate Tester", 0, 0); err != nil {
		t.Fatalf("create commander: %v", err)
	}
	commander := orm.Commander{CommanderID: commanderID}
	if err := commander.Load(); err != nil {
		t.Fatalf("load commander: %v", err)
	}
	return &connection.Client{Commander: &commander}
}

func seedCommanderTaskProgress(t *testing.T, commanderID uint32, taskID uint32, progress uint32, target uint32) {
	t.Helper()
	err := orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		return orm.UpsertCommanderTaskProgressTx(context.Background(), tx, commanderID, taskID, orm.TaskProgressUpdate, progress, target, 1)
	})
	if err != nil {
		t.Fatalf("seed commander task progress: %v", err)
	}
}

func TestEducateGetEventsSortedAndConsumedFiltered(t *testing.T) {
	client := setupEducateHandlerTest(t, 9101)
	seedConfigEntry(t, "ShareCfg/child_event_special.json", "rows", `[
		{"id":200,"show":1,"type":1},
		{"id":100,"show":1,"type":1},
		{"id":300,"show":0,"type":1}
	]`)
	if err := orm.SetCommanderCommonFlag(client.Commander.CommanderID, educateFlagID(educateFlagHomeEventBase, 200)); err != nil {
		t.Fatalf("seed consumed flag: %v", err)
	}

	buf, _ := proto.Marshal(&protobuf.CS_27014{Type: proto.Uint32(0)})
	if _, _, err := EducateGetEvents(&buf, client); err != nil {
		t.Fatalf("EducateGetEvents: %v", err)
	}
	var resp protobuf.SC_27015
	decodePacketAt(t, client, 0, 27015, &resp)
	if resp.GetResult() != 0 {
		t.Fatalf("expected result 0, got %d", resp.GetResult())
	}
	if len(resp.GetEvents()) != 1 || resp.GetEvents()[0] != 100 {
		t.Fatalf("unexpected events: %v", resp.GetEvents())
	}
}

func TestEducateShopRequestAndPurchaseFlow(t *testing.T) {
	client := setupEducateHandlerTest(t, 9104)
	seedConfigEntry(t, "ShareCfg/child_shop.json", "rows", `[{"id":2,"goods_num":2,"goods_pool":[[11,1,500,[]],[12,1,500,[]]],"goods_refresh_time":-1}]`)
	seedConfigEntry(t, "ShareCfg/child_shop_template.json", "rows", `[
		{"id":11,"time":"always","item_id":500,"resource":1,"resource_num":10,"buy_num":1},
		{"id":12,"time":"always","item_id":501,"resource":1,"resource_num":20,"buy_num":1}
	]`)
	if err := client.Commander.SetResource(1, 50); err != nil {
		t.Fatalf("seed resource: %v", err)
	}

	state, err := orm.GetOrCreateLegacyEducateState(9104)
	if err != nil {
		t.Fatal(err)
	}
	state.Resources[1] = 50
	if err := orm.SaveLegacyEducateState(state); err != nil {
		t.Fatal(err)
	}
	seedConfigEntry(t, "ShareCfg/child_item.json", "500", `{"id":500,"display":[]}`)
	getBuf, _ := proto.Marshal(&protobuf.CS_27043{ShopId: proto.Uint32(2)})
	if _, _, err := EducateRequestShopData(&getBuf, client); err != nil {
		t.Fatalf("EducateRequestShopData: %v", err)
	}
	var getResp protobuf.SC_27044
	decodePacketAt(t, client, 0, 27044, &getResp)
	if getResp.GetResult() != 0 || len(getResp.GetShopData().GetGoods()) != 2 {
		t.Fatalf("unexpected shop data response: %+v", getResp)
	}

	client.Buffer.Reset()
	buyBuf, _ := proto.Marshal(&protobuf.CS_27033{ShopId: proto.Uint32(2), Goods: []*protobuf.CHILD_SHOP_GOODS{{Id: proto.Uint32(11), Num: proto.Uint32(1)}}})
	if _, _, err := EducateShopping(&buyBuf, client); err != nil {
		t.Fatalf("EducateShopping: %v", err)
	}
	var buyResp protobuf.SC_27034
	decodePacketAt(t, client, 0, 27034, &buyResp)
	if buyResp.GetResult() != 0 || len(buyResp.GetDrops()) != 1 || buyResp.GetDrops()[0].GetId() != 500 {
		t.Fatalf("unexpected purchase response: %+v", buyResp)
	}
	if got := client.Commander.GetResourceCount(1); got != 50 {
		t.Fatalf("expected main-port resource unchanged at 50, got %d", got)
	}
	if got := client.Commander.GetItemCount(500); got != 0 {
		t.Fatalf("expected no main-port item, got %d", got)
	}

	client.Buffer.Reset()
	if _, _, err := EducateShopping(&buyBuf, client); err != nil {
		t.Fatalf("EducateShopping second: %v", err)
	}
	decodePacketAt(t, client, 0, 27034, &buyResp)
	if buyResp.GetResult() == 0 {
		t.Fatalf("expected sold-out failure")
	}

	client.Buffer.Reset()
	if _, _, err := EducateRequestShopData(&getBuf, client); err != nil {
		t.Fatalf("EducateRequestShopData replay: %v", err)
	}
	decodePacketAt(t, client, 0, 27044, &getResp)
	if getResp.GetShopData().GetGoods()[0].GetNum() != 0 {
		t.Fatalf("expected updated remaining count in shop data")
	}
}

func TestEducateShoppingFailurePathsAndDecodeError(t *testing.T) {
	client := setupEducateHandlerTest(t, 9105)
	seedConfigEntry(t, "ShareCfg/child_shop.json", "rows", `[{"id":3,"goods_num":1,"goods_pool":[[21,1,500,[]]],"goods_refresh_time":-1}]`)
	seedConfigEntry(t, "ShareCfg/child_shop_template.json", "rows", `[{"id":21,"item_id":600,"resource":1,"resource_num":30,"buy_num":1}]`)
	if err := client.Commander.SetResource(1, 5); err != nil {
		t.Fatalf("seed resource: %v", err)
	}

	buyBuf, _ := proto.Marshal(&protobuf.CS_27033{ShopId: proto.Uint32(3), Goods: []*protobuf.CHILD_SHOP_GOODS{{Id: proto.Uint32(21), Num: proto.Uint32(1)}}})
	if _, _, err := EducateShopping(&buyBuf, client); err != nil {
		t.Fatalf("EducateShopping insufficient: %v", err)
	}
	var buyResp protobuf.SC_27034
	decodePacketAt(t, client, 0, 27034, &buyResp)
	if buyResp.GetResult() == 0 {
		t.Fatalf("expected insufficient resource failure")
	}

	bad := []byte{0x01, 0x02}
	if _, packetID, err := EducateRequestShopData(&bad, client); err == nil || packetID != 27044 {
		t.Fatalf("expected decode error with packet 27044")
	}
}

// Real selected-target claims and educate inventory are exercised with current
// configs in TestRecoveryL03LegacyTaskClaims. Main-port progress cannot award them.
func TestEducateTargetAwardRejectsMainPortProgressAndUnsupportedType(t *testing.T) {
	client := setupEducateHandlerTest(t, 9106)
	seedCommanderTaskProgress(t, client.Commander.CommanderID, 1001, 1, 1)
	seedCommanderTaskProgress(t, client.Commander.CommanderID, 1002, 1, 1)
	for _, kind := range []uint32{0, 1} {
		client.Buffer.Reset()
		buf, _ := proto.Marshal(&protobuf.CS_27035{Type: proto.Uint32(kind)})
		if _, _, err := EducateGetTargetAward(&buf, client); err != nil {
			t.Fatal(err)
		}
		var response protobuf.SC_27036
		decodePacketAt(t, client, 0, 27036, &response)
		if response.GetResult() == 0 || len(response.Drops) != 0 {
			t.Fatal("unselected target/main-port progress awarded", &response)
		}
	}
}
