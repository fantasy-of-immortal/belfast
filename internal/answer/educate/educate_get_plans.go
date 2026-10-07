package educate

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const (
	educatePlanValidationFailedResult = 1
)

func EducateGetPlans(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_27012
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 27013, err
	}

	response := protobuf.SC_27013{Result: proto.Uint32(0)}
	if !validateEducatePlanCells(payload.GetPlans()) {
		response.Result = proto.Uint32(educatePlanValidationFailedResult)
		response.Plans = []*protobuf.CHILD_PLAN_CELL{}
		return client.SendMessage(27013, &response)
	}

	// Persist the week plan so CS_27002 (execute) can settle the drops.
	err := orm.UpdateLegacyEducateState(client.Commander.CommanderID, func(state *orm.LegacyEducateState) error {
		return prepareLegacyWeek(state, orm.LegacyEducatePlanCellsFromProto(payload.GetPlans()))
	})
	if err != nil {
		response.Result = proto.Uint32(educatePlanValidationFailedResult)
		return client.SendMessage(27013, &response)
	}

	response.Plans = payload.GetPlans()
	return client.SendMessage(27013, &response)
}

func validateEducatePlanCells(cells []*protobuf.CHILD_PLAN_CELL) bool {
	if len(cells) == 0 {
		return false
	}
	for _, cell := range cells {
		if cell == nil {
			return false
		}
		if cell.GetDay() < 1 || cell.GetDay() > 6 {
			return false
		}
		if cell.GetIndex() < 1 || cell.GetIndex() > 3 {
			return false
		}
		values := cell.GetValue()
		if len(values) != 1 {
			return false
		}
		for _, value := range values {
			if !validateEducatePlanValue(value) {
				return false
			}
		}
	}
	return true
}

func validateEducatePlanValue(value *protobuf.CHILD_PLAN_VAL) bool {
	if value == nil {
		return false
	}

	nonZeroIDs := 0
	if value.GetPlanId() != 0 {
		nonZeroIDs++
	}
	if value.GetEventId() != 0 {
		nonZeroIDs++
	}
	if value.GetSpecEventId() != 0 {
		nonZeroIDs++
	}

	return nonZeroIDs == 1
}
