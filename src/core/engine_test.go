package core

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// engineMock serves a configurable ad and accepts impressions, counting both.
type engineMock struct {
	srv        *httptest.Server
	serveCalls atomic.Int64
	impCalls   atomic.Int64
	serveAd    *Ad // nil => 204
}

func newEngineMock(ad *Ad) *engineMock {
	m := &engineMock{serveAd: ad}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/ads/serve", func(w http.ResponseWriter, r *http.Request) {
		m.serveCalls.Add(1)
		if m.serveAd == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeJSON(w, http.StatusOK, m.serveAd)
	})
	mux.HandleFunc("/v1/impressions", func(w http.ResponseWriter, r *http.Request) {
		m.impCalls.Add(1)
		w.WriteHeader(http.StatusCreated)
	})
	m.srv = httptest.NewServer(mux)
	return m
}

func (m *engineMock) client() *Client { return clientFor(m.srv.URL) }
func (m *engineMock) close()          { m.srv.Close() }

func newMeta() Meta {
	return Meta{CLI: "claude-code", CLIVersion: "1.0", PluginVersion: "test", SessionID: "sess-1"}
}

func TestRefreshOptOutClearsStateNoNetwork(t *testing.T) {
	dir := t.TempDir()
	if err := SaveConfig(dir, Config{OptOut: true}); err != nil {
		t.Fatal(err)
	}
	_ = SaveState(dir, State{Ad: &Ad{AdID: "old"}, ServedAt: 1})
	m := newEngineMock(&Ad{AdID: "new"})
	defer m.close()

	if err := Refresh(context.Background(), dir, m.client(), newMeta(), 100, true); err != nil {
		t.Fatal(err)
	}
	if m.serveCalls.Load() != 0 || m.impCalls.Load() != 0 {
		t.Errorf("opt-out must do no network: serve=%d imp=%d", m.serveCalls.Load(), m.impCalls.Load())
	}
	s, _ := LoadState(dir)
	if s.Ad != nil {
		t.Errorf("opt-out should clear cached ad, got %+v", s.Ad)
	}
}

func TestRefreshForceServesAndCaches(t *testing.T) {
	dir := t.TempDir()
	ad := &Ad{AdID: "a1", Sentence: "s", ImpressionToken: "tok1", RotateSeconds: 20}
	m := newEngineMock(ad)
	defer m.close()

	if err := Refresh(context.Background(), dir, m.client(), newMeta(), 100, true); err != nil {
		t.Fatal(err)
	}
	if m.serveCalls.Load() != 1 {
		t.Errorf("serve calls = %d, want 1", m.serveCalls.Load())
	}
	s, _ := LoadState(dir)
	if s.Ad == nil || s.Ad.AdID != "a1" || s.ServedAt != 100 || s.FirstRenderAt != 0 {
		t.Errorf("ad not cached fresh: %+v", s)
	}
}

func TestRefreshForce204ClearsAd(t *testing.T) {
	dir := t.TempDir()
	_ = SaveState(dir, State{Ad: &Ad{AdID: "old"}, ServedAt: 1})
	m := newEngineMock(nil) // empty inventory
	defer m.close()

	if err := Refresh(context.Background(), dir, m.client(), newMeta(), 100, true); err != nil {
		t.Fatal(err)
	}
	s, _ := LoadState(dir)
	if s.Ad != nil {
		t.Errorf("empty inventory should clear the slot, got %+v", s.Ad)
	}
}

func TestRefreshNotDueDoesNotServe(t *testing.T) {
	dir := t.TempDir()
	_ = SaveState(dir, State{Ad: &Ad{AdID: "a", RotateSeconds: 20}, ServedAt: 100, FirstRenderAt: 100, LastRenderAt: 100})
	m := newEngineMock(&Ad{AdID: "b"})
	defer m.close()

	// now only 5s after serve, no force => not due
	if err := Refresh(context.Background(), dir, m.client(), newMeta(), 105, false); err != nil {
		t.Fatal(err)
	}
	if m.serveCalls.Load() != 0 {
		t.Errorf("should not serve when not due: serve=%d", m.serveCalls.Load())
	}
	s, _ := LoadState(dir)
	if s.Ad.AdID != "a" {
		t.Errorf("cached ad should be unchanged, got %s", s.Ad.AdID)
	}
}

