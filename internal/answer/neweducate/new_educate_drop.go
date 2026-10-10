package neweducate

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

type educatePolaroidConfig struct {
	ID        uint32 `json:"id"`
	Character uint32 `json:"character"`
}

// Drop kinds use the client's distinct delivery paths. A missing choice/tarot
// workflow is an explicit error; its reward cannot be discarded after payment.
func applyEducateDropBatch(state *educateState, rows [][]int32, multiplier uint32, context *educateConditionContext) ([]*protobuf.TBDROP, error) {
	if state.Permanent == nil || state.Lifecycle == nil {
		return nil, fmt.Errorf("drop delivery requires complete role state")
	}
	candidate := *state
	candidate.Info = proto.Clone(state.Info).(*protobuf.TBINFO)
	candidate.Permanent = proto.Clone(state.Permanent).(*protobuf.TBPERMANENT)
	raw, err := json.Marshal(state.Lifecycle)
	if err != nil {
		return nil, err
	}
	candidate.Lifecycle = &educateLifecycle{}
	if err := json.Unmarshal(raw, candidate.Lifecycle); err != nil {
		return nil, err
	}
	actual := []*protobuf.TBDROP{}
	for index, row := range rows {
		if len(row) != 3 || row[0] <= 0 || row[1] < 0 {
			return nil, fmt.Errorf("drop[%d]: invalid triplet", index)
		}
		if row[0] == 1 || row[0] == 2 {
			drops, err := applyEducateGainBatch(&candidate, [][]int32{row}, multiplier, context)
			if err != nil {
				return nil, err
			}
			actual = append(actual, drops...)
			continue
		}
		amount := int64(row[2]) * int64(multiplier)
		if amount < math.MinInt32 || amount > math.MaxInt32 {
			return nil, fmt.Errorf("drop[%d]: amount exceeds protocol", index)
		}
		switch row[0] {
		case 3:
			photo, ok, err := loadNewEducateConfigByID[educatePolaroidConfig]("ShareCfg/child2_polaroid.json", uint32(row[1]))
			if err != nil {
				return nil, err
			}
			if !ok || photo.Character != state.Info.GetId() || amount != 1 {
				return nil, fmt.Errorf("drop[%d]: polaroid requires owned configuration and one acquisition", index)
			}
			candidate.Permanent.Polaroids = append(candidate.Permanent.Polaroids, photo.ID)
		case 4:
			if amount != 1 {
				return nil, fmt.Errorf("drop[%d]: benefit acquisition/removal workflow requires recovery", index)
			}
			if _, err := educateNumericTalentBenefits(&candidate, uint32(row[1])); err != nil {
				return nil, err
			}
			if candidate.Info.Fsm.GetSystemNo() == newEducateSystemPlan || candidate.Info.Fsm.GetSystemNo() == newEducateSystemAssess || candidate.Info.Fsm.GetSystemNo() == newEducateSystemPhase {
				return nil, fmt.Errorf("drop[%d]: pending benefit activation requires recovery", index)
			}
			applyNewEducateTalentSelection(&candidate, uint32(row[1]))
			candidate.Info.Talent.Talents = appendUniqueUint32(candidate.Info.Talent.Talents, uint32(row[1]))
			immediate, err := applyEducateTalentTriggerFor(&candidate, 13, uint32(row[1]))
			if err != nil {
				return nil, err
			}
			actual = append(actual, immediate.BenefitDrop...)
		case 7:
			if amount <= 0 || uint64(candidate.Info.Round.GetTempRound())+uint64(amount) > math.MaxUint32 {
				return nil, fmt.Errorf("drop[%d]: invalid temporary round award", index)
			}
			candidate.Info.Round.TempRound = proto.Uint32(candidate.Info.Round.GetTempRound() + uint32(amount))
			candidate.Lifecycle.TempRound = candidate.Info.Round.GetTempRound()
		case 5, 6, 10000:
			return nil, fmt.Errorf("drop[%d]: choice/entry/tarot type %d requires its recovered delivery workflow", index, row[0])
		default:
			return nil, fmt.Errorf("drop[%d]: unknown delivery type %d", index, row[0])
		}
		actual = append(actual, &protobuf.TBDROP{Type: proto.Uint32(uint32(row[0])), Id: proto.Uint32(uint32(row[1])), Number: proto.Int32(int32(amount))})
	}
	state.Info.Res, state.Info.Benefit, state.Info.Talent = candidate.Info.Res, candidate.Info.Benefit, candidate.Info.Talent
	state.Info.Round.TempRound = candidate.Info.Round.TempRound
	state.Permanent.Polaroids, state.Permanent.TarotArchive = candidate.Permanent.Polaroids, candidate.Permanent.TarotArchive
	state.Lifecycle.NumericLedger, state.Lifecycle.ConditionDraws = candidate.Lifecycle.NumericLedger, candidate.Lifecycle.ConditionDraws
	state.Lifecycle.TempRound = candidate.Lifecycle.TempRound
	return actual, nil
}
