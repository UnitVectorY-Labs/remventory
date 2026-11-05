package authority

import "testing"

func TestCatalogueURLs(t *testing.T) {
	for _, c := range []struct{ url, provider, id string }{
		{"https://musicbrainz.org/release/e0a99890-d3d5-4d88-9e01-7b0b135df0a7", "musicbrainz", "e0a99890-d3d5-4d88-9e01-7b0b135df0a7"},
		{"https://www.wikidata.org/wiki/Q8337", "wikidata", "Q8337"},
		{"https://amiiboapi.org/api/amiibo/?head=00000000&tail=00000002", "amiibo", "0000000000000002"},
	} {
		ref, known, err := ParseURL(c.url)
		if err != nil || !known || ref.Provider != c.provider || ref.ID != c.id {
			t.Fatal(c.url, ref, known, err)
		}
	}
	for _, raw := range []string{"javascript:alert(1)", "https://user:secret@wikidata.org/wiki/Q8337", "https://www.wikidata.org/wiki/not-a-QID", "https://amiiboapi.org/api/amiibo/?id=short"} {
		if _, _, err := ParseURL(raw); err == nil {
			t.Fatal("invalid source URL accepted", raw)
		}
	}
	for _, raw := range []string{"https://example.com/product/42", "http://localhost:1234/reference", "https://musicbrainz.org/release-group/anything"} {
		if err := ValidateURL(raw); err != nil {
			t.Fatal("ordinary reference URL rejected", raw)
		}
	}
	if _, known, err := ParseURL("https://wikidata.org.attacker.example/wiki/Q8337"); err != nil || known {
		t.Fatal("host suffix was trusted")
	}
}
