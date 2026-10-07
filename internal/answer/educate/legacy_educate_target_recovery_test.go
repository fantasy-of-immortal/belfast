package educate_test

import (
	"bytes"
	"testing"

	"github.com/ggmolly/belfast/internal/answer/educate"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/packets"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestRecoveryL03TargetSelectionCalendarAndTasks(t *testing.T) {
	client := recoveryDB(t)
	for _, id := range []uint32{1, 2} {
		if _, err := loadEducateState(client, id); err != nil {
			t.Fatal(err)
		}
	}
	role1, _ := loadAuditRole(90001, 1)
	role2, _ := loadAuditRole(90001, 2)
	state, err := orm.GetOrCreateLegacyEducateState(90001)
	if err != nil {
		t.Fatal(err)
	}
	state.Attrs[102] = 40
	state.Resources[1], state.Resources[2] = 50, 27
	state.Items = map[uint32]uint32{11: 1}
	if err := orm.SaveLegacyEducateState(state); err != nil {
		t.Fatal(err)
	}
	var response protobuf.SC_27020
	selectTarget := func(id uint32) {
		t.Helper()
		recoveryPacket(t, client, educate.EducateSetTarget, &protobuf.CS_27019{Id: proto.Uint32(id)}, &response)
	}
	rejectUnchanged := func(id uint32) {
		t.Helper()
		before, err := orm.GetConfigEntry("Runtime/legacy_educate_state", "90001")
		if err != nil {
			t.Fatal(err)
		}
		selectTarget(id)
		after, err := orm.GetConfigEntry("Runtime/legacy_educate_state", "90001")
		if err != nil || response.GetResult() == 0 || !bytes.Equal(before.Data, after.Data) {
			t.Fatal("invalid selection changed save", id, err, &response)
		}
		wire := client.Buffer.Bytes()
		if packets.GetPacketSize(0, &wire)+2 != len(wire) {
			t.Fatal("failed selection emitted task notifications")
		}
	}
	rejectUnchanged(12) // Stage 2 cannot be chosen at the first selection day.
	selectTarget(2)
	state, err = orm.GetOrCreateLegacyEducateState(90001)
	if err != nil || response.GetResult() != 0 || state.TargetID != 2 || state.TaskProgress[1011] != 30 || state.Resources[1] != 50 || state.Resources[2] != 27 || state.Items[11] != 1 {
		t.Fatal("first target snapshot/rewards", state, err, &response)
	}
	rejectUnchanged(2)
	rejectUnchanged(1) // Cannot change a selected target within the same stage.
	state.CurTime = &orm.LegacyEducateTime{Month: 3, Week: 4, Day: 6}
	if err := orm.SaveLegacyEducateState(state); err != nil {
		t.Fatal(err)
	}
	rejectUnchanged(12)
	state.CurTime.Day = 7
	state.ClaimedTasks = map[uint32]bool{}
	state.TargetAwards = map[uint32]bool{}
	state.ClaimedTasks[1011] = true
	delete(state.TaskProgress, 1011)
	state.TargetAwards[2] = true
	if err := orm.SaveLegacyEducateState(state); err != nil {
		t.Fatal(err)
	}
	before, _ := orm.GetConfigEntry("Runtime/legacy_educate_state", "90001")
	// A bad new-task contract must roll back old-task removal and target ID.
	taskEntry, err := orm.GetConfigEntry("ShareCfg/child_task.json", "2011")
	if err != nil {
		t.Fatal(err)
	}
	if err := orm.UpsertConfigEntry(taskEntry.Category, taskEntry.Key, []byte(`{"id":2011,"type_1":2}`)); err != nil {
		t.Fatal(err)
	}
	rejectUnchanged(12)
	if err := orm.UpsertConfigEntry(taskEntry.Category, taskEntry.Key, taskEntry.Data); err != nil {
		t.Fatal(err)
	}
	selectTarget(12)
	state, err = orm.GetOrCreateLegacyEducateState(90001)
	if err != nil || response.GetResult() != 0 || state.TargetID != 12 || state.TaskProgress[2011] != 40 || !state.ClaimedTasks[1011] || !state.TargetAwards[2] || state.TargetAwards[12] || state.Items[11] != 1 || state.Resources[1] != 50 || state.Resources[2] != 27 {
		t.Fatal("next target persistence or receipt history", state, err, &response)
	}
	if _, exists := state.TaskProgress[1012]; exists {
		t.Fatal("previous target still active")
	}
	// Decode the actual asynchronous client messages, including removals.
	removedOld, addedNew := false, false
	wire := client.Buffer.Bytes()
	for offset := packets.GetPacketSize(0, &wire) + 2; offset < len(wire); {
		size := packets.GetPacketSize(offset, &wire) + 2
		body := wire[offset+packets.HEADER_SIZE : offset+size]
		switch packets.GetPacketId(offset, &wire) {
		case 27022:
			var removed protobuf.SC_27022
			if err := proto.Unmarshal(body, &removed); err != nil {
				t.Fatal(err)
			}
			for _, id := range removed.Ids {
				if id == 1012 {
					removedOld = true
				}
			}
		case 27021:
			var added protobuf.SC_27021
			if err := proto.Unmarshal(body, &added); err != nil {
				t.Fatal(err)
			}
			for _, task := range added.Tasks {
				if task.GetId() == 2011 && task.GetProgress() == 40 {
					addedNew = true
				}
			}
		}
		offset += size
	}
	if !removedOld || !addedNew {
		t.Fatal("selection notifications", removedOld, addedNew)
	}
	after, _ := orm.GetConfigEntry("Runtime/legacy_educate_state", "90001")
	if bytes.Equal(before.Data, after.Data) {
		t.Fatal("selection was not persisted")
	}
	rejectUnchanged(12)
	var query protobuf.SC_27001
	recoveryPacket(t, client, educate.EducateRequest, &protobuf.CS_27000{Type: proto.Uint32(1)}, &query)
	if query.Child.GetTarget() != 12 || query.Child.GetHadTargetStageAward() != 0 {
		t.Fatal("reloaded target or award leaked")
	}
	afterQuery, _ := orm.GetConfigEntry("Runtime/legacy_educate_state", "90001")
	if !bytes.Equal(after.Data, afterQuery.Data) {
		t.Fatal("query changed committed target")
	}
	state, _ = orm.GetOrCreateLegacyEducateState(90001)
	state.CurTime = &orm.LegacyEducateTime{Month: 5, Week: 4, Day: 7}
	state.Attrs[306], state.Attrs[304] = 11, 9
	state.Attrs[101], state.Attrs[104], state.Attrs[103] = 300, 450, 350
	if err := orm.SaveLegacyEducateState(state); err != nil {
		t.Fatal(err)
	}
	selectTarget(22)
	if response.GetResult() != 0 {
		t.Fatal("third-stage real target rejected")
	}
	state, _ = orm.GetOrCreateLegacyEducateState(90001)
	if state.TaskProgress[3019] != 11 || state.TaskProgress[3020] != 450 {
		t.Fatal("alternative attribute tasks summed rather than taking maximum", state.TaskProgress)
	}
	state.CurTime = &orm.LegacyEducateTime{Month: 9, Week: 4, Day: 7}
	if err := orm.SaveLegacyEducateState(state); err != nil {
		t.Fatal(err)
	}
	rejectUnchanged(31) // Conditional later candidates remain explicitly unsupported.
	for i, pair := range [][2]*auditRoleSnapshot{{role1, mustRecoveryRole(t, 1)}, {role2, mustRecoveryRole(t, 2)}} {
		if !bytes.Equal(pair[0].State, pair[1].State) || !bytes.Equal(pair[0].Permanent, pair[1].Permanent) || !bytes.Equal(pair[0].Metadata, pair[1].Metadata) || pair[0].Revision != pair[1].Revision {
			t.Fatal("target selection changed new role", i+1)
		}
	}
}

func mustRecoveryRole(t *testing.T, id uint32) *auditRoleSnapshot {
	t.Helper()
	role, err := loadAuditRole(90001, id)
	if err != nil {
		t.Fatal(err)
	}
	return role
}
