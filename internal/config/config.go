package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/oSEAItic/tidal/internal/runner"
	"gopkg.in/yaml.v3"
)

// Config represents a tidal.yaml file.
type Config struct {
	Harness  string            `yaml:"harness"`
	Name     string            `yaml:"name"`
	Lang     string            `yaml:"lang"`
	Observe  ObserveBlock      `yaml:"observe"`
	Test     map[string]Task   `yaml:"test"`
	Lint     map[string]Task   `yaml:"lint"`
	Ship     ShipBlock         `yaml:"ship"`
	Verify   VerifyBlock       `yaml:"verify"`
	Review   map[string]Task   `yaml:"review"`
	Worktree *WorktreeConfig   `yaml:"worktree"`
	Grade    map[string]Task   `yaml:"grade"`
	Vars     map[string]string `yaml:"vars"`
	Envs     map[string]EnvOver `yaml:"envs"`
}

type ObserveBlock struct {
	Logs    []NamedTask `yaml:"logs"`
	Metrics []NamedTask `yaml:"metrics"`
	Traces  []NamedTask `yaml:"traces"`
	CI      *Task       `yaml:"ci"`
	Issues  *Task       `yaml:"issues"`
	// deprecated: use Issues instead
	Errors *Task `yaml:"errors"`
}

type NamedTask struct {
	Name string `yaml:"name"`
	Cmd  string `yaml:"cmd"`
	API  string `yaml:"api"`
}

type Task struct {
	Cmd      string   `yaml:"cmd"`
	API      string   `yaml:"api"`
	Timeout  int      `yaml:"timeout"`
	Requires []string `yaml:"requires"`
	Retries  int      `yaml:"retries"`
	Interval int      `yaml:"interval"`
	Confirm  bool     `yaml:"confirm"`
}

type ShipBlock struct {
	PR     *PRConfig        `yaml:"pr"`
	Issue  *IssueConfig     `yaml:"issue"`
	Deploy map[string]*Task `yaml:"deploy"`
}

type IssueConfig struct {
	Repo  string                       `yaml:"repo"`
	Types map[string]IssueTypeConfig   `yaml:"types"`
	// deprecated v1 field
	Labels []string `yaml:"labels"`
}

type IssueTypeConfig struct {
	Labels []string `yaml:"labels"`
}

type PRConfig struct {
	Base     string `yaml:"base"`
	Prefix   string `yaml:"prefix"`
	AutoTest bool   `yaml:"auto_test"`
	Template string `yaml:"template"`
}

type VerifyBlock struct {
	Health   *Task       `yaml:"health"`
	Smoke    []NamedTask `yaml:"smoke"`
	Rollback *Task       `yaml:"rollback"`
}

type WorktreeConfig struct {
	Dir     string `yaml:"dir"`
	Setup   string `yaml:"setup"`
	Cleanup string `yaml:"cleanup"`
}

type EnvOver struct {
	Vars map[string]string `yaml:"vars"`
}

// Load reads and parses a tidal.yaml file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid yaml: %w", err)
	}
	if cfg.Vars == nil {
		cfg.Vars = make(map[string]string)
	}
	// backward compat: observe.errors → observe.issues
	if cfg.Observe.Issues == nil && cfg.Observe.Errors != nil {
		cfg.Observe.Issues = cfg.Observe.Errors
		fmt.Fprintln(os.Stderr, "warning: observe.errors is deprecated, use observe.issues")
	}
	// backward compat: ship.issue.labels → ship.issue.types.bug
	if cfg.Ship.Issue != nil && len(cfg.Ship.Issue.Labels) > 0 && len(cfg.Ship.Issue.Types) == 0 {
		cfg.Ship.Issue.Types = map[string]IssueTypeConfig{
			"bug": {Labels: cfg.Ship.Issue.Labels},
		}
	}
	return &cfg, nil
}

// ApplyEnv merges environment-specific vars into the base vars.
func (c *Config) ApplyEnv(name string) {
	env, ok := c.Envs[name]
	if !ok {
		return
	}
	for k, v := range env.Vars {
		c.Vars[k] = v
	}
}

// expand replaces {{var}} placeholders with resolved values.
func (c *Config) expand(s string) string {
	for k, v := range c.Vars {
		s = strings.ReplaceAll(s, "{{"+k+"}}", v)
	}
	return s
}

// TestTasks returns runner tasks for the test block.
func (c *Config) TestTasks(names ...string) []runner.Task {
	return c.blockTasks(c.Test, names...)
}

// LintTasks returns runner tasks for the lint block.
func (c *Config) LintTasks(names ...string) []runner.Task {
	return c.blockTasks(c.Lint, names...)
}

// ReviewTasks returns runner tasks for the review block.
func (c *Config) ReviewTasks(names ...string) []runner.Task {
	return c.blockTasks(c.Review, names...)
}

// GradeTasks returns runner tasks for the grade block.
func (c *Config) GradeTasks(names ...string) []runner.Task {
	return c.blockTasks(c.Grade, names...)
}

// blockTasks is a generic helper for map[string]Task blocks.
func (c *Config) blockTasks(block map[string]Task, names ...string) []runner.Task {
	var tasks []runner.Task
	for name, t := range block {
		if len(names) > 0 && !contains(names, name) {
			continue
		}
		tasks = append(tasks, runner.Task{
			Name:    name,
			Cmd:     c.expand(t.Cmd),
			Timeout: t.Timeout,
		})
	}
	return tasks
}

