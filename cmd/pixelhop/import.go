package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/navi-crwn/pixelhop/internal/app"

	"github.com/navi-crwn/pixelhop/internal/config"
	"github.com/navi-crwn/pixelhop/internal/db"
)

// legacyImage is one entry of PixelHop 2's data/images.json.
type legacyImage struct {
	ID           string            `json:"id"`
	UserID       *int64            `json:"user_id"`
	Filename     string            `json:"filename"`
	Mime         string            `json:"mime_type"`
	Ext          string            `json:"extension"`
	Size         int64             `json:"size"`
	Width        int               `json:"width"`
	Height       int               `json:"height"`
	Hash         string            `json:"hash"`
	URLs         map[string]string `json:"urls"`
	Providers    map[string]string `json:"storage_providers"`
	CreatedAt    int64             `json:"created_at"`
	DeleteAt     *int64            `json:"delete_at"`
	IP           string            `json:"ip"`
	Views        int64             `json:"view_count"`
	LastViewed   any               `json:"last_viewed_at"`
	MarkedDelete bool              `json:"marked_for_deletion"`
}

// importCmd copies users (from the old MySQL database) and images (from
// images.json) into PixelHop 3. Images stay where they are (Contabo / R2) and
// are served through the "remote" backend, so every old link keeps working.
// Old passwords keep working too: PixelHop 3 verifies PHP's argon2id hashes.
func importCmd(cfg *config.Config, d *db.DB, args []string) error {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	imagesPath := fs.String("images", "", "path to PixelHop 2 data/images.json")
	mysqlDSN := fs.String("mysql", "", `PixelHop 2 MySQL DSN, e.g. "user:pass@tcp(127.0.0.1:3306)/pixelhop"`)
	copyFiles := fs.Bool("copy", false, "download every original into PixelHop 3's own storage (and make new thumbnails) instead of linking to the old buckets")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *imagesPath == "" && *mysqlDSN == "" {
		return fmt.Errorf("give --images and/or --mysql")
	}
	ctx := context.Background()
	var importer *app.App
	if *copyFiles {
		var err error
		if importer, err = app.New(cfg, d, slog.New(slog.NewTextHandler(os.Stderr, nil))); err != nil {
			return err
		}
	}

	if *mysqlDSN != "" {
		n, err := importUsers(ctx, cfg, d, *mysqlDSN)
		if err != nil {
			return fmt.Errorf("users: %w", err)
		}
		fmt.Printf("Imported %d users\n", n)
	}

	if *imagesPath != "" {
		raw, err := os.ReadFile(*imagesPath)
		if err != nil {
			return err
		}
		var all map[string]legacyImage
		if err := json.Unmarshal(raw, &all); err != nil {
			return fmt.Errorf("images.json: %w", err)
		}
		added, skipped := 0, 0
		for key, li := range all {
			if li.ID == "" {
				li.ID = key
			}
			full := li.URLs["original"]
			if full == "" || li.MarkedDelete {
				skipped++
				continue
			}
			// PixelHop 2 kept medium/thumb sizes in a private R2 bucket that is only
			// reachable through signed URLs, so prefer sizes on public storage.
			thumb := full
			for _, size := range []string{"medium", "thumb", "large"} {
				if u := li.URLs[size]; u != "" && li.Providers[size] != "r2" && !strings.Contains(u, "r2.") {
					thumb = u
					break
				}
			}
			im := &db.Image{
				ID: li.ID, Title: strings.TrimSuffix(li.Filename, path.Ext(li.Filename)), OrigName: li.Filename,
				Mime: li.Mime, Ext: li.Ext, Size: li.Size, Width: li.Width, Height: li.Height, SHA256: li.Hash,
				Backend: "remote", KeyFull: full, KeyThumb: thumb, Views: li.Views, IP: li.IP, CreatedAt: li.CreatedAt,
			}
			if li.DeleteAt != nil {
				im.ExpiresAt = *li.DeleteAt
			}
			switch v := li.LastViewed.(type) {
			case float64:
				im.LastViewed = int64(v)
			case string:
				if t, err := time.Parse("2006-01-02 15:04:05", v); err == nil {
					im.LastViewed = t.Unix()
				}
			}
			if li.UserID != nil {
				if _, err := d.UserByID(ctx, *li.UserID); err == nil {
					im.UserID = sql.NullInt64{Int64: *li.UserID, Valid: true}
				}
			}
			if im.Mime == "" {
				im.Mime = "image/" + strings.Replace(im.Ext, "jpg", "jpeg", 1)
			}
			if importer != nil {
				if _, err := d.Image(ctx, im.ID); err == nil {
					skipped++
					continue
				}
				if err := importer.ImportCopy(ctx, im, full); err != nil {
					fmt.Printf("  %s: %v (linked to the old storage instead)\n", im.ID, err)
				} else {
					added++
					continue
				}
			}
			if err := d.InsertImage(ctx, im); err != nil {
				if strings.Contains(err.Error(), "UNIQUE") {
					skipped++
					continue
				}
				return fmt.Errorf("image %s: %w", li.ID, err)
			}
			added++
		}
		fmt.Printf("Imported %d images (%d skipped: already there or marked for deletion)\n", added, skipped)
	}
	return nil
}

func importUsers(ctx context.Context, cfg *config.Config, d *db.DB, dsn string) (int, error) {
	if !strings.Contains(dsn, "parseTime") {
		sep := "?"
		if strings.Contains(dsn, "?") {
			sep = "&"
		}
		dsn += sep + "parseTime=true"
	}
	my, err := sql.Open("mysql", dsn)
	if err != nil {
		return 0, err
	}
	defer my.Close()
	rows, err := my.QueryContext(ctx, `SELECT id, email, COALESCE(password_hash,''), COALESCE(google_id,''),
		COALESCE(avatar_url,''), COALESCE(role,'user'), COALESCE(account_status,'active'), COALESCE(storage_limit,0), created_at
		FROM users ORDER BY id`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var u db.User
		var gid string
		var created time.Time
		if err := rows.Scan(&u.ID, &u.Email, &u.PassHash, &gid, &u.AvatarURL, &u.Role, &u.Status, &u.QuotaBytes, &created); err != nil {
			return n, err
		}
		if gid != "" {
			u.GoogleID = sql.NullString{String: gid, Valid: true}
		}
		if u.QuotaBytes <= 0 {
			u.QuotaBytes = cfg.DefaultQuota
		}
		if u.Role != "admin" {
			u.Role = "user"
		}
		u.CreatedAt = created.Unix()
		if _, err := d.UserByEmail(ctx, u.Email); err == nil {
			continue
		}
		if err := d.InsertUser(ctx, &u); err != nil {
			return n, fmt.Errorf("user %s: %w", u.Email, err)
		}
		n++
	}
	return n, rows.Err()
}

func firstNonEmpty(s ...string) string {
	for _, v := range s {
		if v != "" {
			return v
		}
	}
	return ""
}
