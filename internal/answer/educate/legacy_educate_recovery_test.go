package educate_test

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"testing"

	"github.com/ggmolly/belfast/internal/answer/educate"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/packets"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestRecoveryL01LegacyDateAndSnapshot(t *testing.T) {
	client := recoveryDB(t)
	if _, err := loadEducateState(client, 1); err != nil {
		t.Fatal(err)
	}
	newBefore, err := loadAuditRole(90001, 1)
	if err != nil {
		t.Fatal(err)
	}
	const category = "Runtime/legacy_educate_state"
	old := json.RawMessage(`{"commander_id":90001,"call_name":"Saved","attrs":{"102":40},"resources":{"1":0,"2":0,"3":2},"target_id":2,"week_plans":[{"day":1,"index":1,"plan_id":101}],"future_extension":{"saved":[7,8]}}`)
	if err := orm.UpsertConfigEntry(category, "90001", old); err != nil {
		t.Fatal(err)
	}
	var response protobuf.SC_27001
	recoveryPacket(t, client, educate.EducateRequest, &protobuf.CS_27000{Type: proto.Uint32(1)}, &response)
	child := response.Child
	if response.GetResult() != 0 || child.CurTime.GetMonth() != 2 || child.CurTime.GetWeek() != 4 || child.CurTime.GetDay() != 7 || child.GetMoney() != 0 || child.GetMood() != 0 || child.GetSiteNumber() != 2 || child.GetTarget() != 2 || len(child.Plans) != 1 || child.Plans[0].Value[0].GetPlanId() != 101 {
		t.Fatal("legacy snapshot", &response)
	}
	state, err := orm.GetOrCreateLegacyEducateState(90001)
	if err != nil {
		t.Fatal(err)
	}
	if state.Version != 1 || state.DateOrigin != "compat.last-served-27001" || state.Attrs[102] != 40 {
		t.Fatal("compatibility migration", state)
	}
	before, err := orm.GetConfigEntry(category, "90001")
	if err != nil {
		t.Fatal(err)
	}
	recoveryPacket(t, client, educate.EducateRequest, &protobuf.CS_27000{Type: proto.Uint32(1)}, &response)
	after, err := orm.GetConfigEntry(category, "90001")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before.Data, after.Data) {
		t.Fatal("query changed legacy snapshot")
	}
	state.TargetID = 12
	state.WeekPlans = nil
	if err := orm.SaveLegacyEducateState(state); err != nil {
		t.Fatal(err)
	}
	recoveryPacket(t, client, educate.EducateRequest, &protobuf.CS_27000{Type: proto.Uint32(1)}, &response)
	state, err = orm.GetOrCreateLegacyEducateState(90001)
	if err != nil || state.CurTime.Month != 3 || state.CurTime.Week != 4 || state.CompatDateBeforeAlignment == nil || state.CompatDateBeforeAlignment.Month != 2 || state.DateOrigin != "compat.target-selection-date" || state.Attrs[102] != 40 || state.Resources[3] != 2 || state.TargetID != 12 {
		t.Fatal("compatibility alignment changed player values", state, err)
	}
	state.CurTime = &orm.LegacyEducateTime{Month: 13, Week: 1, Day: 7}
	if err := orm.SaveLegacyEducateState(state); err != nil {
		t.Fatal(err)
	}
	recoveryPacket(t, client, educate.EducateRequest, &protobuf.CS_27000{Type: proto.Uint32(1)}, &response)
	if response.Child.CurTime.GetMonth() != 13 || response.Child.GetSiteNumber() != 2 {
		t.Fatal("stored game date not returned")
	}
	entry, err := orm.GetConfigEntry(category, "90001")
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(entry.Data, &fields); err != nil {
		t.Fatal(err)
	}
	if string(fields["future_extension"]) != `{"saved": [7, 8]}` && string(fields["future_extension"]) != `{"saved":[7,8]}` {
		t.Fatal("extension data erased", string(fields["future_extension"]))
	}
	for _, broken := range []json.RawMessage{json.RawMessage(`null`), json.RawMessage(`[]`), json.RawMessage(`{"cur_time":null}`), json.RawMessage(`{"state_version":2}`), json.RawMessage(`{"state_version":1,"cur_time":{"month":3,"week":0,"day":7}}`)} {
		if err := orm.UpsertConfigEntry(category, "90002", broken); err != nil {
			t.Fatal(err)
		}
		before, err := orm.GetConfigEntry(category, "90002")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := orm.GetOrCreateLegacyEducateState(90002); err == nil {
			t.Fatal("invalid state accepted", string(broken))
		}
		after, err := orm.GetConfigEntry(category, "90002")
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before.Data, after.Data) {
			t.Fatal("invalid state overwritten")
		}
	}
	fresh, err := orm.GetOrCreateLegacyEducateState(90003)
	if err != nil || fresh.Version != 1 || fresh.CurTime.Day != 7 || fresh.DateOrigin != "initial.client-date" {
		t.Fatal("new legacy save", fresh, err)
	}
	newAfter, err := loadAuditRole(90001, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(newBefore.State, newAfter.State) || !bytes.Equal(newBefore.Permanent, newAfter.Permanent) || !bytes.Equal(newBefore.Metadata, newAfter.Metadata) || newBefore.Revision != newAfter.Revision {
		t.Fatal("legacy query changed new educate save")
	}

}

