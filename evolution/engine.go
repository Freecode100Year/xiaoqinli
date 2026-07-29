package evolution

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"xiaoqinli/codegen"
)

// MaxSelfEvolutionRetries limits auto-fix / self-update attempts to prevent infinite deadloops.
const MaxSelfEvolutionRetries = 3

// MaxRecursionDepth bounds self-evolution & AST resolution tree depth.
const MaxRecursionDepth = 64

// SafeExecute runs a self-evolution function with panic recovery to guarantee zero crashes.
func SafeExecute(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("XQL_E500: self-evolution panic recovered safely: %v", r)
		}
	}()
	return fn()
}

// LoopBreaker tracks visited keys to prevent graph cycles & infinite execution loops.
type LoopBreaker struct {
	visited map[string]int
	mu      sync.Mutex
}

// NewLoopBreaker creates a new LoopBreaker instance.
func NewLoopBreaker() *LoopBreaker {
	return &LoopBreaker{visited: make(map[string]int)}
}

// Track checks if key exceeds MaxSelfEvolutionRetries, preventing infinite loops.
func (lb *LoopBreaker) Track(key string) error {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.visited[key]++
	if lb.visited[key] > MaxSelfEvolutionRetries {
		return fmt.Errorf("XQL_E501: deadloop intercepted! key '%s' exceeded max retries (%d)", key, MaxSelfEvolutionRetries)
	}
	return nil
}

// Reset clears loop breaker state.
func (lb *LoopBreaker) Reset() {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.visited = make(map[string]int)
}

// ResetMemoryForTesting resets evolution memory maps for unit testing.
func ResetMemoryForTesting() {
	diagMutex.Lock()
	diagFixes = map[string][]*DiagnosticFixRecord{}
	diagMutex.Unlock()

	tsMutex.Lock()
	tsMappings = map[string]*TreeSitterMapping{}
	tsMutex.Unlock()

	skillMutex.Lock()
	dynamicSkills = map[string]*DynamicSkill{}
	skillMutex.Unlock()
}

// ---------------------------------------------------------------------------
// 1. Diagnostic Memory (纠错经验记忆引擎)
// ---------------------------------------------------------------------------

// DiagnosticFixRecord records a successful compiler error fix pattern.
type DiagnosticFixRecord struct {
	ErrorCode    string `json:"error_code"`    // e.g. XQL_E201
	ErrorContext string `json:"error_context"` // error message description
	TargetAST    string `json:"target_ast"`    // AST pattern snippet
	SuggestedFix string `json:"suggested_fix"` // proven fix strategy
	SuccessCount int    `json:"success_count"` // usage counter
	LastUsed     string `json:"last_used"`     // ISO timestamp
}

var (
	diagMutex sync.RWMutex
	diagFixes = map[string][]*DiagnosticFixRecord{}
)

// RecordDiagnosticFix saves or updates a proven diagnostic fix in local memory.
func RecordDiagnosticFix(code, errCtx, astPattern, fix string) *DiagnosticFixRecord {
	diagMutex.Lock()

	code = strings.ToUpper(strings.TrimSpace(code))
	now := time.Now().Format("2006-01-02 15:04:05")

	records := diagFixes[code]
	for _, r := range records {
		if r.SuggestedFix == fix {
			r.SuccessCount++
			r.LastUsed = now
			diagMutex.Unlock()
			autoSave()
			return r
		}
	}

	newRec := &DiagnosticFixRecord{
		ErrorCode:    code,
		ErrorContext: errCtx,
		TargetAST:    astPattern,
		SuggestedFix: fix,
		SuccessCount: 1,
		LastUsed:     now,
	}
	diagFixes[code] = append(diagFixes[code], newRec)
	diagMutex.Unlock()
	autoSave()
	return newRec
}

// InspectDiagnosticFixes retrieves learned diagnostic fixes for a specific error code.
func InspectDiagnosticFixes(code string) []*DiagnosticFixRecord {
	diagMutex.RLock()
	defer diagMutex.RUnlock()

	code = strings.ToUpper(strings.TrimSpace(code))
	records := diagFixes[code]
	out := make([]*DiagnosticFixRecord, len(records))
	for i, r := range records {
		cp := *r
		out[i] = &cp
	}
	return out
}

// ---------------------------------------------------------------------------
// 2. Tree-sitter WASM Mapping Synthesis (小众语言 AST 自适应映射引擎)
// ---------------------------------------------------------------------------

