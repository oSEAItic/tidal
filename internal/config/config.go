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
	Harness string            `yaml:"harness"`
	Name    string            `yaml:"name"`
	Lang    string            `yaml:"lang"`
	Observe ObserveBlock       `yaml:"observe"`
	Test    map[string]Task    `yaml:"test"`
	Ship    ShipBlock          `yaml:"ship"`
	Verify  VerifyBlock        `yaml:"verify"`
	Vars    map[string]string  `yaml:"vars"`
	Envs    map[string]EnvOver `yaml:"envs"`
}

type ObserveBlock struct {
	Logs   []NamedTask `yaml:"logs"`
	Traces *Task       `yaml:"traces"`
	Errors *Task       `yaml:"errors"`
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
	PR     *PRConfig          `yaml:"pr"`
	Deploy map[string]*Task   `yaml:"deploy"`
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
	var tasks []runner.Task
	for name, t := range c.Test {
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
	case "errors":
		if c.Observe.Errors != nil {
			cmd := c.Observe.Errors.Cmd
			tasks = append(tasks, runner.Task{Name: "errors", Cmd: c.expand(cmd)})
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
			cmd := fmt.Sprintf("gh pr create --base %s", c.Ship.PR.Base)
			tasks = append(tasks, runner.Task{Name: "pr", Cmd: c.expand(cmd)})
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
	section("observe", len(c.Observe.Logs) > 0 || c.Observe.Errors != nil)
	section("ship:pr", c.Ship.PR != nil)
	section("ship:deploy", len(c.Ship.Deploy) > 0)
	section("verify", c.Verify.Health != nil || len(c.Verify.Smoke) > 0)
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
