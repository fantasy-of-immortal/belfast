package educate

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/educateprotocol"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func EducateRefresh(buffer *[]byte, client *connection.Client) (int, int, error) {
	return educateprotocol.Reject(buffer, client, &protobuf.CS_27047{}, &protobuf.SC_27048{}, 27048)
}
