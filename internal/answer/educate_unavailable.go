package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/educateprotocol"
	"google.golang.org/protobuf/proto"
)

// A build containing only one educate implementation still answers the other
// protocol family. It never loads a save or falls back to a placeholder engine.
func unavailableEducate(buffer *[]byte, client *connection.Client, request, response proto.Message, packetID int) (int, int, error) {
	return educateprotocol.Reject(buffer, client, request, response, packetID)
}
