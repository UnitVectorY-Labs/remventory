package intake

import (
	"regexp"
	"sort"
	"strings"

	"github.com/UnitVectorY-Labs/remventory/internal/authority"
	"github.com/UnitVectorY-Labs/remventory/internal/store"
)

// ConsoleHint is draft review data, never an owned-item field or confirmation.
type ConsoleHint struct {
	Value     string   `json:"value"`
	Reason    string   `json:"reason"`
	Platforms []string `json:"platforms"`
	Warning   string   `json:"warning,omitempty"`
}

type consoleName struct {
	Name    string
	Aliases []string
}

var consoleNames = []consoleName{
	{"Nintendo Switch 2", []string{"Nintendo Switch 2", "Switch 2"}},
	{"Nintendo Switch", []string{"Nintendo Switch", "Switch"}},
	{"PS5", []string{"PlayStation 5", "PS5"}},
	{"PS4", []string{"PlayStation 4", "PS4"}},
	{"PS3", []string{"PlayStation 3", "PS3"}},
	{"PS2", []string{"PlayStation 2", "PS2"}},
	{"PS1", []string{"PlayStation 1", "PS1"}},
	{"Xbox Series X/S", []string{"Xbox Series X and Series S", "Xbox Series X/S", "Xbox Series X", "Xbox Series S"}},
	{"XboxOne", []string{"Xbox One", "XboxOne"}},
	{"Xbox360", []string{"Xbox 360", "Xbox360"}},
	{"Xbox", []string{"Xbox"}},
	{"Wii U", []string{"Wii U"}},
	{"Wii", []string{"Wii"}},
	{"SNES", []string{"Super Nintendo Entertainment System", "Super Nintendo", "SNES"}},
	{"NES", []string{"Nintendo Entertainment System", "NES"}},
	{"N64", []string{"Nintendo 64", "N64"}},
	{"GBA", []string{"Game Boy Advance", "GBA"}},
	{"GBC", []string{"Game Boy Color", "GBC"}},
	{"GB", []string{"Game Boy", "GB"}},
	{"Genesis", []string{"Sega Genesis", "Mega Drive", "Genesis"}},
}

var consoleUncertainty = regexp.MustCompile(`(?i)\b(unknown|uncertain|unclear|possibly|maybe|either|no console|not (for )?(nintendo|switch|playstation|ps[1-5]|xbox|wii)|(console|platform|branding|logo)[^.;]{0,20}(not visible|cannot|can't))\b`)

var consoleSeparators = regexp.MustCompile(`[^\p{L}\p{N}]+`)

func consoleWords(value string) string {
	return strings.Join(consoleSeparators.Split(strings.ToLower(value), -1), " ")
}

// Match longer branding first so Switch 2 never also becomes Switch, and Wii U
// never also becomes Wii. Multiple distinct names remain ambiguous.
func consoleMentions(text string, options []string) []string {
	text = " " + strings.TrimSpace(consoleWords(text)) + " "
	found := map[string]bool{}
	names := append([]consoleName{}, consoleNames...)
	for _, option := range options {
		names = append(names, consoleName{option, []string{option}})
	}
	for _, name := range names {
		for _, alias := range name.Aliases {
			needle := " " + strings.TrimSpace(consoleWords(alias)) + " "
			if strings.Contains(text, needle) {
				canonical := approvedConsoleName(name, options)
				found[canonical] = true
				text = strings.ReplaceAll(text, needle, " ")
			}
		}
	}
	result := []string{}
	for name := range found {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

func SuggestConsole(result store.IdentificationResult, graph authority.Graph, options []string) ConsoleHint {
	hint := ConsoleHint{Platforms: []string{}}
	seen := map[string]bool{}
	for _, edge := range graph.Edges {
		if edge.From != graph.Root || edge.Relation != "platform" {
			continue
		}
		for _, entity := range graph.Entities {
			if entity.Ref != edge.To || strings.TrimSpace(entity.Label) == "" {
				continue
			}
			names := []string{canonicalPlatform(entity.Label, options)}
			for _, name := range names {
				if !seen[name] {
					hint.Platforms = append(hint.Platforms, name)
					seen[name] = true
				}
			}
		}
	}
	sort.Strings(hint.Platforms)
	photo, _ := result.Inferred["console"].(string)
	reason := "Suggested from the console branding identified in your photo."
	if strings.TrimSpace(photo) == "" {
		photo, _ = result.Inferred["format"].(string)
		reason = "Suggested from the packaging described in your saved photo analysis."
	}
	mentions := consoleMentions(photo, options)
	// Uncertainty is not a positive observation of a console banner.
	uncertain := consoleUncertainty.MatchString(photo)
	if len(mentions) == 1 && !uncertain {
		hint.Value, hint.Reason = mentions[0], reason
		if len(hint.Platforms) > 0 && !seen[hint.Value] {
			hint.Warning = "The catalogue does not list this console. Check the photo; catalogue coverage may be incomplete."
		}
	} else if len(mentions) > 0 {
		hint.Reason = "The photo analysis is uncertain about the console. Choose the console shown on your copy."
	} else if len(hint.Platforms) == 1 {
		hint.Value, hint.Reason = hint.Platforms[0], "Suggested from the game's only listed catalogue platform. Check it against your copy."
	} else {
		hint.Reason = "Choose the console shown on your case or cartridge."
	}
	return hint
}

func approvedConsoleName(name consoleName, options []string) string {
	for _, option := range options {
		for _, alias := range append([]string{name.Name}, name.Aliases...) {
			if consoleWords(option) == consoleWords(alias) {
				return option
			}
		}
	}
	return name.Name
}

func canonicalPlatform(label string, options []string) string {
	for _, name := range consoleNames {
		for _, alias := range append([]string{name.Name}, name.Aliases...) {
			if consoleWords(label) == consoleWords(alias) {
				return approvedConsoleName(name, options)
			}
		}
	}
	for _, option := range options {
		if consoleWords(option) == consoleWords(label) {
			return option
		}
	}
	return label
}