// TreeSitterMapping defines AST node mapping between Tree-sitter and .xql.json.
type TreeSitterMapping struct {
	Language       string            `json:"language"`        // e.g. "Mojo", "Gleam"
	Target         string            `json:"target"`          // e.g. "mojo", "gleam"
	NodeMappings   map[string]string `json:"node_mappings"`   // e.g. "function_definition" -> "FunctionDecl"
	KeywordMapping map[string]string `json:"keyword_mapping"` // e.g. "fn" -> "function"
	LastUpdated    string            `json:"last_updated"`
}

var (
	tsMutex    sync.RWMutex
	tsMappings = map[string]*TreeSitterMapping{}
)

// UpdateTreeSitterMapping registers or updates a Tree-sitter WASM grammar mapping locally.
func UpdateTreeSitterMapping(m TreeSitterMapping) *TreeSitterMapping {
	tsMutex.Lock()

	target := strings.ToLower(strings.TrimSpace(m.Target))
	m.Target = target
	m.LastUpdated = time.Now().Format("2006-01-02 15:04:05")
	tsMappings[target] = &m
	tsMutex.Unlock()

	// Also ensure target is in codegen profile
	_, _ = codegen.UpdateLanguageProfile(codegen.LanguageProfile{
		Target:         target,
		Language:       m.Language,
		LatestVersion:  "WASM Dynamic",
		ModernFeatures: []string{"Tree-sitter WASM grammar mapping", "Dynamic AST synthesis"},
	})

	autoSave()
	return &m
}

// InspectTreeSitterMapping retrieves AST node mapping for a target language.
func InspectTreeSitterMapping(target string) (*TreeSitterMapping, error) {
	tsMutex.RLock()
	defer tsMutex.RUnlock()

	norm := strings.ToLower(strings.TrimSpace(target))
	m, ok := tsMappings[norm]
	if !ok {
		return nil, fmt.Errorf("XQL_E404: tree-sitter mapping for target '%s' not found", target)
	}
	cp := *m
	return &cp, nil
}

// ---------------------------------------------------------------------------
// 3. Security & Capability Policy (环境权限策略演进引擎)
// ---------------------------------------------------------------------------

// SecurityPolicyConfig defines dynamic capability grant rules and sandbox bounds.
type SecurityPolicyConfig struct {
	Environment     string   `json:"environment"`      // e.g. "docker", "cloudflare", "bare_metal"
	AllowedGrants   []string `json:"allowed_grants"`   // e.g. ["io", "net", "fs"]
	ForbiddenGrants []string `json:"forbidden_grants"` // e.g. ["sys_exec"]
	MaxEffectLevel  string   `json:"max_effect_level"` // e.g. "state", "network", "pure"
	LastUpdated     string   `json:"last_updated"`
}

var (
	secMutex      sync.RWMutex
	currentPolicy = &SecurityPolicyConfig{
		Environment:     "default_sandbox",
		AllowedGrants:   []string{"io", "net", "fs", "env", "state"},
		ForbiddenGrants: []string{"unsafe_mem", "kernel_call"},
		MaxEffectLevel:  "state",
		LastUpdated:     time.Now().Format("2006-01-02"),
	}
)

// InspectSecurityPolicy gets current dynamic sandbox security policy.
func InspectSecurityPolicy() SecurityPolicyConfig {
	secMutex.RLock()
	defer secMutex.RUnlock()
	return *currentPolicy
}

// UpdateSecurityPolicy updates dynamic security policy bounds.
func UpdateSecurityPolicy(policy SecurityPolicyConfig) SecurityPolicyConfig {
	secMutex.Lock()
	policy.LastUpdated = time.Now().Format("2006-01-02 15:04:05")
	currentPolicy = &policy
	secMutex.Unlock()

	autoSave()
	return *currentPolicy
}

// ---------------------------------------------------------------------------
// 4. Stdlib API Change Matrix (标准库 API 变动矩阵)
// ---------------------------------------------------------------------------

// StdlibAPIMatrix holds deprecated and new recommended API mappings per language version.
type StdlibAPIMatrix struct {
	Target          string            `json:"target"`           // e.g. "py", "go"
	DeprecatedAPIs  map[string]string `json:"deprecated_apis"`  // old_api -> reason/new_api
	RecommendedAPIs map[string]string `json:"recommended_apis"` // feature -> new_api
	LastUpdated     string            `json:"last_updated"`
}

