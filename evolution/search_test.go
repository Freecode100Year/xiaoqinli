package evolution

import (
	"testing"
)

func TestSearchEngineQueryAndAutoUpdate(t *testing.T) {
	se := GetSearchEngine()

	// 1. Register test skill & diagnostic fix
	_ = DiagnoseAndFillSkillGap("search_test_ctx", "agent_search_skill")
	_ = RecordDiagnosticFix("XQL_E999", "Search error context", "ASTPattern", "Apply search index fix")

	// 2. Trigger AutoUpdateIndex
	count := se.AutoUpdateIndex()
	if count == 0 {
		t.Fatalf("expected non-zero index entries, got 0")
	}

	// 3. Query skill
	skills := se.Query("agent_search_skill", "skill")
	if len(skills) == 0 {
		t.Errorf("expected to find agent_search_skill entry")
	}

	// 4. Query diagnostic fix
	diags := se.Query("XQL_E999", "diagnostic")
	if len(diags) == 0 {
		t.Errorf("expected to find XQL_E999 diagnostic entry")
	}
}
