package curate

import (
	"fmt"
	"strings"

	"github.com/fxdv/patchlog/pkg/render"
)

type Mode int

const (
	ModeBrowse Mode = iota
	ModeEdit
	ModeMove
	ModeMerge
	ModePreview
	ModeHelp
)

type CursorPos struct {
	Section int
	Item    int
}

type CuratorState struct {
	Report      render.Report
	Original    render.Report
	Cursor      CursorPos
	Mode        Mode
	Excluded    map[string]bool
	UndoStack   []UndoAction
	Width       int
	Height      int
	ShowPreview bool
	EditBuffer  string
	MergeSource *CursorPos
	SearchQuery string
	ShowSearch  bool
}

type UndoAction struct {
	Type string
	CursorPos
	OldValue   string
	NewValue   string
	OldSection int
	NewSection int
	Item       *render.Item
}

func NewState(report render.Report, width, height int) *CuratorState {
	return &CuratorState{
		Report:   report,
		Original: copyReport(report),
		Excluded: make(map[string]bool),
		Width:    width,
		Height:   height,
	}
}

func copyReport(r render.Report) render.Report {
	cp := r
	cp.Breaking = make([]render.Item, len(r.Breaking))
	copy(cp.Breaking, r.Breaking)
	cp.Sections = make([]render.Section, len(r.Sections))
	for i, s := range r.Sections {
		cp.Sections[i] = s
		cp.Sections[i].Items = make([]render.Item, len(s.Items))
		copy(cp.Sections[i].Items, s.Items)
	}
	return cp
}

func (s *CuratorState) ItemKey(sectionIdx, itemIdx int) string {
	if sectionIdx < 0 || sectionIdx >= len(s.Report.Sections) {
		return ""
	}
	section := &s.Report.Sections[sectionIdx]
	if itemIdx < 0 || itemIdx >= len(section.Items) {
		return ""
	}
	item := section.Items[itemIdx]
	return fmt.Sprintf("%d:%d:%s:%s", sectionIdx, itemIdx, item.Hash, item.Description)
}

func (s *CuratorState) IsExcluded(sectionIdx, itemIdx int) bool {
	return s.Excluded[s.ItemKey(sectionIdx, itemIdx)]
}

func (s *CuratorState) ToggleItem() {
	key := s.ItemKey(s.Cursor.Section, s.Cursor.Item)
	if key == "" {
		return
	}
	s.Excluded[key] = !s.Excluded[key]
	s.pushUndo(UndoAction{
		Type:      "toggle",
		CursorPos: s.Cursor,
	})
}

func (s *CuratorState) EditItem() {
	if s.Cursor.Section < 0 || s.Cursor.Section >= len(s.Report.Sections) {
		return
	}
	section := &s.Report.Sections[s.Cursor.Section]
	if s.Cursor.Item < 0 || s.Cursor.Item >= len(section.Items) {
		return
	}
	s.EditBuffer = section.Items[s.Cursor.Item].Description
	s.Mode = ModeEdit
}

func (s *CuratorState) SaveEdit() {
	if s.Cursor.Section < 0 || s.Cursor.Section >= len(s.Report.Sections) {
		return
	}
	section := &s.Report.Sections[s.Cursor.Section]
	if s.Cursor.Item < 0 || s.Cursor.Item >= len(section.Items) {
		return
	}
	old := section.Items[s.Cursor.Item].Description
	section.Items[s.Cursor.Item].Description = s.EditBuffer
	s.pushUndo(UndoAction{
		Type:      "edit",
		CursorPos: s.Cursor,
		OldValue:  old,
		NewValue:  s.EditBuffer,
	})
	s.Mode = ModeBrowse
	s.EditBuffer = ""
}

func (s *CuratorState) CancelEdit() {
	s.Mode = ModeBrowse
	s.EditBuffer = ""
}

func (s *CuratorState) MoveItemToSection(targetSection int) {
	if s.Cursor.Section < 0 || s.Cursor.Section >= len(s.Report.Sections) {
		return
	}
	if targetSection < 0 || targetSection >= len(s.Report.Sections) {
		return
	}
	if targetSection == s.Cursor.Section {
		return
	}
	src := &s.Report.Sections[s.Cursor.Section]
	if s.Cursor.Item < 0 || s.Cursor.Item >= len(src.Items) {
		return
	}
	item := src.Items[s.Cursor.Item]
	src.Items = append(src.Items[:s.Cursor.Item], src.Items[s.Cursor.Item+1:]...)
	dst := &s.Report.Sections[targetSection]
	dst.Items = append(dst.Items, item)
	s.pushUndo(UndoAction{
		Type:       "move",
		CursorPos:  s.Cursor,
		OldSection: s.Cursor.Section,
		NewSection: targetSection,
	})
	s.Cursor.Section = targetSection
	s.Cursor.Item = len(dst.Items) - 1
	s.Mode = ModeBrowse
}

func (s *CuratorState) StartMerge() {
	if s.Cursor.Section < 0 || s.Cursor.Section >= len(s.Report.Sections) {
		return
	}
	pos := s.Cursor
	s.MergeSource = &pos
	s.Mode = ModeMerge
}

