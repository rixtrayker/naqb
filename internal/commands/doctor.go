package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/internal/db"
	"github.com/amr/naqb/internal/keycheck"
)

// DoctorCmd returns the `nqb doctor` command.
func DoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "doctor",
		Aliases: []string{"doc"},
		Short:   "Check system health: API keys, dependencies, book, and database",
		Long: `Run a comprehensive health check of the nqb setup.

Validates API keys, required system dependencies (pandoc, git, XeLaTeX),
the current book project (if any), and the SQLite database.

Each check shows PASS, WARN, or FAIL status.`,
		Example: `  nqb doctor
  nqb doc`,
		GroupID: "utility",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctor()
		},
	}
}

type checkResult struct {
	name   string
	status string // PASS, WARN, FAIL
	detail string
}

func runDoctor() error {
	fmt.Print("\nnqb doctor — system health check\n\n")

	var results []checkResult

	// ── API Keys ────────────────────────────────────────────────────────────
	fmt.Println("  API Keys")
	fmt.Println("  " + repeat("─", 58))
	statuses := keycheck.ResolveAll()
	setCount := 0
	for _, k := range statuses {
		s := keycheck.Resolve(k)
		if s.Set {
			setCount++
			results = append(results, checkResult{s.Name, "PASS", fmt.Sprintf("set via %s", s.Source)})
			fmt.Printf("  %-26s  %s  set via %s\n", s.Name, passLabel(), s.Source)
		} else {
			results = append(results, checkResult{s.Name, "WARN", "not set"})
			fmt.Printf("  %-26s  %s  not set\n", s.Name, warnLabel())
		}
	}
	if setCount == 0 {
		fmt.Printf("\n  %s  No API keys configured. Run 'nqb setup' to get started.\n", failLabel())
	}

	// ── Dependencies ────────────────────────────────────────────────────────
	fmt.Println("\n  Dependencies")
	fmt.Println("  " + repeat("─", 58))

	deps := []struct {
		name     string
		commands []string // try each in order
		required bool
	}{
		{"git", []string{"git", "--version"}, true},
		{"pandoc", []string{"pandoc", "--version"}, false},
		{"xelatex", []string{"xelatex", "--version"}, false},
	}

	for _, dep := range deps {
		cmd := exec.Command(dep.commands[0], dep.commands[1:]...)
		out, err := cmd.Output()
		if err != nil {
			if dep.required {
				results = append(results, checkResult{dep.name, "FAIL", "not found"})
				fmt.Printf("  %-26s  %s  not found\n", dep.name, failLabel())
			} else {
				results = append(results, checkResult{dep.name, "WARN", "not found (optional)"})
				fmt.Printf("  %-26s  %s  not found (optional for export)\n", dep.name, warnLabel())
			}
		} else {
			ver := firstLine(string(out))
			results = append(results, checkResult{dep.name, "PASS", ver})
			fmt.Printf("  %-26s  %s  %s\n", dep.name, passLabel(), ver)
		}
	}

	// ── Current Book ────────────────────────────────────────────────────────
	fmt.Println("\n  Current Book")
	fmt.Println("  " + repeat("─", 58))

	bookDir, err := config.FindBookRoot()
	if err != nil {
		results = append(results, checkResult{"book.yaml", "WARN", "no book project found in cwd"})
		fmt.Printf("  %-26s  %s  no book project found in current directory\n", "book.yaml", warnLabel())
	} else {
		cfg, err := config.LoadBook(bookDir)
		if err != nil {
			results = append(results, checkResult{"book.yaml", "FAIL", err.Error()})
			fmt.Printf("  %-26s  %s  %v\n", "book.yaml", failLabel(), err)
		} else {
			results = append(results, checkResult{"book.yaml", "PASS", cfg.Title})
			fmt.Printf("  %-26s  %s  \"%s\" (%d chapters)\n", "book.yaml", passLabel(), cfg.Title, len(cfg.Chapters))

			// Check outline.md
			outlinePath := filepath.Join(bookDir, "outline.md")
			if _, err := os.Stat(outlinePath); err == nil {
				results = append(results, checkResult{"outline.md", "PASS", "exists"})
				fmt.Printf("  %-26s  %s  exists\n", "outline.md", passLabel())
			} else {
				results = append(results, checkResult{"outline.md", "WARN", "missing"})
				fmt.Printf("  %-26s  %s  missing\n", "outline.md", warnLabel())
			}

			// Check chapter files
			written, missing := 0, 0
			for _, ch := range cfg.Chapters {
				chapPath := filepath.Join(bookDir, "chapters", ch.File)
				if _, err := os.Stat(chapPath); err == nil {
					written++
				} else {
					missing++
				}
			}
			if missing == 0 {
				results = append(results, checkResult{"chapters", "PASS", fmt.Sprintf("%d/%d written", written, len(cfg.Chapters))})
				fmt.Printf("  %-26s  %s  %d/%d written\n", "chapters", passLabel(), written, len(cfg.Chapters))
			} else {
				results = append(results, checkResult{"chapters", "WARN", fmt.Sprintf("%d/%d written, %d missing", written, len(cfg.Chapters), missing)})
				fmt.Printf("  %-26s  %s  %d/%d written (%d missing)\n", "chapters", warnLabel(), written, len(cfg.Chapters), missing)
			}
		}
	}

	// ── Database ────────────────────────────────────────────────────────────
	fmt.Println("\n  Database")
	fmt.Println("  " + repeat("─", 58))

	dbPath, err := db.DefaultPath()
	if err != nil {
		results = append(results, checkResult{"sqlite", "FAIL", err.Error()})
		fmt.Printf("  %-26s  %s  %v\n", "sqlite", failLabel(), err)
	} else {
		sqlDB, err := db.Open(dbPath)
		if err != nil {
			results = append(results, checkResult{"sqlite", "FAIL", err.Error()})
			fmt.Printf("  %-26s  %s  %v\n", "sqlite", failLabel(), err)
		} else {
			_ = sqlDB.Close()
			results = append(results, checkResult{"sqlite", "PASS", dbPath})
			fmt.Printf("  %-26s  %s  %s\n", "sqlite", passLabel(), dbPath)
		}
	}

	// ── Summary ─────────────────────────────────────────────────────────────
	pass, warn, fail := 0, 0, 0
	for _, r := range results {
		switch r.status {
		case "PASS":
			pass++
		case "WARN":
			warn++
		case "FAIL":
			fail++
		}
	}

	fmt.Printf("\n  Summary: %d passed, %d warnings, %d failed\n\n", pass, warn, fail)
	if fail > 0 {
		fmt.Println("  Run 'nqb setup' to fix configuration issues.")
		fmt.Println("  Run 'nqb keys --set <NAME>' to add missing API keys.")
	}
	fmt.Println()
	return nil
}

func passLabel() string { return "PASS" }
func warnLabel() string { return "WARN" }
func failLabel() string { return "FAIL" }

func firstLine(s string) string {
	for i, c := range s {
		if c == '\n' {
			return s[:i]
		}
	}
	return s
}
