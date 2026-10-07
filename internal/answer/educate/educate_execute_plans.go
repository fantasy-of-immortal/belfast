package educate

import (
	"encoding/json"
	"strconv"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const (
	educateUnsupportedTypeResult = 1
)

// educatePlanResultConfig mirrors the consumed fields of child_plan.
type educatePlanResultConfig struct {
	ID            uint32          `json:"id"`
	Stage         []uint32        `json:"stage"`
	Time          [][]uint32      `json:"time"`
	Ability       [][]uint32      `json:"ability"`
	Pre           []uint32        `json:"pre"`
	CostResource1 uint32          `json:"cost_resource1"`
	CostResource2 uint32          `json:"cost_resource2"`
	CostResource3 uint32          `json:"cost_resource3"`
	ResultDisplay json.RawMessage `json:"result_display"`
}

// EducateExecutePlans settles the pending week plan stored by CS_27012:
// each planned cell produces a CHILD_PLAN_RESULT with the plan's
// result_display as plan_drops (applied to the persisted attrs), and the
// plan cost is debited from the resources. Without real plan_results the
// client's EducateSchedulePerformLayer.playWeek indexes nil and freezes.
func EducateExecutePlans(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_27002
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 27003, err
	}
	response := &protobuf.SC_27003{Result: proto.Uint32(educateUnsupportedTypeResult)}
	var added, updated []*protobuf.CHILD_TASK
	if payload.GetType() == 1 {
		err := orm.UpdateLegacyEducateState(client.Commander.CommanderID, func(state *orm.LegacyEducateState) error {
			pending := len(state.WeekPlans) != 0
			before := make(map[uint32]uint32, len(state.TaskProgress))
			for id, value := range state.TaskProgress {
				before[id] = value
			}
			settled, err := settleLegacyWeek(state)
			if err == nil && pending {
				err = syncLegacyTasks(state)
				if err == nil {
					added, updated = legacyTaskChanges(before, state.TaskProgress)
				}
			}
			if err == nil {
				response = settled
			}
			return err
		})
		if err != nil {
			response = &protobuf.SC_27003{Result: proto.Uint32(educateUnsupportedTypeResult)}
			added, updated = nil, nil
		}
	}
	bytesWritten, packetID, err := client.SendMessage(27003, response)
	if err != nil {
		return bytesWritten, packetID, err
	}
	return bytesWritten, packetID, sendLegacyTaskChanges(client, added, updated)
}
func loadChildPlanConfig(id uint32) (*educatePlanResultConfig, bool, error) {
	entry, err := orm.GetConfigEntry("ShareCfg/child_plan.json", strconv.FormatUint(uint64(id), 10))
	if err != nil {
		if db.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	var cfg educatePlanResultConfig
	if err := json.Unmarshal(entry.Data, &cfg); err != nil {
		return nil, false, err
	}
	return &cfg, true, nil
}

func parseConfigDropTriplets(raw json.RawMessage) [][]int32 {
	if len(raw) == 0 {
		return nil
	}
	var triplets [][]int32
	if err := json.Unmarshal(raw, &triplets); err != nil {
		return nil
	}
	return triplets
}

// applyLegacyEducateDrop mirrors the drop type semantics used by the client's
// legacy educate helpers: type 1 = attr, type 2/resource-ish types fall to
// the resource pool entry of the same id.
func applyLegacyEducateDrop(state *orm.LegacyEducateState, dropType uint32, id uint32, number int32) {
	switch dropType {
	case 1:
		if state.Attrs == nil {
			state.Attrs = map[uint32]uint32{}
		}
		current := int32(state.Attrs[id])
		current += number
		if current < 0 {
			current = 0
		}
		state.Attrs[id] = uint32(current)
	default:
		if state.Resources == nil {
			state.Resources = map[uint32]int32{}
		}
		state.Resources[id] += number
	}
}
