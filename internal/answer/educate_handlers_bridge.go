package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func EducateExecutePlans(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_27002{}, &protobuf.SC_27003{}, 27003)
}
func EducateGetEvents(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_27014{}, &protobuf.SC_27015{}, 27015)
}
func EducateGetPlans(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_27012{}, &protobuf.SC_27013{}, 27013)
}
func EducateGetTargetAward(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_27035{}, &protobuf.SC_27036{}, 27036)
}
func EducateUpgradeFavor(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_27006{}, &protobuf.SC_27007{}, 27007)
}
func EducateTriggerEnd(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_27008{}, &protobuf.SC_27009{}, 27009)
}
func EducateGetEndings(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_27010{}, &protobuf.SC_27011{}, 27011)
}
func EducateSetTarget(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_27019{}, &protobuf.SC_27020{}, 27020)
}
func EducateSubmitTask(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_27023{}, &protobuf.SC_27024{}, 27024)
}
func EducateSetCall(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_27031{}, &protobuf.SC_27032{}, 27032)
}
func EducateAddTaskProgress(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_27037{}, &protobuf.SC_27038{}, 27038)
}
func EducateAddExtraAttr(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_27039{}, &protobuf.SC_27040{}, 27040)
}
func ChangeEducateCharacter(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_27041{}, &protobuf.SC_27042{}, 27042)
}
func EducateMapSiteOperate(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_27004{}, &protobuf.SC_27005{}, 27005)
}
func EducateRefresh(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_27047{}, &protobuf.SC_27048{}, 27048)
}
func EducateRequest(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_27000{}, &protobuf.SC_27001{}, 27001)
}
func EducateRequestOption(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_27045{}, &protobuf.SC_27046{}, 27046)
}
func EducateRequestShopData(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_27043{}, &protobuf.SC_27044{}, 27044)
}
func EducateReset(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_27029{}, &protobuf.SC_27030{}, 27030)
}
func EducateShopping(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_27033{}, &protobuf.SC_27034{}, 27034)
}
func EducateTriggerEvent(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_27016{}, &protobuf.SC_27017{}, 27017)
}
func EducateTriggerSpecEvent(buffer *[]byte, client *connection.Client) (int, int, error) {
	return unavailableEducate(buffer, client, &protobuf.CS_27027{}, &protobuf.SC_27028{}, 27028)
}