func TestRefreshRotatesAfterRotateSeconds(t *testing.T) {
	dir := t.TempDir()
	_ = SaveState(dir, State{
		Ad:            &Ad{AdID: "a", ImpressionToken: "tokA", RotateSeconds: 20},
		ServedAt:      100,
		FirstRenderAt: 101,
		LastRenderAt:  118,
	})
	m := newEngineMock(&Ad{AdID: "b", ImpressionToken: "tokB", RotateSeconds: 20})
	defer m.close()

	// 30s after serve, rendered => due even without force
	if err := Refresh(context.Background(), dir, m.client(), newMeta(), 130, false); err != nil {
		t.Fatal(err)
	}
	if m.serveCalls.Load() != 1 {
		t.Errorf("serve calls = %d, want 1 (rotation)", m.serveCalls.Load())
	}
	if m.impCalls.Load() != 1 {
		t.Errorf("previous ad impression should be reported: imp=%d", m.impCalls.Load())
	}
	s, _ := LoadState(dir)
	if s.Ad.AdID != "b" {
		t.Errorf("rotated ad should be b, got %s", s.Ad.AdID)
	}
	if got, _ := readQueue(dir); len(got) != 0 {
		t.Errorf("impression should have flushed, queue=%+v", got)
	}
}

func TestRefreshAutoRotationCappedAtMax(t *testing.T) {
	dir := t.TempDir()
	// Already spent the rotation budget; an idle timer tick should not rotate.
	_ = SaveState(dir, State{
		Ad:            &Ad{AdID: "a", ImpressionToken: "tokA", RotateSeconds: 20},
		ServedAt:      100,
		FirstRenderAt: 101,
		LastRenderAt:  118,
		RotateCount:   maxAutoRotations,
	})
	m := newEngineMock(&Ad{AdID: "b", RotateSeconds: 20})
	defer m.close()

	if err := Refresh(context.Background(), dir, m.client(), newMeta(), 130, false); err != nil {
		t.Fatal(err)
	}
	if m.serveCalls.Load() != 0 {
		t.Errorf("serve calls = %d, want 0 (budget exhausted)", m.serveCalls.Load())
	}
	s, _ := LoadState(dir)
	if s.Ad.AdID != "a" {
		t.Errorf("ad should remain a, got %s", s.Ad.AdID)
	}
}

func TestRefreshForceResetsRotationBudget(t *testing.T) {
	dir := t.TempDir()
	_ = SaveState(dir, State{
		Ad:            &Ad{AdID: "a", ImpressionToken: "tokA", RotateSeconds: 20},
		ServedAt:      100,
		FirstRenderAt: 101,
		LastRenderAt:  118,
		RotateCount:   maxAutoRotations,
	})
	m := newEngineMock(&Ad{AdID: "b", RotateSeconds: 20})
	defer m.close()

	if err := Refresh(context.Background(), dir, m.client(), newMeta(), 130, true); err != nil {
		t.Fatal(err)
	}
	s, _ := LoadState(dir)
	if s.Ad.AdID != "b" || s.RotateCount != 0 {
		t.Errorf("force should serve b and reset count, got ad=%s count=%d", s.Ad.AdID, s.RotateCount)
	}
}

func TestRefreshAutoRotationIncrementsCount(t *testing.T) {
	dir := t.TempDir()
	_ = SaveState(dir, State{
		Ad:            &Ad{AdID: "a", ImpressionToken: "tokA", RotateSeconds: 20},
		ServedAt:      100,
		FirstRenderAt: 101,
		LastRenderAt:  118,
		RotateCount:   1,
	})
	m := newEngineMock(&Ad{AdID: "b", RotateSeconds: 20})
	defer m.close()

	if err := Refresh(context.Background(), dir, m.client(), newMeta(), 130, false); err != nil {
		t.Fatal(err)
	}
	s, _ := LoadState(dir)
	if s.Ad.AdID != "b" || s.RotateCount != 2 {
		t.Errorf("auto-rotate should serve b and bump count, got ad=%s count=%d", s.Ad.AdID, s.RotateCount)
	}
}

func TestRefreshUnrenderedAdSkipsImpression(t *testing.T) {
	dir := t.TempDir()
	// previous ad was never rendered (FirstRenderAt == 0)
	_ = SaveState(dir, State{Ad: &Ad{AdID: "a", ImpressionToken: "tokA"}, ServedAt: 100})
	m := newEngineMock(&Ad{AdID: "b", ImpressionToken: "tokB"})
	defer m.close()

	if err := Refresh(context.Background(), dir, m.client(), newMeta(), 130, true); err != nil {
		t.Fatal(err)
	}
	if m.impCalls.Load() != 0 {
		t.Errorf("unrendered ad must not produce an impression: imp=%d", m.impCalls.Load())
	}
}

