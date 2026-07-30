package evolution

import (
	"strings"
	"testing"
)

func TestFullSelfEvolutionEngine(t *testing.T) {
	ResetMemoryForTesting()

	// 1. Test Diagnostic Memory
	rec := RecordDiagnosticFix("XQL_E201", "Type mismatch in function return", "ReturnStmt.value", "Cast float to int explicitly")
	if rec.SuccessCount != 1 {
		t.Errorf("expected SuccessCount 1, got %d", rec.SuccessCount)
	}

	fixes := InspectDiagnosticFixes("XQL_E201")
	if len(fixes) != 1 {
		t.Fatalf("expected 1 fix record, got %d", len(fixes))
	}
	if fixes[0].SuggestedFix != "Cast float to int explicitly" {
		t.Errorf("unexpected suggested fix: %s", fixes[0].SuggestedFix)
	}

	// 2. Test Tree-sitter WASM Mapping
	tsMap := UpdateTreeSitterMapping(TreeSitterMapping{
		Language: "Mojo",
		Target:   "mojo",
		NodeMappings: map[string]string{
			"function_definition": "FunctionDecl",
		},
		KeywordMapping: map[string]string{
			"fn": "function",
		},
	})
	if tsMap.Target != "mojo" {
		t.Errorf("expected target mojo, got %s", tsMap.Target)
	}

	inspectedMap, err := InspectTreeSitterMapping("mojo")
	if err != nil {
		t.Fatalf("InspectTreeSitterMapping error: %v", err)
	}
	if inspectedMap.Language != "Mojo" {
		t.Errorf("expected Mojo, got %s", inspectedMap.Language)
	}

	// 3. Test Security Policy
	pol := UpdateSecurityPolicy(SecurityPolicyConfig{
		Environment:     "wasm_sandbox",
		AllowedGrants:   []string{"io"},
		ForbiddenGrants: []string{"net", "fs"},
		MaxEffectLevel:  "pure",
	})
	if pol.MaxEffectLevel != "pure" {
		t.Errorf("expected MaxEffectLevel pure, got %s", pol.MaxEffectLevel)
	}
	curPol := InspectSecurityPolicy()
	if curPol.Environment != "wasm_sandbox" {
		t.Errorf("expected environment wasm_sandbox, got %s", curPol.Environment)
	}

	// 3. Test Codegen Strategy
	strat := UpdateCodegenStrategy(CodegenStrategyConfig{
		Target:              "py",
		PreferComprehension: true,
		InlineThreshold:     50,
		OptimizationFlags:   map[string]string{"opt_level": "3"},
		BenchmarkScore:      98.5,
	})
	if strat.BenchmarkScore != 98.5 {
		t.Errorf("expected benchmark score 98.5, got %f", strat.BenchmarkScore)
	}
	inspectedStrat := InspectCodegenStrategy("py")
	if inspectedStrat.InlineThreshold != 50 {
		t.Errorf("expected inline threshold 50, got %d", inspectedStrat.InlineThreshold)
	}

	// 6. Test Universal Skill & Gap-Filling Engine
	dynSkill := DiagnoseAndFillSkillGap("quantum_context", "quantum_circuit_codegen")
	if dynSkill.Name != "quantum_circuit_codegen" {
		t.Errorf("expected skill name quantum_circuit_codegen, got %s", dynSkill.Name)
	}

	sk, found := GetDynamicSkill("quantum_circuit_codegen")
	if !found || sk.GapCategory != "capability_gap" {
		t.Errorf("expected found dynamic skill with gap_category capability_gap")
	}

	allSkills := ListDynamicSkills()
	if len(allSkills) == 0 {
		t.Errorf("expected non-empty ListDynamicSkills")
	}
}

func TestPanicShieldAndLoopBreaker(t *testing.T) {
	// 1. Test Panic Recovery (Zero Crash Guarantee)
	err := SafeExecute(func() error {
		var nilPtr *int
		*nilPtr = 42 // Trigger panic
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "self-evolution panic recovered safely") {
		t.Fatalf("expected panic recovery error, got %v", err)
	}

	// 2. Test Deadloop Interception
	lb := NewLoopBreaker()
	for i := 0; i < MaxSelfEvolutionRetries; i++ {
		if err := lb.Track("deadloop_key"); err != nil {
			t.Fatalf("unexpected loop breaker error at iteration %d: %v", i, err)
		}
	}
	// The 4th attempt must be intercepted!
	errLoop := lb.Track("deadloop_key")
	if errLoop == nil || !strings.Contains(errLoop.Error(), "deadloop intercepted") {
		t.Fatalf("expected deadloop error, got %v", errLoop)
	}
}

func TestRegisterDynamicSkillDoubleCallConcurrently(t *testing.T) {
	// Consecutive double-call verification to ensure skillMutex is cleanly released before autoSave
	sk1 := RegisterDynamicSkill(DynamicSkill{
		Name:        "skill_test_double_call_1",
		GapCategory: "test_gap",
		Content:     "fn test1() {}",
	})
	if sk1 == nil || sk1.Name != "skill_test_double_call_1" {
		t.Fatalf("expected first RegisterDynamicSkill call to succeed")
	}

	sk2 := RegisterDynamicSkill(DynamicSkill{
		Name:        "skill_test_double_call_2",
		GapCategory: "test_gap",
		Content:     "fn test2() {}",
	})
	if sk2 == nil || sk2.Name != "skill_test_double_call_2" {
		t.Fatalf("expected second RegisterDynamicSkill call to succeed without deadlock")
	}
}
