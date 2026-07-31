// Package remedy provides parameter validation and schema probe guards.
package remedy

import (
	"encoding/json"
	"fmt"
)

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
