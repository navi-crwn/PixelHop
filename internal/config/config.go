// Package config reads PixelHop's settings from environment variables.
// Every setting has a sane default, so `pixelhop serve` works with none set.
package config

import (
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr      string // listen address, e.g. ":8080"
	BaseURL   string // public URL without trailing slash, e.g. "https://p.hel.ink"
	DataDir   string // SQLite database + local image storage
	SiteName  string
	SecretKey string // signs nothing secret-critical today; reserved for future tokens

	MaxUploadBytes int64
	MaxMediaBytes  int64 // videos and PDFs (uploaded in pieces, then compressed)
	MaxPixels      int64 // decompression-bomb guard (width*height)
	MaxBatch       int   // files per upload request

	GuestUploads     bool
	Registration     bool
	DefaultQuota     int64 // bytes per new account
	InactiveDays     int   // delete images not viewed for N days (0 = never)
	TrustedProxies   []netip.Prefix
	UploadsPerMinute int

	Storage   string // "local" or "s3"
	S3        S3Config
	GoogleID  string
	GoogleKey string
	// GoogleRedirect must match the URI registered in Google Cloud Console.
	// PixelHop 2 used /auth/google-callback; both paths work.
	GoogleRedirect string

	AdminEmail string // shown on legal pages / DMCA contact

	// HEL.ink accounts: when both are set, people sign in with their HEL.ink
	// account and PixelHop's own sign-up, password reset and Google login
	// are handed over to HEL.ink.
	HelinkURL    string
	HelinkSecret string

	// Cloudflare Turnstile on PixelHop's own sign-in and sign-up forms.
	TurnstileSiteKey string
	TurnstileSecret  string

	// Optional background-removal service (see tools/rembg-server.py).
	RembgURL    string
	RembgPerDay int
}

type S3Config struct {
	Endpoint  string // https://s3.example.com
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
	PathStyle bool
	PublicURL string // optional CDN base; if set, image URLs redirect there
}

// Cloudflare's published ranges, used when TRUSTED_PROXIES=cloudflare.
var cloudflare = []string{
	"173.245.48.0/20", "103.21.244.0/22", "103.22.200.0/22", "103.31.4.0/22",
	"141.101.64.0/18", "108.162.192.0/18", "190.93.240.0/20", "188.114.96.0/20",
	"197.234.240.0/22", "198.41.128.0/17", "162.158.0.0/15", "104.16.0.0/13",
	"104.24.0.0/14", "172.64.0.0/13", "131.0.72.0/22",
	"2400:cb00::/32", "2606:4700::/32", "2803:f800::/32", "2405:b500::/32",
	"2405:8100::/32", "2a06:98c0::/29", "2c0f:f248::/32",
}

