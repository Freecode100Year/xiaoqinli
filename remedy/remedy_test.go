package remedy

import (
	"testing"
)

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
