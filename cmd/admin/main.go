// Command admin creates a user and initial organization without public registration.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
	"io"
	"os"
	"regexp"
	"strings"
	"time"
	"xingdu.app/xingdu/internal/config"
	"xingdu.app/xingdu/internal/storage"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	username := flag.String("username", "admin", "username")
	flag.Parse()
	if !regexp.MustCompile(`^[a-zA-Z0-9_-]{3,32}$`).MatchString(*username) {
		return errors.New("username must contain 3–32 letters, digits, underscores or hyphens")
	}
	var password []byte
	var err error
	if term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprint(os.Stderr, "Account password (12–72 bytes): ")
		password, err = term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
	} else {
		password, err = io.ReadAll(io.LimitReader(os.Stdin, 74))
		password = []byte(strings.TrimSuffix(strings.TrimSuffix(string(password), "\n"), "\r"))
	}
	if err != nil {
		return errors.New("could not read password")
	}
	if len(password) < 12 || len(password) > 72 {
		return errors.New("password must contain 12–72 bytes")
	}
	hash, err := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
	clear(password)
	if err != nil {
		return errors.New("could not hash password")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	store, err := storage.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer store.Close()
	if err := store.CreateAdmin(ctx, *username, string(hash)); err != nil {
		if errors.Is(err, storage.ErrConflict) {
			return errors.New("username already exists; no changes were made")
		}
		return errors.New("could not create administrator; check database and migrations")
	}
	fmt.Println("Account and organization created. Sign in through the console.")
	return nil
}
