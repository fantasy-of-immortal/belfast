package educate

import (
	"fmt"
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func EducateRequest(buffer *[]byte, client *connection.Client) (int, int, error) {
	state, err := orm.GetOrCreateLegacyEducateState(client.Commander.CommanderID)
	if err != nil {
		return 0, 27001, err
	}
	if err := alignLegacyCompatibilityDate(state); err != nil {
		return 0, 27001, err
	}
	if state.TaskSnapshotVersion == 0 {
		if err := orm.UpdateLegacyEducateState(state.CommanderID, syncLegacyTasks); err != nil {
			return 0, 27001, err
		}
		state, err = orm.GetOrCreateLegacyEducateState(state.CommanderID)
		if err != nil {
			return 0, 27001, err
		}
	}

	attrs := make([]*protobuf.CHILD_ATTR, 0, len(state.Attrs))
	for _, attrID := range []uint32{101, 102, 103, 104, 201, 202, 203, 301, 302, 303, 304, 305, 306} {
		attrs = append(attrs, &protobuf.CHILD_ATTR{Id: proto.Uint32(attrID), Val: proto.Uint32(state.Attrs[attrID])})
	}
	tasks, err := legacyTaskSnapshot(state)
	if err != nil {
		return 0, 27001, err
	}
	items := make([]*protobuf.CHILD_ITEM, 0, len(state.Items))
	for id, count := range state.Items {
		items = append(items, &protobuf.CHILD_ITEM{Id: proto.Uint32(id), Num: proto.Uint32(count)})
	}
	optionRecords := make([]*protobuf.CHILD_OPTION_RECORD, 0, len(state.OptionRecords))
	for optionID, count := range state.OptionRecords {
		optionRecords = append(optionRecords, &protobuf.CHILD_OPTION_RECORD{Id: proto.Uint32(optionID), Count: proto.Uint32(count)})
	}
	for _, id := range []uint32{1, 2, 3} {
		if state.Resources[id] < 0 {
			return 0, 27001, fmt.Errorf("legacy educate resource %d is negative", id)
		}
	}
	plans := make([]*protobuf.CHILD_PLAN_CELL, 0, len(state.WeekPlans))
	history := make([]*protobuf.CHILD_PLAN_HISTORY, 0, len(state.PlanHistory))
	for id, count := range state.PlanHistory {
		history = append(history, &protobuf.CHILD_PLAN_HISTORY{PlanId: proto.Uint32(id), Count: proto.Uint32(count)})
	}
	for _, cell := range state.WeekPlans {
		if cell.Day < 1 || cell.Index < 1 {
			return 0, 27001, fmt.Errorf("invalid stored legacy plan cell")
		}
		plans = append(plans, &protobuf.CHILD_PLAN_CELL{Day: proto.Uint32(uint32(cell.Day)), Index: proto.Uint32(uint32(cell.Index)), Value: []*protobuf.CHILD_PLAN_VAL{{PlanId: proto.Uint32(cell.PlanID), EventId: proto.Uint32(cell.EventID), SpecEventId: proto.Uint32(cell.SpecEventID)}}})
	}

	response := protobuf.SC_27001{
		Result: proto.Uint32(0),
		Child: &protobuf.CHILD_INFO{
			Tid:        proto.Uint32(1),
			Mood:       proto.Uint32(uint32(state.Resources[2])),
			Money:      proto.Uint32(uint32(state.Resources[1])),
			SiteNumber: proto.Uint32(uint32(state.Resources[3])),
			CurTime: &protobuf.CHILD_TIME{
				Month: proto.Uint32(state.CurTime.Month),
				Day:   proto.Uint32(state.CurTime.Day),
				Week:  proto.Uint32(state.CurTime.Week),
			},
			Favor: &protobuf.CHILD_FAVOR{
				Lv:  proto.Uint32(state.FavorLv),
				Exp: proto.Uint32(state.FavorExp),
			},
			Attrs:                   attrs,
			Items:                   items,
			PlanHistory:             history,
			Memorys:                 []uint32{},
			Plans:                   plans,
			Polaroids:               []*protobuf.CHILD_POLAROID{},
			Target:                  proto.Uint32(state.TargetID),
			Tasks:                   tasks,
			RealizedWish:            []uint32{},
			Buffs:                   []*protobuf.CHILD_BUFF{},
			UserName:                proto.String(state.CallName),
			SpecEvents:              []uint32{},
			CanTriggerHomeEvent:     proto.Uint32(0),
			HomeEvents:              []uint32{},
			DiscountEventId:         []uint32{},
			Shop:                    []*protobuf.CHILD_SHOP_DATA{},
			OptionRecords:           optionRecords,
			FavorAwardHistory:       []uint32{},
			IsEnding:                proto.Uint32(0),
			NewGamePlusCount:        proto.Uint32(0),
			HadTargetStageAward:     proto.Uint32(0),
			HadAdjustment:           proto.Uint32(boolToUint32(state.HadAdjustment)),
			IsSpecialSecretaryValid: proto.Uint32(0),
			EndingBuyCount:          proto.Uint32(0),
			MemoryBuyCount:          proto.Uint32(0),
			PolaroidBuyCount:        proto.Uint32(0),
		},
	}
	if client.Commander != nil {
		if err := populateEducateSnapshot(client.Commander.CommanderID, response.Child); err != nil {
			return 0, 27001, err
		}
	}
	response.Child.HadTargetStageAward = proto.Uint32(boolToUint32(state.TargetAwards[state.TargetID]))
	return client.SendMessage(27001, &response)
}
