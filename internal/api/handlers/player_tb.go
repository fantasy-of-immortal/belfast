package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/kataras/iris/v12"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/api/response"
	"github.com/ggmolly/belfast/internal/api/types"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// PlayerTB godoc
// @Summary     Get player TB state
// @Tags        Players
// @Produce     json
// @Param       id   path  int  true  "Player ID"
// @Success     200  {object}  CommanderTBResponseDoc
// @Failure     404  {object}  APIErrorResponseDoc
// @Failure     500  {object}  APIErrorResponseDoc
// @Router      /api/v1/players/{id}/tb [get]
func (handler *PlayerHandler) PlayerTB(ctx iris.Context) {
	characterID, ok := playerTBCharacter(ctx)
	if !ok {
		return
	}
	commander, err := loadCommanderDetail(ctx)
	if err != nil {
		writeCommanderError(ctx, err)
		return
	}
	entry, err := orm.GetCommanderTB(commander.CommanderID, characterID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			ctx.StatusCode(http.StatusNotFound)
			_ = ctx.JSON(response.Error("not_found", "tb state not found", nil))
			return
		}
		ctx.StatusCode(http.StatusInternalServerError)
		_ = ctx.JSON(response.Error("internal_error", "failed to load tb state", nil))
		return
	}
	info, permanent, err := entry.Decode()
	if err != nil {
		ctx.StatusCode(http.StatusInternalServerError)
		_ = ctx.JSON(response.Error("internal_error", "failed to decode tb state", nil))
		return
	}
	infoJSON, err := protojson.Marshal(info)
	if err != nil {
		ctx.StatusCode(http.StatusInternalServerError)
		_ = ctx.JSON(response.Error("internal_error", "failed to encode tb state", nil))
		return
	}
	permanentJSON, err := protojson.Marshal(permanent)
	if err != nil {
		ctx.StatusCode(http.StatusInternalServerError)
		_ = ctx.JSON(response.Error("internal_error", "failed to encode tb state", nil))
		return
	}
	payload := types.CommanderTBPayload{
		CommanderID: commander.CommanderID,
		Tb:          types.RawJSON{Value: infoJSON},
		Permanent:   types.RawJSON{Value: permanentJSON},
	}
	_ = ctx.JSON(response.Success(payload))
}

// CreatePlayerTB godoc
// @Summary     Create player TB state
// @Tags        Players
// @Accept      json
// @Produce     json
// @Param       id   path  int  true  "Player ID"
// @Param       body body  types.CommanderTBRequest true "TB payload"
// @Success     200  {object}  CommanderTBResponseDoc
// @Failure     400  {object}  APIErrorResponseDoc
// @Failure     404  {object}  APIErrorResponseDoc
// @Failure     409  {object}  APIErrorResponseDoc
// @Failure     500  {object}  APIErrorResponseDoc
// @Router      /api/v1/players/{id}/tb [post]
func (handler *PlayerHandler) CreatePlayerTB(ctx iris.Context) {
	characterID, ok := playerTBCharacter(ctx)
	if !ok {
		return
	}
	commander, err := loadCommanderDetail(ctx)
	if err != nil {
		writeCommanderError(ctx, err)
		return
	}
	var payload types.CommanderTBRequest
	if err := ctx.ReadJSON(&payload); err != nil {
		ctx.StatusCode(http.StatusBadRequest)
		_ = ctx.JSON(response.Error("bad_request", "invalid request payload", nil))
		return
	}
	if len(payload.Tb.Value) == 0 || len(payload.Permanent.Value) == 0 {
		ctx.StatusCode(http.StatusBadRequest)
		_ = ctx.JSON(response.Error("bad_request", "tb and permanent payloads are required", nil))
		return
	}
	if _, err := orm.GetCommanderTB(commander.CommanderID, characterID); err == nil {
		ctx.StatusCode(http.StatusConflict)
		_ = ctx.JSON(response.Error("conflict", "tb state already exists", nil))
		return
	} else if !db.IsNotFound(err) {
		ctx.StatusCode(http.StatusInternalServerError)
		_ = ctx.JSON(response.Error("internal_error", "failed to load tb state", nil))
		return
	}
	info := &protobuf.TBINFO{}
	if err := protojson.Unmarshal(payload.Tb.Value, info); err != nil {
		ctx.StatusCode(http.StatusBadRequest)
		_ = ctx.JSON(response.Error("bad_request", "invalid tb payload", nil))
		return
	}
	if info.GetId() != characterID {
		ctx.StatusCode(http.StatusBadRequest)
		_ = ctx.JSON(response.Error("bad_request", "tb.id must match character_id (default 1)", nil))
		return
	}
	permanent := &protobuf.TBPERMANENT{}
	if err := protojson.Unmarshal(payload.Permanent.Value, permanent); err != nil {
		ctx.StatusCode(http.StatusBadRequest)
		_ = ctx.JSON(response.Error("bad_request", "invalid permanent payload", nil))
		return
	}
	entry, err := orm.NewCommanderTB(commander.CommanderID, info, permanent)
	if err != nil {
		ctx.StatusCode(http.StatusInternalServerError)
		_ = ctx.JSON(response.Error("internal_error", "failed to encode tb state", nil))
		return
	}
	if err := orm.SaveCommanderTB(entry, info, permanent); err != nil {
		ctx.StatusCode(http.StatusInternalServerError)
		_ = ctx.JSON(response.Error("internal_error", "failed to save tb state", nil))
		return
	}
	responsePayload := types.CommanderTBPayload{
		CommanderID: commander.CommanderID,
		Tb:          payload.Tb,
		Permanent:   payload.Permanent,
	}
	_ = ctx.JSON(response.Success(responsePayload))
}