func Load() (*Config, error) {
	c := &Config{
		Addr:             env("PIXELHOP_ADDR", ":8080"),
		BaseURL:          strings.TrimRight(env("PIXELHOP_BASE_URL", "http://localhost:8080"), "/"),
		DataDir:          env("PIXELHOP_DATA", "./data"),
		SiteName:         env("PIXELHOP_SITE_NAME", "PixelHop"),
		SecretKey:        env("PIXELHOP_SECRET", ""),
		MaxUploadBytes:   int64(envInt("PIXELHOP_MAX_UPLOAD_MB", 20)) << 20,
		MaxMediaBytes:    int64(envInt("PIXELHOP_MAX_MEDIA_MB", 500)) << 20,
		MaxPixels:        int64(envInt("PIXELHOP_MAX_MEGAPIXELS", 80)) * 1_000_000,
		MaxBatch:         envInt("PIXELHOP_MAX_BATCH", 20),
		GuestUploads:     envBool("PIXELHOP_GUEST_UPLOADS", true),
		Registration:     envBool("PIXELHOP_REGISTRATION", true),
		DefaultQuota:     int64(envInt("PIXELHOP_DEFAULT_QUOTA_MB", 1024)) << 20,
		InactiveDays:     envInt("PIXELHOP_INACTIVE_DAYS", 0),
		UploadsPerMinute: envInt("PIXELHOP_UPLOADS_PER_MINUTE", 30),
		Storage:          env("PIXELHOP_STORAGE", "local"),
		S3: S3Config{
			Endpoint:  strings.TrimRight(env("S3_ENDPOINT", ""), "/"),
			Region:    env("S3_REGION", "auto"),
			Bucket:    env("S3_BUCKET", ""),
			AccessKey: env("S3_ACCESS_KEY", ""),
			SecretKey: env("S3_SECRET_KEY", ""),
			PathStyle: envBool("S3_PATH_STYLE", true),
			PublicURL: strings.TrimRight(env("S3_PUBLIC_URL", ""), "/"),
		},
		GoogleID:   env("GOOGLE_CLIENT_ID", ""),
		GoogleKey:  env("GOOGLE_CLIENT_SECRET", ""),
		AdminEmail: env("PIXELHOP_CONTACT_EMAIL", ""),
	}

	for _, raw := range strings.Split(env("PIXELHOP_TRUSTED_PROXIES", ""), ",") {
		raw = strings.TrimSpace(raw)
		switch {
		case raw == "":
			continue
		case raw == "cloudflare":
			for _, r := range cloudflare {
				c.TrustedProxies = append(c.TrustedProxies, netip.MustParsePrefix(r))
			}
		default:
			if !strings.Contains(raw, "/") {
				if a, err := netip.ParseAddr(raw); err == nil {
					raw = netip.PrefixFrom(a, a.BitLen()).String()
				}
			}
			p, err := netip.ParsePrefix(raw)
			if err != nil {
				return nil, fmt.Errorf("PIXELHOP_TRUSTED_PROXIES: %q: %w", raw, err)
			}
			c.TrustedProxies = append(c.TrustedProxies, p)
		}
	}

	c.GoogleRedirect = env("GOOGLE_REDIRECT_URI", c.BaseURL+"/auth/google/callback")
	c.HelinkURL = strings.TrimRight(env("HELINK_URL", ""), "/")
	c.HelinkSecret = env("HELINK_SSO_SECRET", "")
	c.TurnstileSiteKey = env("TURNSTILE_SITE_KEY", "")
	c.TurnstileSecret = env("TURNSTILE_SECRET_KEY", "")
	c.RembgURL = strings.TrimRight(env("PIXELHOP_REMBG_URL", ""), "/")
	c.RembgPerDay = envInt("PIXELHOP_REMBG_PER_DAY", 30)
	if c.HelinkURL != "" && len(c.HelinkSecret) < 32 {
		return nil, fmt.Errorf("HELINK_SSO_SECRET must be at least 32 characters (the same value as PIXELHOP_SSO_SECRET on HEL.ink)")
	}

	if c.Storage == "s3" && (c.S3.Endpoint == "" || c.S3.Bucket == "" || c.S3.AccessKey == "") {
		return nil, fmt.Errorf("PIXELHOP_STORAGE=s3 needs S3_ENDPOINT, S3_BUCKET, S3_ACCESS_KEY and S3_SECRET_KEY")
	}
	if c.Storage != "s3" && c.Storage != "local" {
		return nil, fmt.Errorf("PIXELHOP_STORAGE must be local or s3, got %q", c.Storage)
	}
	return c, nil
}

// SecureCookies is true when the site is served over HTTPS.
func (c *Config) SecureCookies() bool { return strings.HasPrefix(c.BaseURL, "https://") }

// SSOEnabled reports whether accounts come from HEL.ink.
func (c *Config) SSOEnabled() bool { return c.HelinkURL != "" && c.HelinkSecret != "" }

// TurnstileEnabled reports whether forms carry a Cloudflare Turnstile check.
func (c *Config) TurnstileEnabled() bool { return c.TurnstileSiteKey != "" && c.TurnstileSecret != "" }

// GoogleEnabled reports whether "Sign in with Google" is configured.
func (c *Config) GoogleEnabled() bool { return c.GoogleID != "" && c.GoogleKey != "" }

const SessionTTL = 30 * 24 * time.Hour

func env(k, def string) string {
	if v, ok := os.LookupEnv(k); ok && v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(k)); err == nil {
		return v
	}
	return def
}

func envBool(k string, def bool) bool {
	if v, err := strconv.ParseBool(os.Getenv(k)); err == nil {
		return v
	}
	return def
}
