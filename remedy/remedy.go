// Package remedy provides bug fixes, sanitization, and resilience guards
// corresponding to security, session pruning, prompt caching, tool history,
// and idle loop gating updates.
package remedy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var userinfoRegex = regexp.MustCompile(`(?i)(https?://)([^:@/]+):([^@/]+)@`)

// StripURLUserinfo strips username and password credentials from a URL string.
func StripURLUserinfo(raw string) string {
	if raw == "" {
		return ""
	}
	if u, err := url.Parse(raw); err == nil && u.User != nil {
		u.User = url.User("REDACTED")
		return u.String()
	}
	return userinfoRegex.ReplaceAllString(raw, "${1}REDACTED@")
}

// Session represents an active session record.
type Session struct {
	ID         string    `json:"id"`
	LastActive time.Time `json:"last_active"`
}

// PreserveRecentlyActiveSessions filters sessions, preserving those active within threshold.
func PreserveRecentlyActiveSessions(sessions []Session, threshold time.Duration, now time.Time) []Session {
	var kept []Session
	for _, s := range sessions {
		if now.Sub(s.LastActive) <= threshold {
			kept = append(kept, s)
		}
	}
	return kept
}

// LockSkillSourceRegistry ensures a skill's original source registry is unchanged on update.
type SkillMetadata struct {
	Name           string `json:"name"`
	SourceRegistry string `json:"source_registry"`
	Content        string `json:"content"`
}

// UpdateSkillWithLockedRegistry applies updates while preserving original SourceRegistry.
func UpdateSkillWithLockedRegistry(orig, update SkillMetadata) SkillMetadata {
	res := update
	res.SourceRegistry = orig.SourceRegistry
	return res
}

// SanitizeSessionEnv cleans multiline session variables from terminal snapshots.
func SanitizeSessionEnv(env map[string]string) map[string]string {
	clean := make(map[string]string)
	for k, v := range env {
		if !strings.Contains(v, "\n") && !strings.Contains(v, "\r") {
			clean[k] = v
		}
	}
	return clean
}

// BoundedStartupRestoreGate bounds startup restore using context timeout.
func BoundedStartupRestoreGate(ctx context.Context, timeout time.Duration, restoreFunc func() error) error {
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- restoreFunc()
	}()

	select {
	case <-cctx.Done():
		return fmt.Errorf("startup-restore timed out after %v", timeout)
	case err := <-errCh:
		return err
	}
}

// ProbeValidateDeferredSchema validates raw tool call args against expected parameter keys.
func ProbeValidateDeferredSchema(rawArgs []byte, requiredKeys []string) (bool, error) {
	if len(rawArgs) == 0 {
		return len(requiredKeys) == 0, nil
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(rawArgs, &parsed); err != nil {
		return false, fmt.Errorf("invalid json args: %w", err)
	}
	for _, key := range requiredKeys {
		if _, exists := parsed[key]; !exists {
			return false, fmt.Errorf("missing required parameter: %s", key)
		}
	}
	return true, nil
}

// IdleLoopGate controls execution loop based on visibility or active state.
type IdleLoopGate struct {
	IsVisible bool
}

// ShouldExecute returns true only if the loop target is currently visible/active.
func (g *IdleLoopGate) ShouldExecute() bool {
	return g.IsVisible
}

// SanitizeGitHubCredentials parses stored credentials securely without false-positive triggers.
func SanitizeGitHubCredentials(token string) string {
	trimmed := strings.TrimSpace(token)
	if strings.HasPrefix(trimmed, "ghp_") || strings.HasPrefix(trimmed, "github_pat_") {
		if len(trimmed) > 8 {
			return trimmed[:4] + "..." + trimmed[len(trimmed)-4:]
		}
	}
	return "[MASKED_CREDENTIAL]"
}
