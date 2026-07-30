package codegen

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// LanguageProfile holds modern language specification details and features for target languages.
type LanguageProfile struct {
	Target         string            `json:"target"`          // e.g. "py", "go", "ts", "rust"
	Language       string            `json:"language"`        // e.g. "Python"
	LatestVersion  string            `json:"latest_version"`  // e.g. "3.12+"
	ModernFeatures []string          `json:"modern_features"` // e.g. ["PEP 604 union types T | None", "Dataclasses", "match-case"]
	CodegenOptions map[string]string `json:"codegen_options"` // custom codegen config
	LastUpdated    string            `json:"last_updated"`    // ISO date/time string
}

var (
	profileMutex sync.RWMutex
	profiles     = map[string]*LanguageProfile{}
)

func init() {
	resetDefaultProfiles()
}

func resetDefaultProfiles() {
	now := time.Now().Format("2006-01-02")
	defaultProfiles := []*LanguageProfile{
		{
			Target:         "py",
			Language:       "Python",
			LatestVersion:  "3.12+",
			ModernFeatures: []string{"PEP 604 union types (T | None)", "dataclasses", "list[T] & dict[K, V] generic annotations", "match-case structural pattern matching", "PEP 695 type parameter syntax"},
			CodegenOptions: map[string]string{"type_style": "modern_union", "use_dataclass": "true"},
			LastUpdated:    now,
		},
		{
			Target:         "go",
			Language:       "Go",
			LatestVersion:  "1.23+",
			ModernFeatures: []string{"Generic Result[T, E] & Option[T]", "GC-free value receivers", "range-over-func iterators", "min/max built-in generics"},
			CodegenOptions: map[string]string{"generics": "true", "iterators": "true"},
			LastUpdated:    now,
		},
		{
			Target:         "ts",
			Language:       "TypeScript",
			LatestVersion:  "5.5+",
			ModernFeatures: []string{"Inferred type predicates", "ES2024 Set methods", "Result<T, E> generic class", "readonly modifier"},
			CodegenOptions: map[string]string{"target_es": "ES2024", "strict": "true"},
			LastUpdated:    now,
		},
		{
			Target:         "rust",
			Language:       "Rust",
			LatestVersion:  "2024 Edition",
			ModernFeatures: []string{"Native Result<T, E> & Option<T>", "Strict pattern matching", "async fn in traits", "GATs"},
			CodegenOptions: map[string]string{"edition": "2024"},
			LastUpdated:    now,
		},
		{
			Target:         "zig",
			Language:       "Zig",
			LatestVersion:  "0.13+",
			ModernFeatures: []string{"Strongly-typed anonymous struct coercion", "Comptime generics", "Explicit allocator injection"},
			CodegenOptions: map[string]string{"version": "0.13+", "verification_status": "local_e2e_validated"},
			LastUpdated:    now,
		},
		{
			Target:         "csharp",
			Language:       "C#",
			LatestVersion:  "12.0 (.NET 8+)",
			ModernFeatures: []string{"Primary constructors", "Collection expressions", "#nullable enable/disable protection", "Generic math"},
			CodegenOptions: map[string]string{"dotnet": "8.0", "verification_status": "local_e2e_validated"},
			LastUpdated:    now,
		},
		{
			Target:         "java",
			Language:       "Java",
			LatestVersion:  "21+",
			ModernFeatures: []string{"Record classes", "Pattern matching for switch", "Virtual threads", "Sealed classes"},
			CodegenOptions: map[string]string{"jdk": "21", "verification_status": "local_e2e_validated"},
			LastUpdated:    now,
		},
		{
			Target:         "swift",
			Language:       "Swift",
			LatestVersion:  "5.10 / 6.0",
			ModernFeatures: []string{"Strict concurrency checking", "Macros", "Result<Success, Failure> enum", "Opaque types"},
			CodegenOptions: map[string]string{"swift_version": "5.10", "verification_status": "local_e2e_validated"},
			LastUpdated:    now,
		},
		{
			Target:         "kotlin",
			Language:       "Kotlin",
			LatestVersion:  "2.0+",
			ModernFeatures: []string{"K2 compiler alignment", "Sealed interfaces", "Inline value classes", "Coroutines flow"},
			CodegenOptions: map[string]string{"kotlin_version": "2.0", "verification_status": "local_e2e_validated"},
			LastUpdated:    now,
		},
		{
			Target:         "dart",
			Language:       "Dart",
			LatestVersion:  "3.4+",
			ModernFeatures: []string{"Sound null safety", "Records & pattern matching", "Class modifiers (base/interface/final)", "Extension types"},
			CodegenOptions: map[string]string{"null_safety": "true", "verification_status": "local_e2e_validated"},
			LastUpdated:    now,
		},
		{
			Target:         "cpp",
			Language:       "C++",
			LatestVersion:  "C++20/C++23",
			ModernFeatures: []string{"std::expected<T, E>", "Concepts & Constraints", "Modules", "std::format"},
			CodegenOptions: map[string]string{"standard": "c++20"},
			LastUpdated:    now,
		},
		{
			Target:         "tccli",
			Language:       "Tencent Cloud CLI (tccli)",
			LatestVersion:  "v3.0+",
			ModernFeatures: []string{"Cloud resource orchestration", "Automated Describe/Create/Delete APIs", "JSON output parsing"},
			CodegenOptions: map[string]string{"shell": "bash", "cli": "tccli"},
			LastUpdated:    now,
		},
		{
			Target:         "android",
			Language:       "Android (Gradle APK)",
			LatestVersion:  "Gradle 8.0+ / Android 13+",
			ModernFeatures: []string{"Complete Gradle Android project scaffolding", "Jetpack AppCompat & Core KTX", "MainActivity.kt UI output binding"},
			CodegenOptions: map[string]string{"gradle": "8.1.0", "kotlin": "1.8.20"},
			LastUpdated:    now,
		},
		{
			Target:         "ios",
			Language:       "iOS (Swift Package Manager)",
			LatestVersion:  "Swift 5.8+ / iOS 14.0+",
			ModernFeatures: []string{"Complete Swift Package Manager project scaffolding", "SwiftUI & Foundation container", "Xcode & swift build compatibility"},
			CodegenOptions: map[string]string{"swift_version": "5.8", "spm": "true"},
			LastUpdated:    now,
		},
		{
			Target:         "deepseek",
			Language:       "DeepSeek Coder / V3",
			LatestVersion:  "V3 / Coder",
			ModernFeatures: []string{"FIM (Fill-In-Middle) completion", "AST-First structured JSON output", "64k context window"},
			CodegenOptions: map[string]string{"reasoning": "true", "ast_first": "true"},
			LastUpdated:    now,
		},
		{
			Target:         "qwen",
			Language:       "Qwen Code",
			LatestVersion:  "Qwen2.5-Coder",
			ModernFeatures: []string{"Multi-file codebase understanding", "Tool calling & MCP integration", "Repo-level completion"},
			CodegenOptions: map[string]string{"mcp_alignment": "true"},
			LastUpdated:    now,
		},
		{
			Target:         "nim",
			Language:       "Nim",
			LatestVersion:  "2.0+",
			ModernFeatures: []string{"ARC/ORC memory management", "Option[T] & Result[T, E]", "strictFuncs & effect tracking", "Macro metaprogramming"},
			CodegenOptions: map[string]string{"verification_status": "local_e2e_validated", "gc": "arc"},
			LastUpdated:    now,
		},
		{
			Target:         "julia",
			Language:       "Julia",
			LatestVersion:  "1.10+",
			ModernFeatures: []string{"Vector{T} typed arrays", "Multiple dispatch type annotations", "Union{T, Nothing} optionals", "Struct type coercion"},
			CodegenOptions: map[string]string{"verification_status": "local_e2e_validated", "opt_level": "2"},
			LastUpdated:    now,
		},
		{
			Target:         "php",
			Language:       "PHP",
			LatestVersion:  "8.3+",
			ModernFeatures: []string{"Readonly classes & properties", "Match expressions", "Typed class constants", "Union & Intersection types"},
			CodegenOptions: map[string]string{"verification_status": "local_e2e_validated", "strict_types": "1"},
			LastUpdated:    now,
		},
		{
			Target:         "ruby",
			Language:       "Ruby",
			LatestVersion:  "3.3+",
			ModernFeatures: []string{"YJIT compiler integration", "Pattern matching (in / case)", "Data value structs", "RBS type signatures"},
			CodegenOptions: map[string]string{"verification_status": "local_e2e_validated", "yjit": "true"},
			LastUpdated:    now,
		},
		{
			Target:         "lua",
			Language:       "Lua",
			LatestVersion:  "5.4 / LuaJIT",
			ModernFeatures: []string{"Const & to-be-closed variables", "Generational GC mode", "Metatable OOP pattern", "Bitwise operators"},
			CodegenOptions: map[string]string{"verification_status": "local_e2e_validated", "version": "5.4"},
			LastUpdated:    now,
		},
		{
			Target:         "kimi",
			Language:       "Kimi Code",
			LatestVersion:  "Moonshot Kimi",
			ModernFeatures: []string{"Long-context prompt caching", "Structured JSON AST output", "Lossless context retrieval"},
			CodegenOptions: map[string]string{"cache_prompt": "true"},
			LastUpdated:    now,
		},
		{
			Target:         "glm",
			Language:       "GLM Coding",
			LatestVersion:  "GLM-4 / CodeGLM",
			ModernFeatures: []string{"Function calling & agent execution", "Code interp & execution sandbox", "Multi-lingual transpilation"},
			CodegenOptions: map[string]string{"agent_framework": "true"},
			LastUpdated:    now,
		},
	}

	for _, p := range defaultProfiles {
		profiles[p.Target] = p
	}

	// Add basic profiles for long-tail 42+ target backends
	allTargets := []string{
		"js", "c", "scala", "haskell", "ocaml", "fsharp", "elixir", "clojure", "lua", "ruby", "php",
		"nim", "julia", "mql4", "mql5", "ada", "awk", "bash", "bat", "crystal", "d", "fortran",
		"objc", "pascal", "perl", "powershell", "tcl", "v", "vala", "groovy", "shortcut", "chrome",
	}

	for _, t := range allTargets {
		if _, exists := profiles[t]; !exists {
			profiles[t] = &LanguageProfile{
				Target:         t,
				Language:       strings.Title(t),
				LatestVersion:  "Modern Stable",
				ModernFeatures: []string{"Standard AST node codegen", "Type & Effect safety checking"},
				CodegenOptions: map[string]string{},
				LastUpdated:    now,
			}
		}
	}
}

