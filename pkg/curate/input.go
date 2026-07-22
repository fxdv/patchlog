package curate

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

type KeyEvent struct {
	Key  string
	Rune rune
	Ctrl bool
	Alt  bool
}

func (s *CuratorState) HandleKey(key KeyEvent) (action string) {
	if s.Mode == ModeEdit {
		return s.handleEditKey(key)
	}
	if s.Mode == ModeMove {
		return s.handleMoveKey(key)
	}
	if s.Mode == ModeMerge {
		return s.handleMergeKey(key)
	}
	if s.ShowSearch {
		return s.handleSearchKey(key)
	}
	return s.handleBrowseKey(key)
}

func (s *CuratorState) handleBrowseKey(key KeyEvent) string {
	switch key.Key {
	case "j", "down":
		s.MoveCursor(1)
	case "k", "up":
		s.MoveCursor(-1)
	case "J":
		s.MoveSection(1)
	case "K":
		s.MoveSection(-1)
	case "g":
		s.JumpToTop()
	case "G":
		s.JumpToBottom()
	case "space":
		s.ToggleItem()
	case "e":
		s.EditItem()
	case "m":
		s.Mode = ModeMove
	case "x":
		if s.MergeSource == nil {
			s.StartMerge()
		} else {
			s.ConfirmMerge()
		}
	case "u":
		s.Undo()
	case "p":
		s.ShowPreview = !s.ShowPreview
	case "?":
		if s.Mode == ModeHelp {
			s.Mode = ModeBrowse
		} else {
			s.Mode = ModeHelp
		}
	case "q":
		return "quit"
	case "enter":
		return "publish"
	case "/":
		s.ShowSearch = true
		s.SearchQuery = ""
	case "ctrl+s":
		return "save"
	}
	return ""
}

func (s *CuratorState) handleEditKey(key KeyEvent) string {
	switch key.Key {
	case "enter":
		s.SaveEdit()
	case "escape":
		s.CancelEdit()
	case "ctrl+a":
		s.EditBuffer = s.moveToStart(s.EditBuffer)
	case "ctrl+e":
		s.EditBuffer = s.moveToEnd(s.EditBuffer)
	case "ctrl+w":
		s.EditBuffer = s.deleteWord(s.EditBuffer)
	case "ctrl+u":
		s.EditBuffer = ""
	case "backspace":
		if len(s.EditBuffer) > 0 {
			_, size := utf8.DecodeLastRuneInString(s.EditBuffer)
			s.EditBuffer = s.EditBuffer[:len(s.EditBuffer)-size]
		}
	default:
		if key.Rune != 0 && key.Key == "" {
			s.EditBuffer += string(key.Rune)
		}
	}
	return ""
}

func (s *CuratorState) handleMoveKey(key KeyEvent) string {
	switch key.Key {
	case "escape":
		s.Mode = ModeBrowse
	case "enter":
		s.MoveItemToSection(s.Cursor.Section)
	case "left", "h":
		s.Cursor.Section--
		if s.Cursor.Section < 0 {
			s.Cursor.Section = 0
		}
	case "right", "l":
		s.Cursor.Section++
		if s.Cursor.Section >= len(s.Report.Sections) {
			s.Cursor.Section = len(s.Report.Sections) - 1
		}
	}
	return ""
}

func (s *CuratorState) handleMergeKey(key KeyEvent) string {
	switch key.Key {
	case "escape":
		s.CancelMerge()
	case "x":
		s.ConfirmMerge()
	case "j", "down":
		s.MoveCursor(1)
	case "k", "up":
		s.MoveCursor(-1)
	}
	return ""
}

func (s *CuratorState) handleSearchKey(key KeyEvent) string {
	switch key.Key {
	case "enter":
		s.ShowSearch = false
	case "escape":
		s.ShowSearch = false
		s.SearchQuery = ""
	case "backspace":
		if len(s.SearchQuery) > 0 {
			_, size := utf8.DecodeLastRuneInString(s.SearchQuery)
			s.SearchQuery = s.SearchQuery[:len(s.SearchQuery)-size]
		}
	default:
		if key.Rune != 0 && key.Key == "" {
			s.SearchQuery += string(key.Rune)
		}
	}
	return ""
}

func (s *CuratorState) moveToStart(buf string) string {
	return buf
}

func (s *CuratorState) moveToEnd(buf string) string {
	return buf
}

func (s *CuratorState) deleteWord(buf string) string {
	trimmed := strings.TrimRight(buf, " ")
	idx := strings.LastIndex(trimmed, " ")
	if idx < 0 {
		return ""
	}
	return buf[:idx+1]
}

func ParseEscapeSequence(buf []byte) (KeyEvent, int) {
	if len(buf) == 0 {
		return KeyEvent{}, 0
	}

	if buf[0] == '\x1b' {
		if len(buf) == 1 {
			return KeyEvent{Key: "escape"}, 1
		}
		if buf[1] == '[' {
			if len(buf) < 3 {
				return KeyEvent{}, 0
			}
			switch buf[2] {
			case 'A':
				return KeyEvent{Key: "up"}, 3
			case 'B':
				return KeyEvent{Key: "down"}, 3
			case 'C':
				return KeyEvent{Key: "right"}, 3
			case 'D':
				return KeyEvent{Key: "left"}, 3
			case 'H':
				return KeyEvent{Key: "home"}, 3
			case 'F':
				return KeyEvent{Key: "end"}, 3
			case '3':
				if len(buf) >= 4 && buf[3] == '~' {
					return KeyEvent{Key: "delete"}, 4
				}
				return KeyEvent{}, 0
			}
			return KeyEvent{}, 0
		}
		if buf[1] == 'O' && len(buf) >= 3 {
			switch buf[2] {
			case 'H':
				return KeyEvent{Key: "home"}, 3
			case 'F':
				return KeyEvent{Key: "end"}, 3
			}
		}
		return KeyEvent{Key: "escape"}, 1
	}

	if buf[0] == '\r' || buf[0] == '\n' {
		return KeyEvent{Key: "enter"}, 1
	}

	if buf[0] == 0x7f || buf[0] == 0x08 {
		return KeyEvent{Key: "backspace"}, 1
	}

	if buf[0] < 0x20 {
		ctrl := KeyEvent{Ctrl: true}
		ctrl.Key = fmt.Sprintf("ctrl+%c", buf[0]+'a'-1)
		return ctrl, 1
	}

	r, size := utf8.DecodeRune(buf)
	if r == utf8.RuneError {
		return KeyEvent{}, 0
	}
	return KeyEvent{Rune: r}, size
}
