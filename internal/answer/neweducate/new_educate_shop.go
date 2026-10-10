package neweducate

import (
	"fmt"
	"math"

	"github.com/ggmolly/belfast/internal/protobuf"
)

// The client sends a good ID, not a shop-site ID. Only already offered goods
// may be purchased; shelf generation and benefit/choice delivery need their
// own recovered contracts. Payment, delivery and stock commit together.
func purchaseEducateNumericGood(state *educateState, id, quantity uint32) (*protobuf.TBDROPS, error) {
	if !educateCanPlanInput(state) || educateHasPending(state.Info) || quantity == 0 {
		return nil, errEducatePhase
	}
	if err := validateEducateNumericActives(state); err != nil {
		return nil, err
	}
	site := ensureEducateCache(state.Info).CacheSite[0]
	offered := false
	for _, good := range site.Shops {
		if good == id {
			offered = true
			break
		}
	}
	if !offered {
		return nil, fmt.Errorf("good %d is not offered", id)
	}
	good, found, err := loadNewEducateConfigByID[newEducateShopConfig](newEducateShopCategory, id)
	if err != nil {
		return nil, err
	}
	if !found || (good.GoodsType != 2 && good.GoodsType != 3) {
		return nil, fmt.Errorf("good %d delivery requires recovery", id)
	}
	count := uint64(educateKVCount(site.Buys, id)) + uint64(quantity)
	if count > math.MaxUint32 || good.LimitNum < -1 || (good.LimitNum != -1 && count > uint64(good.LimitNum)) {
		return nil, fmt.Errorf("good %d stock limit", id)
	}
	resourceID, found, err := resolveNewEducateResourceID(state, good.ResourceType)
	if err != nil {
		return nil, err
	}
	if !found || resourceID > math.MaxInt32 || good.ResourceNum > math.MaxInt32 || good.GoodsID > math.MaxInt32 || good.GoodsNum == 0 || good.GoodsNum > math.MaxInt32 {
		return nil, fmt.Errorf("good %d invalid numeric contract", id)
	}
	// Goods types 2/3 map to numeric drop types 1/2, as in NewEducateGoods.
	row := []int32{int32(good.GoodsType - 1), int32(good.GoodsID), int32(good.GoodsNum)}
	if _, err := applyEducateNumericBatch(state, [][]int32{{2, int32(resourceID), int32(good.ResourceNum)}}, quantity, true); err != nil {
		return nil, err
	}
	delivery, err := applyEducateDropBatch(state, [][]int32{row}, quantity, educateChangeContext(state, educateActionID(state, fmt.Sprintf("shop:%d", id)), 0, nil))
	if err != nil {
		return nil, err
	}
	site.Buys = upsertKVDATACount(site.Buys, id, quantity)
	drops := emptyTBDrops()
	drops.BaseDrop = delivery
	return drops, nil
}
