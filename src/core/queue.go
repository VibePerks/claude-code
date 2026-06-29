package core

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// flushRetryDelay is the single bounded backoff between the two attempts a transient
// impression post gets. There is no nested retry: PostImpression itself never retries.
const flushRetryDelay = 200 * time.Millisecond

func queuePath(dir string) string { return filepath.Join(dir, "impressions.jsonl") }

// Enqueue appends an impression to the local buffer, deduped by impression token so a
// repeated record (e.g. prompt + stop for the same ad) is stored once.
func Enqueue(dir string, imp Impression) error {
	imps, err := readQueue(dir)
	if err != nil {
		return err
	}
	for _, e := range imps {
		if e.ImpressionToken == imp.ImpressionToken {
			return nil
		}
	}
	return writeQueue(dir, append(imps, imp))
}

func readQueue(dir string) ([]Impression, error) {
	f, err := os.Open(queuePath(dir))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Impression
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var imp Impression
		if err := json.Unmarshal(line, &imp); err != nil {
			return nil, err
		}
		out = append(out, imp)
	}
	return out, sc.Err()
}

func writeQueue(dir string, imps []Impression) error {
	if len(imps) == 0 {
		err := os.Remove(queuePath(dir))
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var buf bytes.Buffer
	for _, imp := range imps {
		b, err := json.Marshal(imp)
		if err != nil {
			return err
		}
		buf.Write(b)
		buf.WriteByte('\n')
	}
	return atomicWrite(queuePath(dir), buf.Bytes(), 0o600)
}

// Flush posts every buffered impression. Delivered (2xx) and permanently rejected
// (ErrRejected) impressions are dropped; transient failures are kept for the next flush.
// Each impression gets one bounded retry. The first transient error (if any) propagates
// after the buffer has been rewritten, so the boundary can log it.
func Flush(ctx context.Context, dir string, c *Client) error {
	imps, err := readQueue(dir)
	if err != nil {
		return err
	}
	if len(imps) == 0 {
		return nil
	}
	var remaining []Impression
	var firstErr error
	for _, imp := range imps {
		perr := postWithRetry(ctx, c, imp)
		if perr == nil || errors.Is(perr, ErrRejected) {
			continue
		}
		remaining = append(remaining, imp)
		if firstErr == nil {
			firstErr = perr
		}
	}
	if err := writeQueue(dir, remaining); err != nil {
		return err
	}
	return firstErr
}

// postWithRetry attempts a single impression post with at most one bounded retry, and
// only for transient failures. Permanent outcomes (success, rejection, auth failure)
// return immediately without retrying.
func postWithRetry(ctx context.Context, c *Client, imp Impression) error {
	err := c.PostImpression(ctx, imp)
	if err == nil || errors.Is(err, ErrRejected) || errors.Is(err, ErrUnauthorized) {
		return err
	}
	time.Sleep(flushRetryDelay)
	return c.PostImpression(ctx, imp)
}
