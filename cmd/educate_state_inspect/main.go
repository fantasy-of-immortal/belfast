// educate_state_inspect decodes a read-only psql educate character export.
package main

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func main() {
	if len(os.Args) != 2 {
		panic("usage: educate_state_inspect export.tsv")
	}
	f, err := os.Open(os.Args[1])
	if err != nil {
		panic(err)
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 4096), 1024*1024)
	for s.Scan() {
		p := strings.Split(strings.TrimPrefix(s.Text(), "\ufeff"), "|")
		if len(p) != 6 {
			panic("expected commander|character|revision|state hex|permanent hex|metadata")
		}
		b, err := hex.DecodeString(p[3])
		if err != nil {
			panic(err)
		}
		state := &protobuf.TBINFO{}
		if err := proto.Unmarshal(b, state); err != nil {
			panic(err)
		}
		out, err := protojson.MarshalOptions{Indent: "  "}.Marshal(state)
		if err != nil {
			panic(err)
		}
		fmt.Printf("commander=%s character=%s revision=%s\n%s\nmetadata=%s\n", p[0], p[1], p[2], out, p[5])
	}
	if err := s.Err(); err != nil {
		panic(err)
	}
}
