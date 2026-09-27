package neweducate

import (
	"github.com/ggmolly/belfast/internal/connection"
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
		state = defaultEducateState(client.Commander.CommanderID, payload.GetId())
	}
	response := protobuf.SC_29002{
		Result:    proto.Uint32(0),
		Tb:        state.Info,
		Permanent: state.Permanent,
	}
	_ = saveEducateState(state)
	return client.SendMessage(29002, &response)
}

func defaultEducateState(commanderID uint32, tbID uint32) *educateState {
	info := ensureTBInfoDefaults(tbInfoPlaceholder())
	permanent := ensureTBPermanentDefaults(tbPermanentPlaceholder())
	info.Id = proto.Uint32(tbID)
	seedNewEducateDefaultRes(info, tbID)
	return &educateState{
		Entry:     &orm.CommanderTB{CommanderID: commanderID},
		Info:      info,
		Permanent: permanent,
	}
}
