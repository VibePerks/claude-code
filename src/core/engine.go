package core

import (
	"context"
	"errors"
	"time"
)

// Meta is the per-session adapter metadata attached to every impression.
type Meta struct {
	CLI           string
	CLIVersion    string
	PluginVersion string
	SessionID     string
}

const defaultRotateSeconds = 20

// defaultHourlyCap is the fallback when the serve response omits hourly_cap (older
// backends). 3600 / 12 = 300s between rotations, matching the pre-cap-field default.
const defaultHourlyCap = 12

// rotationIntervalSeconds returns the paced serve interval for a publisher: one ad
// every (3600 / hourlyCap) seconds while active. Falls back to 300s (12/hour) when
// no cap is known (fresh install, old backend).
func rotationIntervalSeconds(s State) int {
	cap := defaultHourlyCap
	if s.Ad != nil && s.Ad.HourlyCap > 0 {
		cap = s.Ad.HourlyCap
	}
	return 3600 / cap
}

// houseAdCopy maps the viewer's language to the localized house ad sentence shown
// while earning-capped (mirrors _HOUSE_AD in the backend). Falls back to "en".
var houseAdCopy = map[string]string{
	"en": "Make your AI pay for itself",
	"es": "Haz que tu IA se pague sola",
}

// cappedLine builds the earning-cap status line: the localized house ad sentence
// followed by a "more ads in hh:mm" countdown computed from try_again_at (ISO-8601)
// relative to now (Unix seconds). The countdown is a best-effort client-clock
// approximation; the server is the authority on when the cap actually resets.
func cappedLine(lang, tryAgainAt string, now int64) string {
	copy := houseAdCopy[lang]
	if copy == "" {
		copy = houseAdCopy["en"]
	}
	t, err := time.Parse(time.RFC3339, tryAgainAt)
	if err != nil {
		return copy
	}
	remaining := t.Unix() - now
	if remaining < 0 {
		remaining = 0
	}
	h := remaining / 3600
	m := (remaining % 3600) / 60
	return copy + " \u2014 more ads in " + itoa(h) + "h " + pad2(m) + "m"
}

func itoa(n int64) string {
	if n < 0 {
		return "0"
	}
	// Simple integer-to-string for small positive numbers used in countdown format.
	s := ""
	if n == 0 {
		return "0"
	}
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

func pad2(n int64) string {
	if n < 10 {
		return "0" + itoa(n)
	}
	return itoa(n)
}

func rotateSeconds(ad *Ad) int {
	if ad != nil && ad.RotateSeconds > 0 {
		return ad.RotateSeconds
	}
	return defaultRotateSeconds
}

// capActive reports whether an earning cap is still in effect at time now (Unix
// seconds). A malformed or past reset time is treated as not capped.
func capActive(s State, now int64) bool {
	if s.TryAgainAt == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, s.TryAgainAt)
	if err != nil {
		return false
	}
	return now < t.Unix()
}

// recordCurrent enqueues an impression for the currently displayed ad exactly once. It
// is a no-op when there is no ad, the ad was never rendered, or it was already recorded.
func recordCurrent(dir string, s *State, meta Meta, now int64) error {
	if s.Ad == nil || s.FirstRenderAt == 0 || s.Recorded {
		return nil
	}
	// Display time spans first render to now (record time). The status line only
	// repaints on its host interval, so FirstRenderAt==LastRenderAt is common and
	// would otherwise report 0ms - failing the server's min-display credit floor.
	displayedMs := int((now - s.FirstRenderAt) * 1000)
	if displayedMs < 0 {
		displayedMs = 0
	}
	imp := Impression{
		ImpressionToken:   s.Ad.ImpressionToken,
		DisplayedMs:       displayedMs,
		SessionID:         meta.SessionID,
		SessionDurationMs: int((now - s.ServedAt) * 1000),
		PluginVersion:     meta.PluginVersion,
		CLI:               meta.CLI,
		CLIVersion:        meta.CLIVersion,
	}
	if err := Enqueue(dir, imp); err != nil {
		return err
	}
	s.Recorded = true
	return nil
}

