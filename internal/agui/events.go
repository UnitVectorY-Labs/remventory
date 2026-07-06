package agui

type Event struct {
	Type    string `json:"type"`
	Payload any    `json:"payload,omitempty"`
}

type Component struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

func EventsFor(state, summary string, components []Component) []Event {
	events := []Event{{
		Type: "remy.activity",
		Payload: map[string]any{
			"state": state,
		},
	}}
	if summary != "" {
		events = append(events, Event{
			Type: "remy.summary",
			Payload: map[string]any{
				"text": summary,
			},
		})
	}
	for _, component := range components {
		events = append(events, Event{
			Type: "remy.component",
			Payload: map[string]any{
				"component_type": component.Type,
				"data":           component.Data,
			},
		})
	}
	return events
}
