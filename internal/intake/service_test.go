package intake

import (
	"github.com/UnitVectorY-Labs/remventory/internal/store"
	"testing"
)

func TestUncertaintyOutcomes(t *testing.T) {
	for _, tt := range []struct {
		matches []store.IdentificationMatch
		want    string
	}{{nil, "no_match"}, {[]store.IdentificationMatch{{Confidence: "rejected"}}, "no_match"}, {[]store.IdentificationMatch{{Confidence: "confident"}}, "confident"}, {[]store.IdentificationMatch{{Confidence: "partial"}}, "partial"}, {[]store.IdentificationMatch{{Confidence: "confident"}, {Confidence: "ambiguous"}}, "ambiguous"}} {
		if got := classify(tt.matches); got != tt.want {
			t.Fatalf("%+v: got %s want %s", tt.matches, got, tt.want)
		}
	}
}

func TestCurationKeepsConflictingAndEditionFacts(t *testing.T) {
	for _, test := range []struct {
		own    any
		source string
		want   bool
	}{
		{"Johnny Mathis", "Johnny Mathis", true}, {"Album: Name", "album name", true}, {float64(1969), "1969", true},
		{float64(1969), "1969-01-01", false}, {"Limited edition", "Standard edition", false}, {"My copy has a handwritten note", "Some source description", false},
	} {
		if got := sameDescriptiveValue(test.own, test.source); got != test.want {
			t.Fatal(test, got)
		}
	}
}
