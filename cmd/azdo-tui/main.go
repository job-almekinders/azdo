package main

import (
	"errors"
	"fmt"
	"os"

	"strings"

	"github.com/Elpulgo/azdo/internal/app"
	"github.com/Elpulgo/azdo/internal/azdevops"
	"github.com/Elpulgo/azdo/internal/cli"
	"github.com/Elpulgo/azdo/internal/config"
	"github.com/Elpulgo/azdo/internal/demo"
	"github.com/Elpulgo/azdo/internal/ui/components"
	"github.com/Elpulgo/azdo/internal/ui/patinput"
	"github.com/Elpulgo/azdo/internal/ui/setupwizard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Build-time variables injected via ldflags by goreleaser.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	action := cli.ParseArgs(args)

	switch action {
	case cli.ActionHelp:
		return runHelp()
	case cli.ActionVersion:
		return runVersion()
	case cli.ActionAuth:
		return runAuth()
	case cli.ActionDemo:
		return demo.Run(version, commit)
	default:
		return runTUI()
	}
}

func runHelp() error {
	configPath, _ := config.GetPath()
	if configPath == "" {
		configPath = "~/.config/azdo-tui/config.yaml"
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("99"))
	fmt.Println(titleStyle.Render(strings.Join(components.LogoArt, "\n")))

	fmt.Printf(`azdo - A TUI for Azure DevOps (%s)

Usage:
  azdo              Start the TUI application
  azdo auth         Set or update your Personal Access Token (PAT)
  azdo demo         Launch with mock data (for screenshots/demos)
  azdo --help       Show this help message
  azdo --version    Show version information

Configuration:
  Config file: %s
  PAT storage: System keyring (service: azdo-tui)
  PAT fallback: AZDO_PAT environment variable
  Auth methods:
    pat    (default) Personal Access Token — stored in system keyring
    az-cli           Uses existing 'az login' session, no PAT needed
                     Set 'auth_method: az-cli' in config.yaml

Required PAT permissions:
  Build        (Read)         - pipelines, build logs
  Code         (Read & Write) - pull requests, voting, comments
  Work Items   (Read & Write) - queries, state changes

Keyboard shortcuts (in TUI):
  Navigation:
    ↑/k          Move up
    ↓/j          Move down
    pgup/pgdn    Page up / down
    enter        View details / expand
    esc          Go back

  Tabs:
    1/2/3        Switch tabs (Pull Requests, Work Items, Pipelines)
    ←/→          Previous / next tab

  Actions:
    f            Search / filter
    m            Toggle my items (PRs / work items)
    A            Toggle as reviewer (PRs)
    T            Filter by tag (work items)
    r            Refresh data
    v            Vote on PR (detail view)
    s            Change work item state (detail view)
    o            Open in browser (PR / work item detail)
    t            Select theme
    ?            Toggle help overlay
    q            Quit

  Code Review (PR diff):
    c            Create new comment
    p            Reply to nearest thread
    x            Resolve nearest thread
    n / N        Jump to next / previous comment

  Log Viewer (pipelines):
    g            Go to top
    G            Go to bottom

For more information, visit: https://github.com/Elpulgo/azdo
`, version, configPath)

	return nil
}

func runVersion() error {
	fmt.Printf("azdo version %s (commit: %s, built: %s)\n", version, commit, date)
	return nil
}

