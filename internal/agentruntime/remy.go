package agentruntime

import (
	"iter"

	"github.com/UnitVectorY-Labs/remventory/internal/config"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/session"
)

const RemyInstruction = `You are Remy, an inventory agent for a self-hosted inventory application.
Use category definitions as context, keep item attributes structured, and never commit data-changing actions without a user-confirmed proposal.`

func NewRemyAgent(cfg config.Config) (agent.Agent, error) {
	description := "Interprets inventory requests and produces structured proposal, query, and list results."
	if cfg.MainModel != "" {
		description += " Configured main model: " + cfg.MainModel + "."
	}
	return agent.New(agent.Config{
		Name:        "remy",
		Description: description,
		Run: func(agent.InvocationContext) iter.Seq2[*session.Event, error] {
			return func(yield func(*session.Event, error) bool) {}
		},
	})
}