func TestRecoveryL02LegacyWeekTransaction(t *testing.T) {
	client := recoveryDB(t)
	state, err := orm.GetOrCreateLegacyEducateState(90001)
	if err != nil {
		t.Fatal(err)
	}
	state.Resources[1], state.Resources[2], state.Resources[3] = 0, 50, 2
	if err := orm.SaveLegacyEducateState(state); err != nil {
		t.Fatal(err)
	}
	var submit protobuf.SC_27013
	var execute protobuf.SC_27003
	cells := make([]*protobuf.CHILD_PLAN_CELL, 0, 6)
	for day := uint32(1); day <= 6; day++ {
		cells = append(cells, &protobuf.CHILD_PLAN_CELL{Day: proto.Uint32(day), Index: proto.Uint32(1), Value: []*protobuf.CHILD_PLAN_VAL{{PlanId: proto.Uint32(1101)}}})
	}
	recoveryPacket(t, client, educate.EducateGetPlans, &protobuf.CS_27012{Plans: cells}, &submit)
	if submit.GetResult() != 0 {
		t.Fatal("real first-stage week rejected", &submit)
	}
	recoveryPacket(t, client, educate.EducateGetPlans, &protobuf.CS_27012{Plans: cells}, &submit)
	reversed := append([]*protobuf.CHILD_PLAN_CELL(nil), cells...)
	for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
		reversed[i], reversed[j] = reversed[j], reversed[i]
	}
	recoveryPacket(t, client, educate.EducateGetPlans, &protobuf.CS_27012{Plans: reversed}, &submit)
	state, err = orm.GetOrCreateLegacyEducateState(90001)
	if err != nil || state.PlanVersion != 1 || len(state.WeekPlans) != 6 {
		t.Fatal("pending repeat", state, err)
	}
	recoveryPacket(t, client, educate.EducateExecutePlans, &protobuf.CS_27002{Type: proto.Uint32(1)}, &execute)
	if execute.GetResult() != 0 || len(execute.PlanResults) != 18 || len(execute.PlanResults[15].PlanDrops) != 2 {
		t.Fatal("18-cell real result", &execute)
	}
	encoded, err := proto.Marshal(&execute)
	if err != nil {
		t.Fatal(err)
	}
	state, err = orm.GetOrCreateLegacyEducateState(90001)
	if err != nil || state.Attrs[101] != 18 || state.Attrs[104] != 18 || state.Resources[1] != 0 || state.Resources[2] != 50 || state.Resources[3] != 0 || state.CurTime.Month != 3 || state.CurTime.Week != 1 || state.PlanHistory[1101] != 6 || len(state.WeekPlans) != 0 {
		t.Fatal("week settlement", state, err)
	}
	before, err := orm.GetConfigEntry("Runtime/legacy_educate_state", "90001")
	if err != nil {
		t.Fatal(err)
	}
	recoveryPacket(t, client, educate.EducateExecutePlans, &protobuf.CS_27002{Type: proto.Uint32(1)}, &execute)
	if replay, err := proto.Marshal(&execute); err != nil || !bytes.Equal(encoded, replay) {
		t.Fatal("cached result changed", err)
	}
	after, err := orm.GetConfigEntry("Runtime/legacy_educate_state", "90001")
	if err != nil || !bytes.Equal(before.Data, after.Data) {
		t.Fatal("repeat paid or advanced twice", err)
	}
	var query protobuf.SC_27001
	recoveryPacket(t, client, educate.EducateRequest, &protobuf.CS_27000{Type: proto.Uint32(1)}, &query)
	if query.Child.CurTime.GetWeek() != 1 || len(query.Child.Plans) != 0 || len(query.Child.PlanHistory) != 1 || query.Child.PlanHistory[0].GetCount() != 6 {
		t.Fatal("cold query/history", &query)
	}
	for _, bad := range [][]*protobuf.CHILD_PLAN_CELL{nil, {cells[0], cells[0]}, {{Day: proto.Uint32(1), Index: proto.Uint32(2), Value: cells[0].Value}}, {{Day: proto.Uint32(1), Index: proto.Uint32(1), Value: []*protobuf.CHILD_PLAN_VAL{{PlanId: proto.Uint32(999999)}}}}} {
		before, err = orm.GetConfigEntry("Runtime/legacy_educate_state", "90001")
		if err != nil {
			t.Fatal(err)
		}
		recoveryPacket(t, client, educate.EducateGetPlans, &protobuf.CS_27012{Plans: bad}, &submit)
		after, err = orm.GetConfigEntry("Runtime/legacy_educate_state", "90001")
		if submit.GetResult() == 0 || err != nil || !bytes.Equal(before.Data, after.Data) {
			t.Fatal("invalid week changed save", &submit, err)
		}
	}
	// Partial weeks still produce all playback cells. Failed settlement rolls
	// back the entire transaction, including fees, rewards and saved date.
	recoveryPacket(t, client, educate.EducateGetPlans, &protobuf.CS_27012{Plans: cells[:1]}, &submit)
	if submit.GetResult() != 0 {
		t.Fatal("partial week", &submit)
	}
	state, err = orm.GetOrCreateLegacyEducateState(90001)
	if err != nil {
		t.Fatal(err)
	}
	state.Attrs[101] = math.MaxUint32
	if err := orm.SaveLegacyEducateState(state); err != nil {
		t.Fatal(err)
	}
	before, err = orm.GetConfigEntry("Runtime/legacy_educate_state", "90001")
	if err != nil {
		t.Fatal(err)
	}
	recoveryPacket(t, client, educate.EducateExecutePlans, &protobuf.CS_27002{Type: proto.Uint32(1)}, &execute)
	after, err = orm.GetConfigEntry("Runtime/legacy_educate_state", "90001")
	if execute.GetResult() == 0 || err != nil || !bytes.Equal(before.Data, after.Data) {
		t.Fatal("failed settlement not atomic", err)
	}
	state.Attrs[101] = 18
	for _, mood := range []int32{0, 20, 40, 60} {
		state.Resources[2] = mood
		if err := orm.SaveLegacyEducateState(state); err != nil {
			t.Fatal(err)
		}
		recoveryPacket(t, client, educate.EducateExecutePlans, &protobuf.CS_27002{Type: proto.Uint32(1)}, &execute)
		if execute.GetResult() == 0 {
			t.Fatal("unrestored mood/boundary fabricated", mood)
		}
	}
	// Real advanced course: predecessor, ability, stage and fee boundaries.
	state.WeekPlans = nil
	state.CurTime = &orm.LegacyEducateTime{Month: 3, Week: 4, Day: 7}
	state.Resources[1], state.Resources[2], state.Resources[3] = 4, 50, 2
	state.Attrs[101] = 800
	state.PlanHistory[1111] = 10
	paid := []*protobuf.CHILD_PLAN_CELL{{Day: proto.Uint32(1), Index: proto.Uint32(1), Value: []*protobuf.CHILD_PLAN_VAL{{PlanId: proto.Uint32(1112)}}}}
	if err := orm.SaveLegacyEducateState(state); err != nil {
		t.Fatal(err)
	}
	before, err = orm.GetConfigEntry("Runtime/legacy_educate_state", "90001")
	if err != nil {
		t.Fatal(err)
	}
	recoveryPacket(t, client, educate.EducateGetPlans, &protobuf.CS_27012{Plans: paid}, &submit)
	after, err = orm.GetConfigEntry("Runtime/legacy_educate_state", "90001")
	if submit.GetResult() == 0 || err != nil || !bytes.Equal(before.Data, after.Data) {
		t.Fatal("insufficient fee paid", err)
	}
	state.Resources[1] = 5
	if err := orm.SaveLegacyEducateState(state); err != nil {
		t.Fatal(err)
	}
	recoveryPacket(t, client, educate.EducateGetPlans, &protobuf.CS_27012{Plans: paid}, &submit)
	if submit.GetResult() != 0 {
		t.Fatal("earned advanced course locked", &submit)
	}
	recoveryPacket(t, client, educate.EducateExecutePlans, &protobuf.CS_27002{Type: proto.Uint32(1)}, &execute)
	state, err = orm.GetOrCreateLegacyEducateState(90001)
	if execute.GetResult() != 0 || err != nil || state.Resources[1] != 0 || state.Resources[2] != 48 || state.Resources[3] != 2 || state.Attrs[101] != 836 || state.Attrs[202] != 1 || state.CurTime.Month != 4 || state.CurTime.Week != 1 {
		t.Fatal("actual advanced course cost/reward/date", state, err)
	}
	state.Resources[2] = 0
	if err := orm.SaveLegacyEducateState(state); err != nil {
		t.Fatal(err)
	}
	sleep := make([]*protobuf.CHILD_PLAN_CELL, 0, 6)
	for day := uint32(1); day <= 6; day++ {
		sleep = append(sleep, &protobuf.CHILD_PLAN_CELL{Day: proto.Uint32(day), Index: proto.Uint32(1), Value: []*protobuf.CHILD_PLAN_VAL{{PlanId: proto.Uint32(1402)}}})
	}
	recoveryPacket(t, client, educate.EducateGetPlans, &protobuf.CS_27012{Plans: sleep}, &submit)
	if submit.GetResult() != 0 {
		t.Fatal("zero-mood sleep week rejected", &submit)
	}
	recoveryPacket(t, client, educate.EducateExecutePlans, &protobuf.CS_27002{Type: proto.Uint32(1)}, &execute)
	state, err = orm.GetOrCreateLegacyEducateState(90001)
	if execute.GetResult() != 0 || err != nil || state.Resources[2] != 12 || state.CurTime.Week != 2 || state.Attrs[101] != 836 {
		t.Fatal("sleep reward/date", state, err)
	}
}

