package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/navi-crwn/pixelhop/internal/config"
)

// S3 talks to any S3-compatible service (AWS, Cloudflare R2, MinIO, Contabo,
// Backblaze B2, Wasabi...) with a small hand-written AWS Signature V4 signer,
// so PixelHop needs no SDK.
type S3 struct {
	cfg    config.S3Config
	client *http.Client
}

func NewS3(cfg config.S3Config) *S3 {
	return &S3{cfg: cfg, client: &http.Client{Timeout: 2 * time.Minute}}
}

func (s *S3) Name() string { return "s3" }

func (s *S3) objectURL(key string) *url.URL {
	u, _ := url.Parse(s.cfg.Endpoint)
	esc := escapeKey(key)
	if s.cfg.PathStyle {
		u.Path = "/" + s.cfg.Bucket + "/" + esc
		u.RawPath = u.Path
	} else {
		u.Host = s.cfg.Bucket + "." + u.Host
		u.Path = "/" + esc
		u.RawPath = u.Path
	}
	return u
}

func escapeKey(key string) string {
	parts := strings.Split(key, "/")
	for i, p := range parts {
		parts[i] = uriEncode(p)
	}
	return strings.Join(parts, "/")
}

// uriEncode is the strict RFC 3986 encoding SigV4 expects.
func uriEncode(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' || c == '~' {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

func (s *S3) do(ctx context.Context, method, key string, body io.Reader, size int64, hdr http.Header) (*http.Response, error) {
	u := s.objectURL(key)
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}
	for k, v := range hdr {
		req.Header[k] = v
	}
	if body != nil {
		req.ContentLength = size
	}
	// Streaming bodies are sent unsigned; TLS protects their integrity.
	payload := "UNSIGNED-PAYLOAD"
	if body == nil {
		payload = hex.EncodeToString(sha256.New().Sum(nil))
	}
	s.sign(req, payload, time.Now().UTC())
	return s.client.Do(req)
}

func (s *S3) sign(req *http.Request, payloadHash string, t time.Time) {
	amzDate := t.Format("20060102T150405Z")
	day := t.Format("20060102")
	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("x-amz-content-sha256", payloadHash)
	req.Header.Set("Host", req.URL.Host)

	var names []string
	canonHdr := map[string]string{"host": req.URL.Host}
	for k, v := range req.Header {
		lk := strings.ToLower(k)
		if lk == "host" || lk == "content-type" || lk == "range" || strings.HasPrefix(lk, "x-amz-") {
			canonHdr[lk] = strings.TrimSpace(strings.Join(v, ","))
		}
	}
	for k := range canonHdr {
		names = append(names, k)
	}
	slices.Sort(names)
	var ch strings.Builder
	for _, n := range names {
		ch.WriteString(n + ":" + canonHdr[n] + "\n")
	}
	signed := strings.Join(names, ";")
	canonical := strings.Join([]string{req.Method, req.URL.EscapedPath(), req.URL.RawQuery, ch.String(), signed, payloadHash}, "\n")
	scope := day + "/" + s.cfg.Region + "/s3/aws4_request"
	sum := sha256.Sum256([]byte(canonical))
	toSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scope + "\n" + hex.EncodeToString(sum[:])

	k := hmacSHA([]byte("AWS4"+s.cfg.SecretKey), day)
	k = hmacSHA(k, s.cfg.Region)
	k = hmacSHA(k, "s3")
	k = hmacSHA(k, "aws4_request")
	sig := hex.EncodeToString(hmacSHA(k, toSign))
	req.Header.Set("Authorization", fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		s.cfg.AccessKey, scope, signed, sig))
	req.Header.Del("Host") // net/http sends req.Host; the header copy is only for signing
}

func hmacSHA(key []byte, msg string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(msg))
	return m.Sum(nil)
}

func (s *S3) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	h := http.Header{}
	h.Set("Content-Type", contentType)
	h.Set("Cache-Control", "public, max-age=31536000, immutable")
	resp, err := s.do(ctx, http.MethodPut, key, r, size, h)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("s3 put %s: %s: %s", key, resp.Status, msg)
	}
	return nil
}

func (s *S3) Get(ctx context.Context, key, rangeHeader string) (*Object, error) {
	h := http.Header{}
	if rangeHeader != "" {
		h.Set("Range", rangeHeader)
	}
	resp, err := s.do(ctx, http.MethodGet, key, nil, 0, h)
	if err != nil {
		return nil, err
	}
	return objectFromResponse(resp)
}

func objectFromResponse(resp *http.Response) (*Object, error) {
	switch resp.StatusCode {
	case 200, 206:
		return &Object{
			Body: resp.Body, Size: resp.ContentLength, ContentType: resp.Header.Get("Content-Type"),
			ETag: resp.Header.Get("ETag"), LastModified: resp.Header.Get("Last-Modified"),
			Status: resp.StatusCode, ContentRange: resp.Header.Get("Content-Range"),
		}, nil
	case 404, 403:
		resp.Body.Close()
		return nil, ErrNotFound
	default:
		resp.Body.Close()
		return nil, fmt.Errorf("storage: %s", resp.Status)
	}
}

func (s *S3) Delete(ctx context.Context, key string) error {
	resp, err := s.do(ctx, http.MethodDelete, key, nil, 0, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode/100 != 2 && resp.StatusCode != 404 {
		return fmt.Errorf("s3 delete %s: %s", key, resp.Status)
	}
	return nil
}

// PublicURL returns the CDN address of key, or "" when none is configured.
func (s *S3) PublicURL(key string) string {
	if s.cfg.PublicURL == "" {
		return ""
	}
	return s.cfg.PublicURL + "/" + escapeKey(key)
}