var (
	matrixMutex sync.RWMutex
	matrices    = map[string]*StdlibAPIMatrix{}
)

// UpdateStdlibMatrix updates standard library API evolution matrix.
func UpdateStdlibMatrix(m StdlibAPIMatrix) *StdlibAPIMatrix {
	matrixMutex.Lock()
	defer matrixMutex.Unlock()

	norm := strings.ToLower(strings.TrimSpace(m.Target))
	m.Target = norm
	m.LastUpdated = time.Now().Format("2006-01-02 15:04:05")
	matrices[norm] = &m
	return &m
}

// InspectStdlibMatrix gets standard library API evolution matrix for target language.
func InspectStdlibMatrix(target string) (*StdlibAPIMatrix, error) {
	matrixMutex.RLock()
	defer matrixMutex.RUnlock()

	norm := strings.ToLower(strings.TrimSpace(target))
	m, ok := matrices[norm]
	if !ok {
		// Return empty default matrix
		return &StdlibAPIMatrix{
			Target:          norm,
			DeprecatedAPIs:  map[string]string{},
			RecommendedAPIs: map[string]string{},
			LastUpdated:     time.Now().Format("2006-01-02"),
		}, nil
	}
	cp := *m
	return &cp, nil
}

// ---------------------------------------------------------------------------
// 5. Codegen Optimization Strategy (Codegen 性能策略反馈引擎)
// ---------------------------------------------------------------------------

// CodegenStrategyConfig holds performance tuned options for code generators.
type CodegenStrategyConfig = codegen.CodegenStrategyConfig

// UpdateCodegenStrategy updates performance strategy settings.
func UpdateCodegenStrategy(s CodegenStrategyConfig) *CodegenStrategyConfig {
	res := codegen.UpdateCodegenStrategy(s)
	autoSave()
	return res
}

// InspectCodegenStrategy gets current performance strategy for a target.
func InspectCodegenStrategy(target string) *CodegenStrategyConfig {
	return codegen.InspectCodegenStrategy(target)
}

// SaveEvolutionState persists all evolution states to disk.
func SaveEvolutionState(dirPath string) error {
	if dirPath == "" {
		dirPath = filepath.Join(".xql", "evolution")
	}
	_ = os.MkdirAll(dirPath, 0755)

	diagMutex.RLock()
	dData, _ := json.MarshalIndent(diagFixes, "", "  ")
	diagMutex.RUnlock()
	_ = os.WriteFile(filepath.Join(dirPath, "diagnostic_memory.json"), dData, 0644)

	tsMutex.RLock()
	tData, _ := json.MarshalIndent(tsMappings, "", "  ")
	tsMutex.RUnlock()
	_ = os.WriteFile(filepath.Join(dirPath, "treesitter_mappings.json"), tData, 0644)

	secMutex.RLock()
	sData, _ := json.MarshalIndent(currentPolicy, "", "  ")
	secMutex.RUnlock()
	_ = os.WriteFile(filepath.Join(dirPath, "security_policy.json"), sData, 0644)

	skillMutex.RLock()
	skData, _ := json.MarshalIndent(dynamicSkills, "", "  ")
	skillMutex.RUnlock()
	_ = os.WriteFile(filepath.Join(dirPath, "skills.json"), skData, 0644)

	_ = codegen.SaveStrategiesToFile(filepath.Join(dirPath, "codegen_strategies.json"))

	GetSearchEngine().AutoUpdateIndex()
	return nil
}

// LoadEvolutionState loads persisted evolution states from disk.
func LoadEvolutionState(dirPath string) error {
	if dirPath == "" {
		dirPath = filepath.Join(".xql", "evolution")
	}

	// 1. Diagnostic memory
	if data, err := os.ReadFile(filepath.Join(dirPath, "diagnostic_memory.json")); err == nil {
		diagMutex.Lock()
		_ = json.Unmarshal(data, &diagFixes)
		diagMutex.Unlock()
	}

	// 2. Tree-sitter mappings
	if data, err := os.ReadFile(filepath.Join(dirPath, "treesitter_mappings.json")); err == nil {
		tsMutex.Lock()
		_ = json.Unmarshal(data, &tsMappings)
		tsMutex.Unlock()
	}

	// 3. Security policy
	if data, err := os.ReadFile(filepath.Join(dirPath, "security_policy.json")); err == nil {
		secMutex.Lock()
		_ = json.Unmarshal(data, currentPolicy)
		secMutex.Unlock()
	}

	// 4. Dynamic skills
	if data, err := os.ReadFile(filepath.Join(dirPath, "skills.json")); err == nil {
		skillMutex.Lock()
		_ = json.Unmarshal(data, &dynamicSkills)
		skillMutex.Unlock()
	}

	// 5. Codegen strategies
	_ = codegen.LoadStrategiesFromFile(filepath.Join(dirPath, "codegen_strategies.json"))

	return nil
}