func TestRecoveryL03LegacyTaskClaims(t *testing.T) {
	client := recoveryDB(t)
	if err := orm.SetCommanderCommonFlag(90001, 270350001); err != nil {
		t.Fatal(err)
	}
	for _, id := range []uint32{1, 2} {
		if _, err := loadEducateState(client, id); err != nil {
			t.Fatal(err)
		}
	}
	before1, _ := loadAuditRole(90001, 1)
	before2, _ := loadAuditRole(90001, 2)
	state, err := orm.GetOrCreateLegacyEducateState(90001)
	if err != nil {
		t.Fatal(err)
	}
	state.TargetID = 12
	state.CurTime = &orm.LegacyEducateTime{Month: 4, Week: 1, Day: 7}
	state.Attrs[102] = 40
	state.Resources[1], state.Resources[2] = 0, 50
	if err := orm.SaveLegacyEducateState(state); err != nil {
		t.Fatal(err)
	}
	var query protobuf.SC_27001
	recoveryPacket(t, client, educate.EducateRequest, &protobuf.CS_27000{Type: proto.Uint32(1)}, &query)
	found := false
	for _, task := range query.Child.Tasks {
		if task.GetId() == 2011 {
			found = true
			if task.GetProgress() != 40 {
				t.Fatal("false attribute task completion", task)
			}
		}
	}
	if !found || query.Child.GetHadTargetStageAward() != 0 {
		t.Fatal("missing outstanding target task")
	}
	migrated, err := orm.GetOrCreateLegacyEducateState(90001)
	if err != nil || !migrated.TargetAwards[1] || migrated.TargetAwards[12] {
		t.Fatal("historical target receipt mixed stages", migrated, err)
	}
	first, _ := orm.GetConfigEntry("Runtime/legacy_educate_state", "90001")
	recoveryPacket(t, client, educate.EducateRequest, &protobuf.CS_27000{Type: proto.Uint32(1)}, &query)
	repeat, _ := orm.GetConfigEntry("Runtime/legacy_educate_state", "90001")
	if !bytes.Equal(first.Data, repeat.Data) {
		t.Fatal("repeated query changed tasks")
	}
	var claim protobuf.SC_27024
	recoveryPacket(t, client, educate.EducateSubmitTask, &protobuf.CS_27023{Id: proto.Uint32(2011), System: proto.Uint32(2)}, &claim)
	if claim.GetResult() == 0 {
		t.Fatal("incomplete task claimed")
	}
	state, _ = orm.GetOrCreateLegacyEducateState(90001)
	state.Attrs[101], state.Attrs[102], state.Attrs[103], state.Attrs[104] = 150, 400, 150, 150
	state.PlanHistory = map[uint32]uint32{1107: 10, 1108: 5}
	if err := orm.SaveLegacyEducateState(state); err != nil {
		t.Fatal(err)
	}
	var award protobuf.SC_27036
	recoveryPacket(t, client, educate.EducateGetTargetAward, &protobuf.CS_27035{Type: proto.Uint32(0)}, &award)
	if award.GetResult() == 0 {
		t.Fatal("unclaimed tasks counted as target points")
	}
	for _, id := range []uint32{2011, 2012, 2013, 2014, 2015} {
		recoveryPacket(t, client, educate.EducateSubmitTask, &protobuf.CS_27023{Id: proto.Uint32(id), System: proto.Uint32(2)}, &claim)
		if claim.GetResult() != 0 || len(claim.Awards) != 1 || claim.Awards[0].GetType() != 3 || claim.Awards[0].GetId() != 302 {
			t.Fatal("real task reward", id, &claim)
		}
	}
	state, _ = orm.GetOrCreateLegacyEducateState(90001)
	if state.Items[302] != 5 || state.Attrs[102] != 400 || state.Resources[1] != 0 {
		t.Fatal("task points not persisted independently", state)
	}
	committed, _ := orm.GetConfigEntry("Runtime/legacy_educate_state", "90001")
	recoveryPacket(t, client, educate.EducateSubmitTask, &protobuf.CS_27023{Id: proto.Uint32(2011), System: proto.Uint32(2)}, &claim)
	after, _ := orm.GetConfigEntry("Runtime/legacy_educate_state", "90001")
	if claim.GetResult() == 0 || !bytes.Equal(committed.Data, after.Data) {
		t.Fatal("duplicate claim changed save")
	}
	recoveryPacket(t, client, educate.EducateGetTargetAward, &protobuf.CS_27035{Type: proto.Uint32(0)}, &award)
	state, _ = orm.GetOrCreateLegacyEducateState(90001)
	if award.GetResult() != 0 || len(award.Drops) != 1 || award.Drops[0].GetId() != 206 || state.Items[206] != 1 || state.Attrs[102] != 500 || state.Resources[1] != 100 || state.Resources[2] != 65 || !state.TargetAwards[12] {
		t.Fatal("actual target12 reward not persisted", state, &award)
	}
	committed, _ = orm.GetConfigEntry("Runtime/legacy_educate_state", "90001")
	recoveryPacket(t, client, educate.EducateGetTargetAward, &protobuf.CS_27035{Type: proto.Uint32(0)}, &award)
	after, _ = orm.GetConfigEntry("Runtime/legacy_educate_state", "90001")
	if award.GetResult() == 0 || !bytes.Equal(committed.Data, after.Data) {
		t.Fatal("duplicate stage reward changed save")
	}
	recoveryPacket(t, client, educate.EducateSubmitTask, &protobuf.CS_27023{Id: proto.Uint32(102), System: proto.Uint32(3)}, &claim)
	state, _ = orm.GetOrCreateLegacyEducateState(90001)
	if claim.GetResult() != 0 || state.Items[301] != 2 || state.Attrs[101] != 152 || state.Attrs[102] != 502 || state.Attrs[103] != 152 || state.Attrs[104] != 152 {
		t.Fatal("main medal immediate numeric effects", state, &claim)
	}
	recoveryPacket(t, client, educate.EducateRequest, &protobuf.CS_27000{Type: proto.Uint32(1)}, &query)
	if query.Child.GetHadTargetStageAward() != 1 || len(query.Child.Items) != 3 {
		t.Fatal("reward query mismatch", &query)
	}
	for _, task := range query.Child.Tasks {
		if task.GetId() == 2011 || task.GetId() == 102 {
			t.Fatal("claimed task respawned", task)
		}
	}
	after1, _ := loadAuditRole(90001, 1)
	after2, _ := loadAuditRole(90001, 2)
	for i, pair := range [][2]*auditRoleSnapshot{{before1, after1}, {before2, after2}} {
		if !bytes.Equal(pair[0].State, pair[1].State) || !bytes.Equal(pair[0].Permanent, pair[1].Permanent) || !bytes.Equal(pair[0].Metadata, pair[1].Metadata) || pair[0].Revision != pair[1].Revision {
			t.Fatal("old tasks changed new role", i+1)
		}
	}
}

