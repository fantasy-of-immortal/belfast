package neweducate

import (
	"fmt"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

// The private pool identifiers are opaque. S03 preserves their source and
// quantity; S06 owns candidate generation and the 291xx choice interaction.
type educatePrioritySource struct {
	Type   uint32 `json:"type"`
	ID     uint32 `json:"id"`
	Action string `json:"action"`
}

func deliverEducatePriorityDrop(state *educateState, kind, id uint32, amount int64, context *educateConditionContext) error {
	if amount <= 0 || amount > 128 || len(state.Info.Fsm.PriorityFsm)+len(state.Info.Fsm.TarotSelects)+int(amount) > 128 {
		return fmt.Errorf("priority delivery type %d: invalid quantity or queue limit", kind)
	}
	switch kind {
	case 5:
		// Every restored round pool reference is 1000 or 1001, for role 2.
		// Do not invent a pool for an arbitrary ID supplied by a malformed config.
		if state.Info.GetId() != 2 || (id != 1000 && id != 1001) {
			return fmt.Errorf("choice pool %d: no restored character pool contract", id)
		}
	case 6:
		if id != 0 {
			return fmt.Errorf("entry upgrade delivery requires id 0")
		}
	case 10000:
		list, _, err := loadEducateBenefitDefinition(state, id)
		if err != nil {
			return err
		}
		if list.Type != 3 {
			return fmt.Errorf("replacement %d is not a tarot", id)
		}
	default:
		return fmt.Errorf("unknown priority drop type %d", kind)
	}
	action := educateActionID(state, "priority")
	if context != nil && context.ExecutionID != "" {
		action = context.ExecutionID
	}
	for i := int64(0); i < amount; i++ {
		if kind == 10000 {
			state.Info.Fsm.TarotSelects = append(state.Info.Fsm.TarotSelects, id)
			continue
		}
		cache := &protobuf.TBFSMCACHE{}
		system := uint32(100)
		if kind == 5 {
			cache.CacheNin1 = []*protobuf.TBFSMCACHENIN1{{Selects: []*protobuf.TBDROP{}, RerollCount: []uint32{}, IsFromShop: proto.Uint32(0)}}
		} else {
			system = 101
			cache.CacheAffixUp = []*protobuf.TBFSMCACHEAFFIXUP{{}}
		}
		pending := &protobuf.TBFSM{SystemNo: proto.Uint32(system), CurrentNode: proto.Uint32(0), Cache: []*protobuf.TBFSMCACHE{cache}}
		state.Info.Fsm.PriorityFsm = append([]*protobuf.TBFSM{pending}, state.Info.Fsm.PriorityFsm...)
		state.Lifecycle.PrioritySources = append([]educatePrioritySource{{Type: kind, ID: id, Action: action}}, state.Lifecycle.PrioritySources...)
	}
	return nil
}
