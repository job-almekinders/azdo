package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/Elpulgo/azdo/internal/config"
)

// selectAuthMethod writes a numbered menu to w, reads one line from r,
// and returns "pat" or "az-cli".
func selectAuthMethod(r io.Reader, w io.Writer) (string, error) {
	fmt.Fprintln(w, "Choose authentication method:")
	fmt.Fprintln(w, "  1) Personal Access Token (PAT)")
	fmt.Fprintln(w, "  2) Azure CLI  (az login)")
	fmt.Fprint(w, "Enter choice [1/2]: ")

	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		return "", fmt.Errorf("no input provided")
	}
	choice := strings.TrimSpace(scanner.Text())
	switch choice {
	case "1":
		return "pat", nil
	case "2":
		return "az-cli", nil
	default:
		return "", fmt.Errorf("invalid choice %q: enter 1 for PAT or 2 for az-cli", choice)
	}
}

// updateConfigAuthMethod loads the config at configPath, sets auth_method,
// and saves it. Returns (false, nil) if the config file doesn't exist yet.
func updateConfigAuthMethod(configPath, method string) (updated bool, err error) {
	cfg, err := config.LoadFrom(configPath)
	if err != nil {
		if errors.Is(err, config.ErrConfigNotFound) {
			return false, nil
		}
		return false, err
	}
	cfg.AuthMethod = method
	if err := cfg.Save(); err != nil {
		return false, err
	}
	return true, nil
}
