package neweducate

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/educateprotocol"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func NewEducateGetEndings(buffer *[]byte, client *connection.Client) (int, int, error) {
	return educateprotocol.Reject(buffer, client, &protobuf.CS_29003{}, &protobuf.SC_29004{}, 29004)
}
func NewEducateSelectEnding(buffer *[]byte, client *connection.Client) (int, int, error) {
	return educateprotocol.Reject(buffer, client, &protobuf.CS_29005{}, &protobuf.SC_29006{}, 29006)
}
func NewEducateReset(buffer *[]byte, client *connection.Client) (int, int, error) {
	return educateprotocol.Reject(buffer, client, &protobuf.CS_29007{}, &protobuf.SC_29008{}, 29008)
}
func NewEducateAssess(buffer *[]byte, client *connection.Client) (int, int, error) {
	return educateprotocol.Reject(buffer, client, &protobuf.CS_29013{}, &protobuf.SC_29014{}, 29014)
}
func NewEducateSelectTopic(buffer *[]byte, client *connection.Client) (int, int, error) {
	return educateprotocol.Reject(buffer, client, &protobuf.CS_29017{}, &protobuf.SC_29018{}, 29018)
}
func NewEducateEnterAssess(buffer *[]byte, client *connection.Client) (int, int, error) {
	return educateprotocol.Reject(buffer, client, &protobuf.CS_29050{}, &protobuf.SC_29051{}, 29051)
}
func NewEducateReplaceTarot(buffer *[]byte, client *connection.Client) (int, int, error) {
	return educateprotocol.Reject(buffer, client, &protobuf.CS_29120{}, &protobuf.SC_29121{}, 29121)
}
func NewEducateUpgradeEntry(buffer *[]byte, client *connection.Client) (int, int, error) {
	return educateprotocol.Reject(buffer, client, &protobuf.CS_29122{}, &protobuf.SC_29123{}, 29123)
}
func NewEducateGiveUpEntryUp(buffer *[]byte, client *connection.Client) (int, int, error) {
	return educateprotocol.Reject(buffer, client, &protobuf.CS_29124{}, &protobuf.SC_29125{}, 29125)
}
func NewEducateRefreshShop(buffer *[]byte, client *connection.Client) (int, int, error) {
	return educateprotocol.Reject(buffer, client, &protobuf.CS_29072{}, &protobuf.SC_29073{}, 29073)
}
func NewEducateMapEvent(buffer *[]byte, client *connection.Client) (int, int, error) {
	return educateprotocol.Reject(buffer, client, &protobuf.CS_29064{}, &protobuf.SC_29065{}, 29065)
}
func NewEducateMapShip(buffer *[]byte, client *connection.Client) (int, int, error) {
	return educateprotocol.Reject(buffer, client, &protobuf.CS_29068{}, &protobuf.SC_29069{}, 29069)
}
func NewEducateUpgradeNormalSite(buffer *[]byte, client *connection.Client) (int, int, error) {
	return educateprotocol.Reject(buffer, client, &protobuf.CS_29070{}, &protobuf.SC_29071{}, 29071)
}
func NewEducateSelectMind(buffer *[]byte, client *connection.Client) (int, int, error) {
	return educateprotocol.Reject(buffer, client, &protobuf.CS_29090{}, &protobuf.SC_29091{}, 29091)
}
func NewEducateRequestChoices(buffer *[]byte, client *connection.Client) (int, int, error) {
	return educateprotocol.Reject(buffer, client, &protobuf.CS_29107{}, &protobuf.SC_29108{}, 29108)
}
func NewEducateRefreshChoice(buffer *[]byte, client *connection.Client) (int, int, error) {
	return educateprotocol.Reject(buffer, client, &protobuf.CS_29105{}, &protobuf.SC_29106{}, 29106)
}
func NewEducateMakeChoice(buffer *[]byte, client *connection.Client) (int, int, error) {
	return educateprotocol.Reject(buffer, client, &protobuf.CS_29103{}, &protobuf.SC_29104{}, 29104)
}
func NewEducateGiveUpChoice(buffer *[]byte, client *connection.Client) (int, int, error) {
	return educateprotocol.Reject(buffer, client, &protobuf.CS_29101{}, &protobuf.SC_29102{}, 29102)
}
