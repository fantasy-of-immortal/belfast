package educate

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func legacyTaskChanges(before, after map[uint32]uint32) (added, updated []*protobuf.CHILD_TASK) {
	ids := make([]uint32, 0, len(after))
	for id := range after {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		task := &protobuf.CHILD_TASK{Id: proto.Uint32(id), Progress: proto.Uint32(after[id])}
		previous, exists := before[id]
		if !exists {
			added = append(added, task)
		} else if previous != after[id] {
			updated = append(updated, task)
		}
	}
	return
}

func sendLegacyTaskChanges(client *connection.Client, added, updated []*protobuf.CHILD_TASK) error {
	if len(added) != 0 {
		if _, _, err := client.SendMessage(27021, &protobuf.SC_27021{Tasks: added}); err != nil {
			return err
		}
	}
	if len(updated) != 0 {
		if _, _, err := client.SendMessage(27025, &protobuf.SC_27025{Tasks: updated}); err != nil {
			return err
		}
	}
	return nil
}

func legacyTaskConfigs() (map[uint32]legacyChildTask, error) {
	entries, err := orm.ListConfigEntries(childTaskCategory)
	if err != nil {
		return nil, err
	}
	rows, err := parseConfigEntries[legacyChildTask](entries)
	if err != nil {
		return nil, err
	}
	result := make(map[uint32]legacyChildTask, len(rows))
	for _, row := range rows {
		result[row.ID] = row
	}
	return result, nil
}

func legacyTaskInTime(task legacyChildTask, date orm.LegacyEducateTime) bool {
	if len(task.TimeLimit) != 2 {
		return false
	}
	position := func(parts []uint32, end bool) uint64 {
		week, day := uint32(1), uint32(1)
		if end {
			week, day = 4, 7
		}
		if len(parts) > 1 {
			week = parts[1]
		}
		if len(parts) > 2 {
			day = parts[2]
		}
		if len(parts) == 0 {
			return 0
		}
		return (uint64(parts[0])*4+uint64(week))*7 + uint64(day)
	}
	current := (uint64(date.Month)*4+uint64(date.Week))*7 + uint64(date.Day)
	return current >= position(task.TimeLimit[0], false) && current <= position(task.TimeLimit[1], true)
}

func legacyTaskSubIDs(raw json.RawMessage) ([]uint32, error) {
	var ids []uint32
	if err := json.Unmarshal(raw, &ids); err == nil {
		return ids, nil
	}
	var scalar string
	if err := json.Unmarshal(raw, &scalar); err != nil {
		return nil, err
	}
	id, err := strconv.ParseUint(scalar, 10, 32)
	return []uint32{uint32(id)}, err
}

// Missing tasks mean "received" to the client. Old saves with an empty map
// have no evidence of receipt, so retain explicit outstanding tasks instead.
// Do not manufacture performance/site/shop history absent from the save.
func syncLegacyTasks(state *orm.LegacyEducateState) error {
	configs, err := legacyTaskConfigs()
	if err != nil {
		return err
	}
	if state.ClaimedTasks == nil {
		state.ClaimedTasks = map[uint32]bool{}
	}
	if state.TargetAwards == nil {
		state.TargetAwards = map[uint32]bool{}
	}
	if state.TaskSnapshotVersion == 0 {
		flags, err := orm.ListCommanderCommonFlags(state.CommanderID)
		if err != nil {
			return err
		}
		for _, flag := range flags {
			if flag >= educateFlagTargetAwardBase && flag < educateFlagTargetAwardBase+100000 {
				state.TargetAwards[flag-educateFlagTargetAwardBase] = true
			}
		}
	}
	selected := map[uint32]bool{}
	if state.TargetID != 0 {
		entry, err := orm.GetConfigEntry(childTargetSetCategory, strconv.FormatUint(uint64(state.TargetID), 10))
		if err != nil {
			return err
		}
		var target struct {
			IDs []uint32 `json:"ids"`
		}
		if err := json.Unmarshal(entry.Data, &target); err != nil {
			return err
		}
		for _, id := range target.IDs {
			selected[id] = true
		}
	}
	for id, task := range configs {
		_, saved := state.TaskProgress[id]
		if state.ClaimedTasks[id] {
			delete(state.TaskProgress, id)
			continue
		}
		if !saved && !selected[id] && !(task.Type1 == 3 && legacyTaskInTime(task, *state.CurTime)) {
			continue
		}
		progress := uint64(state.TaskProgress[id])
		ids, err := legacyTaskSubIDs(task.SubType)
		if err != nil {
			return fmt.Errorf("legacy task %d subtype: %w", id, err)
		}
		switch task.Type2 {
		case 1: // Course history is written by the committed weekly transaction.
			if !selected[id] {
				break
			} // Mind-task activation/history windows are not restored yet.
			progress = 0
			if len(ids) == 1 && ids[0] == 0 {
				for _, count := range state.PlanHistory {
					progress += uint64(count)
				}
			} else {
				for _, plan := range ids {
					progress += uint64(state.PlanHistory[plan])
				}
			}
		case 2:
			if len(ids) == 0 {
				return fmt.Errorf("legacy task %d attribute", id)
			}
			// The eight configured multi-attribute tasks explicitly say "or".
			// A threshold on any listed attribute is the maximum, not a sum.
			progress = 0
			for _, attr := range ids {
				value, exists := state.Attrs[attr]
				if !exists {
					return fmt.Errorf("legacy task %d unknown attribute %d", id, attr)
				}
				if uint64(value) > progress {
					progress = uint64(value)
				}
			}
		case 6:
			if state.TargetID != 0 {
				progress = 1
			}
		case 10:
			// Only the latest persisted settlement has a trustworthy date.
			if state.Settlement != nil && legacyTaskInTime(task, state.Settlement.From) && progress == 0 {
				progress = 1
			}
		}
		if progress > uint64(task.Arg) {
			progress = uint64(task.Arg)
		}
		state.TaskProgress[id] = uint32(progress)
	}
	state.TaskSnapshotVersion = 1
	return nil
}

