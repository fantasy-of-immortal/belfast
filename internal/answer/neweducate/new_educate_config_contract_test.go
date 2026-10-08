package neweducate

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestRecoveryS03NullCannotEndNodeOrEraseDrops(t *testing.T) {
	for _, raw := range []string{"null", " null ", "\nnull\t", ""} {
		node := &newEducateNodeConfig{ID: 3700001, NextType: 1, Next: json.RawMessage(raw)}
		if _, err := resolveEducateNodeNext(node, 0, nil); err == nil || !strings.Contains(err.Error(), "3700001") {
			t.Fatalf("null successor accepted without node context: %q %v", raw, err)
		}
		if _, err := parseEducateDropTriplets(json.RawMessage(raw), newEducatePlanCategory, "1201", "result_display"); err == nil || !strings.Contains(err.Error(), "1201/result_display") {
			t.Fatalf("null drops accepted without field context: %q %v", raw, err)
		}
	}
	for _, raw := range []string{`0`, `"0"`, `""`} {
		if next, err := parseEducateFixedNodeNext(json.RawMessage(raw)); err != nil || next != 0 {
			t.Fatal("configured terminal marker rejected", raw, next, err)
		}
	}
	if rows, err := parseEducateDropTriplets(json.RawMessage(`[]`), "fixture", "1", "cost"); err != nil || len(rows) != 0 {
		t.Fatal("explicit empty drops rejected", err)
	}
	for _, raw := range []string{"null", " null ", ""} {
		var threshold int64
		if err := decodeEducateConditionParam([]json.RawMessage{json.RawMessage(raw)}, 0, &threshold); err == nil || !strings.Contains(err.Error(), "param[0]") {
			t.Fatal("null parameter silently became zero", raw, err)
		}
	}
}

func TestRecoveryS03RealRoundDifficultyAndEndlessMapping(t *testing.T) {
	recoveryDB(t)
	rows, err := listNewEducateConfigs[newEducateRoundConfig](newEducateRoundCategory)
	if err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]uint32{{1, 0}, {2, 0}, {2, 1}} {
		t.Run(fmt.Sprintf("character%d_difficulty%d", pair[0], pair[1]), func(t *testing.T) {
			normal := map[uint32]uint32{}
			var cycles []uint32
			for _, row := range rows {
				if row.Character != pair[0] || row.IsHardMode != pair[1] {
					continue
				}
				if row.RoundType == 1 {
					normal[row.Round] = row.ID
				} else {
					cycles = append(cycles, row.ID)
				}
			}
			sort.Slice(cycles, func(i, j int) bool { return cycles[i] < cycles[j] })
			// Fixture has twenty normal rounds; explorer has ten endless IDs
			// whose round fields are all 1. They are indexed by sorted ID.
			if len(normal) != 20 || (pair[0] == 2 && len(cycles) != 10) {
				t.Fatalf("real timeline changed: normal=%d cycles=%d", len(normal), len(cycles))
			}
			info := &protobuf.TBINFO{Id: proto.Uint32(pair[0]), Difficulty: proto.Uint32(pair[1]), Round: &protobuf.TBROUND{}}
			for round := uint32(1); round <= uint32(len(normal)); round++ {
				info.Round.Round = proto.Uint32(round)
				got, found, err := loadCurrentNewEducateRoundConfig(info)
				if err != nil || !found || got.ID != normal[round] {
					t.Fatalf("normal round %d: %v %v", round, got, err)
				}
			}
			if len(cycles) == 0 {
				info.Round.Round = proto.Uint32(21)
				if _, _, err := loadCurrentNewEducateRoundConfig(info); err == nil {
					t.Fatal("role without endless configuration acquired a timeline")
				}
				return
			}
			for _, wave := range []uint32{1, 9, 10, 11, 20, 21} {
				info.Round.Round = proto.Uint32(20 + wave)
				got, found, err := loadCurrentNewEducateRoundConfig(info)
				if err != nil || !found || got.ID != cycles[(wave-1)%uint32(len(cycles))] {
					t.Fatalf("endless wave %d: %v %v", wave, got, err)
				}
			}
		})
	}
	// There is no hard timeline for role 1 in the real fixture. Never borrow
	// role 2's hard configuration, nor silently fall back to easy mode.
	info := &protobuf.TBINFO{Id: proto.Uint32(1), Difficulty: proto.Uint32(1), Round: &protobuf.TBROUND{Round: proto.Uint32(1)}}
	if _, _, err := loadCurrentNewEducateRoundConfig(info); err == nil {
		t.Fatal("missing hard timeline fell back to another role/difficulty")
	}
}
