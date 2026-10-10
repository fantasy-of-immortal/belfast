package neweducate

import (
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func applyNewEducateTalentSelection(state *educateState, talentID uint32) *protobuf.TBDROPS {
	resetEducateHeldBenefit(state, talentID)
	if state.Info.Benefit == nil {
		state.Info.Benefit = &protobuf.TBBENEFIT{Actives: []*protobuf.TBBF{}}
	}
	state.Info.Benefit.Actives = upsertTBBF(state.Info.Benefit.Actives, talentID, state.Info.Round.GetRound(), 0)
	state.Permanent.TarotArchive = appendUniqueUint32(state.Permanent.TarotArchive, talentID)

	return &protobuf.TBDROPS{
		BaseDrop: []*protobuf.TBDROP{{
			Type:   proto.Uint32(4),
			Id:     proto.Uint32(talentID),
			Number: proto.Int32(1),
		}},
		BenefitDrop: []*protobuf.TBDROP{},
		Display:     emptyTBDisplay(),
	}
}

func upsertTBBF(values []*protobuf.TBBF, id uint32, round uint32, isPending uint32) []*protobuf.TBBF {
	for _, entry := range values {
		if entry.GetId() == id {
			entry.Round = proto.Uint32(round)
			entry.IsPending = proto.Uint32(isPending)
			return values
		}
	}

	return append(values, &protobuf.TBBF{
		Id:        proto.Uint32(id),
		Round:     proto.Uint32(round),
		IsPending: proto.Uint32(isPending),
	})
}
