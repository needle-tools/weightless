package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"weightloss/internal/branding"
	"weightloss/internal/scan"
	"weightloss/internal/tui"
)

func main() {
	var (
		jsonMode   = flag.Bool("json", false, "print results as JSON instead of launching the TUI")
		version    = flag.Bool("version", false, "print version and exit")
		minSizeMB  = flag.Int64("min-size-mb", 32, "minimum file size in megabytes for generic detection")
		extraRoots = flag.String("roots", "", "comma separated extra roots to scan")
		providers  = flag.String("providers", "", "comma separated provider ids to scan, e.g. ollama,lm-studio,huggingface")
	)
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), strings.TrimLeft(branding.Banner, "\n"))
		fmt.Fprintf(flag.CommandLine.Output(), "weightloss %s\n", Version)
		fmt.Fprintf(flag.CommandLine.Output(), "Find local model weights across AI apps, shared caches, and project folders.\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Usage:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  weightloss           launch the TUI\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  weightloss --json    print machine-readable JSON\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  weightloss --version show version\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Options:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *version {
		fmt.Printf("weightloss %s\n", Version)
		return
	}

	cfg := scan.Config{
		MinSizeBytes:      *minSizeMB * 1024 * 1024,
		AdditionalRoots:   splitCSV(*extraRoots),
		Providers:         splitCSV(*providers),
		IncludeHiddenDirs: true,
	}

	if *jsonMode {
		report, err := scan.Run(cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "scan failed: %v\n", err)
			os.Exit(1)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(os.Stderr, "encode failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	report, err := scan.Run(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scan failed: %v\n", err)
		os.Exit(1)
	}

	model := tui.New(report, func() (scan.Report, error) {
		return scan.Run(cfg)
	})
	if _, err := tea.NewProgram(model, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "tui failed: %v\n", err)
		os.Exit(1)
	}
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
