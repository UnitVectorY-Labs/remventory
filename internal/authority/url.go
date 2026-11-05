package authority

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// ParseURL recognizes identifiers only. It never fetches an arbitrary URL.
func ParseURL(raw string) (Ref, bool, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return Ref{}, false, fmt.Errorf("enter a complete http or https URL without credentials")
	}
	host := strings.ToLower(u.Hostname())
	path := strings.Trim(u.Path, "/")
	parts := strings.Split(path, "/")
	switch host {
	case "musicbrainz.org", "www.musicbrainz.org":
		if len(parts) == 2 && parts[0] == "release" && regexp.MustCompile(`^[a-fA-F0-9-]{36}$`).MatchString(parts[1]) {
			return Ref{Provider: "musicbrainz", Kind: "release", ID: strings.ToLower(parts[1])}, true, nil
		}
		return Ref{}, false, fmt.Errorf("use a MusicBrainz release URL for a specific edition or pressing")
	case "wikidata.org", "www.wikidata.org":
		if len(parts) == 2 && (parts[0] == "wiki" || parts[0] == "entity") && regexp.MustCompile(`^Q[1-9][0-9]*$`).MatchString(parts[1]) {
			return Ref{Provider: "wikidata", Kind: "entity", ID: parts[1]}, true, nil
		}
		return Ref{}, false, fmt.Errorf("use a Wikidata entity URL such as https://www.wikidata.org/wiki/Q8337")
	case "amiiboapi.org":
		if path != "api/amiibo" {
			return Ref{}, false, fmt.Errorf("use an AmiiboAPI figure URL with its id")
		}
		id := u.Query().Get("id")
		if id == "" {
			id = u.Query().Get("head") + u.Query().Get("tail")
		}
		if regexp.MustCompile(`^[a-fA-F0-9]{16}$`).MatchString(id) {
			return Ref{Provider: "amiibo", Kind: "figure", ID: strings.ToLower(id)}, true, nil
		}
		return Ref{}, false, fmt.Errorf("AmiiboAPI URL needs a complete 16-character figure id")
	}
	return Ref{}, false, nil
}

func ValidateURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return fmt.Errorf("enter a complete http or https URL without credentials")
	}
	return nil
}