func TestRecoveryL02StableMoodMixedWeek(t *testing.T) {
	client := recoveryDB(t)
	state, err := orm.GetOrCreateLegacyEducateState(90001)
	if err != nil {
		t.Fatal(err)
	}
	state.CurTime = &orm.LegacyEducateTime{Month: 4, Week: 2, Day: 7}
	state.Resources[1], state.Resources[2], state.Resources[3] = 0, 24, 2
	state.TargetID = 12
	state.Attrs[102] = 40
	if err := orm.SaveLegacyEducateState(state); err != nil {
		t.Fatal(err)
	}
	var snapshot protobuf.SC_27001
	recoveryPacket(t, client, educate.EducateRequest, &protobuf.CS_27000{Type: proto.Uint32(1)}, &snapshot)
	cells := make([]*protobuf.CHILD_PLAN_CELL, 0, 6)
	for day := uint32(1); day <= 6; day++ {
		plan := uint32(1402)
		if day == 1 {
			plan = 1404
		}
		cells = append(cells, &protobuf.CHILD_PLAN_CELL{Day: proto.Uint32(day), Index: proto.Uint32(1), Value: []*protobuf.CHILD_PLAN_VAL{{PlanId: proto.Uint32(plan)}}})
	}
	var prepare protobuf.SC_27013
	var execute protobuf.SC_27003
	recoveryPacket(t, client, educate.EducateGetPlans, &protobuf.CS_27012{Plans: cells}, &prepare)
	recoveryPacket(t, client, educate.EducateExecutePlans, &protobuf.CS_27002{Type: proto.Uint32(1)}, &execute)
	state, _ = orm.GetOrCreateLegacyEducateState(90001)
	if prepare.GetResult() != 0 || execute.GetResult() != 0 || state.Resources[1] != 12 || state.Resources[2] != 32 || state.Resources[3] != 2 || state.Attrs[102] != 40 || state.PlanHistory[1404] != 1 || state.PlanHistory[1402] != 5 || state.CurTime.Week != 3 {
		t.Fatal("actual stable -20% mixed week", state, &execute)
	}
	if execute.PlanResults[0].PlanDrops[0].GetNumber() != 12 {
		t.Fatal("unscaled money returned")
	}
	wire := client.Buffer.Bytes()
	added208, updated205 := false, false
	for offset := packets.GetPacketSize(0, &wire) + 2; offset < len(wire); {
		size := packets.GetPacketSize(offset, &wire) + 2
		switch packets.GetPacketId(offset, &wire) {
		case 27021:
			var added protobuf.SC_27021
			if err := proto.Unmarshal(wire[offset+packets.HEADER_SIZE:offset+size], &added); err != nil {
				t.Fatal(err)
			}
			for _, task := range added.Tasks {
				if task.GetId() == 208 && task.GetProgress() == 0 {
					added208 = true
				}
			}
		case 27025:
			var updated protobuf.SC_27025
			if err := proto.Unmarshal(wire[offset+packets.HEADER_SIZE:offset+size], &updated); err != nil {
				t.Fatal(err)
			}
			for _, task := range updated.Tasks {
				if task.GetId() == 205 && task.GetProgress() == 1 {
					updated205 = true
				}
			}
		}
		offset += size
	}
	if !added208 || !updated205 {
		t.Fatal("weekly task notifications missing", added208, updated205)
	}
	saved, _ := orm.GetConfigEntry("Runtime/legacy_educate_state", "90001")
	var repeat protobuf.SC_27003
	recoveryPacket(t, client, educate.EducateExecutePlans, &protobuf.CS_27002{Type: proto.Uint32(1)}, &repeat)
	unchanged, _ := orm.GetConfigEntry("Runtime/legacy_educate_state", "90001")
	if !bytes.Equal(saved.Data, unchanged.Data) || !proto.Equal(&execute, &repeat) {
		t.Fatal("mixed week replay changed save")
	}
	wire = client.Buffer.Bytes()
	if packets.GetPacketSize(0, &wire)+2 != len(wire) {
		t.Fatal("cached execution repeated task notifications")
	}
	// Initial handcraft has raw perception12: at this mood it requires unknown
	// 9.6 rounding. Sleep gains that could cross to another tier also stay refused.
	for _, scenario := range []struct {
		Mood int32
		Plan uint32
	}{{24, 1113}, {35, 1404}, {40, 1404}, {60, 1404}} {
		state, _ = orm.GetOrCreateLegacyEducateState(90001)
		state.Resources[2] = scenario.Mood
		if err := orm.SaveLegacyEducateState(state); err != nil {
			t.Fatal(err)
		}
		cells[0].Value[0].PlanId = proto.Uint32(scenario.Plan)
		saved, _ = orm.GetConfigEntry("Runtime/legacy_educate_state", "90001")
		recoveryPacket(t, client, educate.EducateGetPlans, &protobuf.CS_27012{Plans: cells}, &prepare)
		unchanged, _ = orm.GetConfigEntry("Runtime/legacy_educate_state", "90001")
		if prepare.GetResult() == 0 || !bytes.Equal(saved.Data, unchanged.Data) {
			t.Fatal("unresolved mood semantics accepted", scenario)
		}
	}
}

