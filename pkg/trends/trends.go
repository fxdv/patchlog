package trends

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fxdv/patchlog/internal/atomicfile"
)

type ContributorSnap struct {
	Name    string `json:"name"`
	Commits int    `json:"commits"`
}

type Snapshot struct {
	Version                          string            `json:"version"`
	Date                             string            `json:"date"`
	TotalCommits                     int               `json:"total_commits"`
	TotalAuthors                     int               `json:"total_authors"`
	BreakingChanges                  int               `json:"breaking_changes"`
	ReleaseContributionConcentration int               `json:"release_contribution_concentration"`
	ReleaseCommitSpanHours           float64           `json:"release_commit_span_hours"`
	ReleaseAgeHours                  float64           `json:"release_age_hours"`
	CommitsPerDay                    float64           `json:"commits_per_day"`
	ConventionalRatio                float64           `json:"conventional_ratio"`
	OwnershipEntropy                 float64           `json:"ownership_entropy"`
	OwnershipConc                    float64           `json:"ownership_conc"`
	TechDebtUSD                      float64           `json:"tech_debt_usd"`
	HotspotScore                     float64           `json:"hotspot_score"`
	HotspotDensity                   float64           `json:"hotspot_density"`
	ReleaseRiskScore                 float64           `json:"release_risk_score"`
	NetLines                         int               `json:"net_lines"`
	LinesAdded                       int               `json:"lines_added"`
	LinesDeleted                     int               `json:"lines_deleted"`
	FilesTouched                     int               `json:"files_touched"`
	JiraTickets                      int               `json:"jira_tickets"`
	ChurnFactor                      float64           `json:"churn_factor"`
	ComplexityPerFeat                float64           `json:"complexity_per_feat"`
	FixToFeatureRatio                float64           `json:"fix_to_feature_ratio"`
	TestToSourceRatio                float64           `json:"test_to_source_ratio"`
	RefactoringRatio                 float64           `json:"refactoring_ratio"`
	APISurfaceChange                 int               `json:"api_surface_change"`
	BatchFactor                      float64           `json:"batch_factor"`
	RevertRate                       float64           `json:"revert_rate"`
	ScopeIsolation                   float64           `json:"scope_isolation"`
	CrossCuttingPct                  float64           `json:"cross_cutting_pct"`
	FileVolatility                   float64           `json:"file_volatility"`
	ChangeComplexityProxy            float64           `json:"change_complexity_proxy"`
	CrossCuttingChangeRisk           float64           `json:"cross_cutting_change_risk"`
	TouchedTestFileRatio             float64           `json:"touched_test_file_ratio"`
	CommitSizeSmall                  int               `json:"commit_size_small"`
	CommitSizeMedium                 int               `json:"commit_size_medium"`
	CommitSizeLarge                  int               `json:"commit_size_large"`
	CommitSizeHuge                   int               `json:"commit_size_huge"`
	ReviewLoad                       float64           `json:"review_load"`
	DeleteRatio                      float64           `json:"delete_ratio"`
	CommitMsgQuality                 float64           `json:"commit_msg_quality"`
	AuthorOverlap                    float64           `json:"author_overlap"`
	PeakHourSpread                   float64           `json:"peak_hour_spread"`
	DirectoryBreadth                 float64           `json:"directory_breadth"`
	ConfigChurnRate                  float64           `json:"config_churn_rate"`
	CodeFreshness                    float64           `json:"code_freshness"`
	RhythmConsistency                float64           `json:"rhythm_consistency"`
	ReleaseCadenceDays               int               `json:"release_cadence_days"`
	DepUpdateFreq                    float64           `json:"dep_update_freq"`
	LanguageMix                      float64           `json:"language_mix"`
	TypeCounts                       map[string]int    `json:"type_counts"`
	SignificanceCounts               map[string]int    `json:"significance_counts"`
	TopContributors                  []ContributorSnap `json:"top_contributors"`

	// Read-only compatibility for snapshots written before proxy metrics were
	// named honestly. New snapshots omit these fields.
	LegacyBusFactor            int     `json:"bus_factor,omitempty"`
	LegacyCycleTimeHours       float64 `json:"cycle_time_hours,omitempty"`
	LegacyCognitiveComplexity  float64 `json:"cognitive_complexity,omitempty"`
	LegacyCyclicDependencyRisk float64 `json:"cyclic_dependency_risk,omitempty"`
	LegacyTestCoverageNewCode  float64 `json:"test_coverage_new_code,omitempty"`
}

