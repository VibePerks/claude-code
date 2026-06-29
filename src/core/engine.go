package core

import "context"

// Meta is the per-session adapter metadata attached to every impression.
type Meta struct {
	CLI           string
	CLIVersion    string
	PluginVersion string
	SessionID     string
}

const defaultRotateSeconds = 20

// maxAutoRotations caps timer-driven ad rotations within one prompt cycle so an idle
// session left open doesn't farm impressions indefinitely. A new prompt resets it.
const maxAutoRotations = 3

func rotateSeconds(ad *Ad) int {
	if ad != nil && ad.RotateSeconds > 0 {
		return ad.RotateSeconds
	}
	return defaultRotateSeconds
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

// Refresh is the thinking-start / rotation worker. It records the current ad's
// impression and serves the next ad when forced (a new thinking session) or when
// rotate_seconds has elapsed on a displayed ad, then flushes the impression buffer.
// Timer rotations are capped at maxAutoRotations so an idle session doesn't farm
// impressions; a new prompt (force) resets the budget. Opt-out clears the cached
// ad and does no network I/O.
func Refresh(ctx context.Context, dir string, c *Client, meta Meta, now int64, force bool) error {
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
	rotateDue := s.Ad != nil && s.FirstRenderAt > 0 && now-s.ServedAt >= int64(rotateSeconds(s.Ad)) &&
		s.RotateCount < maxAutoRotations
	due := force || rotateDue
	if !due {
		return Flush(ctx, dir, c)
	}
	if err := recordCurrent(dir, &s, meta, now); err != nil {
		return err
	}
	ad, err := c.Serve(ctx)
	if err != nil {
		// Keep the buffered impression and the recorded flag; surface the serve error
		// (the plugin boundary swallows it so the host CLI is unaffected).
		_ = SaveState(dir, s)
		_ = Flush(ctx, dir, c)
		return err
	}
	if ad == nil {
		s = State{} // empty inventory: clear the slot
	} else if force {
		// A new prompt restarts the rotation budget.
		s = State{Ad: ad, ServedAt: now}
	} else {
		// Timer rotation: spend one of the bounded auto-rotations.
		s = State{Ad: ad, ServedAt: now, RotateCount: s.RotateCount + 1}
	}
	if err := SaveState(dir, s); err != nil {
		return err
	}
	return Flush(ctx, dir, c)
}

// Render marks the cached ad as displayed at time now (setting first/last render
// timestamps) and returns its one-line form. Returns "" when there is no cached ad.
func Render(dir string, now int64) (string, error) {
	s, err := LoadState(dir)
	if err != nil {
		return "", err
	}
	if s.Ad == nil {
		return "", nil
	}
	if s.FirstRenderAt == 0 {
		s.FirstRenderAt = now
	}
	s.LastRenderAt = now
	if err := SaveState(dir, s); err != nil {
		return "", err
	}
	return RenderLine(s.Ad, 0), nil
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
