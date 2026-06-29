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
	ad := &Ad{Sentence: "Fast APIs for builders", Domain: "foo.com"}
	got := RenderLine(ad, 0)
	if got != "Fast APIs for builders - foo.com" {
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
