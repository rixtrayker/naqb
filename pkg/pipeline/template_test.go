package pipeline

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTemplate_BuiltinStandard(t *testing.T) {
	tmpl, err := LoadTemplate("standard")
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	if tmpl.Name != "standard" {
		t.Errorf("Name = %q, want standard", tmpl.Name)
	}
	if len(tmpl.Stages) != 3 {
		t.Fatalf("expected 3 stages, got %d", len(tmpl.Stages))
	}

	dag, err := tmpl.ToDAG()
	if err != nil {
		t.Fatalf("ToDAG: %v", err)
	}
	if len(dag.Stages()) != 3 {
		t.Errorf("DAG stages = %d, want 3", len(dag.Stages()))
	}
}

func TestLoadTemplate_BuiltinThorough(t *testing.T) {
	tmpl, err := LoadTemplate("thorough")
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	if len(tmpl.Stages) != 5 {
		t.Fatalf("expected 5 stages, got %d", len(tmpl.Stages))
	}
}

func TestLoadTemplate_BuiltinQAOnly(t *testing.T) {
	tmpl, err := LoadTemplate("qa-only")
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	if len(tmpl.Stages) != 3 {
		t.Fatalf("expected 3 stages, got %d", len(tmpl.Stages))
	}
}

func TestLoadTemplate_Missing(t *testing.T) {
	_, err := LoadTemplate("nonexistent-template")
	if err == nil {
		t.Fatal("expected error for missing template")
	}
}

func TestLoadTemplate_FromFile(t *testing.T) {
	dir := t.TempDir()
	tmplDir := filepath.Join(dir, ".naqb", "templates")
	if err := os.MkdirAll(tmplDir, 0o750); err != nil {
		t.Fatal(err)
	}

	yaml := `name: custom-test
stages:
  - id: context
    type: CONTEXT
  - id: write
    type: WRITE
    depends_on: [context]
`
	if err := os.WriteFile(filepath.Join(tmplDir, "custom.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	tmpl, err := LoadTemplate("custom", dir)
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	if tmpl.Name != "custom-test" {
		t.Errorf("Name = %q, want custom-test", tmpl.Name)
	}
	if len(tmpl.Stages) != 2 {
		t.Fatalf("expected 2 stages, got %d", len(tmpl.Stages))
	}
}

func TestParseTemplate_InvalidYAML(t *testing.T) {
	_, err := parseTemplate("bad", []byte("not: valid: yaml: ["))
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestParseTemplate_InvalidDAG(t *testing.T) {
	// Cycle in stages
	yaml := `name: cyclic
stages:
  - id: a
    type: CONTEXT
    depends_on: [b]
  - id: b
    type: WRITE
    depends_on: [a]
`
	_, err := parseTemplate("cyclic", []byte(yaml))
	if err == nil {
		t.Fatal("expected error for cyclic DAG")
	}
}

func TestParseTemplate_EmptyName(t *testing.T) {
	yaml := `stages:
  - id: a
    type: CONTEXT
`
	tmpl, err := parseTemplate("fallback", []byte(yaml))
	if err != nil {
		t.Fatalf("parseTemplate: %v", err)
	}
	if tmpl.Name != "fallback" {
		t.Errorf("Name = %q, want fallback", tmpl.Name)
	}
}
