package neweducate

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

type educateNumericConfig struct {
	ID        uint32 `json:"id"`
	Character uint32 `json:"character"`
	Min       int64  `json:"min_value"`
	Max       int64  `json:"max_value"`
}

// Numeric batches are validated on a clone. A later invalid item cannot leave
// a partial payment or reward in the caller's state.
func applyEducateNumericBatch(state *educateState, rows [][]int32, multiplier uint32, cost bool) ([]*protobuf.TBDROP, error) {
	candidate := proto.Clone(state.Info.Res).(*protobuf.TBRES)
	actual := make([]*protobuf.TBDROP, 0, len(rows))
	for i, row := range rows {
		if len(row) != 3 || row[1] <= 0 {
			return nil, fmt.Errorf("drop[%d]: expected [type,positive id,amount]", i)
		}
		category := newEducateAttrCategory
		values := &candidate.Attrs
		switch row[0] {
		case 1:
		case 2:
			category = newEducateResourceCategory
			values = &candidate.Resource
		default:
			return nil, fmt.Errorf("drop[%d]: unsupported numeric drop type %d", i, row[0])
		}
		config, found, err := loadNewEducateConfigByID[educateNumericConfig](category, uint32(row[1]))
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, fmt.Errorf("%s/%d: missing numeric configuration", category, row[1])
		}
		if config.Character != state.Info.GetId() || config.Min < 0 || config.Max < config.Min || config.Max > math.MaxUint32 {
			return nil, fmt.Errorf("%s/%d: invalid bounds or character", category, row[1])
		}
		var entry *protobuf.KVDATA
		for _, v := range *values {
			if v.GetKey() == uint32(row[1]) {
				if entry != nil {
					return nil, fmt.Errorf("duplicate stored numeric id %d", row[1])
				}
				entry = v
			}
		}
		if entry == nil {
			return nil, fmt.Errorf("missing stored numeric id %d", row[1])
		}
		before := int64(entry.GetValue())
		if before < config.Min || before > config.Max {
			return nil, fmt.Errorf("stored numeric id %d outside configuration bounds", row[1])
		}
		delta := int64(row[2]) * int64(multiplier) // int32 * uint32 fits int64.
		if cost {
			if row[2] < 0 {
				return nil, fmt.Errorf("negative cost for id %d", row[1])
			}
			delta = -delta
		}
		after := before + delta
		// A positive reward can overflow the int64 sum at extreme wire values.
		if delta > 0 && after < before {
			return nil, fmt.Errorf("numeric overflow for id %d", row[1])
		}
		if cost && after < config.Min {
			return nil, fmt.Errorf("insufficient id %d: have %d cost %d", row[1], before, -delta)
		}
		if after < config.Min {
			after = config.Min
		}
		if after > config.Max {
			after = config.Max
		}
		applied := after - before
		if applied < math.MinInt32 || applied > math.MaxInt32 {
			return nil, fmt.Errorf("actual drop id %d exceeds protocol int32", row[1])
		}
		entry.Value = proto.Uint32(uint32(after))
		actual = append(actual, &protobuf.TBDROP{Type: proto.Uint32(uint32(row[0])), Id: proto.Uint32(uint32(row[1])), Number: proto.Int32(int32(applied))})
	}
	state.Info.Res = candidate
	return actual, nil
}

func parseEducateDropTriplets(raw json.RawMessage, category, key, field string) ([][]int32, error) {
	var rows [][]int32
	if len(raw) == 0 || string(raw) == "null" {
		return nil, fmt.Errorf("%s/%s/%s: missing triplets", category, key, field)
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("%s/%s/%s: %w", category, key, field, err)
	}
	if rows == nil {
		return nil, fmt.Errorf("%s/%s/%s: missing triplets", category, key, field)
	}
	for i, row := range rows {
		if len(row) != 3 || row[0] <= 0 || row[1] <= 0 {
			return nil, fmt.Errorf("%s/%s/%s[%d]: invalid drop triplet", category, key, field, i)
		}
	}
	return rows, nil
}