func legacyTaskSnapshot(state *orm.LegacyEducateState) ([]*protobuf.CHILD_TASK, error) {
	if err := syncLegacyTasks(state); err != nil {
		return nil, err
	}
	ids := make([]uint32, 0, len(state.TaskProgress))
	for id := range state.TaskProgress {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	tasks := make([]*protobuf.CHILD_TASK, 0, len(ids))
	for _, id := range ids {
		tasks = append(tasks, &protobuf.CHILD_TASK{Id: proto.Uint32(id), Progress: proto.Uint32(state.TaskProgress[id])})
	}
	return tasks, nil
}

// Educate drop types have their own resources/items, separate from main-port
// inventory. Item display is the client's immediate passive numeric gain.
func applyLegacyTaskDrop(state *orm.LegacyEducateState, drop []uint32) error {
	if len(drop) != 3 || drop[2] == 0 {
		return fmt.Errorf("unsupported legacy task reward")
	}
	id, amount := drop[1], uint64(drop[2])
	switch drop[0] {
	case 1:
		if _, exists := state.Attrs[id]; !exists {
			return fmt.Errorf("unknown legacy attribute %d", id)
		}
		value := uint64(state.Attrs[id]) + amount
		if value > math.MaxUint32 {
			return fmt.Errorf("legacy attribute overflow")
		}
		state.Attrs[id] = uint32(value)
	case 2:
		entry, err := orm.GetConfigEntry("ShareCfg/child_resource.json", strconv.FormatUint(uint64(id), 10))
		if err != nil {
			return err
		}
		var bounds struct {
			Min uint32 `json:"min_value"`
			Max uint32 `json:"max_value"`
		}
		if err := json.Unmarshal(entry.Data, &bounds); err != nil {
			return err
		}
		var old uint64
		if id == 4 {
			old = uint64(state.FavorExp)
		} else if id >= 1 && id <= 3 && state.Resources[id] >= 0 {
			old = uint64(state.Resources[id])
		} else {
			return fmt.Errorf("unsupported legacy resource %d", id)
		}
		value := old + amount
		if bounds.Max < bounds.Min || bounds.Max > math.MaxInt32 {
			return fmt.Errorf("invalid legacy resource limits")
		}
		if value > uint64(bounds.Max) {
			value = uint64(bounds.Max)
		}
		if value < uint64(bounds.Min) {
			value = uint64(bounds.Min)
		}
		if id == 4 {
			state.FavorExp = uint32(value)
		} else {
			state.Resources[id] = int32(value)
		}
	case 3:
		entry, err := orm.GetConfigEntry("ShareCfg/child_item.json", strconv.FormatUint(uint64(id), 10))
		if err != nil {
			return err
		}
		var item struct {
			Display [][]uint32 `json:"display"`
		}
		if err := json.Unmarshal(entry.Data, &item); err != nil {
			return err
		}
		if state.Items == nil {
			state.Items = map[uint32]uint32{}
		}
		value := uint64(state.Items[id]) + amount
		if value > math.MaxUint32 {
			return fmt.Errorf("legacy item overflow")
		}
		for _, effect := range item.Display {
			if len(effect) != 3 || (effect[0] != 1 && effect[0] != 2) || uint64(effect[2])*amount > math.MaxUint32 {
				return fmt.Errorf("unsupported legacy item effect")
			}
			if err := applyLegacyTaskDrop(state, []uint32{effect[0], effect[1], uint32(uint64(effect[2]) * amount)}); err != nil {
				return err
			}
		}
		state.Items[id] = uint32(value)
	default:
		return fmt.Errorf("unsupported legacy task drop type %d", drop[0])
	}
	return nil
}
