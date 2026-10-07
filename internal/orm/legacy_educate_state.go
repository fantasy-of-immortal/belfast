package orm

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/ggmolly/belfast/internal/db"
	pb "github.com/ggmolly/belfast/internal/protobuf"
	"github.com/jackc/pgx/v5"
)

const legacyEducateStateCategory = "Runtime/legacy_educate_state"

// The legacy calendar has four weeks per month and seven days per week;
// month can exceed twelve. It is a game date, not a wall-clock timestamp.
type LegacyEducateTime struct {
	Month uint32 `json:"month"`
	Week  uint32 `json:"week"`
	Day   uint32 `json:"day"`
}

type LegacyEducatePlanCell struct {
	Day         int32  `json:"day"`
	Index       int32  `json:"index"`
	PlanID      uint32 `json:"plan_id"`
	EventID     uint32 `json:"event_id,omitempty"`
	SpecEventID uint32 `json:"spec_event_id,omitempty"`
}

type LegacyEducateState struct {
	ShopCalendar              map[uint32]LegacyEducateShopCalendar `json:"shop_calendar,omitempty"`
	TaskSnapshotVersion       uint32                               `json:"task_snapshot_version,omitempty"`
	ClaimedTasks              map[uint32]bool                      `json:"claimed_tasks,omitempty"`
	TargetAwards              map[uint32]bool                      `json:"target_awards,omitempty"`
	Items                     map[uint32]uint32                    `json:"items,omitempty"`
	CompatDateBeforeAlignment *LegacyEducateTime                   `json:"compat_date_before_alignment,omitempty"`
	PlanVersion               uint32                               `json:"plan_version,omitempty"`
	PlanHistory               map[uint32]uint32                    `json:"plan_history,omitempty"`
	Settlement                *LegacyEducateSettlement             `json:"settlement,omitempty"`
	Version                   uint32                               `json:"state_version"`
	CurTime                   *LegacyEducateTime                   `json:"cur_time"`
	DateOrigin                string                               `json:"date_origin"`
	CommanderID               uint32                               `json:"commander_id"`
	CallName                  string                               `json:"call_name"`
	TargetID                  uint32                               `json:"target_id"`
	FavorLv                   uint32                               `json:"favor_lv"`
	FavorExp                  uint32                               `json:"favor_exp"`
	HadAdjustment             bool                                 `json:"had_adjustment"`
	Attrs                     map[uint32]uint32                    `json:"attrs"`
	TaskProgress              map[uint32]uint32                    `json:"task_progress"`
	OptionRecords             map[uint32]uint32                    `json:"option_records"`
	Resources                 map[uint32]int32                     `json:"resources"`
	Endings                   []uint32                             `json:"endings"`
	Qualifieds                []uint32                             `json:"qualifieds"`
	WeekPlans                 []LegacyEducatePlanCell              `json:"week_plans"`
	extra                     map[string]json.RawMessage
}

type LegacyEducateShopCalendar struct {
	Week             uint32 `json:"week"`
	CompatRefreshKey uint32 `json:"compat_refresh_key,omitempty"`
}

// Cached protocol bytes belong to the plan version, not the latest query.
// A retry after commit can return the same result without paying twice.
type LegacyEducateSettlement struct {
	Version  uint32            `json:"version"`
	From     LegacyEducateTime `json:"from"`
	Response []byte            `json:"response"`
}

func UpdateLegacyEducateState(commanderID uint32, update func(*LegacyEducateState) error) error {
	return UpdateLegacyEducateStateTx(commanderID, func(_ pgx.Tx, state *LegacyEducateState) error { return update(state) })
}

