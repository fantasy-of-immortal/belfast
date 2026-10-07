package educate

import (
	"encoding/json"
	"fmt"
	"github.com/ggmolly/belfast/internal/educateprotocol"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/ggmolly/belfast/internal/config"
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const (
	legacyEducateResultOK      = uint32(0)
	legacyEducateResultFailure = uint32(1)

	childSiteCategory             = "ShareCfg/child_site.json"
	childSiteOptionCategory       = "ShareCfg/child_site_option.json"
	childSiteOptionBranchCategory = "ShareCfg/child_site_option_branch.json"
	childTaskCategory             = "ShareCfg/child_task.json"
	childTargetSetCategory        = "ShareCfg/child_target_set.json"
	childDataCategory             = "ShareCfg/child_data.json"
	childEndingCategory           = "ShareCfg/child_ending.json"
	secretarySpecialShipCategory  = "ShareCfg/secretary_special_ship.json"

	legacyEducateCallNameMin = 4
	legacyEducateCallNameMax = 14
)

type legacyChildSite struct {
	Unlock1      []uint32          `json:"unlock_time_1"`
	Unlock2      []uint32          `json:"unlock_time_2"`
	Ability      [][]uint32        `json:"ability"`
	ID           uint32            `json:"id"`
	Option       []uint32          `json:"option"`
	OptionRandom []json.RawMessage `json:"option_random"`
}

type legacyChildSiteOption struct {
	TimeLimit  [][]uint32 `json:"time_limit"`
	Display    [][]int32  `json:"result_display"`
	Polaroids  []uint32   `json:"polarid_list"`
	ID         uint32     `json:"id"`
	Type       uint32     `json:"type"`
	Result     []uint32   `json:"result"`
	Cost       [][]uint32 `json:"cost"`
	CountLimit []uint32   `json:"count_limit"`
}

type legacyChildTask struct {
	ID             uint32          `json:"id"`
	Type1          uint32          `json:"type_1"`
	Arg            uint32          `json:"arg"`
	DropDisplay    []uint32        `json:"drop_display"`
	Type2          uint32          `json:"type_2"`
	SubType        json.RawMessage `json:"sub_type"`
	TimeLimit      [][]uint32      `json:"time_limit"`
	TargetProgress uint32          `json:"task_target_progress"`
}

type legacyChildTargetSet struct {
	ID        uint32          `json:"id"`
	Stage     uint32          `json:"stage"`
	IDs       []uint32        `json:"ids"`
	Condition json.RawMessage `json:"condition"`
}

type legacyChildData struct {
	ID        uint32   `json:"id"`
	Attr2List []uint32 `json:"attr_2_list"`
	Attr2Add  uint32   `json:"attr_2_add"`
	FavorLv   uint32   `json:"favor_level"`
}

type legacySecretarySpecialShip struct {
	ID uint32 `json:"id"`
}

func EducateUpgradeFavor(buffer *[]byte, client *connection.Client) (int, int, error) {
	return educateprotocol.Reject(buffer, client, &protobuf.CS_27006{}, &protobuf.SC_27007{}, 27007)
}
func EducateTriggerEnd(buffer *[]byte, client *connection.Client) (int, int, error) {
	return educateprotocol.Reject(buffer, client, &protobuf.CS_27008{}, &protobuf.SC_27009{}, 27009)
}
func EducateGetEndings(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_27010
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 27011, err
	}

	state, err := orm.GetOrCreateLegacyEducateState(client.Commander.CommanderID)
	if err != nil {
		return 0, 27011, err
	}

	response := protobuf.SC_27011{
		Endings:    append([]uint32{}, state.Endings...),
		Qualifieds: append([]uint32{}, state.Qualifieds...),
	}
	return client.SendMessage(27011, &response)
}

