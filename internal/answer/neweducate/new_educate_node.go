package neweducate

import (
	"encoding/json"
	"fmt"
)

// The node panel sends a zero-based UI index for MAIN_TEXT options, a
// selected node ID for site options, and a story flag for STORY_BRANCH.
// Keep these distinct: accepting both an ID and index hides invalid requests.
// This parser never chooses the first item of an unknown array shape.
func resolveEducateNodeNext(node *newEducateNodeConfig, branch uint32, draw func(uint64) (uint64, error)) (uint32, error) {
	fail := func(err error) (uint32, error) {
		return 0, fmt.Errorf("%s/%d next_type=%d branch=%d: %w", newEducateNodeCategory, node.ID, node.NextType, branch, err)
	}
	switch node.NextType {
	case 1:
		if branch != 0 {
			return fail(fmt.Errorf("fixed successor does not accept branch"))
		}
		next, err := parseEducateFixedNodeNext(node.Next)
		if err != nil {
			return fail(err)
		}
		return next, nil
	case 2:
		var ids []uint32
		if err := json.Unmarshal(node.Next, &ids); err != nil || len(ids) == 0 {
			return fail(fmt.Errorf("option requires nonempty node IDs"))
		}
		seen := map[uint32]bool{}
		for _, id := range ids {
			if id == 0 || seen[id] {
				return fail(fmt.Errorf("zero/duplicate option %d", id))
			}
			seen[id] = true
		}
		switch node.Type {
		case 103:
			if uint64(branch) >= uint64(len(ids)) {
				return fail(fmt.Errorf("option index out of range"))
			}
			return ids[branch], nil
		case 100, 101:
			if !seen[branch] {
				return fail(fmt.Errorf("option node not offered"))
			}
			return branch, nil
		default:
			return fail(fmt.Errorf("unsupported option presentation type %d", node.Type))
		}
	case 3, 4:
		var pairs [][]uint32
		if err := json.Unmarshal(node.Next, &pairs); err != nil || len(pairs) == 0 {
			return fail(fmt.Errorf("requires nonempty pairs"))
		}
		var total uint64
		seen := map[uint32]bool{}
		for _, p := range pairs {
			if len(p) != 2 {
				return fail(fmt.Errorf("pair requires two integers"))
			}
			if seen[p[0]] {
				return fail(fmt.Errorf("duplicate node/flag %d", p[0]))
			}
			seen[p[0]] = true
			if node.NextType == 3 {
				if p[1] == 0 {
					return fail(fmt.Errorf("zero probability weight"))
				}
				total += uint64(p[1])
			}
		}
		if node.NextType == 4 {
			if node.Type != 2 {
				return fail(fmt.Errorf("story flag requires STORY_BRANCH"))
			}
			for _, p := range pairs {
				if p[0] == branch {
					return p[1], nil
				}
			}
			return fail(fmt.Errorf("story flag not offered"))
		}
		if branch != 0 || draw == nil {
			return fail(fmt.Errorf("probability requires server draw and branch zero"))
		}
		value, err := draw(total)
		if err != nil {
			return fail(err)
		}
		if value >= total {
			return fail(fmt.Errorf("random value %d outside [0,%d)", value, total))
		}
		for _, p := range pairs {
			if value < uint64(p[1]) {
				return p[0], nil
			}
			value -= uint64(p[1])
		}
	}
	return fail(fmt.Errorf("unknown next type"))
}
