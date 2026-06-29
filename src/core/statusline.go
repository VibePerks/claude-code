package core

import (
	"fmt"
	"strings"
)

// StatusInput is the subset of Claude Code's status-line JSON the plugin renders.
type StatusInput struct {
	SessionID string `json:"session_id"`
	Model     struct {
		DisplayName string `json:"display_name"`
	} `json:"model"`
	ContextWindow struct {
		UsedPercentage float64 `json:"used_percentage"`
	} `json:"context_window"`
	RateLimits struct {
		SevenDay struct {
			UsedPercentage float64 `json:"used_percentage"`
		} `json:"seven_day"`
	} `json:"rate_limits"`
}

// StatusLine composes the host status fields (model, context %, weekly limit %) with the
// ad line into a single line that fits cols. When too narrow it sheds host fields from
// the front (model first, then context), always keeping the ad; if only the ad fits, it
// is truncated to width. cols <= 0 disables width handling.
func StatusLine(in StatusInput, adLine string, cols int) string {
	model := in.Model.DisplayName
	if i := strings.Index(model, " ("); i >= 0 {
		model = model[:i]
	}
	fields := []string{}
	if model != "" {
		fields = append(fields, model)
	}
	fields = append(fields, fmt.Sprintf("context %d%%", round(in.ContextWindow.UsedPercentage)))
	fields = append(fields, fmt.Sprintf("limit %d%%", round(in.RateLimits.SevenDay.UsedPercentage)))

	if adLine == "" {
		return truncate(strings.Join(fields, " - "), cols)
	}
	for len(fields) > 0 {
		combined := strings.Join(fields, " - ") + " - " + boldWhite(adLine)
		// Width budget counts only the host fields plus the visible ad; ANSI styling
		// adds no printable columns.
		if cols <= 0 || width(strings.Join(fields, " - ")+" - "+adLine) <= cols {
			return combined
		}
		fields = fields[1:]
	}
	return boldWhite(truncate(adLine, cols))
}

// boldWhite wraps the ad in bold, high-contrast white so it stands out from the dimmed
// host status fields. Terminals that don't support color render the text unchanged.
func boldWhite(s string) string { return "\x1b[1;97m" + s + "\x1b[0m" }

func width(s string) int { return len([]rune(s)) }

func round(f float64) int { return int(f + 0.5) }