// Lock the legacy save first so shop stock and educate resources commit together.
func UpdateLegacyEducateStateTx(commanderID uint32, update func(pgx.Tx, *LegacyEducateState) error) error {
	if _, err := GetOrCreateLegacyEducateState(commanderID); err != nil {
		return err
	}
	ctx := context.Background()
	return db.DefaultStore.WithPGXTx(ctx, func(tx pgx.Tx) error {
		key := strconv.FormatUint(uint64(commanderID), 10)
		var raw []byte
		if err := tx.QueryRow(ctx, `SELECT data FROM config_entries WHERE category=$1 AND key=$2 FOR UPDATE`, legacyEducateStateCategory, key).Scan(&raw); err != nil {
			return err
		}
		state := &LegacyEducateState{}
		if err := json.Unmarshal(raw, state); err != nil {
			return err
		}
		if err := normalizeLegacyEducateState(state, commanderID); err != nil {
			return err
		}
		if err := update(tx, state); err != nil {
			return err
		}
		if err := normalizeLegacyEducateState(state, commanderID); err != nil {
			return err
		}
		payload, err := json.Marshal(state)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE config_entries SET data=$3 WHERE category=$1 AND key=$2`, legacyEducateStateCategory, key, payload)
		return err
	})
}

// Preserve fields outside this recovery slice when older handlers save the
// same JSON document. A load/save must not erase existing extension data.
func (state *LegacyEducateState) UnmarshalJSON(data []byte) error {
	type fields LegacyEducateState
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if raw == nil {
		return fmt.Errorf("legacy educate state must be an object")
	}
	if err := json.Unmarshal(data, (*fields)(state)); err != nil {
		return err
	}
	if _, present := raw["cur_time"]; present && state.CurTime == nil {
		return fmt.Errorf("legacy educate game date cannot be null")
	}
	state.extra = raw
	return nil
}

func (state LegacyEducateState) MarshalJSON() ([]byte, error) {
	type fields LegacyEducateState
	payload, err := json.Marshal(fields(state))
	if err != nil {
		return nil, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, err
	}
	for key, value := range state.extra {
		if _, known := raw[key]; !known {
			raw[key] = value
		}
	}
	return json.Marshal(raw)
}

func GetOrCreateLegacyEducateState(commanderID uint32) (*LegacyEducateState, error) {
	entry, err := GetConfigEntry(legacyEducateStateCategory, strconv.FormatUint(uint64(commanderID), 10))
	if err != nil {
		if !db.IsNotFound(err) {
			return nil, err
		}
		state := defaultLegacyEducateState(commanderID)
		if err := SaveLegacyEducateState(state); err != nil {
			return nil, err
		}
		return state, nil
	}

	state := &LegacyEducateState{}
	if err := json.Unmarshal(entry.Data, state); err != nil {
		return nil, err
	}
	migrateDate := state.Version == 0
	if err := normalizeLegacyEducateState(state, commanderID); err != nil {
		return nil, err
	}
	if migrateDate {
		if err := SaveLegacyEducateState(state); err != nil {
			return nil, err
		}
	}
	return state, nil
}

func SaveLegacyEducateState(state *LegacyEducateState) error {
	if err := normalizeLegacyEducateState(state, state.CommanderID); err != nil {
		return err
	}
	payload, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return UpsertConfigEntry(legacyEducateStateCategory, strconv.FormatUint(uint64(state.CommanderID), 10), payload)
}

func defaultLegacyEducateState(commanderID uint32) *LegacyEducateState {
	return &LegacyEducateState{
		Version:       1,
		CurTime:       &LegacyEducateTime{Month: 2, Week: 4, Day: 7},
		DateOrigin:    "initial.client-date",
		CommanderID:   commanderID,
		CallName:      "CHILD_USERNAME_SC_27001",
		FavorLv:       1,
		FavorExp:      0,
		Attrs:         map[uint32]uint32{201: 0, 202: 0, 203: 0},
		TaskProgress:  map[uint32]uint32{},
		OptionRecords: map[uint32]uint32{},
		Resources:     map[uint32]int32{3: 10},
		Endings:       []uint32{},
		Qualifieds:    []uint32{},
	}
}

func normalizeLegacyEducateState(state *LegacyEducateState, commanderID uint32) error {
	if state.Version > 1 {
		return fmt.Errorf("unsupported legacy educate state version %d", state.Version)
	}
	if state.Version == 0 {
		state.Version = 1
		if state.CurTime == nil {
			// The old server always served 2/4/7 and never stored a date. Keep
			// that last observable date, explicitly labelled as compatibility.
			state.CurTime = &LegacyEducateTime{Month: 2, Week: 4, Day: 7}
			state.DateOrigin = "compat.last-served-27001"
		} else if state.DateOrigin == "" {
			state.DateOrigin = "existing.saved-date"
		}
	}
	if state.CurTime == nil || state.CurTime.Month == 0 || state.CurTime.Week < 1 || state.CurTime.Week > 4 || state.CurTime.Day < 1 || state.CurTime.Day > 7 {
		return fmt.Errorf("invalid legacy educate game date")
	}
	state.CommanderID = commanderID
	if state.CallName == "" {
		state.CallName = "CHILD_USERNAME_SC_27001"
	}
	if state.FavorLv == 0 {
		state.FavorLv = 1
	}
	if state.Attrs == nil {
		state.Attrs = map[uint32]uint32{}
	}
	// The client's legacy EducateScheduleScene reads every child_attr id
	// (101-104 primary, 201-203 personality, 301-306 ability); a missing key
	// makes EducateChar GetAttrInfo compare number with nil and freezes the
	// schedule panel. Seed all of them.
	for _, attrID := range []uint32{101, 102, 103, 104, 201, 202, 203, 301, 302, 303, 304, 305, 306} {
		if _, ok := state.Attrs[attrID]; !ok {
			state.Attrs[attrID] = 0
		}
	}
	if state.TaskProgress == nil {
		state.TaskProgress = map[uint32]uint32{}
	}
	if state.OptionRecords == nil {
		state.OptionRecords = map[uint32]uint32{}
	}
	if state.Resources == nil {
		state.Resources = map[uint32]int32{}
	}
	if _, ok := state.Resources[3]; !ok {
		state.Resources[3] = 10
	}
	if state.Endings == nil {
		state.Endings = []uint32{}
	}
	if state.Qualifieds == nil {
		state.Qualifieds = []uint32{}
	}
	if state.WeekPlans == nil {
		state.WeekPlans = []LegacyEducatePlanCell{}
	}
	return nil
}

// LegacyEducatePlanCellsFromProto flattens the CHILD_PLAN_CELL proto list
// (cells with plan/event id values) into the JSON-friendly storage shape.
func LegacyEducatePlanCellsFromProto(cells []*pb.CHILD_PLAN_CELL) []LegacyEducatePlanCell {
	out := make([]LegacyEducatePlanCell, 0, len(cells))
	for _, cell := range cells {
		converted := LegacyEducatePlanCell{Day: int32(cell.GetDay()), Index: int32(cell.GetIndex())}
		for _, value := range cell.GetValue() {
			if value.GetPlanId() != 0 {
				converted.PlanID = value.GetPlanId()
			}
			if value.GetEventId() != 0 {
				converted.EventID = value.GetEventId()
			}
			if value.GetSpecEventId() != 0 {
				converted.SpecEventID = value.GetSpecEventId()
			}
		}
		out = append(out, converted)
	}
	return out
}
