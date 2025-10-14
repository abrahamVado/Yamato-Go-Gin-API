package tests

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestADR004JobsContainsCoreSections(t *testing.T) {
	//1.- Discover the repository root by using the location of this test file as an anchor point.
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("unable to determine caller information")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), ".."))

	//2.- Read the ADR file contents to ensure the documentation is accessible to contributors and tooling.
	docPath := filepath.Join(repoRoot, "docs", "adrs", "adr-004-jobs", "README.md")
	data, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("failed to read ADR document: %v", err)
	}

	//3.- Define the list of critical sections that the document must include for architectural clarity.
	requiredSections := []string{
		"## Queue Backend",
		"## Worker Binary Responsibilities",
		"## Scheduler Triggers",
		"## Retry Policies",
	}

	//4.- Verify each required section exists so that the ADR remains actionable and complete.
	content := string(data)
	for _, section := range requiredSections {
		if !strings.Contains(content, section) {
			t.Fatalf("missing required section %s in ADR 004 document", section)
		}
	}
}
