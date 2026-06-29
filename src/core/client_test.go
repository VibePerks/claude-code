package core

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServeOKReturnsSanitizedAd(t *testing.T) {
	var gotToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get("X-Device-Token")
		writeJSON(w, http.StatusOK, Ad{
			AdID:            "ad1",
			Sentence:        "Fast\tAPIs at foo.com\n",
			Domain:          "foo.com",
			ImpressionToken: "imp-tok",
			RotateSeconds:   25,
		})
	}))
	defer srv.Close()

	ad, err := clientFor(srv.URL).Serve(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if gotToken != "dev-token" {
		t.Errorf("device token header = %q", gotToken)
	}
	if ad.Sentence != "FastAPIs at foo.com" {
		t.Errorf("sentence not sanitized: %q", ad.Sentence)
	}
	if ad.ImpressionToken != "imp-tok" || ad.RotateSeconds != 25 {
		t.Errorf("ad decoded wrong: %+v", ad)
	}
}

func TestServeNoContentReturnsNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	ad, err := clientFor(srv.URL).Serve(context.Background())
	if err != nil || ad != nil {
		t.Fatalf("204 should yield (nil,nil), got ad=%v err=%v", ad, err)
	}
}

func TestServeUnauthorized(t *testing.T) {
	for _, code := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
		}))
		_, err := clientFor(srv.URL).Serve(context.Background())
		srv.Close()
		if !errors.Is(err, ErrUnauthorized) {
			t.Errorf("status %d: err = %v, want ErrUnauthorized", code, err)
		}
	}
}

func TestServeServerErrorPropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	if _, err := clientFor(srv.URL).Serve(context.Background()); err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestPostImpressionSuccess(t *testing.T) {
	for _, code := range []int{http.StatusOK, http.StatusCreated} {
		var got Impression
		var ct string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ct = r.Header.Get("Content-Type")
			_ = decodeBody(r, &got)
			w.WriteHeader(code)
		}))
		err := clientFor(srv.URL).PostImpression(context.Background(), Impression{
			ImpressionToken: "tok", DisplayedMs: 2000, CLI: "claude-code",
		})
		srv.Close()
		if err != nil {
			t.Errorf("status %d: unexpected error %v", code, err)
		}
		if got.ImpressionToken != "tok" || got.DisplayedMs != 2000 || got.CLI != "claude-code" {
			t.Errorf("status %d: body decoded wrong: %+v", code, got)
		}
		if ct != "application/json" {
			t.Errorf("content-type = %q", ct)
		}
	}
}

func TestPostImpressionRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad token", http.StatusBadRequest)
	}))
	defer srv.Close()
	err := clientFor(srv.URL).PostImpression(context.Background(), Impression{ImpressionToken: "tok"})
	if !errors.Is(err, ErrRejected) {
		t.Errorf("err = %v, want ErrRejected", err)
	}
}

func TestPostImpressionUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	err := clientFor(srv.URL).PostImpression(context.Background(), Impression{ImpressionToken: "tok"})
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("err = %v, want ErrUnauthorized", err)
	}
}

func TestPostImpressionServerErrorPropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	err := clientFor(srv.URL).PostImpression(context.Background(), Impression{ImpressionToken: "tok"})
	if err == nil || errors.Is(err, ErrRejected) || errors.Is(err, ErrUnauthorized) {
		t.Errorf("err = %v, want a transient (propagating) error", err)
	}
}
