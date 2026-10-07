package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func NewEducateGetEndings(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_29003{}, &protobuf.SC_29004{}, 29004)
}
func NewEducateSelectEnding(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_29005{}, &protobuf.SC_29006{}, 29006)
}
func NewEducateReset(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_29007{}, &protobuf.SC_29008{}, 29008)
}
func NewEducateSetCall(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_29009{}, &protobuf.SC_29010{}, 29010)
}
func NewEducateMainEvent(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_29011{}, &protobuf.SC_29012{}, 29012)
}
func NewEducateAssess(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_29013{}, &protobuf.SC_29014{}, 29014)
}
func NewEducateGetTopics(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_29015{}, &protobuf.SC_29016{}, 29016)
}
func NewEducateSelectTopic(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_29017{}, &protobuf.SC_29018{}, 29018)
}
func NewEducateGetTalents(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_29019{}, &protobuf.SC_29020{}, 29020)
}
func NewEducateRefreshTalent(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_29021{}, &protobuf.SC_29022{}, 29022)
}
func NewEducateSelectTalent(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_29023{}, &protobuf.SC_29024{}, 29024)
}
func NewEducateChangePhase(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_29025{}, &protobuf.SC_29026{}, 29026)
}
func NewEducateUpgradeFavor(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_29027{}, &protobuf.SC_29028{}, 29028)
}
func NewEducateTriggerNode(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_29030{}, &protobuf.SC_29031{}, 29031)
}
func NewEducateClearNodeChain(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_29032{}, &protobuf.SC_29033{}, 29033)
}
func NewEducateSchedule(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_29040{}, &protobuf.SC_29041{}, 29041)
}
func NewEducateNextPlan(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_29042{}, &protobuf.SC_29043{}, 29043)
}
func NewEducateUpgradePlan(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_29044{}, &protobuf.SC_29045{}, 29045)
}
func NewEducateScheduleSkip(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_29046{}, &protobuf.SC_29047{}, 29047)
}
func NewEducateGetExtraDrop(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_29048{}, &protobuf.SC_29049{}, 29049)
}
func NewEducateEnterAssess(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_29050{}, &protobuf.SC_29051{}, 29051)
}
func NewEducateGetChoose(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_29126{}, &protobuf.SC_29127{}, 29127)
}
func NewEducateRequestChoices(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_29107{}, &protobuf.SC_29108{}, 29108)
}
func NewEducateRefreshChoice(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_29105{}, &protobuf.SC_29106{}, 29106)
}
func NewEducateMakeChoice(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_29103{}, &protobuf.SC_29104{}, 29104)
}
func NewEducateGiveUpChoice(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_29101{}, &protobuf.SC_29102{}, 29102)
}
func NewEducateReplaceTarot(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_29120{}, &protobuf.SC_29121{}, 29121)
}
func NewEducateUpgradeEntry(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_29122{}, &protobuf.SC_29123{}, 29123)
}
func NewEducateGiveUpEntryUp(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_29124{}, &protobuf.SC_29125{}, 29125)
}
func NewEducateRefreshShop(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_29072{}, &protobuf.SC_29073{}, 29073)
}
func NewEducateGetMap(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_29060{}, &protobuf.SC_29061{}, 29061)
}
func NewEducateMapNormal(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_29062{}, &protobuf.SC_29063{}, 29063)
}
func NewEducateMapEvent(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_29064{}, &protobuf.SC_29065{}, 29065)
}
func NewEducateShopping(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_29066{}, &protobuf.SC_29067{}, 29067)
}
func NewEducateMapShip(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_29068{}, &protobuf.SC_29069{}, 29069)
}
func NewEducateUpgradeNormalSite(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_29070{}, &protobuf.SC_29071{}, 29071)
}
func NewEducateSelectMind(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_29090{}, &protobuf.SC_29091{}, 29091)
}
func NewEducateRefresh(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_29092{}, &protobuf.SC_29093{}, 29093)
}
func NewEducateRequest(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_29001{}, &protobuf.SC_29002{}, 29002)
}