func TestRefreshServeErrorPropagatesAndKeepsImpression(t *testing.T) {
	dir := t.TempDir()
	_ = SaveState(dir, State{
		Ad:            &Ad{AdID: "a", ImpressionToken: "tokA"},
		ServedAt:      100,
		FirstRenderAt: 101,
		LastRenderAt:  110,
	})
	// serve returns 500; impressions endpoint also 500 so the buffered impression stays
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	err := Refresh(context.Background(), dir, clientFor(srv.URL), newMeta(), 130, true)
	if err == nil {
		t.Fatal("serve error should propagate")
	}
	got, _ := readQueue(dir)
	if len(got) != 1 || got[0].ImpressionToken != "tokA" {
		t.Errorf("previous impression should be buffered, queue=%+v", got)
	}
}

func TestRefreshUnauthorizedFlagsNeedsLogin(t *testing.T) {
	dir := t.TempDir()
	_ = SaveState(dir, State{
		Ad:            &Ad{AdID: "a", ImpressionToken: "tokA"},
		ServedAt:      100,
		FirstRenderAt: 101,
		LastRenderAt:  110,
	})
	// serve returns 401: the device token was rejected.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	err := Refresh(context.Background(), dir, clientFor(srv.URL), newMeta(), 130, true)
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
	s, _ := LoadState(dir)
	if !s.NeedsLogin {
		t.Error("NeedsLogin should be set after a rejected token")
	}
	if s.NeedsLoginReason != "device token invalid or revoked" {
		t.Errorf("NeedsLoginReason = %q", s.NeedsLoginReason)
	}
	if s.Ad != nil {
		t.Errorf("cached ad should be cleared, got %+v", s.Ad)
	}
}

func TestRenderSetsTimestampsAndReturnsLine(t *testing.T) {
	dir := t.TempDir()
	_ = SaveState(dir, State{Ad: &Ad{Sentence: "hello", Domain: "foo.com", ImpressionToken: "t"}, ServedAt: 100})

	line, notice, err := Render(dir, 105, "vibeperks login")
	if err != nil {
		t.Fatal(err)
	}
	if notice {
		t.Error("ad render should not be a notice")
	}
	if line != "hello - foo.com" {
		t.Errorf("line = %q", line)
	}
	s, _ := LoadState(dir)
	if s.FirstRenderAt != 105 || s.LastRenderAt != 105 {
		t.Errorf("render timestamps wrong: %+v", s)
	}
	// second render keeps first, updates last
	if _, _, err := Render(dir, 112, "vibeperks login"); err != nil {
		t.Fatal(err)
	}
	s, _ = LoadState(dir)
	if s.FirstRenderAt != 105 || s.LastRenderAt != 112 {
		t.Errorf("second render timestamps wrong: %+v", s)
	}
}

func TestRenderNoAd(t *testing.T) {
	line, notice, err := Render(t.TempDir(), 100, "vibeperks login")
	if err != nil || line != "" || notice {
		t.Fatalf("no ad: line=%q notice=%v err=%v", line, notice, err)
	}
}

func TestRenderNeedsLoginReturnsNotice(t *testing.T) {
	dir := t.TempDir()
	_ = SaveState(dir, State{NeedsLogin: true})

	line, notice, err := Render(dir, 100, "vibeperks login")
	if err != nil {
		t.Fatal(err)
	}
	if !notice {
		t.Error("NeedsLogin state should render a notice")
	}
	if line != LoginNotice("vibeperks login", "") {
		t.Errorf("line = %q", line)
	}
}

func TestEndSessionRecordsAndFlushes(t *testing.T) {
	dir := t.TempDir()
	_ = SaveState(dir, State{
		Ad:            &Ad{AdID: "a", ImpressionToken: "tokA"},
		ServedAt:      100,
		FirstRenderAt: 102,
		LastRenderAt:  120,
	})
	m := newEngineMock(nil)
	defer m.close()

	if err := EndSession(context.Background(), dir, m.client(), newMeta(), 121); err != nil {
		t.Fatal(err)
	}
	if m.impCalls.Load() != 1 {
		t.Errorf("impression calls = %d, want 1", m.impCalls.Load())
	}
	s, _ := LoadState(dir)
	if !s.Recorded {
		t.Error("ad should be marked recorded")
	}
	if got, _ := readQueue(dir); len(got) != 0 {
		t.Errorf("queue should be flushed: %+v", got)
	}
}

