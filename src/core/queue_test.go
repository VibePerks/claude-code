package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestEnqueueDedupesByToken(t *testing.T) {
	dir := t.TempDir()
	imp := Impression{ImpressionToken: "tok", DisplayedMs: 1000}
	if err := Enqueue(dir, imp); err != nil {
		t.Fatal(err)
	}
	if err := Enqueue(dir, imp); err != nil {
		t.Fatal(err)
	}
	got, err := readQueue(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Errorf("queue len = %d, want 1 (deduped)", len(got))
	}
}

func TestEnqueueKeepsDistinct(t *testing.T) {
	dir := t.TempDir()
	_ = Enqueue(dir, Impression{ImpressionToken: "a"})
	_ = Enqueue(dir, Impression{ImpressionToken: "b"})
	got, _ := readQueue(dir)
	if len(got) != 2 || got[0].ImpressionToken != "a" || got[1].ImpressionToken != "b" {
		t.Errorf("queue = %+v, want [a b]", got)
	}
}

func TestReadQueueMissing(t *testing.T) {
	got, err := readQueue(t.TempDir())
	if err != nil || got != nil {
		t.Fatalf("missing queue: got %v err %v", got, err)
	}
}

func TestFlushEmptyNoop(t *testing.T) {
	if err := Flush(context.Background(), t.TempDir(), clientFor("http://unused")); err != nil {
		t.Fatalf("flush empty: %v", err)
	}
}

func TestFlushDeliversAndClears(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()
	dir := t.TempDir()
	_ = Enqueue(dir, Impression{ImpressionToken: "a"})
	_ = Enqueue(dir, Impression{ImpressionToken: "b"})

	if err := Flush(context.Background(), dir, clientFor(srv.URL)); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Errorf("post calls = %d, want 2", calls.Load())
	}
	if got, _ := readQueue(dir); len(got) != 0 {
		t.Errorf("queue not cleared: %+v", got)
	}
}

func TestFlushKeepsTransientFailuresWithOneRetry(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	dir := t.TempDir()
	_ = Enqueue(dir, Impression{ImpressionToken: "a"})

	err := Flush(context.Background(), dir, clientFor(srv.URL))
	if err == nil {
		t.Fatal("expected transient error to propagate")
	}
	if calls.Load() != 2 {
		t.Errorf("post calls = %d, want 2 (one bounded retry)", calls.Load())
	}
	if got, _ := readQueue(dir); len(got) != 1 {
		t.Errorf("transient failure should be kept, queue = %+v", got)
	}
}

func TestFlushDropsRejectedWithoutRetry(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()
	dir := t.TempDir()
	_ = Enqueue(dir, Impression{ImpressionToken: "a"})

	if err := Flush(context.Background(), dir, clientFor(srv.URL)); err != nil {
		t.Fatalf("rejected impression should be dropped silently: %v", err)
	}
	if calls.Load() != 1 {
		t.Errorf("post calls = %d, want 1 (no retry on rejection)", calls.Load())
	}
	if got, _ := readQueue(dir); len(got) != 0 {
		t.Errorf("rejected impression should be dropped, queue = %+v", got)
	}
}

func TestFlushRetrySucceedsOnSecondAttempt(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()
	dir := t.TempDir()
	_ = Enqueue(dir, Impression{ImpressionToken: "a"})

	if err := Flush(context.Background(), dir, clientFor(srv.URL)); err != nil {
		t.Fatalf("retry should succeed: %v", err)
	}
	if calls.Load() != 2 {
		t.Errorf("post calls = %d, want 2", calls.Load())
	}
	if got, _ := readQueue(dir); len(got) != 0 {
		t.Errorf("delivered after retry, queue should be empty: %+v", got)
	}
}