// ObserveTasks returns runner tasks for the observe block.
func (c *Config) ObserveTasks(kind string, names ...string) []runner.Task {
	var tasks []runner.Task
	switch kind {
	case "logs":
		for _, l := range c.Observe.Logs {
			if len(names) > 0 && !contains(names, l.Name) {
				continue
			}
			cmd := l.Cmd
			if cmd == "" && l.API != "" {
				cmd = "curl -sf " + l.API
			}
			tasks = append(tasks, runner.Task{Name: "logs:" + l.Name, Cmd: c.expand(cmd)})
		}
	case "metrics":
		for _, m := range c.Observe.Metrics {
			if len(names) > 0 && !contains(names, m.Name) {
				continue
			}
			cmd := m.Cmd
			if cmd == "" && m.API != "" {
				cmd = "curl -sf " + m.API
			}
			tasks = append(tasks, runner.Task{Name: "metrics:" + m.Name, Cmd: c.expand(cmd)})
		}
	case "traces":
		for _, t := range c.Observe.Traces {
			if len(names) > 0 && !contains(names, t.Name) {
				continue
			}
			cmd := t.Cmd
			if cmd == "" && t.API != "" {
				cmd = "curl -sf " + t.API
			}
			tasks = append(tasks, runner.Task{Name: "traces:" + t.Name, Cmd: c.expand(cmd)})
		}
	case "ci":
		if c.Observe.CI != nil {
			tasks = append(tasks, runner.Task{Name: "ci", Cmd: c.expand(c.Observe.CI.Cmd)})
		}
	case "issues":
		if c.Observe.Issues != nil {
			tasks = append(tasks, runner.Task{Name: "issues", Cmd: c.expand(c.Observe.Issues.Cmd)})
		}
	case "errors":
		// backward compat
		if c.Observe.Issues != nil {
			tasks = append(tasks, runner.Task{Name: "issues", Cmd: c.expand(c.Observe.Issues.Cmd)})
		}
	}
	return tasks
}

// ShipTasks returns runner tasks for ship operations.
func (c *Config) ShipTasks(kind string, args ...string) []runner.Task {
	var tasks []runner.Task
	switch kind {
	case "pr":
		if c.Ship.PR != nil {
			ghArgs := []string{"gh", "pr", "create", "--base", c.Ship.PR.Base}
			if len(args) >= 1 && args[0] != "" {
				ghArgs = append(ghArgs, "--title", args[0])
			}
			if len(args) >= 2 && args[1] != "" {
				ghArgs = append(ghArgs, "--body", args[1])
			}
			tasks = append(tasks, runner.Task{Name: "pr", Args: ghArgs})
		}
	case "issue":
		if c.Ship.Issue != nil {
			issueType := "feat"
			title := "auto-reported issue"
			body := ""
			if len(args) >= 1 {
				issueType = args[0]
			}
			if len(args) >= 2 {
				title = args[1]
			}
			if len(args) >= 3 {
				body = args[2]
			}
			repo := c.Ship.Issue.Repo
			if repo == "" {
				repo = c.Vars["repo"]
			}
			ghArgs := []string{"gh", "issue", "create", "--repo", c.expand(repo), "--title", title, "--body", body}
			if tc, ok := c.Ship.Issue.Types[issueType]; ok {
				for _, l := range tc.Labels {
					ghArgs = append(ghArgs, "--label", l)
				}
			}
			tasks = append(tasks, runner.Task{Name: "issue", Args: ghArgs})
		}
	case "deploy":
		target := "staging"
		if len(args) > 0 {
			target = args[0]
		}
		if t, ok := c.Ship.Deploy[target]; ok {
			tasks = append(tasks, runner.Task{
				Name:    "deploy:" + target,
				Cmd:     c.expand(t.Cmd),
				Confirm: t.Confirm,
			})
		}
	}
	return tasks
}

// VerifyTasks returns runner tasks for verification.
func (c *Config) VerifyTasks(names ...string) []runner.Task {
	var tasks []runner.Task
	if c.Verify.Health != nil {
		tasks = append(tasks, runner.Task{
			Name:    "health",
			Cmd:     c.expand(c.Verify.Health.Cmd),
			Retries: c.Verify.Health.Retries,
		})
	}
	for _, s := range c.Verify.Smoke {
		cmd := s.Cmd
		if cmd == "" && s.API != "" {
			cmd = "curl -sf " + s.API
		}
		tasks = append(tasks, runner.Task{Name: "smoke:" + s.Name, Cmd: c.expand(cmd)})
	}
	return tasks
}

// PrintStatus shows which capabilities are configured.
func (c *Config) PrintStatus() {
	fmt.Printf("Project: %s (%s)\n\n", c.Name, c.Lang)

	section := func(name string, ok bool) {
		status := "not configured"
		if ok {
			status = "ready"
		}
		fmt.Printf("  %-12s %s\n", name, status)
	}

	fmt.Println("Capabilities:")
	section("test", len(c.Test) > 0)
	section("lint", len(c.Lint) > 0)
	section("observe", len(c.Observe.Logs) > 0 || c.Observe.Issues != nil || c.Observe.CI != nil)
	section("review", len(c.Review) > 0)
	section("ship:pr", c.Ship.PR != nil)
	section("ship:issue", c.Ship.Issue != nil)
	section("ship:deploy", len(c.Ship.Deploy) > 0)
	section("verify", c.Verify.Health != nil || len(c.Verify.Smoke) > 0)
	section("worktree", c.Worktree != nil)
	section("grade", len(c.Grade) > 0)
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
