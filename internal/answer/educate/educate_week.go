package educate

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

type legacyWeekConfig struct {
	Stage     [][][]uint32 `json:"stage"`
	PlanCount []uint32     `json:"stage_plan_number"`
	SiteCount []uint32     `json:"stage_site_number"`
}

// Old snapshots served a fixed date even after selecting a later-stage
// target. Align only that labelled compatibility date, preserving its value.
// This does not invent a historical date or replace a real saved date.
func alignLegacyCompatibilityDate(state *orm.LegacyEducateState) error {
	if state.DateOrigin != "compat.last-served-27001" || *state.CurTime != (orm.LegacyEducateTime{Month: 2, Week: 4, Day: 7}) || state.TargetID == 0 {
		return nil
	}
	entry, err := orm.GetConfigEntry("ShareCfg/child_target_set.json", strconv.FormatUint(uint64(state.TargetID), 10))
	if err != nil {
		return err
	}
	var target struct {
		Stage uint32 `json:"stage"`
	}
	if err := json.Unmarshal(entry.Data, &target); err != nil {
		return err
	}
	if target.Stage <= 1 {
		return nil
	}
	entry, err = orm.GetConfigEntry("ShareCfg/gameset.json", "child_target_set_date")
	if err != nil {
		return err
	}
	var dates struct {
		Description [][]uint32 `json:"description"`
	}
	if err := json.Unmarshal(entry.Data, &dates); err != nil {
		return err
	}
	if int(target.Stage) > len(dates.Description) || len(dates.Description[target.Stage-1]) != 3 {
		return fmt.Errorf("missing legacy target selection date")
	}
	date := dates.Description[target.Stage-1]
	err = orm.UpdateLegacyEducateState(state.CommanderID, func(saved *orm.LegacyEducateState) error {
		if saved.DateOrigin != state.DateOrigin || *saved.CurTime != *state.CurTime || saved.TargetID != state.TargetID {
			return fmt.Errorf("legacy compatibility date changed during alignment")
		}
		previous := *saved.CurTime
		saved.CompatDateBeforeAlignment = &previous
		saved.CurTime = &orm.LegacyEducateTime{Month: date[0], Week: date[1], Day: date[2]}
		saved.DateOrigin = "compat.target-selection-date"
		return nil
	})
	if err != nil {
		return err
	}
	updated, err := orm.GetOrCreateLegacyEducateState(state.CommanderID)
	if err == nil {
		*state = *updated
	}
	return err
}

func nextLegacyWeek(current orm.LegacyEducateTime) orm.LegacyEducateTime {
	current.Week++
	if current.Week > 4 {
		current.Week = 1
		current.Month++
	}
	return current
}

func legacyWeekStage(date orm.LegacyEducateTime) (*legacyWeekConfig, int, error) {
	entry, err := orm.GetConfigEntry("ShareCfg/child_data.json", "1")
	if err != nil {
		return nil, 0, err
	}
	var cfg legacyWeekConfig
	if err := json.Unmarshal(entry.Data, &cfg); err != nil {
		return nil, 0, err
	}
	position := date.Month*4 + date.Week
	for i, span := range cfg.Stage {
		if len(span) != 2 || len(span[0]) != 2 || len(span[1]) != 2 {
			return nil, 0, fmt.Errorf("invalid legacy stage calendar")
		}
		if position >= span[0][0]*4+span[0][1] && position <= span[1][0]*4+span[1][1] {
			if i >= len(cfg.PlanCount) || i >= len(cfg.SiteCount) {
				return nil, 0, fmt.Errorf("missing legacy stage counts")
			}
			return &cfg, i, nil
		}
	}
	return nil, 0, fmt.Errorf("legacy next week outside restored calendar")
}

