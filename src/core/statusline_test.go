package core

import (
	"strings"
	"testing"
)

func statusInput(model string, ctxPct, limitPct float64) StatusInput {
	var in StatusInput
	in.Model.DisplayName = model
	in.ContextWindow.UsedPercentage = ctxPct
	in.RateLimits.SevenDay.UsedPercentage = limitPct
	return in
}

func TestStatusLineFull(t *testing.T) {
	got := StatusLine(statusInput("Sonnet", 30, 10), "ad here", false, 80)
	if got != "Sonnet - context 30% - limit 10% - \x1b[1;97mad here\x1b[0m" {
		t.Errorf("got %q", got)
	}
}

func TestStatusLineNoticeUsesNonBoldWhite(t *testing.T) {
	notice := LoginNotice("vibeperks login", "")
	got := StatusLine(statusInput("Sonnet", 30, 10), notice, true, 0)
	want := "Sonnet - context 30% - limit 10% - \x1b[97m" + notice + "\x1b[0m"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStatusLineTrimsModelParen(t *testing.T) {
	got := StatusLine(statusInput("Sonnet (claude-sonnet-4)", 0, 0), "", false, 80)
	if !strings.HasPrefix(got, "Sonnet - ") {
		t.Errorf("model not trimmed: %q", got)
	}
}

func TestStatusLineRounds(t *testing.T) {
	got := StatusLine(statusInput("M", 66.6, 12.4), "", false, 80)
	if got != "M - context 67% - limit 12%" {
		t.Errorf("got %q", got)
	}
}

func TestStatusLineNoAd(t *testing.T) {
	got := StatusLine(statusInput("M", 5, 5), "", false, 80)
	if got != "M - context 5% - limit 5%" {
		t.Errorf("got %q", got)
	}
}

func TestStatusLineShedsModelWhenNarrow(t *testing.T) {
	ad := "buy widgets at widgets.io"
	got := StatusLine(statusInput("ClaudeModelLongName", 30, 10), ad, false, 45)
	if strings.Contains(got, "ClaudeModelLongName") {
		t.Errorf("model should have been shed: %q", got)
	}
	if !strings.Contains(got, ad) {
		t.Errorf("ad must be kept: %q", got)
	}
	visible := strings.ReplaceAll(strings.ReplaceAll(got, "\x1b[1;97m", ""), "\x1b[0m", "")
	if len([]rune(visible)) > 45 {
		t.Errorf("width %d exceeds 45: %q", len([]rune(visible)), got)
	}
}

func TestStatusLineAdOnlyWhenTooNarrow(t *testing.T) {
	got := StatusLine(statusInput("M", 30, 10), "abcdefghijklmnop", false, 8)
	if got != "\x1b[1;97mabcde...\x1b[0m" {
		t.Errorf("got %q, want truncated ad only", got)
	}
}
