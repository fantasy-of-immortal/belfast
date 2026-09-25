package packets

import (
	"fmt"
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/debug"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/protobuf"
	"github.com/ggmolly/belfast/internal/region"
	"google.golang.org/protobuf/proto"
)

const (
	HEADER_SIZE                                  = 7
	missingPacketResultUnsupportedCommand uint32 = 1
)

type PacketHandler func(*[]byte, *connection.Client) (int, int, error)

var PacketDecisionFn = map[int][]PacketHandler{}

type LocalizedHandler struct {
	CN      *[]PacketHandler
	EN      *[]PacketHandler
	JP      *[]PacketHandler
	KR      *[]PacketHandler
	TW      *[]PacketHandler
	Default *[]PacketHandler
}

// Registers a region agnostic packet handler.
func RegisterPacketHandler(packetId int, handlers []PacketHandler) {
	logger.LogEvent("Handler", "Added", fmt.Sprintf("CS_%d", packetId), logger.LOG_LEVEL_DEBUG)
	PacketDecisionFn[packetId] = handlers
}

// dispatchHandlerSafely runs one packet handler and converts a panic into an
// ordinary error. A nil deref in a single handler used to take down the whole
// server process, dropping every connected client and leaving the game stuck on
// a loading screen without any reply. Containing it here keeps the blast radius
// to the one offending packet.
func dispatchHandlerSafely(handler PacketHandler, buffer *[]byte, client *connection.Client, packetID int) (replyID int, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			replyID = packetID
			err = fmt.Errorf("handler for CS_%d panicked: %v", packetID, recovered)
		}
	}()
	_, replyID, err = handler(buffer, client)
	return replyID, err
}

// Registers a localized packet handler, will call specific handler(s) based on
// the server's region.
func RegisterLocalizedPacketHandler(packetId int, localizedHandler LocalizedHandler) {
	switch region.Current() {
	case "CN":
		if localizedHandler.CN != nil {
			PacketDecisionFn[packetId] = *localizedHandler.CN
		} else if localizedHandler.Default != nil {
			PacketDecisionFn[packetId] = *localizedHandler.Default
		}
	case "EN":
		if localizedHandler.EN != nil {
			PacketDecisionFn[packetId] = *localizedHandler.EN
		} else if localizedHandler.Default != nil {
			PacketDecisionFn[packetId] = *localizedHandler.Default
		}
	case "JP":
		if localizedHandler.JP != nil {
			PacketDecisionFn[packetId] = *localizedHandler.JP
		} else if localizedHandler.Default != nil {
			PacketDecisionFn[packetId] = *localizedHandler.Default
		}
	case "KR":
		if localizedHandler.KR != nil {
			PacketDecisionFn[packetId] = *localizedHandler.KR
		} else if localizedHandler.Default != nil {
			PacketDecisionFn[packetId] = *localizedHandler.Default
		}
	case "TW":
		if localizedHandler.TW != nil {
			PacketDecisionFn[packetId] = *localizedHandler.TW
		} else if localizedHandler.Default != nil {
			PacketDecisionFn[packetId] = *localizedHandler.Default
		}
	default:
		if localizedHandler.Default != nil {
			PacketDecisionFn[packetId] = *localizedHandler.Default
		}
	}
}

// Find each packet in the buffer and dispatch it to the appropriate handler.
func Dispatch(buffer *[]byte, client *connection.Client, n int) {
	offset := 0
	for offset < n {
		packetId := GetPacketId(offset, buffer)
		packetSize := GetPacketSize(offset, buffer) + 2
		client.PacketIndex = GetPacketIndex(offset, buffer)
		handlers, ok := PacketDecisionFn[packetId]
		logger.WithFields(
			"Handler",
			logger.FieldValue("remote", fmt.Sprintf("%s:%d", client.IP, client.Port)),
			logger.FieldValue("packet", packetId),
			logger.FieldValue("size", packetSize),
			logger.FieldValue("has_handler", ok),
		).Debug("received packet")
		headerlessBuffer := (*buffer)[offset+HEADER_SIZE:]
		if !ok {
			logger.LogEvent("Handler", "Missing", fmt.Sprintf("CS_%d", packetId), logger.LOG_LEVEL_ERROR)
			debug.InsertPacket(packetId, &headerlessBuffer)
			_, _, err := client.SendMessage(10998, &protobuf.SC_10998{
				Cmd:    proto.Uint32(uint32(packetId)),
				Result: proto.Uint32(missingPacketResultUnsupportedCommand),
			})
			if err != nil {
				logger.LogEvent("Handler", "Error", fmt.Sprintf("SC_10998 - %v", err), logger.LOG_LEVEL_ERROR)
				return
			}
		} else {
			debug.InsertPacket(packetId, &headerlessBuffer)
			for _, handler := range handlers {
				start := time.Now()
				replyID, err := dispatchHandlerSafely(handler, &headerlessBuffer, client, packetId)
				elapsed := time.Since(start)
				logger.LogEvent("Metrics", "HandlerMs", fmt.Sprintf("CS_%d -> %s", replyID, elapsed), logger.LOG_LEVEL_DEBUG)
				if err != nil {
					client.RecordHandlerError()
					logger.LogEvent("Handler", "Error", fmt.Sprintf("SC_%d - %v", replyID, err), logger.LOG_LEVEL_ERROR)
					client.CloseWithError(err)
					return
				}
			}
		}
		offset += packetSize
	}
	if err := client.Flush(); err != nil {
		logger.LogEvent("Handler", "Flush", fmt.Sprintf("%s:%d -> %v", client.IP, client.Port, err), logger.LOG_LEVEL_ERROR)
	}
}
