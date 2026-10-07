package neweducate

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestRecoveryS03ConditionsRejectOtherCharacterValues(t *testing.T) {
	client := recoveryDB(t)
	cases := []struct {
		name                                                                        string
		character, condition, foreignCondition, kind, ownedID, foreignID, threshold uint32
	}{
		{"leader attribute", 1, 1, 211, 1, 101, 301, 50},
		{"explorer attribute", 2, 211, 1, 1, 301, 101, 100},
		{"leader money", 1, 10008, 209, 2, 1, 301, 100},
		{"explorer money", 2, 209, 10008, 2, 301, 1, 50},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := defaultEducateState(client.Commander.CommanderID, tc.character)
			values := &state.Info.Res.Attrs
			if tc.kind == 2 {
				values = &state.Info.Res.Resource
			}
			for _, entry := range *values {
				if entry.GetKey() == tc.ownedID {
					entry.Value = proto.Uint32(tc.threshold)
				}
			}
			// Historical mixed fields are retained in storage, but the client
			// projection hides them. They cannot satisfy another role's gate.
			*values = append(*values, &protobuf.KVDATA{Key: proto.Uint32(tc.foreignID), Value: proto.Uint32(1000)})
			before := proto.Clone(state.Info)
			owned, err := evaluateEducateCondition(state, json.RawMessage(fmt.Sprint(tc.condition)))
			if err != nil || !owned {
				t.Fatalf("real owned condition rejected: %v %v", owned, err)
			}
			foreign, err := evaluateEducateCondition(state, json.RawMessage(fmt.Sprint(tc.foreignCondition)))
			if err == nil || foreign || !strings.Contains(err.Error(), "character") || !strings.Contains(err.Error(), fmt.Sprint(tc.foreignCondition)+"/param") {
				t.Fatalf("mixed foreign value used by gate: %v %v", foreign, err)
			}
			if !proto.Equal(before, state.Info) {
				t.Fatal("condition evaluation changed retained save fields")
			}
		})
	}
}
