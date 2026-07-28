package evolution

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// IndexEntry represents a searchable item in the AI Agent Search Engine.
type IndexEntry struct {
	ID        string   `json:"id"`
	Category  string   `json:"category"` // e.g. "skill", "diagnostic", "spec", "policy"
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

// Query searches entries matching keyword and category.
func (se *SearchEngine) Query(keyword, category string) []*IndexEntry {
	se.mu.RLock()
	defer se.mu.RUnlock()

	var results []*IndexEntry
	kw := strings.ToLower(strings.TrimSpace(keyword))
	cat := strings.ToLower(strings.TrimSpace(category))

	for _, entry := range se.entries {
		if cat != "" && strings.ToLower(entry.Category) != cat {
			continue
		}
		if matchEntry(entry, kw) {
			cp := *entry
			results = append(results, &cp)
		}
	}
	return results
}

// matchEntry checks if an IndexEntry matches the keyword.
func matchEntry(e *IndexEntry, kw string) bool {
	if kw == "" {
		return true
	}
	if strings.Contains(strings.ToLower(e.Title), kw) || strings.Contains(strings.ToLower(e.Content), kw) {
		return true
	}
	for _, tag := range e.Tags {
		if strings.Contains(strings.ToLower(tag), kw) {
			return true
		}
	}
	return false
}

// AutoUpdateIndex scans evolution memory and re-indexes all entries.
func (se *SearchEngine) AutoUpdateIndex() int {
	se.indexSkills()
	se.indexDiagnostics()
	se.indexSecurityPolicies()
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
		for _, r := range records {
			se.RegisterEntry(IndexEntry{
				ID:       fmt.Sprintf("diag-%s-%s", code, r.SuggestedFix),
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
