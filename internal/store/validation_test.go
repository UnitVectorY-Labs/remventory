package store

import (
	"encoding/json"
	"testing"
)

func TestValidateCategoryProposalAcceptsEnumeration(t *testing.T) {
	payload := CategoryProposalPayload{Operation: "create", Name: "Games", Attributes: []AttributeDraft{{
		Key: "platform", Label: "Platform", DataType: "enum", Config: json.RawMessage(`{"options":["Switch","PC"]}`),
	}}}
	if err := validateCategoryProposalPayload(&payload); err != nil {
		t.Fatal(err)
	}
}

func TestValidateCategoryProposalRejectsEnumerationWithoutOptions(t *testing.T) {
	payload := CategoryProposalPayload{Operation: "create", Name: "Games", Attributes: []AttributeDraft{{
		Key: "platform", Label: "Platform", DataType: "enum", Config: json.RawMessage(`{}`),
	}}}
	if err := validateCategoryProposalPayload(&payload); err == nil {
		t.Fatal("expected enum options validation error")
	}
}

func TestValidateItemAttributeEnumeration(t *testing.T) {
	definitions := []Attribute{{Key: "platform", DataType: "enum", Config: json.RawMessage(`{"options":["Switch","PC"]}`)}}
	if err := validateItemAttributeValues(json.RawMessage(`{"platform":"Switch"}`), definitions); err != nil {
		t.Fatal(err)
	}
	if err := validateItemAttributeValues(json.RawMessage(`{"platform":"PlayStation"}`), definitions); err == nil {
		t.Fatal("expected invalid enum value error")
	}
}

func TestRegisteredFieldsAreOptionalAndUnknownKeysNeedApproval(t *testing.T) {
	definitions := []Attribute{{Key: "title", DataType: "text"}, {Key: "year", DataType: "number", Required: true}, {Key: "links", DataType: "links"}}
	for _, raw := range []string{`{}`, `{"year":2006}`, `{"links":[{"name":"Wikipedia","url":"https://en.wikipedia.org/wiki/Wii"}]}`} {
		if err := validateItemAttributeValues(json.RawMessage(raw), definitions); err != nil {
			t.Fatal(err)
		}
	}
	for _, raw := range []string{`{"unapproved_field":"value"}`, `{"year":"not a number"}`, `{"links":"https://example.com"}`, `{"links":[{"url":"https://example.com"}]}`, `{"links":[{"name":"Unsafe","url":"javascript:alert(1)"}]}`} {
		if err := validateItemAttributeValues(json.RawMessage(raw), definitions); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}
