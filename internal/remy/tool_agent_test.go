package remy

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAgentToolSchemasNeverMarshalNullRequired(t *testing.T) {
	for _, definition := range (&Service{}).agentToolDefinitions() {
		function := definition["function"].(map[string]any)
		encoded, err := json.Marshal(function["parameters"])
		if err != nil {
			t.Fatalf("tool %s: %v", function["name"], err)
		}
		if strings.Contains(string(encoded), `"required":null`) {
			t.Errorf("tool %s marshals required as null; strict OpenAI-compatible endpoints reject it", function["name"])
		}
	}
}
