package core

import (
	"fmt"
)

// StatusInput is the subset of Claude Code's status-line JSON the plugin renders.
type StatusInput struct {
	SessionID     string `json:"session_id"`
	ContextWindow struct {
		UsedPercentage float64 `json:"used_percentage"`
	} `json:"context_window"`
}

// StatusLine composes the host context field (context %) with the ad line into a single
// line: "context <n>% - <ad>". The ad (its domain link and sentence) is never truncated,
// so the sponsor line is always shown in full; when the composed line would exceed cols
// the context field is dropped entirely rather than clipping the ad. cols <= 0 disables
// width handling. When notice is true the line is a sign-in notice (styled non-bold
// white); otherwise it is a paid ad, styled with a bold sentence and an underlined,
// clickable domain link (domain) whose target is the advertiser's full destination URL
// (websiteURL).
func StatusLine(in StatusInput, adLine, domain, websiteURL string, notice bool, cols int) string {
	styled := func(a string) string {
		if notice {
			return White(a)
		}
		return StyleAdStatus(a, domain, websiteURL)
	}
	context := fmt.Sprintf("context %d%%", round(in.ContextWindow.UsedPercentage))

	if adLine == "" {
		return truncate(context, cols)
	}
	// Width budget counts only the visible printable columns; ANSI styling and OSC 8
	// hyperlinks add none. The ad is kept intact; only the context field is shed when
	// it doesn't fit.
	if cols <= 0 || width(context+" - "+adLine) <= cols {
		return context + " - " + styled(adLine)
	}
	return styled(adLine)
}

// White wraps a sign-in notice in non-bold white: visible but visually distinct from a
// paid ad's bold styling. Surfaces that print to a normal prompt line use it directly.
func White(s string) string { return "\x1b[97m" + s + "\x1b[0m" }

func width(s string) int { return len([]rune(s)) }

func round(f float64) int { return int(f + 0.5) }