func TestEndSessionRecordsImpressionOnlyOnce(t *testing.T) {
	dir := t.TempDir()
	_ = SaveState(dir, State{
		Ad:            &Ad{AdID: "a", ImpressionToken: "tokA"},
		ServedAt:      100,
		FirstRenderAt: 102,
		LastRenderAt:  120,
	})
	m := newEngineMock(nil)
	defer m.close()

	for i := 0; i < 3; i++ {
		if err := EndSession(context.Background(), dir, m.client(), newMeta(), 121); err != nil {
			t.Fatal(err)
		}
	}
	if m.impCalls.Load() != 1 {
		t.Errorf("impression must be reported once, got %d", m.impCalls.Load())
	}
}

func TestRotateSecondsDefault(t *testing.T) {
	if rotateSeconds(nil) != defaultRotateSeconds {
		t.Errorf("nil ad => %d, want default", rotateSeconds(nil))
	}
	if rotateSeconds(&Ad{RotateSeconds: 0}) != defaultRotateSeconds {
		t.Errorf("zero rotate => %d, want default", rotateSeconds(&Ad{}))
	}
	if rotateSeconds(&Ad{RotateSeconds: 45}) != 45 {
		t.Errorf("explicit rotate not honored")
	}
}

func TestEndSessionNoAdNoop(t *testing.T) {
	dir := t.TempDir()
	m := newEngineMock(nil)
	defer m.close()
	if err := EndSession(context.Background(), dir, m.client(), newMeta(), 100); err != nil {
		t.Fatal(err)
	}
	if m.impCalls.Load() != 0 {
		t.Errorf("no ad => no impression, got %d", m.impCalls.Load())
	}
}

func TestRefreshNotDueFlushesPendingQueue(t *testing.T) {
	dir := t.TempDir()
	_ = SaveState(dir, State{Ad: &Ad{AdID: "a", RotateSeconds: 20}, ServedAt: 100, FirstRenderAt: 100, LastRenderAt: 100})
	_ = Enqueue(dir, Impression{ImpressionToken: "pending"})
	m := newEngineMock(&Ad{AdID: "b"})
	defer m.close()

	if err := Refresh(context.Background(), dir, m.client(), newMeta(), 105, false); err != nil {
		t.Fatal(err)
	}
	if m.serveCalls.Load() != 0 {
		t.Errorf("not due should not serve, got %d", m.serveCalls.Load())
	}
	if m.impCalls.Load() != 1 {
		t.Errorf("pending impression should still flush, got %d", m.impCalls.Load())
	}
}

func TestEndSessionOptOutNoop(t *testing.T) {
	dir := t.TempDir()
	_ = SaveConfig(dir, Config{OptOut: true})
	_ = SaveState(dir, State{Ad: &Ad{ImpressionToken: "t"}, FirstRenderAt: 1, LastRenderAt: 2})
	m := newEngineMock(nil)
	defer m.close()

	if err := EndSession(context.Background(), dir, m.client(), newMeta(), 100); err != nil {
		t.Fatal(err)
	}
	if m.impCalls.Load() != 0 {
		t.Errorf("opt-out should report nothing, imp=%d", m.impCalls.Load())
	}
}

func TestRecordCurrentComputesDisplayedMs(t *testing.T) {
	dir := t.TempDir()
	s := State{
		Ad:            &Ad{ImpressionToken: "tok"},
		ServedAt:      100,
		FirstRenderAt: 102,
		LastRenderAt:  120,
	}
	if err := recordCurrent(dir, &s, newMeta(), 121); err != nil {
		t.Fatal(err)
	}
	q, _ := readQueue(dir)
	if len(q) != 1 {
		t.Fatalf("expected 1 buffered impression, got %d", len(q))
	}
	if q[0].DisplayedMs != (121-102)*1000 {
		t.Errorf("displayed_ms = %d, want %d", q[0].DisplayedMs, (121-102)*1000)
	}
	if q[0].SessionDurationMs != (121-100)*1000 {
		t.Errorf("session_duration_ms = %d", q[0].SessionDurationMs)
	}
	if !s.Recorded {
		t.Error("state should be marked recorded")
	}
}
