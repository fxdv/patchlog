package curate

import (
	"testing"

	"github.com/fxdv/patchlog/pkg/render"
)

func makeTestReport() render.Report {
	return render.Report{
		Version: "1.0.0",
		Sections: []render.Section{
			{Heading: "Features", Type: "feat", Items: []render.Item{
				{Description: "add login", Hash: "abc", Author: "alice"},
				{Description: "add logout", Hash: "def", Author: "bob"},
			}},
			{Heading: "Bug Fixes", Type: "fix", Items: []render.Item{
				{Description: "fix crash", Hash: "ghi", Author: "alice"},
			}},
		},
	}
}

func TestNewState(t *testing.T) {
	report := makeTestReport()
	state := NewState(report, 80, 24)
	if state.Cursor.Section != 0 || state.Cursor.Item != 0 {
		t.Error("expected cursor at 0,0")
	}
	if len(state.Excluded) != 0 {
		t.Error("expected empty excluded map")
	}
	if state.Mode != ModeBrowse {
		t.Error("expected browse mode")
	}
}

func TestToggleItem(t *testing.T) {
	state := NewState(makeTestReport(), 80, 24)
	key := state.ItemKey(0, 0)
	if state.Excluded[key] {
		t.Error("item should start included")
	}
	state.ToggleItem()
	if !state.Excluded[key] {
		t.Error("item should be excluded after toggle")
	}
	state.ToggleItem()
	if state.Excluded[key] {
		t.Error("item should be included after second toggle")
	}
}

func TestEditItem(t *testing.T) {
	state := NewState(makeTestReport(), 80, 24)
	state.Cursor.Section = 0
	state.Cursor.Item = 0
	state.EditItem()
	if state.Mode != ModeEdit {
		t.Error("expected edit mode")
	}
	if state.EditBuffer != "add login" {
		t.Errorf("expected edit buffer 'add login', got %q", state.EditBuffer)
	}
	state.EditBuffer = "add user login"
	state.SaveEdit()
	if state.Mode != ModeBrowse {
		t.Error("expected browse mode after save")
	}
	if state.Report.Sections[0].Items[0].Description != "add user login" {
		t.Errorf("expected updated description, got %q", state.Report.Sections[0].Items[0].Description)
	}
}

func TestCancelEdit(t *testing.T) {
	state := NewState(makeTestReport(), 80, 24)
	state.Cursor.Section = 0
	state.Cursor.Item = 0
	state.EditItem()
	state.EditBuffer = "changed"
	state.CancelEdit()
	if state.Mode != ModeBrowse {
		t.Error("expected browse mode after cancel")
	}
	if state.Report.Sections[0].Items[0].Description != "add login" {
		t.Error("description should be unchanged after cancel")
	}
}

func TestMoveItem(t *testing.T) {
	state := NewState(makeTestReport(), 80, 24)
	state.Cursor.Section = 0
	state.Cursor.Item = 0
	state.MoveItemToSection(1)
	if len(state.Report.Sections[0].Items) != 1 {
		t.Errorf("expected 1 item in source section, got %d", len(state.Report.Sections[0].Items))
	}
	if len(state.Report.Sections[1].Items) != 2 {
		t.Errorf("expected 2 items in target section, got %d", len(state.Report.Sections[1].Items))
	}
	if state.Cursor.Section != 1 {
		t.Error("cursor should be in target section")
	}
}

func TestMerge(t *testing.T) {
	state := NewState(makeTestReport(), 80, 24)
	state.Cursor.Section = 0
	state.Cursor.Item = 0
	state.StartMerge()
	if state.Mode != ModeMerge {
		t.Error("expected merge mode")
	}
	if state.MergeSource == nil {
		t.Fatal("expected merge source to be set")
	}
	state.Cursor.Item = 1
	state.ConfirmMerge()
	if state.Mode != ModeBrowse {
		t.Error("expected browse mode after merge")
	}
	if len(state.Report.Sections[0].Items) != 1 {
		t.Errorf("expected 1 item after merge, got %d", len(state.Report.Sections[0].Items))
	}
	desc := state.Report.Sections[0].Items[0].Description
	if !contains(desc, "+") {
		t.Errorf("expected merged description with +, got %q", desc)
	}
}

func TestUndoToggle(t *testing.T) {
	state := NewState(makeTestReport(), 80, 24)
	key := state.ItemKey(0, 0)
	state.ToggleItem()
	if !state.Excluded[key] {
		t.Error("should be excluded after toggle")
	}
	state.Undo()
	if state.Excluded[key] {
		t.Error("should be included after undo")
	}
}

