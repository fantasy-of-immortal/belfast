package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// A build containing only one educate implementation still answers the other
// protocol family. It never loads a save or falls back to a placeholder engine.
func unavailableEducate(buffer *[]byte, client *connection.Client, request, response proto.Message, packetID int) (int, int, error) {
	if err := proto.Unmarshal(*buffer, request); err != nil {
		return 0, packetID, err
	}
	message := response.ProtoReflect()
	message.Set(message.Descriptor().Fields().ByName("result"), protoreflect.ValueOfUint32(1))
	fillRequiredEducateResponse(message)
	return client.SendMessage(packetID, response)
}

// Some failure packets require a nested TB snapshot even though the client
// consumes it only on success. Populate wire fields, never gameplay state.
func fillRequiredEducateResponse(message protoreflect.Message) {
	fields := message.Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		field := fields.Get(i)
		if field.Cardinality() != protoreflect.Required || message.Has(field) {
			continue
		}
		if field.Kind() == protoreflect.MessageKind {
			fillRequiredEducateResponse(message.Mutable(field).Message())
		} else {
			message.Set(field, field.Default())
		}
	}
}
