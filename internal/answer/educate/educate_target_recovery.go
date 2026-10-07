package educate

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// CheckTargetSet uses exact game dates; the target layer offers stage 1
// initially and the next stage thereafter. Later personality-dependent
// lists still need their candidate ordering contract and are not guessed.
func selectLegacyTarget(state *orm.LegacyEducateState, targetID uint32) ([]uint32, []*protobuf.CHILD_TASK, []*protobuf.CHILD_TASK, error) {
	target, ok, err := loadLegacyConfigByID[legacyChildTargetSet](childTargetSetCategory, targetID)
	if err != nil {
		return nil, nil, nil, err
	}
	if !ok || target.ID != targetID || target.Stage == 0 || len(target.IDs) == 0 {
		return nil, nil, nil, fmt.Errorf("invalid legacy target %d", targetID)
	}
	var condition [][]uint32
	var text string
	if err := json.Unmarshal(target.Condition, &condition); err != nil {
		if err := json.Unmarshal(target.Condition, &text); err != nil || text != "" {
			return nil, nil, nil, fmt.Errorf("legacy target %d condition not restored", targetID)
		}
	}
	if len(condition) != 0 {
		return nil, nil, nil, fmt.Errorf("legacy target %d conditional candidate list not restored", targetID)
	}
	entry, err := orm.GetConfigEntry("ShareCfg/gameset.json", "child_target_set_date")
	if err != nil {
		return nil, nil, nil, err
	}
	var dates struct {
		Description [][]uint32 `json:"description"`
	}
	if err := json.Unmarshal(entry.Data, &dates); err != nil {
		return nil, nil, nil, err
	}
	if int(target.Stage) > len(dates.Description) {
		return nil, nil, nil, fmt.Errorf("legacy target selection date missing")
	}
	date := dates.Description[target.Stage-1]
	if len(date) != 3 || *state.CurTime != (orm.LegacyEducateTime{Month: date[0], Week: date[1], Day: date[2]}) || len(state.WeekPlans) != 0 {
		return nil, nil, nil, fmt.Errorf("legacy target selection not open")
	}
	previousStage := uint32(0)
	if state.TargetID != 0 {
		previous, ok, err := loadLegacyConfigByID[legacyChildTargetSet](childTargetSetCategory, state.TargetID)
		if err != nil {
			return nil, nil, nil, err
		}
		if !ok || previous.Stage == 0 {
			return nil, nil, nil, fmt.Errorf("invalid saved legacy target")
		}
		previousStage = previous.Stage
	}
	if target.Stage != previousStage+1 {
		return nil, nil, nil, fmt.Errorf("legacy target stage already selected or skipped")
	}
	configs, err := legacyTaskConfigs()
	if err != nil {
		return nil, nil, nil, err
	}
	// Selection is on the preceding Sunday; target tasks open Monday.
	start := nextLegacyWeek(*state.CurTime)
	start.Day = 1
	for _, id := range target.IDs {
		if task, ok := configs[id]; !ok || task.Type1 != 2 || !legacyTaskInTime(task, start) {
			return nil, nil, nil, fmt.Errorf("legacy target %d task %d unavailable", targetID, id)
		}
	}
	before := make(map[uint32]uint32, len(state.TaskProgress))
	for id, progress := range state.TaskProgress {
		before[id] = progress
	}
	// Historical receipt and rewards stay intact. Only outstanding tasks of
	// the old target leave the active snapshot; main and mind tasks stay.
	selected := make(map[uint32]bool, len(target.IDs))
	for _, id := range target.IDs {
		selected[id] = true
	}
	var removed []uint32
	for id := range state.TaskProgress {
		if task, ok := configs[id]; ok && task.Type1 == 2 && !selected[id] {
			delete(state.TaskProgress, id)
			removed = append(removed, id)
		}
	}
	state.TargetID = targetID
	if err := syncLegacyTasks(state); err != nil {
		return nil, nil, nil, err
	}
	sort.Slice(removed, func(i, j int) bool { return removed[i] < removed[j] })
	added, updated := legacyTaskChanges(before, state.TaskProgress)
	return removed, added, updated, nil
}