// Plans target the next week. cost_resource3 is occupied grid width;
// EducateGrid.GetOccupyGridCnt and ReduceResForPlans do not charge site.
func validateLegacyWeek(state *orm.LegacyEducateState, cells []orm.LegacyEducatePlanCell) ([]*educatePlanResultConfig, int64, int64, error) {
	if len(cells) == 0 {
		return nil, 0, 0, fmt.Errorf("empty legacy week")
	}
	cfg, stage, err := legacyWeekStage(nextLegacyWeek(*state.CurTime))
	if err != nil {
		return nil, 0, 0, err
	}
	occupied := map[[2]int32]bool{}
	plans := make([]*educatePlanResultConfig, 0, len(cells))
	var money, mood int64
	for _, cell := range cells {
		if cell.EventID != 0 || cell.SpecEventID != 0 || cell.PlanID == 0 {
			return nil, 0, 0, fmt.Errorf("legacy event settlement not restored")
		}
		plan, ok, err := loadChildPlanConfig(cell.PlanID)
		if err != nil {
			return nil, 0, 0, err
		}
		if !ok {
			return nil, 0, 0, fmt.Errorf("unknown legacy plan %d", cell.PlanID)
		}
		if cell.Day < 1 || cell.Day > 6 || cell.Index < 1 || plan.CostResource3 < 1 || uint64(cell.Index)+uint64(plan.CostResource3)-1 > uint64(cfg.PlanCount[stage]) {
			return nil, 0, 0, fmt.Errorf("legacy plan outside open slots")
		}
		available := len(plan.Stage) == 0
		for _, value := range plan.Stage {
			if value == uint32(stage+1) {
				available = true
			}
		}
		if !available {
			return nil, 0, 0, fmt.Errorf("legacy plan stage locked")
		}
		available = false
		for _, time := range plan.Time {
			if len(time) == 2 && time[0] == uint32(cell.Day) && time[1] == uint32(cell.Index) {
				available = true
			}
		}
		if !available {
			return nil, 0, 0, fmt.Errorf("legacy plan slot locked")
		}
		for _, condition := range plan.Ability {
			if len(condition) != 3 || condition[0] != 1 || state.Attrs[condition[1]] < condition[2] {
				return nil, 0, 0, fmt.Errorf("legacy plan ability locked")
			}
		}
		if len(plan.Pre) != 0 && (len(plan.Pre) != 2 || state.PlanHistory[plan.Pre[0]] < plan.Pre[1]) {
			return nil, 0, 0, fmt.Errorf("legacy plan predecessor locked")
		}
		for index := cell.Index; index < cell.Index+int32(plan.CostResource3); index++ {
			key := [2]int32{cell.Day, index}
			if occupied[key] {
				return nil, 0, 0, fmt.Errorf("overlapping legacy plan slots")
			}
			occupied[key] = true
		}
		var drops [][]int32
		if err := json.Unmarshal(plan.ResultDisplay, &drops); err != nil {
			return nil, 0, 0, err
		}
		for _, drop := range drops {
			if len(drop) != 3 || drop[1] <= 0 || (drop[0] != 1 && drop[0] != 2) {
				return nil, 0, 0, fmt.Errorf("legacy plan reward type not restored")
			}
			if drop[0] == 1 {
				if _, exists := state.Attrs[uint32(drop[1])]; !exists {
					return nil, 0, 0, fmt.Errorf("unknown legacy attribute")
				}
			} else if drop[1] < 1 || drop[1] > 3 {
				return nil, 0, 0, fmt.Errorf("legacy favor reward not restored")
			}
		}
		money += int64(plan.CostResource1)
		mood += int64(plan.CostResource2)
		plans = append(plans, plan)
	}
	if money > int64(state.Resources[1]) || mood > int64(state.Resources[2]) {
		return nil, 0, 0, fmt.Errorf("insufficient legacy week resources")
	}
	if err := scaleStableLegacyWeek(state, plans, mood); err != nil {
		return nil, 0, 0, err
	}
	return plans, money, mood, nil
}

