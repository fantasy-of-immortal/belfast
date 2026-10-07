package educateprotocol

import (
	"fmt"
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// A build containing only one educate implementation still answers the other
// protocol family. It never loads a save or falls back to a placeholder engine.
func Reject(buffer *[]byte, client *connection.Client, request, response proto.Message, packetID int) (int, int, error) {
	if err := proto.Unmarshal(*buffer, request); err != nil {
		return 0, packetID, err
	}
	message := response.ProtoReflect()
	result := message.Descriptor().Fields().ByName("result")
	if result == nil {
		// Queries without a result field cannot represent failure in their normal
		// reply. Use the dispatcher's existing unsupported-command response.
		var requestID uint32
		if _, err := fmt.Sscanf(string(request.ProtoReflect().Descriptor().Name()), "CS_%d", &requestID); err != nil {
			return 0, packetID, err
		}
		return client.SendMessage(10998, &protobuf.SC_10998{Cmd: proto.Uint32(requestID), Result: proto.Uint32(1)})
	}
	message.Set(result, protoreflect.ValueOfUint32(1))
	fillRequiredResponse(message)
	return client.SendMessage(packetID, response)
}

// Some failure packets require a nested TB snapshot even though the client
// consumes it only on success. Populate wire fields, never gameplay state.
func fillRequiredResponse(message protoreflect.Message) {
	fields := message.Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		field := fields.Get(i)
		if field.Cardinality() != protoreflect.Required || message.Has(field) {
			continue
		}
		if field.Kind() == protoreflect.MessageKind {
			fillRequiredResponse(message.Mutable(field).Message())
		} else {
			message.Set(field, field.Default())
		}
	}
}
