package tui

import (
	"strings"
	"testing"

	"weightless/internal/scan"
)

func TestSummaryTypeLabelsArePlainTableData(t *testing.T) {
	report := scan.Report{
		Summary: []scan.ProviderSummary{
			{Provider: "huggingface", Artifacts: 1, BytesHuman: "1.0 GiB"},
			{Provider: "docker", Artifacts: 1, BytesHuman: "2.0 GiB"},
			{Provider: "codex", Artifacts: 1, BytesHuman: "3.0 GiB"},
		},
		Artifacts: []scan.Artifact{
			{Category: categoryModels, PrimaryProvider: "huggingface", Path: "/models/a"},
			{Category: categoryVirtualMachines, PrimaryProvider: "docker", Path: "/vm/a"},
			{Category: categoryLLMSessions, PrimaryProvider: "codex", Path: "/sessions/a"},
		},
	}

	tables, _ := buildTables(report, report, 120, "", "", false, scan.ProviderSummary{}, "")
	rows := tables[tabSummary].Rows()
	for _, row := range rows[:3] {
		if strings.Contains(row[0], "\x1b") {
			t.Fatalf("type label should not contain ANSI escapes: %q", row[0])
		}
	}
	if rows[0][0] != "MODEL" || rows[1][0] != "VM" || rows[2][0] != "SESSION" {
		t.Fatalf("unexpected type labels: %#v", []string{rows[0][0], rows[1][0], rows[2][0]})
	}
}
