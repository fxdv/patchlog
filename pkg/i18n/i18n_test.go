package i18n

import "testing"

func TestParseLang(t *testing.T) {
	tests := []struct {
		input string
		want  Lang
	}{
		{"en", LangEN},
		{"", LangEN},
		{"ru", LangRU},
		{"zh", LangZH},
	}
	for _, tc := range tests {
		got, err := ParseLang(tc.input)
		if err != nil {
			t.Fatalf("ParseLang(%q) error: %v", tc.input, err)
		}
		if got != tc.want {
			t.Errorf("ParseLang(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestParseLangInvalid(t *testing.T) {
	_, err := ParseLang("fr")
	if err == nil {
		t.Error("expected error for invalid language")
	}
}

func TestHeading(t *testing.T) {
	tests := []struct {
		lang Lang
		typ  string
		want string
	}{
		{LangEN, "feat", "Features"},
		{LangRU, "feat", "Возможности"},
		{LangZH, "feat", "新功能"},
		{LangEN, "fix", "Bug Fixes"},
		{LangRU, "fix", "Исправления"},
		{LangZH, "unknown", "unknown"},
	}
	for _, tc := range tests {
		got := Heading(tc.lang, tc.typ)
		if got != tc.want {
			t.Errorf("Heading(%q, %q) = %q, want %q", tc.lang, tc.typ, got, tc.want)
		}
	}
}

func TestBreakingHeading(t *testing.T) {
	if BreakingHeading(LangEN) != "Breaking Changes" {
		t.Error("en breaking heading mismatch")
	}
	if BreakingHeading(LangRU) != "Критические изменения" {
		t.Error("ru breaking heading mismatch")
	}
	if BreakingHeading(LangZH) != "破坏性变更" {
		t.Error("zh breaking heading mismatch")
	}
}

func TestPromptPrefix(t *testing.T) {
	if PromptPrefix(LangEN) == "" {
		t.Error("en prompt prefix should not be empty")
	}
	if PromptPrefix(LangRU) == "" {
		t.Error("ru prompt prefix should not be empty")
	}
	if PromptPrefix(LangZH) == "" {
		t.Error("zh prompt prefix should not be empty")
	}
}

func TestSummaryInstructions(t *testing.T) {
	if SummaryInstructions(LangEN) == "" {
		t.Error("en summary instructions should not be empty")
	}
	if SummaryInstructions(LangRU) == "" {
		t.Error("ru summary instructions should not be empty")
	}
}

func TestLocalizeSections(t *testing.T) {
	sections := map[string]string{
		"feat":        "Features",
		"fix":         "Bug Fixes",
		"custom_type": "Custom Heading",
	}
	localized := LocalizeSections(LangRU, sections)
	if localized["feat"] != "Возможности" {
		t.Errorf("expected 'Возможности', got %q", localized["feat"])
	}
	if localized["fix"] != "Исправления" {
		t.Errorf("expected 'Исправления', got %q", localized["fix"])
	}
	if localized["custom_type"] != "Custom Heading" {
		t.Errorf("expected custom heading preserved, got %q", localized["custom_type"])
	}
}

func TestLocalizeSectionsEnglishNoOp(t *testing.T) {
	sections := map[string]string{"feat": "Features"}
	localized := LocalizeSections(LangEN, sections)
	if localized["feat"] != "Features" {
		t.Error("English should return sections unchanged")
	}
}

func TestParseLangs(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 1},
		{"en", 1},
		{"ru", 1},
		{"en,ru", 2},
		{"en, ru", 2},
		{"en,ru,zh", 3},
	}
	for _, tc := range tests {
		langs, err := ParseLangs(tc.input)
		if err != nil {
			t.Fatalf("ParseLangs(%q) error: %v", tc.input, err)
		}
		if len(langs) != tc.want {
			t.Errorf("ParseLangs(%q) returned %d langs, want %d", tc.input, len(langs), tc.want)
		}
	}
}

func TestParseLangsInvalid(t *testing.T) {
	_, err := ParseLangs("en,fr")
	if err == nil {
		t.Error("expected error for invalid language in list")
	}
}

func TestBilingualHeading(t *testing.T) {
	tests := []struct {
		langs []Lang
		typ   string
		want  string
	}{
		{[]Lang{LangEN}, "feat", "Features"},
		{[]Lang{LangRU}, "feat", "Возможности"},
		{[]Lang{LangEN, LangRU}, "feat", "Features / Возможности"},
		{[]Lang{LangEN, LangRU, LangZH}, "feat", "Features / Возможности / 新功能"},
		{[]Lang{}, "feat", "Features"},
	}
	for _, tc := range tests {
		got := BilingualHeading(tc.langs, tc.typ)
		if got != tc.want {
			t.Errorf("BilingualHeading(%v, %q) = %q, want %q", tc.langs, tc.typ, got, tc.want)
		}
	}
}

func TestBilingualHeadingDedup(t *testing.T) {
	got := BilingualHeading([]Lang{LangEN, LangEN}, "feat")
	if got != "Features" {
		t.Errorf("BilingualHeading should deduplicate, got %q", got)
	}
}

func TestConfluenceLabelsFor(t *testing.T) {
	en := ConfluenceLabelsFor(LangEN)
	if en.AnalyticsTitle == "" {
		t.Error("EN analytics title should not be empty")
	}
	if en.Metric != "Metric" {
		t.Errorf("EN Metric label: got %q, want Metric", en.Metric)
	}

	ru := ConfluenceLabelsFor(LangRU)
	if ru.AnalyticsTitle == "" {
		t.Error("RU analytics title should not be empty")
	}
	if ru.Metric != "Метрика" {
		t.Errorf("RU Metric label: got %q, want Метрика", ru.Metric)
	}

	zh := ConfluenceLabelsFor(LangZH)
	if zh.AnalyticsTitle == "" {
		t.Error("ZH analytics title should not be empty")
	}

	unknown := ConfluenceLabelsFor(Lang("fr"))
	if unknown.Metric != "Metric" {
		t.Error("Unknown language should fall back to English")
	}
}
