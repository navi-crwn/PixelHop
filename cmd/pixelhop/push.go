package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The same binary is also a small client for any PixelHop server:
//
//	pixelhop login --server https://p.hel.ink --key ph_...
//	pixelhop push photo.jpg screenshot.png [--expire 1d] [--format markdown]
//
// Settings live in ~/.config/pixelhop/client.json (or PIXELHOP_SERVER /
// PIXELHOP_KEY in the environment).

type clientConfig struct {
	Server string `json:"server"`
	Key    string `json:"key"`
}

func clientConfigPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "pixelhop", "client.json")
}

func loadClientConfig() clientConfig {
	var c clientConfig
	if b, err := os.ReadFile(clientConfigPath()); err == nil {
		json.Unmarshal(b, &c)
	}
	if v := os.Getenv("PIXELHOP_SERVER"); v != "" {
		c.Server = v
	}
	if v := os.Getenv("PIXELHOP_KEY"); v != "" {
		c.Key = v
	}
	if c.Server == "" {
		c.Server = "https://p.hel.ink"
	}
	c.Server = strings.TrimRight(c.Server, "/")
	return c
}

func loginCmd(args []string) error {
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	server := fs.String("server", "https://p.hel.ink", "PixelHop server")
	key := fs.String("key", "", "API key from Settings → API keys (starts with ph_)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if !strings.HasPrefix(*key, "ph_") {
		return errors.New("give your API key: pixelhop login --key ph_…  (create one in Settings → API keys)")
	}
	c := clientConfig{Server: strings.TrimRight(*server, "/"), Key: *key}
	req, _ := http.NewRequest(http.MethodGet, c.Server+"/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+c.Key)
	var me struct {
		Success bool `json:"success"`
		Error   string
		Data    struct{ Email, Name string }
	}
	if err := doJSON(req, &me); err != nil {
		return err
	}
	if !me.Success {
		return fmt.Errorf("the server said: %s", me.Error)
	}
	p := clientConfigPath()
	os.MkdirAll(filepath.Dir(p), 0o700)
	b, _ := json.MarshalIndent(c, "", "  ")
	if err := os.WriteFile(p, b, 0o600); err != nil {
		return err
	}
	fmt.Printf("Signed in to %s as %s. Saved in %s\n", c.Server, me.Data.Email, p)
	return nil
}

func pushCmd(args []string) error {
	fs := flag.NewFlagSet("push", flag.ContinueOnError)
	expire := fs.String("expire", "", "delete after: 10m, 1h, 1d, 7d, 30d, 90d")
	format := fs.String("format", "page", "what to print: page, direct, markdown, html, bbcode, json")
	listed := fs.Bool("profile", false, "show on your public profile")
	noAlbum := fs.Bool("no-album", false, "don't group several files into an album")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: pixelhop push [flags] FILE...  (or - to read one image from stdin)")
		fs.PrintDefaults()
	}
	// Allow flags after the file names too.
	var files []string
	for len(args) > 0 {
		if err := fs.Parse(args); err != nil {
			return err
		}
		args = fs.Args()
		if len(args) > 0 {
			files = append(files, args[0])
			args = args[1:]
		}
	}
	if len(files) == 0 {
		fs.Usage()
		return errors.New("no files given")
	}
	c := loadClientConfig()

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for _, f := range files {
		var data []byte
		var err error
		name := filepath.Base(f)
		if f == "-" {
			data, err = io.ReadAll(io.LimitReader(os.Stdin, 100<<20))
			name = "stdin.png"
		} else {
			data, err = os.ReadFile(f)
		}
		if err != nil {
			return err
		}
		fw, _ := mw.CreateFormFile("file", name)
		fw.Write(data)
	}
	if *expire != "" {
		mw.WriteField("expire", *expire)
	}
	if *listed {
		mw.WriteField("listed", "1")
	}
	if *noAlbum {
		mw.WriteField("album", "0")
	}
	mw.Close()

	req, _ := http.NewRequest(http.MethodPost, c.Server+"/api/v1/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if c.Key != "" {
		req.Header.Set("Authorization", "Bearer "+c.Key)
	}
	var res struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
		Data    struct {
			AlbumURL string                       `json:"album_url"`
			Images   []map[string]json.RawMessage `json:"images"`
			Errors   []struct{ File, Error string }
		} `json:"data"`
	}
	raw, err := doRaw(req)
	if err != nil {
		return err
	}
	if *format == "json" {
		os.Stdout.Write(raw)
		return nil
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return fmt.Errorf("unexpected answer from %s", c.Server)
	}
	if !res.Success {
		return errors.New(res.Error)
	}
	str := func(m map[string]json.RawMessage, k string) string {
		var s string
		json.Unmarshal(m[k], &s)
		return s
	}
	for _, im := range res.Data.Images {
		switch *format {
		case "direct":
			fmt.Println(str(im, "url"))
		case "markdown", "html", "bbcode":
			var e map[string]string
			json.Unmarshal(im["embed"], &e)
			fmt.Println(e[*format])
		default:
			fmt.Println(str(im, "view_url"))
		}
		if d := str(im, "delete_url"); d != "" {
			fmt.Fprintln(os.Stderr, "  delete:", d)
		}
	}
	for _, e := range res.Data.Errors {
		fmt.Fprintf(os.Stderr, "%s: %s\n", e.File, e.Error)
	}
	if res.Data.AlbumURL != "" {
		fmt.Fprintln(os.Stderr, "album:", res.Data.AlbumURL)
	}
	return nil
}

func doRaw(req *http.Request) ([]byte, error) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "pixelhop-cli")
	resp, err := (&http.Client{Timeout: 5 * time.Minute}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(io.LimitReader(resp.Body, 10<<20))
}

func doJSON(req *http.Request, v any) error {
	b, err := doRaw(req)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}
