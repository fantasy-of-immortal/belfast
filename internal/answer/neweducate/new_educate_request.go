package neweducate

import (
	"fmt"
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func NewEducateRequest(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_29001
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 29002, err
	}
	state, err := loadEducateState(client, payload.GetId())
	if err != nil {
		return failEducateRequest(client, payload.GetId(), err)
	}
	response := protobuf.SC_29002{
		Result:    proto.Uint32(0),
		Tb:        newEducateClientInfo(state.Info, state.Lifecycle),
		Permanent: state.Permanent,
	}
	if err := saveEducateState(state); err != nil {
		return failEducateRequest(client, payload.GetId(), err)
	}
	return client.SendMessage(29002, &response)
}

func failEducateRequest(client *connection.Client, id uint32, err error) (int, int, error) {
	logger.LogEvent("NewEducate", "Request", fmt.Sprintf("character=%d load/save failed: %v", id, err), logger.LOG_LEVEL_ERROR)
	// These are required protobuf fields even when result != 0. They are never
	// persisted, and the client only consumes them on success.
	return client.SendMessage(29002, &protobuf.SC_29002{Result: proto.Uint32(1), Tb: tbInfoPlaceholder(), Permanent: tbPermanentPlaceholder()})
}

func defaultEducateState(commanderID uint32, tbID uint32) *educateState {
	info := ensureTBInfoDefaults(tbInfoPlaceholder())
	permanent := ensureTBPermanentDefaults(tbPermanentPlaceholder())
	info.Id = proto.Uint32(tbID)
	seedNewEducateDefaultRes(info, tbID)
	return &educateState{
		Entry:     &orm.CommanderTB{CommanderID: commanderID, CharacterID: tbID},
		Info:      info,
		Permanent: permanent,
		Lifecycle: freshEducateLifecycle(info),
	}
}
