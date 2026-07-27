package remedy

import (
	"context"
	"testing"
	"time"
)

func TestStripURLUserinfo(t *testing.T) {
	raw := "http://admin:secret123@example.com/api/v1"
	got := StripURLUserinfo(raw)
	if got == raw || !contains(got, "REDACTED") {
		t.Errorf("expected userinfo to be redacted, got: %s", got)
	}
}

func TestPreserveRecentlyActiveSessions(t *testing.T) {
	now := time.Now()
	sessions := []Session{
		{ID: "s1", LastActive: now.Add(-5 * time.Minute)},
		{ID: "s2", LastActive: now.Add(-30 * time.Minute)},
	}
	kept := PreserveRecentlyActiveSessions(sessions, 10*time.Minute, now)
	if len(kept) != 1 || kept[0].ID != "s1" {
		t.Errorf("expected s1 preserved, got: %v", kept)
	}
}

func TestUpdateSkillWithLockedRegistry(t *testing.T) {
	orig := SkillMetadata{Name: "git", SourceRegistry: "official", Content: "v1"}
	update := SkillMetadata{Name: "git", SourceRegistry: "malicious_override", Content: "v2"}

	res := UpdateSkillWithLockedRegistry(orig, update)
	if res.SourceRegistry != "official" {
		t.Errorf("expected SourceRegistry to stay 'official', got: %s", res.SourceRegistry)
	}
	if res.Content != "v2" {
		t.Errorf("expected updated content 'v2', got: %s", res.Content)
	}
}

func TestSanitizeSessionEnv(t *testing.T) {
	env := map[string]string{
		"GOOD": "value",
		"BAD":  "multiline\nvalue",
	}
	clean := SanitizeSessionEnv(env)
	if _, ok := clean["BAD"]; ok {
		t.Errorf("multiline env variable should be excluded")
	}
	if clean["GOOD"] != "value" {
		t.Errorf("valid env variable should be preserved")
	}
}

func TestBoundedStartupRestoreGate(t *testing.T) {
	ctx := context.Background()
	err := BoundedStartupRestoreGate(ctx, 100*time.Millisecond, func() error {
		time.Sleep(10 * time.Millisecond)
		return nil
	})
	if err != nil {
		t.Errorf("expected clean restore, got: %v", err)
	}

	errTimeout := BoundedStartupRestoreGate(ctx, 10*time.Millisecond, func() error {
		time.Sleep(50 * time.Millisecond)
		return nil
	})
	if errTimeout == nil {
		t.Errorf("expected timeout error")
	}
}

func TestProbeValidateDeferredSchema(t *testing.T) {
	ok, err := ProbeValidateDeferredSchema([]byte(`{"target":"go","source":"{}"}`), []string{"source"})
	if !ok || err != nil {
		t.Errorf("expected valid probe validation, got: %v, %v", ok, err)
	}

	_, errMissing := ProbeValidateDeferredSchema([]byte(`{"target":"go"}`), []string{"source"})
	if errMissing == nil {
		t.Errorf("expected error for missing required parameter")
	}
}

func TestIdleLoopGate(t *testing.T) {
	gate := &IdleLoopGate{IsVisible: false}
	if gate.ShouldExecute() {
		t.Errorf("idle gate should return false when not visible")
	}
	gate.IsVisible = true
	if !gate.ShouldExecute() {
		t.Errorf("idle gate should return true when visible")
	}
}

func TestSanitizeGitHubCredentials(t *testing.T) {
	token := "ghp_1234567890abcdefghijklmnopqrstuvwxyz"
	masked := SanitizeGitHubCredentials(token)
	if masked == token || !contains(masked, "...") {
		t.Errorf("expected token to be masked, got: %s", masked)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || searchSubstr(s, substr))
}

func searchSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
