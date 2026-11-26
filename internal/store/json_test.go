package store

import (
	"encoding/json/v2"
	"strings"
	"testing"
)

func TestItemProposalJSONDefaults(t *testing.T) {
	b, err := json.Marshal(ItemProposalPayload{})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if _, ok := got["quantity"]; ok {
		t.Fatalf("zero quantity should be omitted: %s", b)
	}
	if _, ok := got["quantity_delta"]; ok {
		t.Fatalf("zero quantity delta should be omitted: %s", b)
	}
	if !strings.Contains(string(b), `"tags":[]`) {
		t.Fatalf("nil tags should encode as an empty array: %s", b)
	}
}
