package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tidal.yaml")

	yaml := `harness: v2
name: test-project
lang: go
test:
  unit:
    cmd: "echo ok"
vars:
  repo: "test/repo"
`
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Name != "test-project" {
		t.Errorf("Name = %q, want %q", cfg.Name, "test-project")
	}
	if cfg.Lang != "go" {
		t.Errorf("Lang = %q, want %q", cfg.Lang, "go")
	}
	if len(cfg.Test) != 1 {
		t.Errorf("Test count = %d, want 1", len(cfg.Test))
	}
}

func TestLoadInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	os.WriteFile(path, []byte("{{invalid yaml"), 0644)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid yaml")
	}
}

func TestLoadMissing(t *testing.T) {
	_, err := Load("/nonexistent/tidal.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestExpand(t *testing.T) {
	cfg := &Config{
		Vars: map[string]string{
			"service": "my-app",
			"env":     "staging",
		},
	}

	got := cfg.expand("deploy {{service}} to {{env}}")
	want := "deploy my-app to staging"
	if got != want {
		t.Errorf("expand = %q, want %q", got, want)
	}
}

func TestExpandEnvVar(t *testing.T) {
	cfg := &Config{Vars: map[string]string{}}
	os.Setenv("TIDAL_TEST_KEY", "secret123")
	defer os.Unsetenv("TIDAL_TEST_KEY")

	got := cfg.expand("key=$TIDAL_TEST_KEY")
	want := "key=secret123"
	if got != want {
		t.Errorf("expand env = %q, want %q", got, want)
	}
}

func TestTestTasks(t *testing.T) {
	cfg := &Config{
		Test: map[string]Task{
			"build": {Cmd: "go build"},
			"unit":  {Cmd: "go test"},
			"lint":  {Cmd: "golangci-lint run"},
		},
		Vars: map[string]string{},
	}

	// all
	all := cfg.TestTasks()
	if len(all) != 3 {
		t.Errorf("TestTasks() count = %d, want 3", len(all))
	}

	// filtered
	filtered := cfg.TestTasks("unit")
	if len(filtered) != 1 {
		t.Errorf("TestTasks(unit) count = %d, want 1", len(filtered))
	}
	if filtered[0].Name != "unit" {
		t.Errorf("TestTasks(unit)[0].Name = %q, want %q", filtered[0].Name, "unit")
	}
}

func TestLintTasks(t *testing.T) {
	cfg := &Config{
		Lint: map[string]Task{
			"vet":      {Cmd: "go vet"},
			"golangci": {Cmd: "golangci-lint run"},
		},
		Vars: map[string]string{},
	}

	all := cfg.LintTasks()
	if len(all) != 2 {
		t.Errorf("LintTasks() count = %d, want 2", len(all))
	}

	filtered := cfg.LintTasks("vet")
	if len(filtered) != 1 {
		t.Errorf("LintTasks(vet) count = %d, want 1", len(filtered))
	}
}

func TestReviewTasks(t *testing.T) {
	cfg := &Config{
		Review: map[string]Task{
			"diff":    {Cmd: "git diff"},
			"secrets": {Cmd: "grep secret"},
		},
		Vars: map[string]string{},
	}

	all := cfg.ReviewTasks()
	if len(all) != 2 {
		t.Errorf("ReviewTasks() count = %d, want 2", len(all))
	}
}

func TestGradeTasks(t *testing.T) {
	cfg := &Config{
		Grade: map[string]Task{
			"coverage": {Cmd: "go test -cover"},
		},
		Vars: map[string]string{},
	}

	all := cfg.GradeTasks()
	if len(all) != 1 {
		t.Errorf("GradeTasks() count = %d, want 1", len(all))
	}
}

func TestShipTasksPR(t *testing.T) {
	cfg := &Config{
		Ship: ShipBlock{
			PR: &PRConfig{Base: "main"},
		},
		Vars: map[string]string{},
	}

	tasks := cfg.ShipTasks("pr", "my title", "my body")
	if len(tasks) != 1 {
		t.Fatalf("ShipTasks(pr) count = %d, want 1", len(tasks))
	}
	// should use Args not Cmd
	if len(tasks[0].Args) == 0 {
		t.Fatal("expected Args to be set for PR task")
	}
	// check title and body are in args (not shell-escaped)
	found := false
	for _, a := range tasks[0].Args {
		if a == "my title" {
			found = true
		}
	}
	if !found {
		t.Errorf("title not found in Args: %v", tasks[0].Args)
	}
}

func TestShipTasksIssue(t *testing.T) {
	cfg := &Config{
		Ship: ShipBlock{
			Issue: &IssueConfig{
				Repo: "test/repo",
				Types: map[string]IssueTypeConfig{
					"feat": {Labels: []string{"enhancement"}},
					"bug":  {Labels: []string{"bug"}},
				},
			},
		},
		Vars: map[string]string{},
	}

	tasks := cfg.ShipTasks("issue", "feat", "my title", "my body")
	if len(tasks) != 1 {
		t.Fatalf("ShipTasks(issue) count = %d, want 1", len(tasks))
	}
	// should use Args not Cmd
	if len(tasks[0].Args) == 0 {
		t.Fatal("expected Args to be set for issue task")
	}
	// check label is in args
	foundLabel := false
	for _, a := range tasks[0].Args {
		if a == "enhancement" {
			foundLabel = true
		}
	}
	if !foundLabel {
		t.Errorf("enhancement label not found in Args: %v", tasks[0].Args)
	}
}

func TestBackwardCompatErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tidal.yaml")

	yaml := `harness: v1
name: old-project
observe:
  errors:
    cmd: "gh issue list --label bug"
`
	os.WriteFile(path, []byte(yaml), 0644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	// errors should be migrated to issues
	if cfg.Observe.Issues == nil {
		t.Fatal("observe.errors should migrate to observe.issues")
	}
	if cfg.Observe.Issues.Cmd != "gh issue list --label bug" {
		t.Errorf("migrated cmd = %q", cfg.Observe.Issues.Cmd)
	}
}

func TestBackwardCompatIssueLabels(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tidal.yaml")

	yaml := `harness: v1
name: old-project
ship:
  issue:
    repo: "test/repo"
    labels:
      - bug
`
	os.WriteFile(path, []byte(yaml), 0644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	// labels should migrate to types.bug
	if cfg.Ship.Issue.Types == nil {
		t.Fatal("ship.issue.labels should migrate to types")
	}
	if _, ok := cfg.Ship.Issue.Types["bug"]; !ok {
		t.Fatal("expected types.bug to exist")
	}
}
