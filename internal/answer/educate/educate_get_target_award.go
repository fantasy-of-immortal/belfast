package educate

import (
	"encoding/json"
	"fmt"
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
	"strconv"
)

func EducateGetTargetAward(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_27035
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 27036, err
	}
	response := &protobuf.SC_27036{Result: proto.Uint32(educateResultFailed), Drops: []*protobuf.CHILD_DROP{}}
	if client.Commander == nil || payload.GetType() != 0 {
		return client.SendMessage(27036, response)
	}
	var drop []uint32
	err := orm.UpdateLegacyEducateState(client.Commander.CommanderID, func(state *orm.LegacyEducateState) error {
		if err := syncLegacyTasks(state); err != nil {
			return err
		}
		if state.TargetID == 0 || state.TargetAwards[state.TargetID] {
			return fmt.Errorf("legacy target award unavailable")
		}
		entry, err := orm.GetConfigEntry(childTargetSetCategory, strconv.FormatUint(uint64(state.TargetID), 10))
		if err != nil {
			return err
		}
		var target struct {
			IDs      []uint32 `json:"ids"`
			Progress uint32   `json:"target_progress"`
			Drop     []uint32 `json:"drop_display"`
		}
		if err := json.Unmarshal(entry.Data, &target); err != nil {
			return err
		}
		tasks, err := legacyTaskConfigs()
		if err != nil {
			return err
		}
		progress := uint64(0)
		for _, id := range target.IDs {
			task, known := tasks[id]
			if !known || !legacyTaskInTime(task, *state.CurTime) {
				return fmt.Errorf("legacy target task unavailable")
			}
			if state.ClaimedTasks[id] {
				progress += uint64(task.TargetProgress)
			}
		}
		if target.Progress == 0 || progress < uint64(target.Progress) {
			return fmt.Errorf("legacy target progress insufficient")
		}
		if err := applyLegacyTaskDrop(state, target.Drop); err != nil {
			return err
		}
		state.TargetAwards[state.TargetID] = true
		drop = target.Drop
		return nil
	})
	if err != nil {
		return client.SendMessage(27036, response)
	}
	response.Result = proto.Uint32(educateResultOK)
	response.Drops = append(response.Drops, buildLegacyChildDrop(drop[0], drop[1], int32(drop[2])))
	return client.SendMessage(27036, response)
}
