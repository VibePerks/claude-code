package core

import (
	"strings"
	"testing"
)

func TestSanitizeAd(t *testing.T) {
	in := "  hello\x1b[31m\tworld\n\x00 try foo.com  "
	got := SanitizeAd(in)
	want := "hello[31mworld try foo.com"
	if got != want {
		t.Errorf("SanitizeAd = %q, want %q", got, want)
	}
}

func TestRenderLineNil(t *testing.T) {
	if RenderLine(nil, 80) != "" {
		t.Error("nil ad should render empty")
	}
}

func TestRenderLineAppendsDomainWhenAbsent(t *testing.T) {
	ad := &Ad{Sentence: "Get paid while vibe coding", Domain: "foo.com"}
	got := RenderLine(ad, 0)
	if got != "foo.com - Get paid while vibe coding" {
		t.Errorf("got %q", got)
	}
}

func TestRenderLineKeepsDomainWhenPresent(t *testing.T) {
	ad := &Ad{Sentence: "Try the fastest CDN at fast.com", Domain: "fast.com"}
	got := RenderLine(ad, 0)
	if got != "Try the fastest CDN at fast.com" {
		t.Errorf("got %q", got)
	}
}

func TestRenderLineTruncates(t *testing.T) {
	ad := &Ad{Sentence: "abcdefghij", Domain: ""}
	got := RenderLine(ad, 5)
	if got != "ab..." {
		t.Errorf("got %q, want ab...", got)
	}
	if len([]rune(got)) != 5 {
		t.Errorf("width = %d, want 5", len([]rune(got)))
	}
}

func TestRenderLineNoTruncateWhenColsZero(t *testing.T) {
	ad := &Ad{Sentence: strings.Repeat("x", 200)}
	if got := RenderLine(ad, 0); len(got) != 200 {
		t.Errorf("expected no truncation, got len %d", len(got))
	}
}

func TestRenderLineColsOne(t *testing.T) {
	ad := &Ad{Sentence: "abcdef"}
	if got := RenderLine(ad, 1); got != "a" {
		t.Errorf("got %q, want a", got)
	}
}

func TestLoginNoticeIncludesCommand(t *testing.T) {
	got := LoginNotice("vibeperks login", "")
	if !strings.Contains(got, "VibePerks") || !strings.Contains(got, "vibeperks login") {
		t.Errorf("notice = %q", got)
	}
}

func TestLoginNoticeIncludesReason(t *testing.T) {
	got := LoginNotice("vibeperks login", "account suspended")
	if !strings.Contains(got, "account suspended") || !strings.Contains(got, "vibeperks login") {
		t.Errorf("notice = %q", got)
	}
}

func TestLoginNoticeOmitsEmptyCommand(t *testing.T) {
	got := LoginNotice("", "device token invalid or revoked")
	if strings.Contains(got, "run:") {
		t.Errorf("empty command should omit run hint, got %q", got)
	}
	if !strings.Contains(got, "device token invalid or revoked") {
		t.Errorf("notice should keep the reason, got %q", got)
	}
}

func TestStyleAdLineBoldsSentenceAndLinksDomain(t *testing.T) {
	got := StyleAdLine("shop.com - buy things", "shop.com", "")
	want := "\x1b]8;;https://shop.com\x1b\\\x1b[4mshop.com\x1b[0m\x1b]8;;\x1b\\" +
		"\x1b[1m - buy things\x1b[0m"
	if got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestStyleAdLineLinksFullWebsiteURLButShowsDomain(t *testing.T) {
	// The visible link text is the bare domain; the OSC 8 hyperlink target is the
	// advertiser's full destination URL (path + query preserved), so a click opens
	// the exact deep link while the surface only shows the host.
	got := StyleAdLine("shop.com - buy things", "shop.com",
		"https://shop.com/deals?utm_source=cli")
	want := "\x1b]8;;https://shop.com/deals?utm_source=cli\x1b\\" +
		"\x1b[4mshop.com\x1b[0m\x1b]8;;\x1b\\" +
		"\x1b[1m - buy things\x1b[0m"
	if got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestStyleAdLineFallsBackToDomainWhenWebsiteURLUnsafe(t *testing.T) {
	// A hostile/unsafe website URL is rejected and the link falls back to the bare
	// domain promoted to https - never a javascript:/file: target.
	got := StyleAdLine("shop.com - buy things", "shop.com", "javascript:alert(1)")
	if !strings.Contains(got, "\x1b]8;;https://shop.com\x1b\\") {
		t.Errorf("unsafe website url should fall back to https://<domain>, got %q", got)
	}
}

func TestStyleAdLineBoldsWholeLineWhenDomainAbsent(t *testing.T) {
	got := StyleAdLine("a plain line with no domain", "", "")
	if got != "\x1b[1ma plain line with no domain\x1b[0m" {
		t.Errorf("got %q", got)
	}
}

func TestStyleAdLineKeepsExistingHttpScheme(t *testing.T) {
	got := StyleAdLine("see https://shop.com/deals", "https://shop.com/deals", "")
	if !strings.Contains(got, "\x1b]8;;https://shop.com/deals\x1b\\") {
		t.Errorf("http(s) domain should be linked verbatim, got %q", got)
	}
}

func TestAdURLRejectsUnsafeScheme(t *testing.T) {
	if adURL("javascript:alert(1)") != "" {
		t.Error("javascript: scheme must be rejected")
	}
	if adURL("file:///etc/passwd") != "" {
		t.Error("file: scheme must be rejected")
	}
	if adURL("shop.com") != "https://shop.com" {
		t.Errorf("bare domain should get https, got %q", adURL("shop.com"))
	}
}
