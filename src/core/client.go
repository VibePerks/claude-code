package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ErrUnauthorized means the device token was rejected (401/403). The caller should
// prompt the user to re-link the device; retrying with the same token will not help.
var ErrUnauthorized = errors.New("device token unauthorized")

// AuthRejection wraps ErrUnauthorized with a short, user-facing reason explaining why
// the device token was rejected, so surfaces can tell the user what went wrong. It
// satisfies errors.Is(err, ErrUnauthorized) so existing auth checks keep working.
type AuthRejection struct{ Reason string }

func (e *AuthRejection) Error() string { return "device token unauthorized: " + e.Reason }

func (e *AuthRejection) Is(target error) bool { return target == ErrUnauthorized }

// authReason maps a rejection status to the user-facing reason. The backend returns
// 403 only for a suspended account and 401 for an invalid/revoked/unknown token, so
// the status alone is an accurate, non-guessing reason.
func authReason(status int) string {
	if status == http.StatusForbidden {
		return "account suspended"
	}
	return "device token invalid or revoked"
}

// UnauthorizedReason extracts the user-facing reason from an auth-rejection error,
// returning "" when the error carries none.
func UnauthorizedReason(err error) string {
	var ar *AuthRejection
	if errors.As(err, &ar) {
		return ar.Reason
	}
	return ""
}

// ErrRejected means the backend permanently refused an impression (a 4xx other than
// auth, e.g. an expired or malformed token). Such impressions are dropped, not retried.
var ErrRejected = errors.New("impression rejected")

const httpTimeout = 5 * time.Second

// Client talks to the VibePerks backend with the device token attached to every request.
type Client struct {
	http  *http.Client
	base  string
	token string
}

// NewClient builds a client from config with a hard per-request timeout so a slow or
// hung backend can never stall the host CLI.
func NewClient(cfg Config) *Client {
	return &Client{
		http:  &http.Client{Timeout: httpTimeout},
		base:  strings.TrimRight(cfg.APIBase, "/"),
		token: cfg.DeviceToken,
	}
}

// Serve fetches the next eligible ad. A 204 (empty inventory) returns (nil, nil).
func (c *Client) Serve(ctx context.Context) (*Ad, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/v1/ads/serve", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Device-Token", c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil, nil
	case http.StatusOK:
		var ad Ad
		if err := json.NewDecoder(resp.Body).Decode(&ad); err != nil {
			return nil, err
		}
		ad.Sentence = SanitizeAd(ad.Sentence)
		ad.Domain = SanitizeAd(ad.Domain)
		return &ad, nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, &AuthRejection{Reason: authReason(resp.StatusCode)}
	default:
		return nil, fmt.Errorf("serve: unexpected status %d: %s", resp.StatusCode, readSnippet(resp.Body))
	}
}

// PostImpression reports one impression. Success is 200/201. A 4xx (non-auth) is a
// permanent rejection (ErrRejected); 401/403 is ErrUnauthorized; 5xx/transport errors
// propagate so the caller can retry.
func (c *Client) PostImpression(ctx context.Context, imp Impression) error {
	body, err := json.Marshal(imp)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/v1/impressions", strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("X-Device-Token", c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated:
		return nil
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return &AuthRejection{Reason: authReason(resp.StatusCode)}
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		return ErrRejected
	default:
		return fmt.Errorf("impression: unexpected status %d: %s", resp.StatusCode, readSnippet(resp.Body))
	}
}

func readSnippet(r io.Reader) string {
	b, _ := io.ReadAll(io.LimitReader(r, 512))
	return strings.TrimSpace(string(b))
}
