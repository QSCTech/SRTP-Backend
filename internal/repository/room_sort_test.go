package repository

import "testing"

func TestParseRoomSort(t *testing.T) {
	sort, err := ParseRoomSort("")
	if err != nil || sort != RoomSortStartTimeAsc {
		t.Fatalf("expected start_time_asc default, got %q err=%v", sort, err)
	}

	sort, err = ParseRoomSort("created_at_desc")
	if err != nil || sort != RoomSortCreatedAtDesc {
		t.Fatalf("expected created_at_desc, got %q err=%v", sort, err)
	}

	if _, err := ParseRoomSort("unknown"); err == nil {
		t.Fatal("expected error for unknown sort")
	}
}
