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

// billableIntervalSeconds paces serving to at most one new ad (one impression) every
// 5 minutes while active, so a continuously busy session earns at most 12 ads/hour -
// matching the backend's per-hour earning cap. Between serves the surface keeps the
// cached ad; an idle session (no prompts) never serves.
const billableIntervalSeconds = 300

// earnCapNotice is the subtle line shown while an earning cap is in effect: no ad is
// served until it resets, but the surface tells the publisher why.
const earnCapNotice = "VibePerks: earning limit reached - more ads soon"

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

// Refresh is the thinking-start / rotation worker. It serves the next billable ad
// only when there is no ad, or when at least billableIntervalSeconds have elapsed
// since the last serve (so serving is paced to <=12/hour), recording the current
// ad's impression first, then flushes the impression buffer. While an earning cap is
// active it serves nothing until the reset time. The `force` argument is retained for
// adapter compatibility but no longer forces a serve: pacing is purely time-based so
// a burst of prompts cannot exceed the cap. Opt-out clears the cached ad and does no
// network I/O.
func Refresh(ctx context.Context, dir string, c *Client, meta Meta, now int64, force bool) error {
	_ = force // pacing is time-based; a new prompt no longer forces an out-of-window serve
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
	due := s.Ad == nil || now-s.ServedAt >= billableIntervalSeconds
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
		s = State{TryAgainAt: result.TryAgainAt} // earning-capped: no ad, back off until reset
	default:
		s = State{Ad: result.Ad, ServedAt: now}
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
		// Earning cap active: show a subtle paused notice instead of an ad.
		if capActive(s, now) {
			return earnCapNotice, "", "", false, nil
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