// ---------------------------------------------------------------------------
// 6. Universal Skill & Gap-Filling Engine (通用 Skill 自我进化补齐引擎)
// ---------------------------------------------------------------------------

// DynamicSkill represents an auto-evolved or user-registered agent skill module.
type DynamicSkill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Content     string `json:"content"`
	GapCategory string `json:"gap_category,omitempty"` // e.g. "missing_backend", "new_protocol"
	LastUpdated string `json:"last_updated"`
}

var (
	skillMutex    sync.RWMutex
	dynamicSkills = map[string]*DynamicSkill{}
)

// RegisterDynamicSkill registers or updates a dynamic self-evolved skill.
func RegisterDynamicSkill(skill DynamicSkill) *DynamicSkill {
	skillMutex.Lock()

	name := strings.ToLower(strings.TrimSpace(skill.Name))
	skill.Name = name
	skill.LastUpdated = time.Now().Format("2006-01-02 15:04:05")
	dynamicSkills[name] = &skill
	skillMutex.Unlock()

	autoSave()
	return &skill
}

var (
	autoSaveDir      string
	autoSaveDisabled bool
	autoSaveMutex    sync.RWMutex
)

// SetAutoSaveDir configures custom target directory for write-through state persistence.
func SetAutoSaveDir(dir string) {
	autoSaveMutex.Lock()
	defer autoSaveMutex.Unlock()
	autoSaveDir = dir
}

// SetAutoSaveDisabled enables or disables write-through disk side effects.
func SetAutoSaveDisabled(disabled bool) {
	autoSaveMutex.Lock()
	defer autoSaveMutex.Unlock()
	autoSaveDisabled = disabled
}

func autoSave() {
	autoSaveMutex.RLock()
	disabled := autoSaveDisabled
	dir := autoSaveDir
	autoSaveMutex.RUnlock()

	if disabled {
		GetSearchEngine().AutoUpdateIndex()
		return
	}
	_ = SaveEvolutionState(dir)
}

// GetDynamicSkill retrieves a registered dynamic skill by name.
func GetDynamicSkill(name string) (*DynamicSkill, bool) {
	skillMutex.RLock()
	defer skillMutex.RUnlock()

	norm := strings.ToLower(strings.TrimSpace(name))
	sk, ok := dynamicSkills[norm]
	if !ok {
		return nil, false
	}
	cp := *sk
	return &cp, true
}

// ListDynamicSkills returns all registered dynamic self-evolved skills.
func ListDynamicSkills() []*DynamicSkill {
	skillMutex.RLock()
	defer skillMutex.RUnlock()

	out := make([]*DynamicSkill, 0, len(dynamicSkills))
	for _, sk := range dynamicSkills {
		cp := *sk
		out = append(out, &cp)
	}
	return out
}

// DiagnoseAndFillSkillGap automatically generates and registers a skill snippet to fill a detected capability gap.
func DiagnoseAndFillSkillGap(taskContext, missingCapability string) *DynamicSkill {
	name := strings.ToLower(strings.TrimSpace(missingCapability))
	if name == "" {
		name = "auto_evolved_skill"
	}

	content := fmt.Sprintf(`---
name: %s
description: "Auto-evolved Skill created to fill capability gap in %s."
version: 1.0.0
author: xiaoqinli-evolution-engine
---

# Auto-Evolved Skill: %s

> **Self-Evolution Diagnostic**: Detected missing capability '%s' during task context: %s.

## Evolution Guidance & Operational Rules
1. **Self-Healing Protocol**: Synthesize required AST nodes and effect boundaries.
2. **Deterministic Output**: Fallback to safest type checks before code generation.
`, name, missingCapability, name, missingCapability, taskContext)

	skill := DynamicSkill{
		Name:        name,
		Description: fmt.Sprintf("Auto-evolved skill for capability '%s'", missingCapability),
		Content:     content,
		GapCategory: "capability_gap",
	}

	return RegisterDynamicSkill(skill)
}
