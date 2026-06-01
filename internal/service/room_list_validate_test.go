package service

import (
	"testing"

	"github.com/QSCTech/SRTP-Backend/internal/repository"
)

func TestNormalizeListRoomsInputKeywordTrim(t *testing.T) {
	keyword := "  周末局  "
	input := ListRoomsInput{Keyword: &keyword, Page: 1, PageSize: 20}

	sort, err := normalizeListRoomsInput(&input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sort != repository.RoomSortStartTimeAsc {
		t.Fatalf("expected default sort, got %q", sort)
	}
	if input.Keyword == nil || *input.Keyword != "周末局" {
		t.Fatalf("expected trimmed keyword, got %#v", input.Keyword)
	}
}

func TestNormalizeListRoomsInputBlankKeyword(t *testing.T) {
	keyword := "   "
	input := ListRoomsInput{Keyword: &keyword, Page: 1, PageSize: 20}

	if _, err := normalizeListRoomsInput(&input); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if input.Keyword != nil {
		t.Fatalf("expected nil keyword, got %#v", input.Keyword)
	}
}

func TestNormalizeListRoomsInputInvalidTimeRange(t *testing.T) {
	value := "night"
	input := ListRoomsInput{TimeRange: &value, Page: 1, PageSize: 20}

	if _, err := normalizeListRoomsInput(&input); err == nil {
		t.Fatal("expected invalid time_range error")
	}
}

func TestNormalizeListRoomsInputInvalidSort(t *testing.T) {
	input := ListRoomsInput{Sort: "popular_desc", Page: 1, PageSize: 20}

	if _, err := normalizeListRoomsInput(&input); err == nil {
		t.Fatal("expected invalid sort error")
	}
}

func TestNormalizeListRoomsInputPageSizeCap(t *testing.T) {
	input := ListRoomsInput{Page: 0, PageSize: 500}

	sort, err := normalizeListRoomsInput(&input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sort != repository.RoomSortStartTimeAsc {
		t.Fatalf("expected default sort, got %q", sort)
	}
	if input.Page != 1 {
		t.Fatalf("expected page=1, got %d", input.Page)
	}
	if input.PageSize != 100 {
		t.Fatalf("expected page_size=100, got %d", input.PageSize)
	}
}
