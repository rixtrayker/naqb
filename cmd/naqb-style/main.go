// naqb-style is the style engine CLI for نقب.
// It provides commands for extracting, applying, blending, and managing
// StyleImage profiles.
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/pkg/llm"
	"github.com/amr/naqb/internal/style"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "naqb-style",
	Short: "Style engine for نقب — extract, apply, blend, and manage style profiles",
	Long:  "naqb-style manages StyleImage profiles: linguistic, structural, rhetorical, and Arabic voice patterns.",
}

func init() {
	rootCmd.AddCommand(
		extractCmd,
		applyCmd,
		blendCmd,
		diffCmd,
		listCmd,
		showCmd,
		forkCmd,
		fingerprintCmd,
		deleteCmd,
	)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func openRegistry() (*style.Registry, error) {
	dir := style.DefaultStylesDir()
	return style.NewRegistry(dir)
}

func openClient() llm.Provider {
	key, err := config.APIKey()
	if err != nil {
		return nil
	}
	pcfg, _ := config.ProviderConfigFor("")
	client, err := llm.NewProvider(pcfg)
	if err != nil {
		_ = key
		return nil
	}
	return client
}

// ── Commands ──────────────────────────────────────────────────────────────────

var (
	extractOutput string
	extractAuthor string
)

var extractCmd = &cobra.Command{
	Use:   "extract <file...>",
	Short: "Extract a style profile from text files",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		texts := make([]string, 0, len(args))
		for _, path := range args {
			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read %s: %w", path, err)
			}
			texts = append(texts, string(data))
		}

		id := extractOutput
		if id == "" {
			id = "extracted"
		}
		name := id
		if extractAuthor != "" {
			name = extractAuthor + " style"
		}

		client := openClient()
		img, err := style.Extract(context.Background(), texts, client, id, name, extractAuthor)
		if err != nil {
			return err
		}

		reg, err := openRegistry()
		if err != nil {
			return err
		}
		if err := reg.Save(img); err != nil {
			return err
		}
		fmt.Printf("Saved style %q (fingerprint: %s)\n", img.ID, style.Fingerprint(img))
		return nil
	},
}

func init() {
	extractCmd.Flags().StringVarP(&extractOutput, "output", "o", "", "Style ID/name to save as")
	extractCmd.Flags().StringVarP(&extractAuthor, "author", "a", "", "Author name for the style profile")
}

var (
	applyChapter int
	applyMode    string
)

var applyCmd = &cobra.Command{
	Use:   "apply <style-id> --to <chapter-file>",
	Short: "Apply a style to a chapter file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		reg, err := openRegistry()
		if err != nil {
			return err
		}
		img, err := reg.Get(args[0])
		if err != nil {
			return err
		}

		chapterFile, _ := cmd.Flags().GetString("to")
		if chapterFile == "" {
			return fmt.Errorf("--to <chapter-file> is required")
		}

		data, err := os.ReadFile(chapterFile)
		if err != nil {
			return fmt.Errorf("read chapter: %w", err)
		}

		mode := style.PromptMode
		if applyMode == "postprocess" {
			mode = style.PostprocessMode
		}

		result, err := style.Apply(context.Background(), img, string(data), mode, openClient())
		if err != nil && mode == style.PostprocessMode {
			return err
		}

		if mode == style.PromptMode {
			fmt.Println(result)
		} else {
			if err := os.WriteFile(chapterFile, []byte(result), 0o644); err != nil {
				return fmt.Errorf("write chapter: %w", err)
			}
			fmt.Printf("Applied style %q to %s\n", img.ID, chapterFile)
		}
		return nil
	},
}

func init() {
	applyCmd.Flags().String("to", "", "Chapter file to apply style to")
	applyCmd.Flags().StringVarP(&applyMode, "mode", "m", "prompt", "Apply mode: prompt or postprocess")
}

var blendCmd = &cobra.Command{
	Use:   "blend <style-a> <style-b>",
	Short: "Blend two styles together",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		weight, _ := cmd.Flags().GetFloat64("weight")
		reg, err := openRegistry()
		if err != nil {
			return err
		}
		a, err := reg.Get(args[0])
		if err != nil {
			return fmt.Errorf("load style A: %w", err)
		}
		b, err := reg.Get(args[1])
		if err != nil {
			return fmt.Errorf("load style B: %w", err)
		}
		result := style.Blend(a, b, weight)
		if err := reg.Save(result); err != nil {
			return err
		}
		fmt.Printf("Created blend: %q (fingerprint: %s)\n", result.ID, style.Fingerprint(result))
		return nil
	},
}

func init() {
	blendCmd.Flags().Float64("weight", 0.5, "Blend weight: 0.0=pure A, 1.0=pure B")
}