func runAuth() error {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("99"))
	fmt.Println(titleStyle.Render(strings.Join(components.LogoArt, "\n")))
	fmt.Println()

	method, err := selectAuthMethod(os.Stdin, os.Stdout)
	if err != nil {
		return fmt.Errorf("auth setup cancelled: %w", err)
	}

	configPath, _ := config.GetPath()

	switch method {
	case "az-cli":
		fmt.Println("\nConfiguring Azure CLI authentication.")
		fmt.Println("Make sure you are logged in with: az login")
		updated, err := updateConfigAuthMethod(configPath, "az-cli")
		if err != nil {
			return fmt.Errorf("failed to update config: %w", err)
		}
		if updated {
			fmt.Println("\nConfig updated: auth_method set to az-cli.")
		} else {
			fmt.Println("\nNo config file found — add 'auth_method: az-cli' to your config.yaml when you set it up.")
		}

	default: // "pat"
		store := config.NewKeyringStore()
		_, err := store.GetPAT()
		isUpdate := err == nil

		if isUpdate {
			fmt.Println("Azure DevOps PAT Update")
			fmt.Println("This will replace your existing Personal Access Token in the system keyring.")
		} else {
			fmt.Println("Azure DevOps PAT Setup")
			fmt.Println("This will store your Personal Access Token in the system keyring.")
		}
		fmt.Println()
		fmt.Println(patinput.PermissionInfoPlain())
		fmt.Println()

		pat, err := promptForPATWithMode(store, isUpdate)
		if err != nil {
			return fmt.Errorf("failed to set PAT: %w", err)
		}
		if pat != "" {
			fmt.Println("\nPAT saved successfully to system keyring.")
		}

		if _, err := updateConfigAuthMethod(configPath, "pat"); err != nil {
			fmt.Printf("Warning: could not update config auth_method: %v\n", err)
		}
	}

	return nil
}

func runTUI() error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		if errors.Is(err, config.ErrConfigNotFound) {
			cfg, err = runSetupWizard()
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	// Build token provider based on configured auth method
	var provider azdevops.TokenProvider
	if cfg.IsAzCLIAuth() {
		provider = azdevops.NewAzCLITokenProvider()
	} else {
		store := config.NewKeyringStore()
		pat, err := store.GetPAT()
		if err != nil {
			if errors.Is(err, config.ErrNotFound) {
				pat, err = promptForPAT(store)
				if err != nil {
					return fmt.Errorf("failed to set PAT: %w", err)
				}
			} else {
				return fmt.Errorf("failed to get PAT: %w", err)
			}
		}
		provider, err = azdevops.NewPATTokenProvider(pat)
		if err != nil {
			return fmt.Errorf("failed to create PAT provider: %w", err)
		}
	}

	// Create multi-project Azure DevOps client
	client, err := azdevops.NewMultiClientWithProvider(cfg.Organization, cfg.Projects, provider, cfg.DisplayNames)
	if err != nil {
		return fmt.Errorf("failed to create Azure DevOps client: %w", err)
	}

	// Create and run the TUI application
	model := app.NewModel(client, cfg, version, commit)
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI application error: %w", err)
	}

	return nil
}

// runSetupWizard launches the interactive setup wizard and saves the config.
func runSetupWizard() (*config.Config, error) {
	model := setupwizard.NewModel()
	p := tea.NewProgram(model)

	m, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("setup wizard error: %w", err)
	}

	finalModel, ok := m.(setupwizard.Model)
	if !ok {
		return nil, fmt.Errorf("unexpected model type from setup wizard")
	}

	if finalModel.Cancelled() {
		return nil, fmt.Errorf("setup cancelled")
	}

	cfg := finalModel.GetConfig()
	if cfg == nil {
		return nil, fmt.Errorf("setup wizard did not produce a configuration")
	}

	if err := cfg.Save(); err != nil {
		return nil, fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println("\nConfiguration saved successfully!")
	return cfg, nil
}

// promptForPAT displays a TUI to prompt the user for their PAT (first-time setup)
func promptForPAT(store *config.KeyringStore) (string, error) {
	return promptForPATWithMode(store, false)
}

// promptForPATWithMode displays a TUI to prompt the user for their PAT.
// If isUpdate is true, shows an "update" message instead of "first-time setup".
func promptForPATWithMode(store *config.KeyringStore, isUpdate bool) (string, error) {
	var model patinput.Model
	if isUpdate {
		model = patinput.NewModelForUpdate()
	} else {
		model = patinput.NewModel()
	}
	p := tea.NewProgram(model)

	m, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("failed to run PAT input: %w", err)
	}

	// Extract the final model
	finalModel, ok := m.(patinput.Model)
	if !ok {
		return "", fmt.Errorf("unexpected model type")
	}

	pat := finalModel.GetPAT()
	if pat == "" {
		return "", fmt.Errorf("PAT input cancelled or empty")
	}

	// Save PAT to keyring
	if err := store.SetPAT(pat); err != nil {
		return "", fmt.Errorf("failed to save PAT to keyring: %w", err)
	}

	return pat, nil
}
