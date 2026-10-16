// Command pixelhop runs the PixelHop image host.
//
//	pixelhop                      start the server (same as "serve")
//	pixelhop user add EMAIL [--admin]
//	pixelhop user passwd EMAIL
//	pixelhop user admin EMAIL
//	pixelhop user merge OLD_EMAIL NEW_EMAIL
//	pixelhop import --images images.json [--mysql DSN]
//	pixelhop version
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/navi-crwn/pixelhop/internal/app"
	"github.com/navi-crwn/pixelhop/internal/config"
	"github.com/navi-crwn/pixelhop/internal/db"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "pixelhop:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cmd := "serve"
	if len(args) > 0 {
		cmd, args = args[0], args[1:]
	}
	if cmd == "version" || cmd == "--version" || cmd == "-v" {
		fmt.Println("pixelhop", app.Version)
		return nil
	}
	if cmd == "help" || cmd == "--help" || cmd == "-h" {
		fmt.Print(usage)
		return nil
	}
	// Client commands talk to a server and need no local data folder.
	switch cmd {
	case "push":
		return pushCmd(args)
	case "login":
		return loginCmd(args)
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		return err
	}
	d, err := db.Open(cfg.DataDir)
	if err != nil {
		return err
	}
	defer d.Close()

	switch cmd {
	case "serve":
		log := slog.New(slog.NewTextHandler(os.Stdout, nil))
		a, err := app.New(cfg, d, log)
		if err != nil {
			return err
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return a.Run(ctx)
	case "user":
		return userCmd(cfg, d, args)
	case "import":
		return importCmd(cfg, d, args)
	}
	fmt.Print(usage)
	return fmt.Errorf("unknown command %q", cmd)
}

const usage = `PixelHop — image hosting in one small binary.

Usage:
  pixelhop [serve]                     start the web server
  pixelhop user add EMAIL [--admin]    create an account (asks for a password)
  pixelhop user passwd EMAIL           set a new password
  pixelhop user admin EMAIL            make an account an admin
  pixelhop user merge OLD NEW          move OLD's images into NEW, then delete OLD
  pixelhop import --images FILE [--mysql DSN]
                                       bring over images (and users) from PixelHop 2
  pixelhop version

Settings come from environment variables; see .env.example.
`

func userCmd(cfg *config.Config, d *db.DB, args []string) error {
	if len(args) < 2 {
		return errors.New("usage: pixelhop user add|passwd|admin EMAIL")
	}
	ctx := context.Background()
	sub, email := args[0], strings.TrimSpace(args[1])
	switch sub {
	case "add":
		pw, err := askPassword()
		if err != nil {
			return err
		}
		u := &db.User{Email: email, PassHash: app.HashPassword(pw), QuotaBytes: cfg.DefaultQuota}
		if len(args) > 2 && args[2] == "--admin" {
			u.Role = "admin"
		}
		if err := d.InsertUser(ctx, u); err != nil {
			return err
		}
		fmt.Printf("Created %s (%s)\n", email, u.Role)
	case "merge":
		// pixelhop user merge OLD_EMAIL NEW_EMAIL — move OLD's images into NEW.
		if len(args) < 3 {
			return errors.New("usage: pixelhop user merge OLD_EMAIL NEW_EMAIL")
		}
		from, err := d.UserByEmail(ctx, email)
		if err != nil {
			return fmt.Errorf("no account with email %s", email)
		}
		into, err := d.UserByEmail(ctx, strings.TrimSpace(args[2]))
		if err != nil {
			return fmt.Errorf("no account with email %s (sign in once first)", args[2])
		}
		if from.ID == into.ID {
			return errors.New("that is the same account")
		}
		if from.Role == "admin" {
			into.Role = "admin"
			if err := d.UpdateUser(ctx, into); err != nil {
				return err
			}
		}
		if err := d.MergeUsers(ctx, from.ID, into.ID); err != nil {
			return err
		}
		fmt.Printf("Moved everything from %s into %s and removed the old account.\n", email, into.Email)
	case "passwd", "admin":
		u, err := d.UserByEmail(ctx, email)
		if err != nil {
			return fmt.Errorf("no account with email %s", email)
		}
		if sub == "admin" {
			u.Role = "admin"
		} else {
			pw, err := askPassword()
			if err != nil {
				return err
			}
			u.PassHash = app.HashPassword(pw)
			d.DeleteUserSessions(ctx, u.ID)
		}
		if err := d.UpdateUser(ctx, u); err != nil {
			return err
		}
		fmt.Println("Done.")
	default:
		return fmt.Errorf("unknown user command %q", sub)
	}
	return nil
}

func askPassword() (string, error) {
	if pw := os.Getenv("PIXELHOP_PASSWORD"); pw != "" {
		return pw, nil
	}
	fmt.Print("Password (8+ characters): ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	pw := strings.TrimRight(line, "\r\n")
	if err != nil && pw == "" {
		return "", err
	}
	if len(pw) < 8 {
		return "", errors.New("password must be at least 8 characters")
	}
	return pw, nil
}
