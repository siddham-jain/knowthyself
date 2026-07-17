package score

import (
	"github.com/siddham/reflect/internal/model"
	"github.com/siddham/reflect/internal/profile"
)

// minAssistantTurns is the fairness floor for the dimensions that need meaningful
// assistant activity (Tool Leverage, Context Management, Token Economy).
const minAssistantTurns = 3

// toolLeverage rewards breadth and sophistication of tool use: diversity of tools,
// volume of tool calls, and adoption of advanced capabilities (sub-agents, MCP
// servers, skills, slash commands). A session that barely uses tools scores low —
// that's a real signal, not insufficient data — as long as there was activity.
type toolLeverage struct{}

func (toolLeverage) Dimension() profile.Dimension { return profile.ToolLeverage }
func (toolLeverage) Weight() float64              { return 0.20 }

func (toolLeverage) Score(s model.Session) profile.Signal {
	v := newView(s)
	if len(v.assistant) < minAssistantTurns {
		return profile.Signal{Sufficient: false}
	}

	distinct := map[string]bool{}
	var calls, mcp, subAgent, skill int
	for _, a := range v.assistant {
		for _, tc := range a.ToolCalls {
			calls++
			distinct[tc.Name] = true
			if tc.IsMCP {
				mcp++
			}
			if tc.IsSubAgent {
				subAgent++
			}
			if tc.Name == "Skill" {
				skill++
			}
		}
	}
	slash := 0
	for _, t := range v.all {
		if t.SlashCommand != "" {
			slash++
		}
	}

	// Diversity (0..4): up to 8 distinct tools.
	diversity := clamp(float64(len(distinct))/8.0*4.0, 0, 4)
	// Volume (0..2): calls per assistant turn, saturating around 1.5/turn.
	volume := clamp(ratio(float64(calls), float64(len(v.assistant)))/1.5*2.0, 0, 2)
	// Advanced adoption (0..3): one point each for sub-agents, MCP, skills/slash.
	advanced := 0.0
	if subAgent > 0 {
		advanced++
	}
	if mcp > 0 {
		advanced++
	}
	if skill > 0 || slash > 0 {
		advanced++
	}

	score := clamp(1.0+diversity+volume+advanced, 0, 10)

	return profile.Signal{
		Score:        score,
		Sufficient:   true,
		Observations: float64(len(v.assistant)),
		Evidence: map[string]float64{
			"distinct_tools": float64(len(distinct)),
			"tool_calls":     float64(calls),
			"mcp_calls":      float64(mcp),
			"subagent_calls": float64(subAgent),
			"skill_calls":    float64(skill),
			"slash_commands": float64(slash),
		},
	}
}

func init() { Register(toolLeverage{}) }
