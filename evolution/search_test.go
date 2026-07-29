package evolution

import (
	"strings"
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

func TestSearchEngineIndexSpecs(t *testing.T) {
	se := GetSearchEngine()
	specs := se.Query("python", "spec")
	if len(specs) == 0 {
		t.Fatalf("expected to find python spec entry in category 'spec'")
	}
	found := false
	for _, s := range specs {
		if strings.Contains(strings.ToLower(s.Title), "python") || s.ID == "spec-py" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected spec-py entry to be indexed")
	}
}

func TestSearchEngineCapabilityRiskIndex(t *testing.T) {
	se := GetSearchEngine()
	risks := se.Query("strict-caps", "risk")
	if len(risks) == 0 {
		t.Fatalf("expected to find capability risk entry in category 'risk'")
	}
	if risks[0].ID != "risk-unresolved-calls" {
		t.Errorf("expected risk-unresolved-calls, got: %s", risks[0].ID)
	}
}

func TestSearchEngineDeterministicSorting(t *testing.T) {
	se := GetSearchEngine()
	res1 := se.Query("python", "")
	res2 := se.Query("python", "")

	if len(res1) != len(res2) {
		t.Fatalf("result lengths differ across consecutive queries")
	}

	for i := range res1 {
		if res1[i].ID != res2[i].ID {
			t.Errorf("non-deterministic search result order at index %d: %s vs %s", i, res1[i].ID, res2[i].ID)
		}
	}
}

func TestSearchEngineDiagnosticOverwrite(t *testing.T) {
	ResetMemoryForTesting()
	se := GetSearchEngine()
	_ = RecordDiagnosticFix("XQL_E888", "Old error", "ASTPattern", "Old fix")
	_ = RecordDiagnosticFix("XQL_E888", "New error", "ASTPattern", "Latest updated fix")

	diags := se.Query("XQL_E888", "diagnostic")
	if len(diags) != 1 {
		t.Fatalf("expected exactly 1 overwritten diagnostic entry for XQL_E888, got %d", len(diags))
	}
	if !strings.Contains(diags[0].Content, "Latest updated fix") {
		t.Errorf("expected diagnostic entry to contain latest fix, got: %s", diags[0].Content)
	}
}