func TestRecoveryL03LegacyShopWalletAndCalendar(t *testing.T) {
	client := recoveryDB(t)
	if err := orm.SetCommanderCommonFlag(90001, 270270000+1104); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DefaultStore.Pool.Exec(context.Background(), `INSERT INTO resources (id,name) VALUES (1,'Gold')`); err != nil {
		t.Fatal(err)
	}
	for _, id := range []uint32{1, 2} {
		if _, err := loadEducateState(client, id); err != nil {
			t.Fatal(err)
		}
	}
	role1, _ := loadAuditRole(90001, 1)
	role2, _ := loadAuditRole(90001, 2)
	if err := client.Commander.Load(); err != nil {
		t.Fatal(err)
	}
	if err := client.Commander.SetResource(1, 500); err != nil {
		t.Fatal(err)
	}
	state, err := orm.GetOrCreateLegacyEducateState(90001)
	if err != nil {
		t.Fatal(err)
	}
	state.CurTime = &orm.LegacyEducateTime{Month: 4, Week: 4, Day: 7}
	state.Resources = map[uint32]int32{1: 60, 2: 28, 3: 2}
	state.Attrs[102] = 40
	if err := orm.SaveLegacyEducateState(state); err != nil {
		t.Fatal(err)
	}
	// Adopt a pre-existing, partially depleted wall-clock inventory unchanged.
	if err := orm.UpsertEducateShopState(&orm.EducateShopState{CommanderID: 90001, ShopID: 2, RefreshKey: 20732, Goods: []orm.EducateShopGoodsState{{ID: 2, Num: 0}, {ID: 3, Num: 1}, {ID: 4, Num: 1}, {ID: 5, Num: 1}, {ID: 11, Num: 1}}}); err != nil {
		t.Fatal(err)
	}
	query := &protobuf.CS_27043{ShopId: proto.Uint32(2)}
	var listed protobuf.SC_27044
	recoveryPacket(t, client, educate.EducateRequestShopData, query, &listed)
	if listed.GetResult() != 0 || len(listed.ShopData.Goods) != 5 || listed.ShopData.Goods[0].GetNum() != 0 {
		t.Fatal("migration refilled stock", &listed)
	}
	state, _ = orm.GetOrCreateLegacyEducateState(90001)
	if state.ShopCalendar[2].CompatRefreshKey != 20732 || state.ShopCalendar[2].Week != 16 {
		t.Fatal("calendar adoption", state.ShopCalendar)
	}
	buy := &protobuf.CS_27033{ShopId: proto.Uint32(2), Goods: []*protobuf.CHILD_SHOP_GOODS{{Id: proto.Uint32(11), Num: proto.Uint32(1)}}}
	var bought protobuf.SC_27034
	recoveryPacket(t, client, educate.EducateShopping, buy, &bought)
	state, _ = orm.GetOrCreateLegacyEducateState(90001)
	if bought.GetResult() != 0 || len(bought.Drops) != 1 || bought.Drops[0].GetType() != 3 || bought.Drops[0].GetId() != 11 || state.Resources[1] != 10 || state.Resources[2] != 28 || state.Attrs[101] != 10 || state.Attrs[103] != 10 || state.Attrs[102] != 40 || state.Items[11] != 1 {
		t.Fatal("educate cost and item gains", state, &bought)
	}
	if client.Commander.GetResourceCount(1) != 500 || client.Commander.GetItemCount(11) != 0 {
		t.Fatal("shop changed main-port inventory")
	}
	saved, _ := orm.GetConfigEntry("Runtime/legacy_educate_state", "90001")
	for _, invalid := range []*protobuf.CS_27033{buy, {ShopId: proto.Uint32(2), Goods: []*protobuf.CHILD_SHOP_GOODS{{Id: proto.Uint32(3), Num: proto.Uint32(1)}}}, {ShopId: proto.Uint32(22), Goods: []*protobuf.CHILD_SHOP_GOODS{{Id: proto.Uint32(1), Num: proto.Uint32(1)}}}, {ShopId: proto.Uint32(2), Goods: []*protobuf.CHILD_SHOP_GOODS{{Id: proto.Uint32(3), Num: proto.Uint32(math.MaxUint32)}}}} {
		recoveryPacket(t, client, educate.EducateShopping, invalid, &bought)
		after, _ := orm.GetConfigEntry("Runtime/legacy_educate_state", "90001")
		if bought.GetResult() == 0 || !bytes.Equal(saved.Data, after.Data) {
			t.Fatal("rejected purchase changed save")
		}
	}
	recoveryPacket(t, client, educate.EducateRequestShopData, query, &listed)
	if listed.ShopData.Goods[4].GetNum() != 0 || listed.ShopData.Goods[0].GetNum() != 0 {
		t.Fatal("same week restocked")
	}
	state.CurTime = &orm.LegacyEducateTime{Month: 5, Week: 1, Day: 7}
	if err := orm.SaveLegacyEducateState(state); err != nil {
		t.Fatal(err)
	}
	recoveryPacket(t, client, educate.EducateRequestShopData, query, &listed)
	if listed.ShopData.Goods[4].GetNum() != 1 || listed.ShopData.Goods[0].GetNum() != 1 {
		t.Fatal("game week failed to refresh")
	}
	after1, _ := loadAuditRole(90001, 1)
	after2, _ := loadAuditRole(90001, 2)
	if !bytes.Equal(role1.State, after1.State) || !bytes.Equal(role2.State, after2.State) || role1.Revision != after1.Revision || role2.Revision != after2.Revision {
		t.Fatal("legacy shop touched new roles")
	}
}