// Restrict recovery to weeks whose reward multiplier is independent of
// fee timing and cell ordering. Fractional rounding and tier crossings still
// need evidence; neither the client preview nor result_display resolves them.
func scaleStableLegacyWeek(state *orm.LegacyEducateState, plans []*educatePlanResultConfig, moodCost int64) error {
	load := func(key string, out any) error {
		entry, err := orm.GetConfigEntry("ShareCfg/gameset.json", key)
		if err != nil {
			return err
		}
		return json.Unmarshal(entry.Data, out)
	}
	var attributes, resources struct {
		Description []int32 `json:"description"`
	}
	if err := load("child_emotion_attr", &attributes); err != nil {
		return err
	}
	if err := load("child_emotion_resource", &resources); err != nil {
		return err
	}
	contains := func(ids []int32, id int32) bool {
		for _, value := range ids {
			if value == id {
				return true
			}
		}
		return false
	}
	affected := func(drop []int32) bool {
		return drop[2] > 0 && ((drop[0] == 1 && contains(attributes.Description, drop[1])) || (drop[0] == 2 && contains(resources.Description, drop[1])))
	}
	minimum, maximum := int64(state.Resources[2])-moodCost, int64(state.Resources[2])
	rows := make([][][]int32, len(plans))
	needsScale := false
	for i, plan := range plans {
		if err := json.Unmarshal(plan.ResultDisplay, &rows[i]); err != nil {
			return err
		}
		for _, drop := range rows[i] {
			if affected(drop) {
				needsScale = true
			}
			if drop[0] == 2 && drop[1] == 2 {
				if drop[2] < 0 {
					minimum += int64(drop[2])
				} else {
					maximum += int64(drop[2])
				}
			}
		}
	}
	if !needsScale {
		return nil
	}
	if minimum < 0 {
		minimum = 0
	}
	if maximum > 100 {
		maximum = 100
	}
	var emotion struct {
		Description [][]json.RawMessage `json:"description"`
	}
	if err := load("child_emotion", &emotion); err != nil {
		return err
	}
	factor := int64(0)
	for _, row := range emotion.Description {
		if len(row) != 2 {
			return fmt.Errorf("invalid legacy emotion config")
		}
		var span []int64
		var change int64
		if err := json.Unmarshal(row[0], &span); err != nil {
			return err
		}
		if err := json.Unmarshal(row[1], &change); err != nil {
			return err
		}
		if len(span) != 2 {
			return fmt.Errorf("invalid legacy emotion interval")
		}
		// 20/40/60 boundaries differ between client ranges and descriptions.
		if (minimum > span[0] || (span[0] == 0 && minimum == 0)) && maximum < span[1] {
			factor = 10000 + change
			break
		}
	}
	if factor <= 0 {
		return fmt.Errorf("legacy mood tier crossing or boundary not restored")
	}
	for i, drops := range rows {
		for _, drop := range drops {
			if !affected(drop) {
				continue
			}
			value := int64(drop[2]) * factor
			if value%10000 != 0 || value/10000 > math.MaxInt32 {
				return fmt.Errorf("legacy fractional mood reward not restored")
			}
			drop[2] = int32(value / 10000)
		}
		encoded, err := json.Marshal(drops)
		if err != nil {
			return err
		}
		plans[i].ResultDisplay = encoded
	}
	return nil
}

func prepareLegacyWeek(state *orm.LegacyEducateState, cells []orm.LegacyEducatePlanCell) error {
	sort.Slice(cells, func(i, j int) bool {
		if cells[i].Day == cells[j].Day {
			return cells[i].Index < cells[j].Index
		}
		return cells[i].Day < cells[j].Day
	})
	if _, _, _, err := validateLegacyWeek(state, cells); err != nil {
		return err
	}
	if len(state.WeekPlans) > 0 {
		if reflect.DeepEqual(state.WeekPlans, cells) {
			return nil
		}
		return fmt.Errorf("legacy week already pending")
	}
	if state.PlanVersion == math.MaxUint32 {
		return fmt.Errorf("legacy plan version exhausted")
	}
	state.PlanVersion++
	state.WeekPlans = cells
	return nil
}

