package orm

import "testing"

// A character can have several topics. Reloading must retain replies to the
// earlier topics after later topics have been appended to the response slice.
func TestJuustagramRepliesSurviveMultipleTopics(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &JuustagramReply{})
	clearTable(t, &JuustagramChatGroup{})
	clearTable(t, &JuustagramGroup{})
	if _, err := CreateJuustagramGroup(1, 10, 100); err != nil {
		t.Fatal(err)
	}
	for id := uint32(101); id < 108; id++ {
		if _, err := CreateJuustagramChatGroup(1, 10, id, 0); err != nil {
			t.Fatal(err)
		}
	}
	for id := uint32(100); id < 108; id++ {
		if _, err := AddJuustagramChatReply(1, id, id+1000, 1, 100); err != nil {
			t.Fatal(err)
		}
	}
	check := func(t *testing.T, groups []JuustagramGroup) {
		t.Helper()
		if len(groups) != 1 || len(groups[0].ChatGroups) != 8 {
			t.Fatalf("unexpected groups: %+v", groups)
		}
		for _, chat := range groups[0].ChatGroups {
			if len(chat.ReplyList) != 1 || chat.ReplyList[0].Key != chat.ChatGroupID+1000 {
				t.Errorf("topic %d lost its stored reply: %+v", chat.ChatGroupID, chat.ReplyList)
			}
		}
	}
	t.Run("login", func(t *testing.T) {
		groups, err := GetJuustagramGroups(1)
		if err != nil {
			t.Fatal(err)
		}
		check(t, groups)
	})
	t.Run("paged", func(t *testing.T) {
		groups, _, err := ListJuustagramGroups(1, 0, 10)
		if err != nil {
			t.Fatal(err)
		}
		check(t, groups)
	})
	t.Run("single", func(t *testing.T) {
		group, err := GetJuustagramGroup(1, 10)
		if err != nil {
			t.Fatal(err)
		}
		check(t, []JuustagramGroup{*group})
	})
}
