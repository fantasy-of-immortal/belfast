package educate

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/educateprotocol"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func EducateReset(buffer *[]byte, client *connection.Client) (int, int, error) {
	return educateprotocol.Reject(buffer, client, &protobuf.CS_27029{}, &protobuf.SC_27030{}, 27030)
}