func settleLegacyWeek(state *orm.LegacyEducateState) (*protobuf.SC_27003, error) {
	if len(state.WeekPlans) == 0 {
		if state.Settlement == nil || state.Settlement.Version != state.PlanVersion {
			return nil, fmt.Errorf("no legacy week to execute")
		}
		response := &protobuf.SC_27003{}
		if err := proto.Unmarshal(state.Settlement.Response, response); err != nil {
			return nil, err
		}
		return response, nil
	}
	plans, money, mood, err := validateLegacyWeek(state, state.WeekPlans)
	if err != nil {
		return nil, err
	}
	response := &protobuf.SC_27003{Result: proto.Uint32(0), Events: []*protobuf.CHILD_PLAN_CELL{}}
	for day := uint32(1); day <= 6; day++ {
		for index := uint32(1); index <= 3; index++ {
			response.PlanResults = append(response.PlanResults, &protobuf.CHILD_PLAN_RESULT{Day: proto.Uint32(day), Index: proto.Uint32(index)})
		}
	}
	state.Resources[1] -= int32(money)
	state.Resources[2] -= int32(mood)
	if state.PlanHistory == nil {
		state.PlanHistory = map[uint32]uint32{}
	}
	for i, cell := range state.WeekPlans {
		var drops [][]int32
		if err := json.Unmarshal(plans[i].ResultDisplay, &drops); err != nil {
			return nil, err
		}
		for _, drop := range drops {
			id := uint32(drop[1])
			value := int64(drop[2])
			if drop[0] == 1 {
				value += int64(state.Attrs[id])
				if value < 0 || value > math.MaxUint32 {
					return nil, fmt.Errorf("legacy attribute out of range")
				}
				state.Attrs[id] = uint32(value)
			} else {
				value += int64(state.Resources[id])
				entry, err := orm.GetConfigEntry("ShareCfg/child_resource.json", strconv.FormatUint(uint64(id), 10))
				if err != nil {
					return nil, err
				}
				var bounds struct {
					Min int64 `json:"min_value"`
					Max int64 `json:"max_value"`
				}
				if err := json.Unmarshal(entry.Data, &bounds); err != nil {
					return nil, err
				}
				if bounds.Min < 0 || bounds.Max < bounds.Min || bounds.Max > math.MaxInt32 {
					return nil, fmt.Errorf("invalid legacy resource limits")
				}
				if value < bounds.Min {
					value = bounds.Min
				}
				if value > bounds.Max {
					value = bounds.Max
				}
				state.Resources[id] = int32(value)
			}
			result := response.PlanResults[(cell.Day-1)*3+cell.Index-1]
			result.PlanDrops = append(result.PlanDrops, &protobuf.CHILD_DROP{Type: proto.Uint32(uint32(drop[0])), Id: proto.Uint32(id), Number: proto.Int32(drop[2])})
		}
		if state.PlanHistory[cell.PlanID] == math.MaxUint32 {
			return nil, fmt.Errorf("legacy plan history exhausted")
		}
		state.PlanHistory[cell.PlanID]++
	}
	if err := syncLegacyTasks(state); err != nil {
		return nil, err
	}
	tasks, err := legacyTaskConfigs()
	if err != nil {
		return nil, err
	}
	for id, task := range tasks {
		if task.Type2 != 10 || state.ClaimedTasks[id] || !legacyTaskInTime(task, *state.CurTime) {
			continue
		}
		progress := state.TaskProgress[id]
		if progress < task.Arg {
			state.TaskProgress[id] = progress + 1
		}
	}
	from := *state.CurTime
	next := nextLegacyWeek(from)
	cfg, stage, err := legacyWeekStage(next)
	if err != nil {
		return nil, err
	}
	state.CurTime = &next
	if err := refreshLegacyOptionCounts(state, from, next); err != nil {
		return nil, err
	}
	state.Resources[3] = int32(cfg.SiteCount[stage])
	state.WeekPlans = []orm.LegacyEducatePlanCell{}
	encoded, err := proto.Marshal(response)
	if err != nil {
		return nil, err
	}
	state.Settlement = &orm.LegacyEducateSettlement{Version: state.PlanVersion, From: from, Response: encoded}
	return response, nil
}