// Refresh is the prompt / rotation worker. It serves the next billable ad only when
// there is no ad, or when at least rotationIntervalSeconds have elapsed since the last
// serve (paced to the publisher's hourly_cap), recording the current ad's impression
// first, then flushes the impression buffer. While an earning cap is active it serves
// nothing until the reset time. Opt-out clears the cached ad and does no network I/O.
func Refresh(ctx context.Context, dir string, c *Client, meta Meta, now int64) error {
	cfg, err := LoadConfig(dir)
	if err != nil {
		return err
	}
	if OptedOut(cfg) {
		return SaveState(dir, State{})
	}
	s, err := LoadState(dir)
	if err != nil {
		return err
	}
	// Earning-cap backoff: while capped, do not serve until the reset time passes.
	if capActive(s, now) {
		return Flush(ctx, dir, c)
	}
	interval := int64(rotationIntervalSeconds(s))
	due := s.Ad == nil || now-s.ServedAt >= interval
	if !due {
		return Flush(ctx, dir, c)
	}
	if err := recordCurrent(dir, &s, meta, now); err != nil {
		return err
	}
	result, err := c.Serve(ctx)
	if err != nil {
		// A rejected device token means no amount of retrying helps: clear the cached
		// ad and flag the slot (with the reason) so the surface shows a sign-in notice.
		if errors.Is(err, ErrUnauthorized) {
			_ = SaveState(dir, State{NeedsLogin: true, NeedsLoginReason: UnauthorizedReason(err)})
			_ = Flush(ctx, dir, c)
			return err
		}
		// Keep the buffered impression and the recorded flag; surface the serve error
		// (the plugin boundary swallows it so the host CLI is unaffected).
		_ = SaveState(dir, s)
		_ = Flush(ctx, dir, c)
		return err
	}
	switch {
	case result == nil || (result.Ad == nil && result.TryAgainAt == ""):
		s = State{} // empty inventory: clear the slot
	case result.TryAgainAt != "":
		// Earning-capped: store the reset time and language so Render can show the
		// house ad + countdown instead of a blank slot.
		s = State{TryAgainAt: result.TryAgainAt, Lang: result.Lang}
	default:
		s = State{Ad: result.Ad, ServedAt: now, Lang: result.Lang}
	}
	if err := SaveState(dir, s); err != nil {
		return err
	}
	return Flush(ctx, dir, c)
}

// Render marks the cached ad as displayed at time now (setting first/last render
// timestamps) and returns its one-line plain form, the ad's domain (so a surface can
// style the domain as a clickable link), and the ad's full destination URL (the click
// target the domain link points at). When the device token was rejected it returns a
// sign-in notice (built from loginCmd) and needsLogin=true instead. Returns "" when there
// is no cached ad and no pending login.
func Render(dir string, now int64, loginCmd string) (string, string, string, bool, error) {
	s, err := LoadState(dir)
	if err != nil {
		return "", "", "", false, err
	}
	if s.NeedsLogin {
		return LoginNotice(loginCmd, s.NeedsLoginReason), "", "", true, nil
	}
	if s.Ad == nil {
		// Earning cap active: show the house ad sentence + "more ads in hh:mm"
		// countdown so the publisher knows earning will resume and when.
		if capActive(s, now) {
			return cappedLine(s.Lang, s.TryAgainAt, now), "", "", false, nil
		}
		return "", "", "", false, nil
	}
	if s.FirstRenderAt == 0 {
		s.FirstRenderAt = now
	}
	s.LastRenderAt = now
	if err := SaveState(dir, s); err != nil {
		return "", "", "", false, err
	}
	return RenderLine(s.Ad, 0), s.Ad.Domain, s.Ad.WebsiteURL, false, nil
}

// EndSession is the thinking-end worker: it records the current ad's impression (if
// displayed and not yet recorded) and flushes the buffer. Opt-out is a no-op.
func EndSession(ctx context.Context, dir string, c *Client, meta Meta, now int64) error {
	cfg, err := LoadConfig(dir)
	if err != nil {
		return err
	}
	if OptedOut(cfg) {
		return nil
	}
	s, err := LoadState(dir)
	if err != nil {
		return err
	}
	if err := recordCurrent(dir, &s, meta, now); err != nil {
		return err
	}
	if err := SaveState(dir, s); err != nil {
		return err
	}
	return Flush(ctx, dir, c)
}
