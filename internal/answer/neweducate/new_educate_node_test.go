package neweducate

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestRecoveryS05NodeSuccessorContracts(t *testing.T) {
	tests := []struct {
		name              string
		kind, typ, branch uint32
		raw               string
		roll              uint64
		want              uint32
		bad               bool
	}{
		{"fixed", 1, 1, 0, `"3700002"`, 0, 3700002, false},
		{"terminal", 1, 102, 0, `""`, 0, 0, false},
		{"fixed array rejected", 1, 1, 0, `[4,5]`, 0, 0, true},
		{"fixed branch rejected", 1, 1, 1, `4`, 0, 0, true},
		{"main option zero", 2, 103, 0, `[142,143]`, 0, 142, false},
		{"main option one", 2, 103, 1, `[142,143]`, 0, 143, false},
		{"main node ID not index", 2, 103, 142, `[142,143]`, 0, 0, true},
		{"site node ID", 2, 100, 143, `[142,143]`, 0, 143, false},
		{"site index not ID", 2, 100, 1, `[142,143]`, 0, 0, true},
		{"duplicate option", 2, 103, 0, `[142,142]`, 0, 0, true},
		{"random lower", 3, 102, 0, `[[0,50],[30007,50]]`, 0, 0, false},
		{"random boundary minus", 3, 102, 0, `[[0,50],[30007,50]]`, 49, 0, false},
		{"random boundary", 3, 102, 0, `[[0,50],[30007,50]]`, 50, 30007, false},
		{"random upper", 3, 102, 0, `[[0,50],[30007,50]]`, 99, 30007, false},
		{"random outside", 3, 102, 0, `[[0,50],[30007,50]]`, 100, 0, true},
		{"random branch rejected", 3, 102, 1, `[[0,50],[30007,50]]`, 0, 0, true},
		{"zero weight", 3, 102, 0, `[[1,0]]`, 0, 0, true},
		{"malformed pair", 3, 102, 0, `[[1,2,3]]`, 0, 0, true},
		{"story flag first", 4, 2, 1, `[[1,10022],[2,10023]]`, 0, 10022, false},
		{"story flag second", 4, 2, 2, `[[1,10022],[2,10023]]`, 0, 10023, false},
		{"story unknown flag", 4, 2, 0, `[[1,10022],[2,10023]]`, 0, 0, true},
		{"story duplicate", 4, 2, 1, `[[1,10022],[1,10023]]`, 0, 0, true},
		{"null", 2, 103, 0, `null`, 0, 0, true},
		{"unknown type", 99, 1, 0, `1`, 0, 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			n := &newEducateNodeConfig{ID: 999, Type: tc.typ, NextType: tc.kind, Next: json.RawMessage(tc.raw)}
			got, err := resolveEducateNodeNext(n, tc.branch, func(uint64) (uint64, error) { return tc.roll, nil })
			if (err != nil) != tc.bad || (!tc.bad && got != tc.want) {
				t.Fatalf("next=%d err=%v want=%d bad=%v", got, err, tc.want, tc.bad)
			}
		})
	}
	n := &newEducateNodeConfig{ID: 999, NextType: 3, Next: json.RawMessage(`[[1,2]]`)}
	sentinel := errors.New("injected entropy failure")
	if _, err := resolveEducateNodeNext(n, 0, func(uint64) (uint64, error) { return 0, sentinel }); !errors.Is(err, sentinel) {
		t.Fatalf("lost draw failure: %v", err)
	}
}

func TestRecoveryS05CurrentNextTypeFixtures(t *testing.T) {
	recoveryDB(t)
	for _, tc := range []struct{ id, branch, want uint32 }{{3700001, 0, 3700002}, {141, 0, 142}, {141, 1, 143}, {30004, 0, 30007}, {10021, 1, 10022}, {10021, 2, 10023}} {
		n, ok, err := loadNewEducateConfigByID[newEducateNodeConfig](newEducateNodeCategory, tc.id)
		if err != nil || !ok {
			t.Fatalf("node %d: %v", tc.id, err)
		}
		next, err := resolveEducateNodeNext(n, tc.branch, func(total uint64) (uint64, error) { return total - 1, nil })
		if err != nil || next != tc.want {
			t.Fatalf("node %d ->%d want %d: %v", tc.id, next, tc.want, err)
		}
	}
}
