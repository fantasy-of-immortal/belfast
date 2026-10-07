package answer

import (
	"github.com/ggmolly/belfast/internal/answer/neweducate"
	"github.com/ggmolly/belfast/internal/connection"
)

func NewEducateGetEndings(buffer *[]byte, client *connection.Client) (int, int, error) {
	return neweducate.NewEducateGetEndings(buffer, client)
}

func NewEducateSelectEnding(buffer *[]byte, client *connection.Client) (int, int, error) {
	return neweducate.NewEducateSelectEnding(buffer, client)
}

func NewEducateReset(buffer *[]byte, client *connection.Client) (int, int, error) {
	return neweducate.NewEducateReset(buffer, client)
}

func NewEducateSetCall(buffer *[]byte, client *connection.Client) (int, int, error) {
	return neweducate.NewEducateSetCall(buffer, client)
}

func NewEducateMainEvent(buffer *[]byte, client *connection.Client) (int, int, error) {
	return neweducate.NewEducateMainEvent(buffer, client)
}

func NewEducateAssess(buffer *[]byte, client *connection.Client) (int, int, error) {
	return neweducate.NewEducateAssess(buffer, client)
}

func NewEducateGetTopics(buffer *[]byte, client *connection.Client) (int, int, error) {
	return neweducate.NewEducateGetTopics(buffer, client)
}

func NewEducateSelectTopic(buffer *[]byte, client *connection.Client) (int, int, error) {
	return neweducate.NewEducateSelectTopic(buffer, client)
}

func NewEducateGetTalents(buffer *[]byte, client *connection.Client) (int, int, error) {
	return neweducate.NewEducateGetTalents(buffer, client)
}

func NewEducateRefreshTalent(buffer *[]byte, client *connection.Client) (int, int, error) {
	return neweducate.NewEducateRefreshTalent(buffer, client)
}

func NewEducateSelectTalent(buffer *[]byte, client *connection.Client) (int, int, error) {
	return neweducate.NewEducateSelectTalent(buffer, client)
}

func NewEducateChangePhase(buffer *[]byte, client *connection.Client) (int, int, error) {
	return neweducate.NewEducateChangePhase(buffer, client)
}

func NewEducateUpgradeFavor(buffer *[]byte, client *connection.Client) (int, int, error) {
	return neweducate.NewEducateUpgradeFavor(buffer, client)
}

func NewEducateTriggerNode(buffer *[]byte, client *connection.Client) (int, int, error) {
	return neweducate.NewEducateTriggerNode(buffer, client)
}

func NewEducateClearNodeChain(buffer *[]byte, client *connection.Client) (int, int, error) {
	return neweducate.NewEducateClearNodeChain(buffer, client)
}

func NewEducateSchedule(buffer *[]byte, client *connection.Client) (int, int, error) {
	return neweducate.NewEducateSchedule(buffer, client)
}

func NewEducateNextPlan(buffer *[]byte, client *connection.Client) (int, int, error) {
	return neweducate.NewEducateNextPlan(buffer, client)
}

func NewEducateUpgradePlan(buffer *[]byte, client *connection.Client) (int, int, error) {
	return neweducate.NewEducateUpgradePlan(buffer, client)
}

func NewEducateScheduleSkip(buffer *[]byte, client *connection.Client) (int, int, error) {
	return neweducate.NewEducateScheduleSkip(buffer, client)
}

func NewEducateGetExtraDrop(buffer *[]byte, client *connection.Client) (int, int, error) {
	return neweducate.NewEducateGetExtraDrop(buffer, client)
}

func NewEducateEnterAssess(buffer *[]byte, client *connection.Client) (int, int, error) {
	return neweducate.NewEducateEnterAssess(buffer, client)
}

func NewEducateGetChoose(buffer *[]byte, client *connection.Client) (int, int, error) {
	return neweducate.NewEducateGetChoose(buffer, client)
}

func NewEducateRequestChoices(buffer *[]byte, client *connection.Client) (int, int, error) {
	return neweducate.NewEducateRequestChoices(buffer, client)
}

func NewEducateRefreshChoice(buffer *[]byte, client *connection.Client) (int, int, error) {
	return neweducate.NewEducateRefreshChoice(buffer, client)
}

func NewEducateMakeChoice(buffer *[]byte, client *connection.Client) (int, int, error) {
	return neweducate.NewEducateMakeChoice(buffer, client)
}

func NewEducateGiveUpChoice(buffer *[]byte, client *connection.Client) (int, int, error) {
	return neweducate.NewEducateGiveUpChoice(buffer, client)
}

func NewEducateReplaceTarot(buffer *[]byte, client *connection.Client) (int, int, error) {
	return neweducate.NewEducateReplaceTarot(buffer, client)
}

func NewEducateUpgradeEntry(buffer *[]byte, client *connection.Client) (int, int, error) {
	return neweducate.NewEducateUpgradeEntry(buffer, client)
}

func NewEducateGiveUpEntryUp(buffer *[]byte, client *connection.Client) (int, int, error) {
	return neweducate.NewEducateGiveUpEntryUp(buffer, client)
}

func NewEducateRefreshShop(buffer *[]byte, client *connection.Client) (int, int, error) {
	return neweducate.NewEducateRefreshShop(buffer, client)
}

func NewEducateGetMap(buffer *[]byte, client *connection.Client) (int, int, error) {
	return neweducate.NewEducateGetMap(buffer, client)
}

func NewEducateMapNormal(buffer *[]byte, client *connection.Client) (int, int, error) {
	return neweducate.NewEducateMapNormal(buffer, client)
}

func NewEducateMapEvent(buffer *[]byte, client *connection.Client) (int, int, error) {
	return neweducate.NewEducateMapEvent(buffer, client)
}

func NewEducateShopping(buffer *[]byte, client *connection.Client) (int, int, error) {
	return neweducate.NewEducateShopping(buffer, client)
}

func NewEducateMapShip(buffer *[]byte, client *connection.Client) (int, int, error) {
	return neweducate.NewEducateMapShip(buffer, client)
}

func NewEducateUpgradeNormalSite(buffer *[]byte, client *connection.Client) (int, int, error) {
	return neweducate.NewEducateUpgradeNormalSite(buffer, client)
}

func NewEducateSelectMind(buffer *[]byte, client *connection.Client) (int, int, error) {
	return neweducate.NewEducateSelectMind(buffer, client)
}

func NewEducateRefresh(buffer *[]byte, client *connection.Client) (int, int, error) {
	return neweducate.NewEducateRefresh(buffer, client)
}

func NewEducateRequest(buffer *[]byte, client *connection.Client) (int, int, error) {
	return neweducate.NewEducateRequest(buffer, client)
}
