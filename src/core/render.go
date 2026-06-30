package core

import (
	"regexp"
	"strings"
)

// ctrlRe matches every C0 control byte (incl. ESC, tab, newline) and DEL. Server ad copy
// is single-line plain text, so stripping these neutralizes terminal escape injection on
// render and keeps tabs/newlines out of any logs.
var ctrlRe = regexp.MustCompile(`[\x00-\x1f\x7f]`)

// SanitizeAd strips control bytes and trims whitespace from untrusted, server-supplied
// ad copy before it is ever cached or rendered.
func SanitizeAd(s string) string {
	return strings.TrimSpace(ctrlRe.ReplaceAllString(s, ""))
}

// RenderLine formats an ad as a single plain-text line. The sentence already ends with
// the domain per the product spec; if it does not, the domain is appended defensively.
// cols <= 0 means no truncation.
func RenderLine(ad *Ad, cols int) string {
	if ad == nil {
		return ""
	}
	line := SanitizeAd(ad.Sentence)
	domain := SanitizeAd(ad.Domain)
	if domain != "" && !strings.Contains(line, domain) {
		line = strings.TrimSpace(line + " - " + domain)
	}
	return truncate(line, cols)
}

// LoginNotice is the sign-in line shown when the device token was rejected, in place of
// an ad. It tells the user that authentication failed (with the reason, when known) and
// how to fix it. loginCmd is the adapter's login command (e.g. "vibeperks login"). The
// text is plain ASCII; the calling surface applies any styling (non-bold white).
func LoginNotice(loginCmd, reason string) string {
	loginCmd = SanitizeAd(loginCmd)
	reason = SanitizeAd(reason)
	notice := "VibePerks: sign-in required"
	if reason != "" {
		notice = "VibePerks: " + reason
	}
	if loginCmd != "" {
		notice += " - run: " + loginCmd
	}
	return notice
}

// ellipsis is the ASCII truncation marker appended when a line is clipped to width.
const ellipsis = "..."

// truncate clips s to cols runes, replacing the tail with an ASCII ellipsis. cols <= 0
// returns s unchanged. When cols is too narrow to fit the marker, s is hard-clipped.
func truncate(s string, cols int) string {
	if cols <= 0 {
		return s
	}
	r := []rune(s)
	if len(r) <= cols {
		return s
	}
	if cols <= len(ellipsis) {
		return string(r[:cols])
	}
	return string(r[:cols-len(ellipsis)]) + ellipsis
}
