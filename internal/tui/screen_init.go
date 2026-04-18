package tui

import (
	"fmt"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/amr/naqb/pkg/agents"
	"github.com/amr/naqb/internal/tui/components"
)

// InitFormResult holds the answers from the init interview form.
type InitFormResult struct {
	Answers agents.InterviewAnswers
	Done    bool
	Err     error
}

// RunInitForm runs the Bubble Tea init form and returns the answers.
func RunInitForm() (*InitFormResult, error) {
	steps := []components.WizardStep{
		{
			Label:       "Template (1=Arabic Research  2=CS Book  3=General)",
			Placeholder: "3",
			Default:     "3",
			Validate: func(v string) error {
				if v != "1" && v != "2" && v != "3" {
					return fmt.Errorf("choose 1, 2, or 3")
				}
				return nil
			},
		},
		{
			Label:       "Book title",
			Placeholder: "",
			Validate: func(v string) error {
				if v == "" {
					return fmt.Errorf("title cannot be empty")
				}
				return nil
			},
		},
		{
			Label:       "Author name",
			Placeholder: "",
		},
		{
			Label:       "Language (ar/en)",
			Placeholder: "ar",
			Default:     "ar",
			Validate: func(v string) error {
				if v != "ar" && v != "en" {
					return fmt.Errorf("language must be 'ar' or 'en'")
				}
				return nil
			},
		},
		{
			Label:       "Domain/subject (e.g. 'Arabic history', 'Computer Science')",
			Placeholder: "",
		},
		{
			Label:       "One-sentence synopsis",
			Placeholder: "",
		},
		{
			Label:       "Number of chapters",
			Placeholder: "8",
			Default:     "8",
			Validate: func(v string) error {
				n, err := strconv.Atoi(v)
				if err != nil || n < 1 || n > 50 {
					return fmt.Errorf("number of chapters must be between 1 and 50")
				}
				return nil
			},
		},
	}

	wiz := components.NewWizard("نقب  New Book", steps)
	wiz.Hint = "Templates:\n" +
		"  1 — Arabic Research (كتاب بحثي) — RTL, Amiri font, scholarly\n" +
		"  2 — CS / Technical Book   — Code blocks, English, precise\n" +
		"  3 — General Book          — Flexible, fill in yourself"

	p := tea.NewProgram(wiz)
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}
	fm, ok := finalModel.(*components.WizardModel)
	if !ok {
		return &InitFormResult{Err: fmt.Errorf("form exited unexpectedly")}, nil
	}
	if fm.Current < len(fm.Steps) {
		return &InitFormResult{Err: fmt.Errorf("cancelled")}, nil
	}

	vals := fm.Collect()
	n, _ := strconv.Atoi(vals[6])
	if n == 0 {
		n = 8
	}

	return &InitFormResult{
		Answers: agents.InterviewAnswers{
			Template:    templateIDFromChoice(vals[0]),
			Title:       vals[1],
			Author:      vals[2],
			Language:    vals[3],
			Domain:      vals[4],
			Synopsis:    vals[5],
			NumChapters: n,
		},
		Done: true,
	}, nil
}

func templateIDFromChoice(choice string) string {
	switch choice {
	case "1":
		return "arabic-research"
	case "2":
		return "cs-book"
	default:
		return "general"
	}
}