func EducateSetTarget(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_27019
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 27020, err
	}

	response := protobuf.SC_27020{Result: proto.Uint32(legacyEducateResultFailure)}
	targetID := payload.GetId()
	if targetID == 0 {
		return client.SendMessage(27020, &response)
	}
	var added, updated []*protobuf.CHILD_TASK
	var removed []uint32
	err := orm.UpdateLegacyEducateState(client.Commander.CommanderID, func(state *orm.LegacyEducateState) error {
		var err error
		removed, added, updated, err = selectLegacyTarget(state, targetID)
		return err
	})
	if err != nil {
		return client.SendMessage(27020, &response)
	}

	response.Result = proto.Uint32(legacyEducateResultOK)
	written, packetID, err := client.SendMessage(27020, &response)
	if err != nil {
		return written, packetID, err
	}
	if len(removed) != 0 {
		if _, _, err := client.SendMessage(27022, &protobuf.SC_27022{Ids: removed}); err != nil {
			return written, packetID, err
		}
	}
	return written, packetID, sendLegacyTaskChanges(client, added, updated)
}

func EducateSubmitTask(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_27023
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 27024, err
	}

	response := protobuf.SC_27024{
		Result: proto.Uint32(legacyEducateResultFailure),
		Awards: []*protobuf.CHILD_DROP{},
	}

	taskID := payload.GetId()
	if taskID == 0 || payload.GetSystem() == 0 {
		return client.SendMessage(27024, &response)
	}
	taskConfig, ok, err := loadLegacyConfigByID[legacyChildTask](childTaskCategory, taskID)
	if err != nil || !ok || taskConfig.Type1 != payload.GetSystem() {
		return client.SendMessage(27024, &response)
	}

	err = orm.UpdateLegacyEducateState(client.Commander.CommanderID, func(state *orm.LegacyEducateState) error {
		if err := syncLegacyTasks(state); err != nil {
			return err
		}
		progress, active := state.TaskProgress[taskID]
		if !active || state.ClaimedTasks[taskID] || progress < taskConfig.Arg || !legacyTaskInTime(*taskConfig, *state.CurTime) {
			return fmt.Errorf("legacy task not claimable")
		}
		if err := applyLegacyTaskDrop(state, taskConfig.DropDisplay); err != nil {
			return err
		}
		state.ClaimedTasks[taskID] = true
		delete(state.TaskProgress, taskID)
		return nil
	})
	if err != nil {
		return client.SendMessage(27024, &response)
	}

	response.Result = proto.Uint32(legacyEducateResultOK)
	if len(taskConfig.DropDisplay) >= 3 {
		response.Awards = append(response.Awards, buildLegacyChildDrop(taskConfig.DropDisplay[0], taskConfig.DropDisplay[1], int32(taskConfig.DropDisplay[2])))
	}
	return client.SendMessage(27024, &response)
}

func EducateSetCall(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_27031
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 27032, err
	}

	response := protobuf.SC_27032{Result: proto.Uint32(legacyEducateResultFailure)}
	name := strings.TrimSpace(payload.GetName())
	if !isValidLegacyCallName(name) {
		return client.SendMessage(27032, &response)
	}

	state, err := orm.GetOrCreateLegacyEducateState(client.Commander.CommanderID)
	if err != nil {
		return client.SendMessage(27032, &response)
	}
	state.CallName = name
	if err := orm.SaveLegacyEducateState(state); err != nil {
		return client.SendMessage(27032, &response)
	}

	response.Result = proto.Uint32(legacyEducateResultOK)
	return client.SendMessage(27032, &response)
}

