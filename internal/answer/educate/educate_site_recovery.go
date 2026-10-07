package educate

import (
	"encoding/json"
	"fmt"
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
	"math"
	"reflect"
)

// This personal local slice settles the explicit numeric result_display of a
// single unconditional branch. Private/random rewards remain unrecovered.
func recoverLegacyNumericSite(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_27004
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 27005, err
	}
	response := &protobuf.SC_27005{Result: proto.Uint32(legacyEducateResultFailure), BranchId: proto.Uint32(0)}
	if client.Commander == nil {
		return client.SendMessage(27005, response)
	}
	site, ok, err := loadLegacyConfigByID[legacyChildSite](childSiteCategory, payload.GetSiteid())
	if err != nil || !ok || !legacySiteHasOption(site, payload.GetOptionid()) {
		return client.SendMessage(27005, response)
	}
	option, ok, err := loadLegacyConfigByID[legacyChildSiteOption](childSiteOptionCategory, payload.GetOptionid())
	if err != nil || !ok || option.Type != 2 || len(option.Result) != 1 || len(option.Display) == 0 || len(option.Polaroids) != 0 {
		return client.SendMessage(27005, response)
	}
	var branch *struct {
		ID, Ratio                   uint32
		Attr                        json.RawMessage
		Date, Item, State, Resource json.RawMessage
	}
	branch, ok, err = loadLegacyConfigByID[struct {
		ID, Ratio                   uint32
		Attr                        json.RawMessage
		Date, Item, State, Resource json.RawMessage
	}](childSiteOptionBranchCategory, option.Result[0])
	if err != nil || !ok || branch.Ratio != 10000 || string(branch.Attr) != "[]" || string(branch.Date) != `""` || string(branch.Item) != `""` || string(branch.State) != `""` || string(branch.Resource) != `""` {
		return client.SendMessage(27005, response)
	}
	drops := []*protobuf.CHILD_DROP{}
	for _, drop := range option.Display {
		if len(drop) != 3 || (drop[0] != 1 && drop[0] != 2) || drop[1] <= 0 || drop[2] <= 0 {
			return client.SendMessage(27005, response)
		}
		drops = append(drops, &protobuf.CHILD_DROP{Type: proto.Uint32(uint32(drop[0])), Id: proto.Uint32(uint32(drop[1])), Number: proto.Int32(drop[2])})
	}
	err = orm.UpdateLegacyEducateState(client.Commander.CommanderID, func(state *orm.LegacyEducateState) error {
		if len(site.Ability) != 0 || !reflect.DeepEqual(site.Unlock1, site.Unlock2) || len(site.Unlock1) != 3 || !legacyTaskInTime(legacyChildTask{TimeLimit: [][]uint32{site.Unlock1, {math.MaxUint32, 4, 7}}}, *state.CurTime) {
			return fmt.Errorf("site unlock not restored")
		}
		if len(option.TimeLimit) != 0 && !legacyTaskInTime(legacyChildTask{TimeLimit: option.TimeLimit}, *state.CurTime) {
			return fmt.Errorf("site option time")
		}
		if len(option.CountLimit) != 0 && (len(option.CountLimit) != 2 || state.OptionRecords[option.ID] >= option.CountLimit[0]) {
			return fmt.Errorf("site option exhausted")
		}
		costs := map[uint32]uint64{}
		for _, cost := range option.Cost {
			if len(cost) != 3 || cost[0] != 2 || cost[1] < 1 || cost[1] > 3 {
				return fmt.Errorf("site cost not restored")
			}
			costs[cost[1]] += uint64(cost[2])
		}
		for id, cost := range costs {
			if state.Resources[id] < 0 || cost > uint64(state.Resources[id]) {
				return fmt.Errorf("site resources insufficient")
			}
			state.Resources[id] -= int32(cost)
		}
		for _, drop := range drops {
			if err := applyLegacyTaskDrop(state, []uint32{drop.GetType(), drop.GetId(), uint32(drop.GetNumber())}); err != nil {
				return err
			}
		}
		if state.OptionRecords[option.ID] == math.MaxUint32 {
			return fmt.Errorf("site count overflow")
		}
		state.OptionRecords[option.ID]++
		return nil
	})
	if err == nil {
		response.Result = proto.Uint32(legacyEducateResultOK)
		response.BranchId = proto.Uint32(branch.ID)
		response.Drops = drops
	}
	return client.SendMessage(27005, response)
}

func refreshLegacyOptionCounts(state *orm.LegacyEducateState, from, to orm.LegacyEducateTime) error {
	entries, err := orm.ListConfigEntries(childSiteOptionCategory)
	if err != nil {
		return err
	}
	rows, err := parseConfigEntries[legacyChildSiteOption](entries)
	if err != nil {
		return err
	}
	oldWeek := (from.Month-1)*4 + from.Week
	newWeek := (to.Month-1)*4 + to.Week
	for _, option := range rows {
		if option.Type == 2 && len(option.CountLimit) == 2 && option.CountLimit[1] > 0 && legacyShopRefreshWeek(newWeek, int32(option.CountLimit[1])) > legacyShopRefreshWeek(oldWeek, int32(option.CountLimit[1])) {
			if _, saved := state.OptionRecords[option.ID]; saved {
				state.OptionRecords[option.ID] = 0
			}
		}
	}
	return nil
}
