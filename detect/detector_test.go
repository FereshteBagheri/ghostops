package detect

import (
	"slices"
	"testing"

	"ghostops/logentry"
)

func entry(level logentry.Level, msg string) logentry.Entry {
	return logentry.Entry{Level: level, Service: "svc", Message: msg}
}

func TestKeywordRuleMatch(t *testing.T) {
	db := NewKeywordRule("db", High, logentry.Error, "Database", "deadlock")

	tests := []struct {
		name  string
		entry logentry.Entry
		want  bool
	}{
		{"keyword at min level", entry(logentry.Error, "database down"), true},
		{"keyword above min level", entry(logentry.Fatal, "deadlock detected"), true},
		{"case is ignored", entry(logentry.Error, "DATABASE down"), true},
		{"keyword inside a word", entry(logentry.Error, "mydatabase down"), true},
		{"below min level", entry(logentry.Warn, "database slow"), false},
		{"no keyword", entry(logentry.Error, "disk full"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := db.Match(tt.entry); got != tt.want {
				t.Errorf("Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKeywordRuleWithoutKeywords(t *testing.T) {
	anyError := NewKeywordRule("any", Low, logentry.Error)
	if !anyError.Match(entry(logentry.Error, "whatever")) {
		t.Error("rule without keywords should match any message at its level")
	}
	if anyError.Match(entry(logentry.Info, "whatever")) {
		t.Error("rule without keywords should still respect the min level")
	}
}

func TestClassifyDefaultRules(t *testing.T) {
	tests := []struct {
		name        string
		entry       logentry.Entry
		wantRule    string // "" means no rule
		wantMatches int
	}{
		{"fatal also matches the catch-all", entry(logentry.Fatal, "Process killed"), "service-crash", 2},
		{"fatal beats database", entry(logentry.Fatal, "Cannot reach database"), "service-crash", 3},
		{"fatal beats out-of-memory", entry(logentry.Fatal, "out of memory"), "service-crash", 3},
		{"oom error", entry(logentry.Error, "Out of memory: cannot allocate"), "out-of-memory", 2},
		{"database beats timeout", entry(logentry.Error, "Database query timed out"), "database-failure", 3},
		{"database error", entry(logentry.Error, "connection refused: postgres"), "database-failure", 2},
		{"auth warn", entry(logentry.Warn, "Authentication failed for alice"), "auth-failure", 1},
		{"auth error beats catch-all", entry(logentry.Error, "Permission denied"), "auth-failure", 2},
		{"timeout warn", entry(logentry.Warn, "Request timeout"), "timeout", 1},
		{"catch-all error", entry(logentry.Error, "Failed to parse SKU"), "error-burst", 1},
		{"database warn is not an incident", entry(logentry.Warn, "database slow"), "", 0},
		{"info is never an incident", entry(logentry.Info, "timeout config loaded"), "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule, matches := Classify(tt.entry, DefaultRules())
			got := ""
			if rule != nil {
				got = rule.Name()
			}
			if got != tt.wantRule || matches != tt.wantMatches {
				t.Errorf("Classify() = %q, %d matches; want %q, %d", got, matches, tt.wantRule, tt.wantMatches)
			}
		})
	}
}

func TestRuleOrderDecidesOverlap(t *testing.T) {
	a := NewKeywordRule("a", Low, logentry.Error, "x")
	b := NewKeywordRule("b", High, logentry.Error, "x")
	e := entry(logentry.Error, "x")

	if r, _ := Classify(e, []Rule{a, b}); r.Name() != "a" {
		t.Errorf("first rule should win, got %s", r.Name())
	}
	if r, _ := Classify(e, []Rule{b, a}); r.Name() != "b" {
		t.Errorf("first rule should win, got %s", r.Name())
	}
}

func TestDetectorCounts(t *testing.T) {
	d := NewDetector(DefaultRules(), 0)
	for _, e := range []logentry.Entry{
		entry(logentry.Fatal, "Cannot reach database"),
		entry(logentry.Error, "Database connection refused"),
		entry(logentry.Error, "Database connection refused"),
		entry(logentry.Warn, "Request timeout"),
		entry(logentry.Info, "all good"),
	} {
		d.Observe(e)
	}

	want := []RuleCount{
		{"service-crash", Critical, 1},
		{"out-of-memory", Critical, 0},
		{"database-failure", High, 2},
		{"auth-failure", Medium, 0},
		{"timeout", Medium, 1},
		{"error-burst", Low, 0},
	}
	if got := d.Counts(); !slices.Equal(got, want) {
		t.Errorf("Counts() =\n %v\nwant\n %v", got, want)
	}
	if d.Overlapping != 3 {
		t.Errorf("Overlapping = %d, want 3", d.Overlapping)
	}
}

func TestSeverityString(t *testing.T) {
	if Critical.String() != "CRITICAL" || Low.String() != "LOW" {
		t.Error("unexpected severity names")
	}
	if !(Low < Medium && Medium < High && High < Critical) {
		t.Error("severities are not ordered")
	}
}

func TestParseSeverity(t *testing.T) {
	for in, want := range map[string]Severity{"low": Low, "MEDIUM": Medium, "High": High, "critical": Critical} {
		got, err := ParseSeverity(in)
		if err != nil || got != want {
			t.Errorf("ParseSeverity(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	if _, err := ParseSeverity("urgent"); err == nil {
		t.Error("ParseSeverity(urgent) should fail")
	}
}