// UpdatePlayerTB godoc
// @Summary     Update player TB state
// @Tags        Players
// @Accept      json
// @Produce     json
// @Param       id   path  int  true  "Player ID"
// @Param       body body  types.CommanderTBRequest true "TB payload"
// @Success     200  {object}  CommanderTBResponseDoc
// @Failure     400  {object}  APIErrorResponseDoc
// @Failure     404  {object}  APIErrorResponseDoc
// @Failure     500  {object}  APIErrorResponseDoc
// @Router      /api/v1/players/{id}/tb [put]
func (handler *PlayerHandler) UpdatePlayerTB(ctx iris.Context) {
	characterID, ok := playerTBCharacter(ctx)
	if !ok {
		return
	}
	commander, err := loadCommanderDetail(ctx)
	if err != nil {
		writeCommanderError(ctx, err)
		return
	}
	var payload types.CommanderTBRequest
	if err := ctx.ReadJSON(&payload); err != nil {
		ctx.StatusCode(http.StatusBadRequest)
		_ = ctx.JSON(response.Error("bad_request", "invalid request payload", nil))
		return
	}
	if len(payload.Tb.Value) == 0 || len(payload.Permanent.Value) == 0 {
		ctx.StatusCode(http.StatusBadRequest)
		_ = ctx.JSON(response.Error("bad_request", "tb and permanent payloads are required", nil))
		return
	}
	entry, err := orm.GetCommanderTB(commander.CommanderID, characterID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			ctx.StatusCode(http.StatusNotFound)
			_ = ctx.JSON(response.Error("not_found", "tb state not found", nil))
			return
		}
		ctx.StatusCode(http.StatusInternalServerError)
		_ = ctx.JSON(response.Error("internal_error", "failed to load tb state", nil))
		return
	}
	info := &protobuf.TBINFO{}
	if err := protojson.Unmarshal(payload.Tb.Value, info); err != nil {
		ctx.StatusCode(http.StatusBadRequest)
		_ = ctx.JSON(response.Error("bad_request", "invalid tb payload", nil))
		return
	}
	if info.GetId() != entry.CharacterID {
		ctx.StatusCode(http.StatusBadRequest)
		_ = ctx.JSON(response.Error("bad_request", "tb.id must match character_id", nil))
		return
	}
	permanent := &protobuf.TBPERMANENT{}
	if err := protojson.Unmarshal(payload.Permanent.Value, permanent); err != nil {
		ctx.StatusCode(http.StatusBadRequest)
		_ = ctx.JSON(response.Error("bad_request", "invalid permanent payload", nil))
		return
	}
	var metadata struct {
		Lifecycle json.RawMessage `json:"lifecycle"`
	}
	if err := json.Unmarshal(entry.Metadata, &metadata); err != nil {
		ctx.StatusCode(http.StatusInternalServerError)
		_ = ctx.JSON(response.Error("internal_error", "invalid stored educate metadata", nil))
		return
	}
	if len(metadata.Lifecycle) != 0 {
		previous, previousPermanent, err := entry.Decode()
		if err != nil {
			ctx.StatusCode(http.StatusInternalServerError)
			_ = ctx.JSON(response.Error("internal_error", "invalid stored educate snapshot", nil))
			return
		}
		if !proto.Equal(previous, info) || !proto.Equal(previousPermanent, permanent) {
			ctx.StatusCode(http.StatusConflict)
			_ = ctx.JSON(response.Error("conflict", "managed educate saves must be changed through game transactions", nil))
			return
		}
	} else if err := orm.SaveCommanderTB(entry, info, permanent); err != nil {
		ctx.StatusCode(http.StatusInternalServerError)
		_ = ctx.JSON(response.Error("internal_error", "failed to save tb state", nil))
		return
	}
	responsePayload := types.CommanderTBPayload{
		CommanderID: commander.CommanderID,
		Tb:          payload.Tb,
		Permanent:   payload.Permanent,
	}
	_ = ctx.JSON(response.Success(responsePayload))
}

// DeletePlayerTB godoc
// @Summary     Delete player TB state
// @Tags        Players
// @Produce     json
// @Param       id   path  int  true  "Player ID"
// @Success     200  {object}  OKResponseDoc
// @Failure     404  {object}  APIErrorResponseDoc
// @Failure     500  {object}  APIErrorResponseDoc
// @Router      /api/v1/players/{id}/tb [delete]
func (handler *PlayerHandler) DeletePlayerTB(ctx iris.Context) {
	characterID, ok := playerTBCharacter(ctx)
	if !ok {
		return
	}
	commander, err := loadCommanderDetail(ctx)
	if err != nil {
		writeCommanderError(ctx, err)
		return
	}
	deleted, err := orm.DeleteCommanderTB(commander.CommanderID, characterID)
	if err != nil {
		ctx.StatusCode(http.StatusInternalServerError)
		_ = ctx.JSON(response.Error("internal_error", "failed to delete tb state", nil))
		return
	}
	if !deleted {
		ctx.StatusCode(http.StatusNotFound)
		_ = ctx.JSON(response.Error("not_found", "tb state not found", nil))
		return
	}
	_ = ctx.JSON(response.Success(nil))
}

// Existing local admin URLs refer deterministically to character 1; callers
// editing another character must explicitly pass ?character_id=2.
func playerTBCharacter(ctx iris.Context) (uint32, bool) {
	raw := ctx.URLParam("character_id")
	if raw == "" {
		return 1, true
	}
	id, err := strconv.ParseUint(raw, 10, 32)
	if err != nil || id == 0 {
		ctx.StatusCode(http.StatusBadRequest)
		_ = ctx.JSON(response.Error("bad_request", "invalid character_id", nil))
		return 0, false
	}
	return uint32(id), true
}