func (s *CuratorState) ConfirmMerge() {
	if s.MergeSource == nil {
		return
	}
	src := &s.Report.Sections[s.MergeSource.Section]
	dst := &s.Report.Sections[s.Cursor.Section]
	if s.MergeSource.Item < 0 || s.MergeSource.Item >= len(src.Items) {
		return
	}
	if s.Cursor.Item < 0 || s.Cursor.Item >= len(dst.Items) {
		return
	}
	if s.MergeSource.Section == s.Cursor.Section && s.MergeSource.Item == s.Cursor.Item {
		return
	}
	srcItem := src.Items[s.MergeSource.Item]
	dstItem := dst.Items[s.Cursor.Item]
	merged := dstItem
	merged.Description = dstItem.Description + " + " + srcItem.Description
	if merged.Hash == "" {
		merged.Hash = srcItem.Hash
	}
	for _, k := range srcItem.JiraKeys {
		merged.JiraKeys = append(merged.JiraKeys, k)
	}
	oldDesc := dst.Items[s.Cursor.Item].Description
	dst.Items[s.Cursor.Item] = merged
	src.Items = append(src.Items[:s.MergeSource.Item], src.Items[s.MergeSource.Item+1:]...)
	s.pushUndo(UndoAction{
		Type:      "merge",
		CursorPos: s.Cursor,
		OldValue:  oldDesc,
	})
	s.Mode = ModeBrowse
	s.MergeSource = nil
}

func (s *CuratorState) CancelMerge() {
	s.Mode = ModeBrowse
	s.MergeSource = nil
}

func (s *CuratorState) Undo() {
	if len(s.UndoStack) == 0 {
		return
	}
	action := s.UndoStack[len(s.UndoStack)-1]
	s.UndoStack = s.UndoStack[:len(s.UndoStack)-1]
	switch action.Type {
	case "toggle":
		key := s.ItemKey(action.CursorPos.Section, action.CursorPos.Item)
		s.Excluded[key] = !s.Excluded[key]
	case "edit":
		if action.CursorPos.Section < len(s.Report.Sections) && action.CursorPos.Item < len(s.Report.Sections[action.CursorPos.Section].Items) {
			s.Report.Sections[action.CursorPos.Section].Items[action.CursorPos.Item].Description = action.OldValue
		}
	case "move":
		if action.NewSection < len(s.Report.Sections) && action.OldSection < len(s.Report.Sections) {
			dst := &s.Report.Sections[action.NewSection]
			src := &s.Report.Sections[action.OldSection]
			if len(dst.Items) > 0 {
				item := dst.Items[len(dst.Items)-1]
				dst.Items = dst.Items[:len(dst.Items)-1]
				src.Items = append(src.Items, item)
			}
		}
	case "merge":
		s.Report = copyReport(s.Original)
	}
	s.Cursor = action.CursorPos
}

func (s *CuratorState) pushUndo(action UndoAction) {
	if len(s.UndoStack) >= 50 {
		s.UndoStack = s.UndoStack[1:]
	}
	s.UndoStack = append(s.UndoStack, action)
}

func (s *CuratorState) TotalItems() int {
	n := 0
	for i, section := range s.Report.Sections {
		if i < 0 || i >= len(s.Report.Sections) {
			continue
		}
		n += len(section.Items)
	}
	return n
}

func (s *CuratorState) VisibleItemCount() int {
	n := 0
	for i, section := range s.Report.Sections {
		for j := range section.Items {
			if !s.IsExcluded(i, j) {
				n++
			}
		}
	}
	return n
}

func (s *CuratorState) SectionCount() int {
	count := 0
	for _, section := range s.Report.Sections {
		if len(section.Items) > 0 {
			count++
		}
	}
	return count
}

func (s *CuratorState) FilteredReport() render.Report {
	filtered := s.Report
	filtered.Sections = nil
	for i, section := range s.Report.Sections {
		var items []render.Item
		for j, item := range section.Items {
			if !s.IsExcluded(i, j) {
				items = append(items, item)
			}
		}
		if len(items) > 0 {
			filtered.Sections = append(filtered.Sections, render.Section{
				Heading: section.Heading,
				Type:    section.Type,
				Items:   items,
			})
		}
	}
	return filtered
}

func (s *CuratorState) MoveCursor(direction int) {
	if len(s.Report.Sections) == 0 {
		return
	}
	s.Cursor.Item += direction
	for {
		section := &s.Report.Sections[s.Cursor.Section]
		if s.Cursor.Item >= 0 && s.Cursor.Item < len(section.Items) {
			return
		}
		if s.Cursor.Item < 0 {
			s.Cursor.Section--
			if s.Cursor.Section < 0 {
				s.Cursor.Section = 0
				s.Cursor.Item = 0
				return
			}
			s.Cursor.Item = len(s.Report.Sections[s.Cursor.Section].Items) - 1
		} else {
			s.Cursor.Section++
			if s.Cursor.Section >= len(s.Report.Sections) {
				s.Cursor.Section = len(s.Report.Sections) - 1
				s.Cursor.Item = len(s.Report.Sections[s.Cursor.Section].Items) - 1
				return
			}
			s.Cursor.Item = 0
		}
	}
}

func (s *CuratorState) MoveSection(direction int) {
	s.Cursor.Section += direction
	if s.Cursor.Section < 0 {
		s.Cursor.Section = 0
	}
	if s.Cursor.Section >= len(s.Report.Sections) {
		s.Cursor.Section = len(s.Report.Sections) - 1
	}
	s.Cursor.Item = 0
}

func (s *CuratorState) JumpToTop() {
	s.Cursor.Section = 0
	s.Cursor.Item = 0
}

func (s *CuratorState) JumpToBottom() {
	s.Cursor.Section = len(s.Report.Sections) - 1
	if s.Cursor.Section < 0 {
		return
	}
	s.Cursor.Item = len(s.Report.Sections[s.Cursor.Section].Items) - 1
}

func (s *CuratorState) MatchesSearch(desc string) bool {
	if !s.ShowSearch || s.SearchQuery == "" {
		return true
	}
	return strings.Contains(strings.ToLower(desc), strings.ToLower(s.SearchQuery))
}
