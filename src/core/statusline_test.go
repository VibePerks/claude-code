package core

import (
	"strings"
	"testing"
)

func statusInput(ctxPct float64) StatusInput {
	var in StatusInput
	in.ContextWindow.UsedPercentage = ctxPct
	return in
}

func TestStatusLineFull(t *testing.T) {
	got := StatusLine(statusInput(30), "ad here", "", "", false, 80)
	if got != "context 30% - \x1b[1;97mad here\x1b[0m" {
		t.Errorf("got %q", got)
	}
}

func TestStatusLineStylesDomainAsClickableLink(t *testing.T) {
	got := StatusLine(statusInput(0), "shop.com - buy things", "shop.com", "", false, 0)
	// Domain underlined-white wrapped in an OSC 8 hyperlink, then the sentence bold-white.
	want := "context 0% - " +
		"\x1b]8;;https://shop.com\x1b\\\x1b[4;97mshop.com\x1b[0m\x1b]8;;\x1b\\" +
		"\x1b[1;97m - buy things\x1b[0m"
	if got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestStatusLineLinksFullWebsiteURL(t *testing.T) {
	// The domain link's OSC 8 target is the advertiser's full destination URL; the
	// visible text stays the bare domain.
	got := StatusLine(statusInput(0), "shop.com - buy things", "shop.com",
		"https://shop.com/deals?utm_source=cli", false, 0)
	want := "context 0% - " +
		"\x1b]8;;https://shop.com/deals?utm_source=cli\x1b\\" +
		"\x1b[4;97mshop.com\x1b[0m\x1b]8;;\x1b\\" +
		"\x1b[1;97m - buy things\x1b[0m"
	if got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestStatusLineNoticeUsesNonBoldWhite(t *testing.T) {
	notice := LoginNotice("vibeperks login", "")
	got := StatusLine(statusInput(30), notice, "", "", true, 0)
	want := "context 30% - \x1b[97m" + notice + "\x1b[0m"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStatusLineRounds(t *testing.T) {
	got := StatusLine(statusInput(66.6), "", "", "", false, 80)
	if got != "context 67%" {
		t.Errorf("got %q", got)
	}
}

func TestStatusLineNoAd(t *testing.T) {
	got := StatusLine(statusInput(5), "", "", "", false, 80)
	if got != "context 5%" {
		t.Errorf("got %q", got)
	}
}

func TestStatusLineDropsContextWhenNarrow(t *testing.T) {
	ad := "widgets.io - buy widgets"
	got := StatusLine(statusInput(30), ad, "widgets.io", "", false, 20)
	if strings.Contains(got, "context") {
		t.Errorf("context should have been dropped: %q", got)
	}
	// The full ad (domain + sentence) is always kept, never truncated.
	visible := stripStyling(got)
	if visible != ad {
		t.Errorf("ad must be kept in full, got %q want %q", visible, ad)
	}
}

func TestStatusLineNeverTruncatesAd(t *testing.T) {
	ad := "example.com - this is a long sponsor sentence that must not be cut off"
	got := StatusLine(statusInput(30), ad, "example.com", "", false, 8)
	visible := stripStyling(got)
	if visible != ad {
		t.Errorf("ad must not be truncated, got %q want %q", visible, ad)
	}
}

// stripStyling removes the SGR and OSC 8 sequences the status line adds, leaving only the
// printable columns so a width assertion counts what the user actually sees.
func stripStyling(s string) string {
	for _, seq := range []string{"\x1b[1;97m", "\x1b[4;97m", "\x1b[97m", "\x1b[0m", "\x1b]8;;\x1b\\"} {
		s = strings.ReplaceAll(s, seq, "")
	}
	// Strip the OSC 8 open sequence with its URL: ESC ]8;;<url> ESC \
	for {
		i := strings.Index(s, "\x1b]8;;")
		if i < 0 {
			break
		}
		j := strings.Index(s[i:], "\x1b\\")
		if j < 0 {
			break
		}
		s = s[:i] + s[i+j+2:]
	}
	return s
}
