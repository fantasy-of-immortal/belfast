package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ggmolly/belfast/internal/orm"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func TestPlayerTBUpdateCannotInvalidateManagedLifecycle(t *testing.T) {
	app := newPlayerHandlerTestApp(t)
	clearCommanderTB(t)
	clearCommanders(t)
	seedCommander(t, 9199, "Managed TB")
	body := `{"tb":` + buildTestTBInfoJSON(t) + `,"permanent":` + buildTestTBPermanentJSON(t) + `}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/players/9199/tb", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	app.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatal(recorder.Code, recorder.Body.String())
	}
	entry, err := orm.GetCommanderTB(9199, 1)
	if err != nil {
		t.Fatal(err)
	}
	info, permanent, err := entry.Decode()
	if err != nil {
		t.Fatal(err)
	}
	entry.Metadata = []byte(`{"lifecycle":{"version":1,"round":1,"stages":{}}}`)
	if err := orm.SaveCommanderTB(entry, info, permanent); err != nil {
		t.Fatal(err)
	}
	before, _ := orm.GetCommanderTB(9199, 1)
	info.Round.Round = proto.Uint32(info.Round.GetRound() + 1)
	infoJSON, _ := protojson.Marshal(info)
	permanentJSON, _ := protojson.Marshal(permanent)
	body = `{"tb":` + string(infoJSON) + `,"permanent":` + string(permanentJSON) + `}`
	request = httptest.NewRequest(http.MethodPut, "/api/v1/players/9199/tb", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder = httptest.NewRecorder()
	app.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusConflict {
		t.Fatal("managed round change accepted", recorder.Code, recorder.Body.String())
	}
	after, _ := orm.GetCommanderTB(9199, 1)
	if before.Revision != after.Revision || !bytes.Equal(before.State, after.State) || !bytes.Equal(before.Metadata, after.Metadata) || !bytes.Equal(before.Permanent, after.Permanent) {
		t.Fatal("rejected API write changed save")
	}
}