func EducateAddTaskProgress(buffer *[]byte, client *connection.Client) (int, int, error) {
	return educateprotocol.Reject(buffer, client, &protobuf.CS_27037{}, &protobuf.SC_27038{}, 27038)
}
func EducateAddExtraAttr(buffer *[]byte, client *connection.Client) (int, int, error) {
	return educateprotocol.Reject(buffer, client, &protobuf.CS_27039{}, &protobuf.SC_27040{}, 27040)
}
func ChangeEducateCharacter(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_27041
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 27042, err
	}

	response := protobuf.SC_27042{Result: proto.Uint32(legacyEducateResultFailure)}
	endingID := payload.GetEndingId()
	if endingID == 0 {
		return client.SendMessage(27042, &response)
	}
	if _, ok, err := loadLegacyConfigByID[legacySecretarySpecialShip](secretarySpecialShipCategory, endingID); err != nil || !ok {
		return client.SendMessage(27042, &response)
	}

	state, err := orm.GetOrCreateLegacyEducateState(client.Commander.CommanderID)
	if err != nil {
		return client.SendMessage(27042, &response)
	}
	if !containsUint32(state.Endings, endingID) {
		return client.SendMessage(27042, &response)
	}

	if err := orm.UpdateCommanderChildDisplay(client.Commander.CommanderID, endingID); err != nil {
		return client.SendMessage(27042, &response)
	}
	client.Commander.ChildDisplay = endingID

	response.Result = proto.Uint32(legacyEducateResultOK)
	return client.SendMessage(27042, &response)
}

func EducateMapSiteOperate(buffer *[]byte, client *connection.Client) (int, int, error) {
	return recoverLegacyNumericSite(buffer, client)
}
func isValidLegacyCallName(name string) bool {
	if name == "" {
		return false
	}
	nameLength := utf8.RuneCountInString(name)
	if nameLength < legacyEducateCallNameMin || nameLength > legacyEducateCallNameMax {
		return false
	}
	createConfig := config.Current().CreatePlayer
	if len(createConfig.NameBlacklist) > 0 {
		lowerName := strings.ToLower(name)
		for _, blocked := range createConfig.NameBlacklist {
			blocked = strings.TrimSpace(blocked)
			if blocked == "" {
				continue
			}
			if strings.Contains(lowerName, strings.ToLower(blocked)) {
				return false
			}
		}
	}
	if createConfig.NameIllegalPattern != "" {
		matcher, err := regexp.Compile(createConfig.NameIllegalPattern)
		if err != nil {
			return false
		}
		if matcher.MatchString(name) {
			return false
		}
	}
	return true
}

func buildLegacyChildDrop(dropType uint32, id uint32, number int32) *protobuf.CHILD_DROP {
	return &protobuf.CHILD_DROP{
		Type:   proto.Uint32(dropType),
		Id:     proto.Uint32(id),
		Number: proto.Int32(number),
	}
}

func legacySiteHasOption(site *legacyChildSite, optionID uint32) bool {
	if site == nil {
		return false
	}
	if containsUint32(site.Option, optionID) {
		return true
	}
	for _, row := range site.OptionRandom {
		var groups []json.RawMessage
		if err := json.Unmarshal(row, &groups); err != nil {
			continue
		}
		for _, group := range groups {
			var values []json.RawMessage
			if err := json.Unmarshal(group, &values); err != nil || len(values) == 0 {
				continue
			}
			var candidate uint32
			if err := json.Unmarshal(values[0], &candidate); err != nil {
				continue
			}
			if candidate == optionID {
				return true
			}
		}
	}
	return false
}

func loadLegacyChildData() (*legacyChildData, bool, error) {
	entry, err := orm.GetConfigEntry(childDataCategory, "1")
	if err != nil {
		if db.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	var configData legacyChildData
	if err := json.Unmarshal(entry.Data, &configData); err != nil {
		return nil, false, err
	}
	return &configData, true, nil
}

func legacyConfigExists(category string, id uint32) (bool, error) {
	_, err := orm.GetConfigEntry(category, strconv.FormatUint(uint64(id), 10))
	if err != nil {
		if db.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func loadLegacyConfigByID[T any](category string, id uint32) (*T, bool, error) {
	entry, err := orm.GetConfigEntry(category, strconv.FormatUint(uint64(id), 10))
	if err != nil {
		if db.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	var configData T
	if err := json.Unmarshal(entry.Data, &configData); err != nil {
		return nil, false, err
	}
	return &configData, true, nil
}