// InspectLanguageProfile retrieves the modern language specification profile for a given target.
func InspectLanguageProfile(target string) (*LanguageProfile, error) {
	profileMutex.RLock()
	defer profileMutex.RUnlock()

	norm := strings.ToLower(strings.TrimSpace(target))
	p, ok := profiles[norm]
	if !ok {
		return nil, fmt.Errorf("XQL_E404: language profile for target '%s' not found", target)
	}
	// Return a copy
	cp := *p
	return &cp, nil
}

// ListAllLanguageProfiles returns all registered 42+ target language profiles.
func ListAllLanguageProfiles() map[string]*LanguageProfile {
	profileMutex.RLock()
	defer profileMutex.RUnlock()

	res := make(map[string]*LanguageProfile, len(profiles))
	for k, v := range profiles {
		cp := *v
		res[k] = &cp
	}
	return res
}

// UpdateLanguageProfile updates or registers a target language profile locally (Self-Updating).
func UpdateLanguageProfile(profile LanguageProfile) (*LanguageProfile, error) {
	if strings.TrimSpace(profile.Target) == "" {
		return nil, fmt.Errorf("XQL_E400: profile target cannot be empty")
	}

	profileMutex.Lock()
	defer profileMutex.Unlock()

	norm := strings.ToLower(strings.TrimSpace(profile.Target))
	profile.Target = norm
	profile.LastUpdated = time.Now().Format("2006-01-02 15:04:05")

	if existing, ok := profiles[norm]; ok {
		if profile.Language == "" {
			profile.Language = existing.Language
		}
		if len(profile.ModernFeatures) == 0 {
			profile.ModernFeatures = existing.ModernFeatures
		}
		if profile.CodegenOptions == nil {
			profile.CodegenOptions = existing.CodegenOptions
		}
	}

	profiles[norm] = &profile
	return &profile, nil
}

// LoadProfilesFromFile loads language profiles from a local JSON file for persistence & self-updating.
func LoadProfilesFromFile(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	var loaded map[string]*LanguageProfile
	if err := json.Unmarshal(data, &loaded); err != nil {
		return err
	}

	profileMutex.Lock()
	defer profileMutex.Unlock()
	for k, v := range loaded {
		profiles[strings.ToLower(k)] = v
	}
	return nil
}

// SaveProfilesToFile persists current 42+ language profiles to a local JSON file.
func SaveProfilesToFile(filePath string) error {
	profileMutex.RLock()
	defer profileMutex.RUnlock()
	return saveProfilesToFileLocked(filePath)
}

func saveProfilesToFileLocked(filePath string) error {
	data, err := json.MarshalIndent(profiles, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(filePath)
	if dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0755)
	}
	return os.WriteFile(filePath, data, 0644)
}