func Store(dir string, snap Snapshot) error {
	trendsDir := filepath.Join(dir, ".patchlog", "trends")
	if err := os.MkdirAll(trendsDir, 0755); err != nil {
		return fmt.Errorf("creating trends dir: %w", err)
	}

	safeName := sanitizeVersion(snap.Version)
	path := filepath.Join(trendsDir, safeName+".json")
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling snapshot: %w", err)
	}
	return atomicfile.Write(path, data, 0644)
}

func LoadAll(dir string) ([]Snapshot, error) {
	trendsDir := filepath.Join(dir, ".patchlog", "trends")
	entries, err := os.ReadDir(trendsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading trends dir: %w", err)
	}

	var snapshots []Snapshot
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(trendsDir, entry.Name()))
		if err != nil {
			continue
		}
		var snap Snapshot
		if err := json.Unmarshal(data, &snap); err != nil {
			continue
		}
		if snap.Version == "" {
			continue
		}
		snap.normalizeLegacyFields()
		snapshots = append(snapshots, snap)
	}

	sort.Slice(snapshots, func(i, j int) bool {
		return parseDate(snapshots[i].Date).Before(parseDate(snapshots[j].Date))
	})

	return snapshots, nil
}

func (s *Snapshot) normalizeLegacyFields() {
	if s.ReleaseContributionConcentration == 0 {
		s.ReleaseContributionConcentration = s.LegacyBusFactor
	}
	if s.ReleaseCommitSpanHours == 0 {
		s.ReleaseCommitSpanHours = s.LegacyCycleTimeHours
	}
	if s.ChangeComplexityProxy == 0 {
		s.ChangeComplexityProxy = s.LegacyCognitiveComplexity
	}
	if s.CrossCuttingChangeRisk == 0 {
		s.CrossCuttingChangeRisk = s.LegacyCyclicDependencyRisk
	}
	if s.TouchedTestFileRatio == 0 {
		s.TouchedTestFileRatio = s.LegacyTestCoverageNewCode
	}
	s.LegacyBusFactor = 0
	s.LegacyCycleTimeHours = 0
	s.LegacyCognitiveComplexity = 0
	s.LegacyCyclicDependencyRisk = 0
	s.LegacyTestCoverageNewCode = 0
}

func Load(dir string, count int) ([]Snapshot, error) {
	all, err := LoadAll(dir)
	if err != nil {
		return nil, err
	}
	if count > 0 && len(all) > count {
		all = all[len(all)-count:]
	}
	return all, nil
}

func sanitizeVersion(v string) string {
	r := strings.NewReplacer("/", "_", ".", "-", " ", "_")
	return r.Replace(v)
}

func parseDate(s string) time.Time {
	formats := []string{"2006-01-02", "2006-01-02 15:04:05", time.RFC3339}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

var sparkChars = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

func Sparkline(values []float64) string {
	if len(values) == 0 {
		return ""
	}
	if len(values) == 1 {
		return "▄"
	}

	min, max := values[0], values[0]
	for _, v := range values[1:] {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}

	if max == min {
		var sb strings.Builder
		for range values {
			sb.WriteRune('▄')
		}
		return sb.String()
	}

	range_ := max - min
	var sb strings.Builder
	for _, v := range values {
		normalized := (v - min) / range_
		idx := int(normalized * float64(len(sparkChars)-1))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(sparkChars) {
			idx = len(sparkChars) - 1
		}
		sb.WriteRune(sparkChars[idx])
	}
	return sb.String()
}

func SparklineInt(values []int) string {
	floats := make([]float64, len(values))
	for i, v := range values {
		floats[i] = float64(v)
	}
	return Sparkline(floats)
}

type Delta struct {
	Version string
	Change  float64
	Percent float64
}

func ComputeDelta(prev, curr float64) Delta {
	change := curr - prev
	var pct float64
	if prev != 0 {
		pct = (change / math.Abs(prev)) * 100
	}
	return Delta{Change: change, Percent: pct}
}

func TrendArrow(d Delta) string {
	if d.Change > 0 {
		return "▲"
	}
	if d.Change < 0 {
		return "▼"
	}
	return "▬"
}