func TestRecoveryL03LegacyFixedWork(t *testing.T) {
	client := recoveryDB(t)
	state, err := orm.GetOrCreateLegacyEducateState(90001)
	if err != nil {
		t.Fatal(err)
	}
	state.CurTime = &orm.LegacyEducateTime{Month: 4, Week: 4, Day: 7}
	state.Resources = map[uint32]int32{1: 10, 2: 28, 3: 1}
	if err := orm.SaveLegacyEducateState(state); err != nil {
		t.Fatal(err)
	}
	var response protobuf.SC_27005
	request := &protobuf.CS_27004{Siteid: proto.Uint32(111), Optionid: proto.Uint32(1112)}
	recoveryPacket(t, client, educate.EducateMapSiteOperate, request, &response)
	state, _ = orm.GetOrCreateLegacyEducateState(90001)
	if response.GetResult() != 0 || response.GetBranchId() != 11120 || len(response.Drops) != 1 || response.Drops[0].GetNumber() != 40 || state.Resources[1] != 50 || state.Resources[2] != 27 || state.Resources[3] != 0 || state.OptionRecords[1112] != 1 {
		t.Fatal("fixed work costs/results", state, &response)
	}
	before, _ := orm.GetConfigEntry("Runtime/legacy_educate_state", "90001")
	for _, invalid := range []*protobuf.CS_27004{request, {Siteid: proto.Uint32(110), Optionid: proto.Uint32(1103)}} {
		recoveryPacket(t, client, educate.EducateMapSiteOperate, invalid, &response)
		after, _ := orm.GetConfigEntry("Runtime/legacy_educate_state", "90001")
		if response.GetResult() == 0 || !bytes.Equal(before.Data, after.Data) {
			t.Fatal("unsupported/repeated site charged resources")
		}
	}
	// An actual weekly settlement resets a count-limited option, but preserves
	// the old unlimited walk record. This fixture never touches the player save.
	state.OptionRecords[1103] = 1
	if err := orm.SaveLegacyEducateState(state); err != nil {
		t.Fatal(err)
	}
	var prepared protobuf.SC_27013
	cells := []*protobuf.CHILD_PLAN_CELL{}
	for day := uint32(1); day <= 6; day++ {
		cells = append(cells, &protobuf.CHILD_PLAN_CELL{Day: proto.Uint32(day), Index: proto.Uint32(1), Value: []*protobuf.CHILD_PLAN_VAL{{PlanId: proto.Uint32(1402)}}})
	}
	recoveryPacket(t, client, educate.EducateGetPlans, &protobuf.CS_27012{Plans: cells}, &prepared)
	if prepared.GetResult() != 0 {
		t.Fatal(&prepared)
	}
	var settled protobuf.SC_27003
	recoveryPacket(t, client, educate.EducateExecutePlans, &protobuf.CS_27002{Type: proto.Uint32(1)}, &settled)
	state, _ = orm.GetOrCreateLegacyEducateState(90001)
	if settled.GetResult() != 0 || state.CurTime.Month != 5 || state.CurTime.Week != 1 || state.OptionRecords[1112] != 0 || state.OptionRecords[1103] != 1 {
		t.Fatal("weekly option refresh", state, &settled)
	}
}