var diffCmd = &cobra.Command{
	Use:   "diff <style-a> <style-b>",
	Short: "Show differences between two styles",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		reg, err := openRegistry()
		if err != nil {
			return err
		}
		a, err := reg.Get(args[0])
		if err != nil {
			return fmt.Errorf("load style A: %w", err)
		}
		b, err := reg.Get(args[1])
		if err != nil {
			return fmt.Errorf("load style B: %w", err)
		}
		diff := style.Diff(a, b)
		if len(diff.Changes) == 0 {
			fmt.Printf("Styles %q and %q are identical.\n", a.Name, b.Name)
			return nil
		}
		fmt.Printf("Differences between %q and %q:\n\n", a.Name, b.Name)
		for field, vals := range diff.Changes {
			fmt.Printf("  %-40s  A: %-20s  B: %s\n", field, vals[0], vals[1])
		}
		return nil
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available style profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		reg, err := openRegistry()
		if err != nil {
			return err
		}
		imgs, err := reg.List()
		if err != nil {
			return err
		}
		if len(imgs) == 0 {
			fmt.Println("No style profiles found. Use 'naqb-style extract' to create one.")
			return nil
		}
		fmt.Printf("%-20s %-30s %s\n", "ID", "NAME", "FINGERPRINT")
		for _, img := range imgs {
			fmt.Printf("%-20s %-30s %s\n", img.ID, img.Name, style.Fingerprint(&img))
		}
		return nil
	},
}

var showCmd = &cobra.Command{
	Use:   "show <style-id>",
	Short: "Show details of a style profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		reg, err := openRegistry()
		if err != nil {
			return err
		}
		img, err := reg.Get(args[0])
		if err != nil {
			return err
		}
		fmt.Printf("Style: %s (%s)\n", img.Name, img.ID)
		if img.Author != "" {
			fmt.Printf("Author: %s\n", img.Author)
		}
		fmt.Printf("Fingerprint: %s\n\n", style.Fingerprint(img))
		fmt.Printf("Linguistic:\n")
		fmt.Printf("  Avg sentence length: %.1f words\n", img.Linguistic.AvgSentenceLength)
		fmt.Printf("  Vocabulary richness: %.3f\n", img.Linguistic.VocabularyRichness)
		fmt.Printf("  Punctuation density: %.2f/100w\n", img.Linguistic.PunctuationDensity)
		fmt.Printf("\nStructural:\n")
		fmt.Printf("  Avg paragraph length: %.1f sentences\n", img.Structural.AvgParagraphLength)
		fmt.Printf("  Section density: %.2f/1000w\n", img.Structural.SectionDensity)
		fmt.Printf("\nRhetorical:\n")
		fmt.Printf("  Argumentation: %s\n", orNA(img.Rhetorical.ArgumentationMode))
		fmt.Printf("  Assertion strength: %s\n", orNA(img.Rhetorical.AssertionStrength))
		fmt.Printf("\nVoice:\n")
		fmt.Printf("  Register: %s\n", orNA(img.Voice.Register))
		fmt.Printf("  Persona: %s\n", orNA(img.Voice.Persona))
		fmt.Printf("  Formality: %.2f\n", img.Voice.Formality)
		if img.Arabic.DiacriticalPolicy != "" {
			fmt.Printf("\nArabic:\n")
			fmt.Printf("  Diacritical policy: %s\n", img.Arabic.DiacriticalPolicy)
			fmt.Printf("  Register boundary: %s\n", orNA(img.Arabic.RegisterBoundary))
		}
		return nil
	},
}

var forkCmd = &cobra.Command{
	Use:   "fork <style-id>",
	Short: "Create a copy of a style for modification",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		newName, _ := cmd.Flags().GetString("as")
		reg, err := openRegistry()
		if err != nil {
			return err
		}
		img, err := reg.Get(args[0])
		if err != nil {
			return err
		}
		forked := style.Fork(img)
		if newName != "" {
			forked.ID = newName
			forked.Name = newName
		}
		if err := reg.Save(forked); err != nil {
			return err
		}
		fmt.Printf("Created fork: %q\n", forked.ID)
		return nil
	},
}

func init() {
	forkCmd.Flags().String("as", "", "New style ID/name for the fork")
}

var fingerprintCmd = &cobra.Command{
	Use:   "fingerprint <style-id>",
	Short: "Show the deterministic fingerprint of a style",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		reg, err := openRegistry()
		if err != nil {
			return err
		}
		img, err := reg.Get(args[0])
		if err != nil {
			return err
		}
		fmt.Println(style.Fingerprint(img))
		return nil
	},
}

var deleteCmd = &cobra.Command{
	Use:   "delete <style-id>",
	Short: "Delete a style profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		reg, err := openRegistry()
		if err != nil {
			return err
		}
		if err := reg.Delete(args[0]); err != nil {
			return err
		}
		fmt.Printf("Deleted style %q\n", args[0])
		return nil
	},
}

func orNA(s string) string {
	if s == "" {
		return "N/A"
	}
	return s
}

// Ensure strconv is used (for potential future numeric formatting).
var _ = strconv.Itoa