func TestUndoEdit(t *testing.T) {
	state := NewState(makeTestReport(), 80, 24)
	state.Cursor.Section = 0
	state.Cursor.Item = 0
	state.EditItem()
	state.EditBuffer = "changed"
	state.SaveEdit()
	if state.Report.Sections[0].Items[0].Description != "changed" {
		t.Fatal("edit should have changed description")
	}
	state.Undo()
	if state.Report.Sections[0].Items[0].Description != "add login" {
		t.Errorf("expected original after undo, got %q", state.Report.Sections[0].Items[0].Description)
	}
}

func TestUndoMove(t *testing.T) {
	state := NewState(makeTestReport(), 80, 24)
	state.Cursor.Section = 0
	state.Cursor.Item = 0
	state.MoveItemToSection(1)
	if len(state.Report.Sections[0].Items) != 1 {
		t.Fatal("move should have removed item from source")
	}
	state.Undo()
	if len(state.Report.Sections[0].Items) != 2 {
		t.Errorf("expected 2 items after undo, got %d", len(state.Report.Sections[0].Items))
	}
}

func TestFilteredReport(t *testing.T) {
	state := NewState(makeTestReport(), 80, 24)
	state.ToggleItem()
	filtered := state.FilteredReport()
	if len(filtered.Sections[0].Items) != 1 {
		t.Errorf("expected 1 item in filtered section, got %d", len(filtered.Sections[0].Items))
	}
}

func TestMoveCursor(t *testing.T) {
	state := NewState(makeTestReport(), 80, 24)
	state.MoveCursor(1)
	if state.Cursor.Item != 1 {
		t.Errorf("expected item 1, got %d", state.Cursor.Item)
	}
	state.MoveCursor(1)
	if state.Cursor.Section != 1 || state.Cursor.Item != 0 {
		t.Errorf("expected section 1 item 0, got section %d item %d", state.Cursor.Section, state.Cursor.Item)
	}
	state.MoveCursor(-1)
	if state.Cursor.Section != 0 || state.Cursor.Item != 1 {
		t.Errorf("expected section 0 item 1, got section %d item %d", state.Cursor.Section, state.Cursor.Item)
	}
}

func TestJumpToTop(t *testing.T) {
	state := NewState(makeTestReport(), 80, 24)
	state.Cursor.Section = 1
	state.Cursor.Item = 0
	state.JumpToTop()
	if state.Cursor.Section != 0 || state.Cursor.Item != 0 {
		t.Error("expected cursor at 0,0")
	}
}

func TestJumpToBottom(t *testing.T) {
	state := NewState(makeTestReport(), 80, 24)
	state.JumpToBottom()
	if state.Cursor.Section != 1 {
		t.Errorf("expected section 1, got %d", state.Cursor.Section)
	}
}

func TestTotalItems(t *testing.T) {
	state := NewState(makeTestReport(), 80, 24)
	if state.TotalItems() != 3 {
		t.Errorf("expected 3 total items, got %d", state.TotalItems())
	}
}

func TestVisibleItemCount(t *testing.T) {
	state := NewState(makeTestReport(), 80, 24)
	state.ToggleItem()
	if state.VisibleItemCount() != 2 {
		t.Errorf("expected 2 visible items, got %d", state.VisibleItemCount())
	}
}

func TestUndoStackLimit(t *testing.T) {
	state := NewState(makeTestReport(), 80, 24)
	for i := 0; i < 55; i++ {
		state.ToggleItem()
		state.ToggleItem()
	}
	if len(state.UndoStack) > 50 {
		t.Errorf("undo stack should be capped at 50, got %d", len(state.UndoStack))
	}
}

func TestDeleteWord(t *testing.T) {
	state := NewState(makeTestReport(), 80, 24)
	result := state.deleteWord("hello world test")
	if result != "hello world " {
		t.Errorf("expected 'hello world ', got %q", result)
	}
}

func TestMatchesSearch(t *testing.T) {
	state := NewState(makeTestReport(), 80, 24)
	state.ShowSearch = true
	state.SearchQuery = "login"
	if !state.MatchesSearch("add login page") {
		t.Error("should match 'login' in description")
	}
	if state.MatchesSearch("fix crash") {
		t.Error("should not match 'login' in 'fix crash'")
	}
}

func TestCancelMerge(t *testing.T) {
	state := NewState(makeTestReport(), 80, 24)
	state.StartMerge()
	state.CancelMerge()
	if state.Mode != ModeBrowse {
		t.Error("expected browse mode after cancel merge")
	}
	if state.MergeSource != nil {
		t.Error("merge source should be nil after cancel")
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
