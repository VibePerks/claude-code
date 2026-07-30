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

// RenderLine formats an ad as a single plain-text line. The advertiser domain leads the
// line, followed by the sentence ("<domain> - <sentence>"); when the sentence already
// contains the domain it is rendered as-is. cols <= 0 means no truncation.
func RenderLine(ad *Ad, cols int) string {
	if ad == nil {
		return ""
	}
	line := SanitizeAd(ad.Sentence)
	domain := SanitizeAd(ad.Domain)
	if domain != "" && !strings.Contains(line, domain) {
		line = strings.TrimSpace(domain + " - " + line)
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

// reset clears all SGR styling.
const reset = "\x1b[0m"

// osc8 wraps visible text in an OSC 8 terminal hyperlink so terminals that support it
// render a clickable link; terminals that don't simply show text unchanged.
func osc8(url, text string) string {
	return "\x1b]8;;" + url + "\x1b\\" + text + "\x1b]8;;\x1b\\"
}

// schemeRe matches a leading URL scheme (e.g. "http:", "javascript:") so a scheme
// other than http(s) can be rejected before it is ever turned into a clickable link.
var schemeRe = regexp.MustCompile(`^([a-z][a-z0-9+.-]*):`)

// adURL builds a safe http(s) link target from an ad domain. A bare domain gets an
// https scheme; a value carrying any non-http(s) scheme (or whitespace) is rejected
// (returns "") so a hostile creative can never turn the clickable link into a
// file:/javascript: URL.
func adURL(domain string) string {
	d := SanitizeAd(domain)
	if d == "" || strings.ContainsAny(d, " \t") {
		return ""
	}
	if m := schemeRe.FindStringSubmatch(strings.ToLower(d)); m != nil {
		if m[1] != "http" && m[1] != "https" {
			return ""
		}
		return d
	}
	return "https://" + d
}

// clickURL resolves the click target for an ad. It prefers the advertiser's full
// destination URL (website, incl. path + query such as UTM tags) when that is a safe
// http(s) link, and falls back to the bare display domain promoted to https. Returns
// "" when neither is a safe http(s) target (so no link is emitted).
func clickURL(websiteURL, domain string) string {
	if u := adURL(websiteURL); u != "" {
		return u
	}
	return adURL(domain)
}

// styleAd renders a plain ad line with the advertiser domain underlined (underlineSeq)
// and turned into an OSC 8 hyperlink (clickable where the terminal supports it) and the
// surrounding sentence text wrapped in boldSeq. The visible link text is always the bare
// domain; linkURL is the (already validated) target it points at - the advertiser's full
// destination URL, so a click opens the exact deep link while the surface only shows the
// host. The domain may lead the line ("<domain> - <sentence>") or, for legacy copy,
// appear anywhere within it; text on either side of the domain is bolded. When the domain
// can't be located (e.g. it was truncated away) the whole line is simply bolded.
func styleAd(line, domain, linkURL, boldSeq, underlineSeq string) string {
	d := SanitizeAd(domain)
	idx := strings.Index(line, d)
	if d == "" || idx < 0 {
		return boldSeq + line + reset
	}
	out := ""
	if before := line[:idx]; before != "" {
		out += boldSeq + before + reset
	}
	linked := underlineSeq + d + reset
	if linkURL != "" {
		linked = osc8(linkURL, linked)
	}
	out += linked
	if after := line[idx+len(d):]; after != "" {
		out += boldSeq + after + reset
	}
	return out
}

// StyleAdLine styles an already-composed plain ad line for a terminal prompt surface:
// bold sentence and an underlined, clickable domain link. The visible text is the bare
// domain; the link opens the advertiser's full destination URL (websiteURL), falling
// back to the domain when websiteURL is absent or unsafe.
func StyleAdLine(line, domain, websiteURL string) string {
	return styleAd(line, domain, clickURL(websiteURL, domain), "\x1b[1m", "\x1b[4m")
}

// StyleAdStatus styles an ad line for the status-line surface: bold high-contrast white
// sentence and an underlined white clickable domain link, so it stands out from the
// dimmed host status fields. The visible text is the bare domain; the link opens the
// advertiser's full destination URL (websiteURL).
func StyleAdStatus(line, domain, websiteURL string) string {
	return styleAd(line, domain, clickURL(websiteURL, domain), "\x1b[1;97m", "\x1b[4;97m")
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
