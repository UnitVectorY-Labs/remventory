package remy

import (
	"github.com/UnitVectorY-Labs/remventory/internal/store"
	"testing"
)

func TestPresentationLibraryAllowsLLMToChooseDifferentViews(t *testing.T) {
	for _, view := range []string{"item_cards", "item_list", "item_detail", "comparison", "text"} {
		if !compatiblePresentation("query_result", view) {
			t.Errorf("query_result cannot use %q", view)
		}
	}
	if !compatiblePresentation("category_definition", "category_definition") {
		t.Fatal("single category evidence must support its definition view")
	}
	if !compatiblePresentation("categories", "category_list") {
		t.Fatal("paged category evidence must support a category list")
	}
	if compatiblePresentation("categories", "category_definition") {
		t.Fatal("a category page must not render as a single definition")
	}
	for _, view := range []string{"statistic", "grouped_statistic"} {
		if !compatiblePresentation("statistic", view) {
			t.Errorf("statistic evidence cannot use %q", view)
		}
	}
	for _, view := range []string{"raw_html", "script"} {
		if allowedPresentation(view) {
			t.Errorf("unsafe/unregistered presentation %q accepted", view)
		}
	}
	if compatiblePresentation("comparison", "item_list") {
		t.Fatal("comparison evidence must not render as an item-list payload")
	}
}

func TestComparisonEvidenceFromCompareToolHydratesRows(t *testing.T) {
	entries := []map[string]any{{"item": store.Item{Title: "Lamp", Quantity: 2, Attributes: []byte(`{"color":"blue"}`)}}, {"item": store.Item{Title: "Chair", Quantity: 1, Attributes: []byte(`{"material":"oak"}`)}}}
	rows := comparisonRowsFrom(entries)
	if len(rows) != 3 || rows[1][0] != "Lamp" || rows[2][0] != "Chair" {
		t.Fatalf("compare_items evidence rows = %#v", rows)
	}
}

func TestComparisonRowsUseCanonicalInventoryItems(t *testing.T) {
	items := []store.Item{{ID: "a", Title: "Lamp", Quantity: 2, Attributes: []byte(`{"color":"blue"}`)}, {ID: "b", Title: "Chair", Quantity: 1, Attributes: []byte(`{"material":"oak"}`)}}
	rows := comparisonRows(map[string]any{"matches": items})
	if len(rows) != 3 || rows[1][0] != "Lamp" || rows[2][0] != "Chair" {
		t.Fatalf("comparison rows = %#v", rows)
	}
}
