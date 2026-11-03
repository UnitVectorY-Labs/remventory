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
