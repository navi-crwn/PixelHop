package db

import (
	"context"
	"sort"
	"strings"
	"unicode"
)

// CleanTags makes a tag list safe and tidy: lower case, letters, digits and
// dashes only, at most 12 tags of 24 characters, no repeats.
func CleanTags(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range in {
		var b strings.Builder
		for _, r := range strings.ToLower(strings.TrimSpace(t)) {
			switch {
			case unicode.IsLetter(r) || unicode.IsDigit(r):
				b.WriteRune(r)
			case r == '-' || r == '_' || r == ' ':
				b.WriteRune('-')
			}
		}
		tag := strings.Trim(b.String(), "-")
		if len([]rune(tag)) > 24 {
			tag = string([]rune(tag)[:24])
		}
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		out = append(out, tag)
		if len(out) == 12 {
			break
		}
	}
	return out
}

func (d *DB) SetImageTags(ctx context.Context, id string, tags []string) error {
	tags = CleanTags(tags)
	v := ""
	if len(tags) > 0 {
		v = "," + strings.Join(tags, ",") + ","
	}
	_, err := d.ExecContext(ctx, `UPDATE images SET tags=? WHERE id=?`, v, id)
	return err
}

func (d *DB) ImageTags(ctx context.Context, id string) []string {
	var v string
	d.QueryRowContext(ctx, `SELECT tags FROM images WHERE id=?`, id).Scan(&v)
	return splitTags(v)
}

func splitTags(v string) []string {
	var out []string
	for _, t := range strings.Split(v, ",") {
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

// TagCount is a tag and how many of a user's files carry it.
type TagCount struct {
	Tag   string
	Count int
}

// UserTags lists the tags a user has used, most used first.
func (d *DB) UserTags(ctx context.Context, userID int64) []TagCount {
	rows, err := d.QueryContext(ctx, `SELECT tags FROM images WHERE user_id=? AND tags<>'' AND deleted_at=0`, userID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	count := map[string]int{}
	for rows.Next() {
		var v string
		if rows.Scan(&v) == nil {
			for _, t := range splitTags(v) {
				count[t]++
			}
		}
	}
	var out []TagCount
	for t, n := range count {
		out = append(out, TagCount{t, n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Tag < out[j].Tag
	})
	if len(out) > 40 {
		out = out[:40]
	}
	return out
}
