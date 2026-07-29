package evolution

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"xiaoqinli/codegen"
)

// IndexEntry represents a searchable item in the AI Agent Search Engine.
type IndexEntry struct {
	ID        string   `json:"id"`
	Category  string   `json:"category"` // e.g. "skill", "diagnostic", "spec", "policy", "risk"
	Title     string   `json:"title"`
	Content   string   `json:"content"`
	Tags      []string `json:"tags"`
	UpdatedAt string   `json:"updated_at"`
}

// SearchEngine manages knowledge index and real-time query for AI Agents.
type SearchEngine struct {
	mu      sync.RWMutex
	entries map[string]*IndexEntry
}

var (
	globalEngine *SearchEngine
	engineOnce   sync.Once
)

// GetSearchEngine returns the singleton instance of SearchEngine.
func GetSearchEngine() *SearchEngine {
	engineOnce.Do(func() {
		globalEngine = &SearchEngine{
			entries: make(map[string]*IndexEntry),
		}
		globalEngine.AutoUpdateIndex()
	})
	return globalEngine
}

// RegisterEntry adds or updates an index entry.
func (se *SearchEngine) RegisterEntry(entry IndexEntry) *IndexEntry {
	se.mu.Lock()
	defer se.mu.Unlock()

	if entry.ID == "" {
		entry.ID = fmt.Sprintf("%s-%d", entry.Category, time.Now().UnixNano())
	}
	entry.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
	se.entries[entry.ID] = &entry
	return &entry
}

type scoredEntry struct {
	entry *IndexEntry
	score int
}

// Query searches entries matching keyword and category with deterministic relevance sorting.
func (se *SearchEngine) Query(keyword, category string) []*IndexEntry {
	se.mu.RLock()
	defer se.mu.RUnlock()

	var scored []*scoredEntry
	kw := strings.ToLower(strings.TrimSpace(keyword))
	cat := strings.ToLower(strings.TrimSpace(category))

	for _, entry := range se.entries {
		if cat != "" && strings.ToLower(entry.Category) != cat {
			continue
		}
		if kw == "" {
			cp := *entry
			scored = append(scored, &scoredEntry{entry: &cp, score: 0})
			continue
		}

		score := calculateScore(entry, kw, cat)
		if score > 0 {
			cp := *entry
			scored = append(scored, &scoredEntry{entry: &cp, score: score})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		if scored[i].entry.UpdatedAt != scored[j].entry.UpdatedAt {
			return scored[i].entry.UpdatedAt > scored[j].entry.UpdatedAt
		}
		return scored[i].entry.ID < scored[j].entry.ID
	})

	results := make([]*IndexEntry, len(scored))
	for i, s := range scored {
		results[i] = s.entry
	}
	return results
}

func calculateScore(e *IndexEntry, kw, cat string) int {
	score := 0
	titleLower := strings.ToLower(e.Title)
	contentLower := strings.ToLower(e.Content)

	if titleLower == kw {
		score += 20
	} else if strings.Contains(titleLower, kw) {
		score += 10
	}

	for _, tag := range e.Tags {
		if strings.ToLower(tag) == kw {
			score += 8
		} else if strings.Contains(strings.ToLower(tag), kw) {
			score += 4
		}
	}

	if strings.Contains(contentLower, kw) {
		score += 2
	}

	if cat != "" && strings.ToLower(e.Category) == cat {
		score += 1
	}

	return score
}

// AutoUpdateIndex scans evolution memory and re-indexes all entries.
func (se *SearchEngine) AutoUpdateIndex() int {
	se.mu.Lock()
	se.entries = make(map[string]*IndexEntry)
	se.mu.Unlock()

	se.indexSkills()
	se.indexDiagnostics()
	se.indexSecurityPolicies()
	se.indexSpecs()
	se.indexCapabilityRisks()
	se.mu.RLock()
	defer se.mu.RUnlock()
	return len(se.entries)
}

func (se *SearchEngine) indexSkills() {
	skills := ListDynamicSkills()
	for _, sk := range skills {
		se.RegisterEntry(IndexEntry{
			ID:       "skill-" + sk.Name,
			Category: "skill",
			Title:    sk.Name,
			Content:  sk.Content + " " + sk.Description,
			Tags:     []string{"skill", sk.GapCategory},
		})
	}
}

func (se *SearchEngine) indexDiagnostics() {
	diagMutex.RLock()
	defer diagMutex.RUnlock()
	for code, records := range diagFixes {
		if len(records) > 0 {
			r := records[len(records)-1]
			se.RegisterEntry(IndexEntry{
				ID:       "diag-" + code,
				Category: "diagnostic",
				Title:    "Diagnostic Fix " + code,
				Content:  r.ErrorContext + " " + r.SuggestedFix,
				Tags:     []string{"diagnostic", code},
			})
		}
	}
}

func (se *SearchEngine) indexSecurityPolicies() {
	pol := InspectSecurityPolicy()
	se.RegisterEntry(IndexEntry{
		ID:       "policy-" + pol.Environment,
		Category: "policy",
		Title:    "Security Policy " + pol.Environment,
		Content:  fmt.Sprintf("Allowed: %v, Forbidden: %v", pol.AllowedGrants, pol.ForbiddenGrants),
		Tags:     []string{"policy", pol.Environment},
	})
}

func (se *SearchEngine) indexSpecs() {
	allProfiles := codegen.ListAllLanguageProfiles()
	for target, prof := range allProfiles {
		featuresStr := strings.Join(prof.ModernFeatures, ", ")
		content := fmt.Sprintf("Language: %s (Target: %s), Version: %s, Modern Features: [%s]", prof.Language, target, prof.LatestVersion, featuresStr)
		if len(prof.CodegenOptions) > 0 {
			optsStr, _ := json.Marshal(prof.CodegenOptions)
			content += fmt.Sprintf(", CodegenOptions: %s", string(optsStr))
		}
		tags := append([]string{"spec", target, prof.Language}, prof.ModernFeatures...)
		se.RegisterEntry(IndexEntry{
			ID:       "spec-" + target,
			Category: "spec",
			Title:    fmt.Sprintf("Language Spec %s (%s)", prof.Language, target),
			Content:  content,
			Tags:     tags,
		})
	}
}

func (se *SearchEngine) indexCapabilityRisks() {
	se.RegisterEntry(IndexEntry{
		ID:       "risk-unresolved-calls",
		Category: "risk",
		Title:    "Capability Security Audit: Unresolved Function Call Fail-Open Risk",
		Content:  "Unresolved or unknown function calls pass capability check with zero required grant in default mode. Enable strict mode (--strict-caps / CheckCapabilitiesStrict) to intercept unresolved calls with error XQL_E303.",
		Tags:     []string{"risk", "capability", "effect", "XQL_E303", "strict-caps", "fail-open"},
	})
}
