package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/oSEAItic/tidal/internal/config"
	"github.com/oSEAItic/tidal/internal/runner"
	"github.com/spf13/cobra"
)

var (
	cfgFile    string
	env        string
	jsonOutput bool
	stdinInput bool
)

func main() {
	root := &cobra.Command{
		Use:   "tidal",
		Short: "Universal dev harness for AI agents and humans",
		Long:  "Tidal — declare once, observe/test/ship/verify from anywhere.",
	}

	root.PersistentFlags().StringVarP(&cfgFile, "config", "c", "tidal.yaml", "config file path")
	root.PersistentFlags().StringVarP(&env, "env", "e", "", "environment override (e.g. production)")
	root.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output as structured JSON")
	root.PersistentFlags().BoolVar(&stdinInput, "stdin", false, "read JSON input from stdin")

	root.AddCommand(initCmd())
	root.AddCommand(testCmd())
	root.AddCommand(lintCmd())
	root.AddCommand(observeCmd())
	root.AddCommand(shipCmd())
	root.AddCommand(verifyCmd())
	root.AddCommand(worktreeCmd())
	root.AddCommand(gradeCmd())
	root.AddCommand(statusCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func loadConfig() (*config.Config, error) {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return nil, err
	}
	if env != "" {
		cfg.ApplyEnv(env)
	}
	return cfg, nil
}

// readStdinJSON reads JSON from stdin into the provided target.
func readStdinJSON(target interface{}) error {
	return json.NewDecoder(os.Stdin).Decode(target)
}

// ── init ──

func initCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Generate a tidal.yaml template in the current directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			return config.WriteTemplate("tidal.yaml")
		},
	}
}

// ── test ──

func testCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test [name...]",
		Short: "Run test tasks (all or by name)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			tasks := cfg.TestTasks(args...)
			if len(tasks) == 0 {
				return fmt.Errorf("no test tasks configured")
			}
			return runner.Run("test", tasks, jsonOutput)
		},
	}
}

// ── lint ──

func lintCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lint [name...]",
		Short: "Run lint/rule checks (all or by name)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			tasks := cfg.LintTasks(args...)
			if len(tasks) == 0 {
				return fmt.Errorf("no lint tasks configured")
			}
			return runner.Run("lint", tasks, jsonOutput)
		},
	}
}

// ── observe ──

func observeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "observe",
		Short: "Observe logs, metrics, traces, CI, issues",
	}

	for _, sub := range []struct {
		use   string
		short string
		kind  string
	}{
		{"logs [name]", "View logs", "logs"},
		{"metrics [name]", "View metrics", "metrics"},
		{"traces [name]", "View traces", "traces"},
		{"ci", "View CI status", "ci"},
		{"issues", "View GitHub issues", "issues"},
		{"errors", "View errors (deprecated: use issues)", "errors"},
	} {
		kind := sub.kind // capture
		cmd.AddCommand(&cobra.Command{
			Use:   sub.use,
			Short: sub.short,
			RunE: func(c *cobra.Command, args []string) error {
				cfg, err := loadConfig()
				if err != nil {
					return err
				}
				tasks := cfg.ObserveTasks(kind, args...)
				if len(tasks) == 0 {
					return fmt.Errorf("observe.%s not configured", kind)
				}
				return runner.Run("observe", tasks, jsonOutput)
			},
		})
	}
	return cmd
}

// ── ship ──

func shipCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ship",
		Short: "Ship code: PR, issue, deploy",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "pr [title] [body]",
		Short: "Create a pull request",
		RunE: func(c *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			if stdinInput {
				var input struct {
					Title  string `json:"title"`
					Body   string `json:"body"`
					Closes []int  `json:"closes"`
					Branch string `json:"branch"`
				}
				if err := readStdinJSON(&input); err != nil {
					return fmt.Errorf("invalid JSON input: %w", err)
				}
				// append closes to body
				body := input.Body
				for _, n := range input.Closes {
					body += fmt.Sprintf("\n\nCloses #%d", n)
				}
				args = []string{input.Title, body}
			}
			tasks := cfg.ShipTasks("pr", args...)
			if len(tasks) == 0 {
				return fmt.Errorf("ship.pr not configured")
			}
			return runner.Run("ship", tasks, jsonOutput)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "issue [type] [title] [body]",
		Short: "Create a GitHub issue (--type: feat/bug/chore/doc)",
		RunE: func(c *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			if stdinInput {
				var input struct {
					Type  string `json:"type"`
					Title string `json:"title"`
					Body  string `json:"body"`
				}
				if err := readStdinJSON(&input); err != nil {
					return fmt.Errorf("invalid JSON input: %w", err)
				}
				if input.Type == "" {
					input.Type = "feat"
				}
				args = []string{input.Type, input.Title, input.Body}
			}
			tasks := cfg.ShipTasks("issue", args...)
			if len(tasks) == 0 {
				return fmt.Errorf("ship.issue not configured")
			}
			return runner.Run("ship", tasks, jsonOutput)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "deploy [env]",
		Short: "Deploy to environment",
		RunE: func(c *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			target := "staging"
			if len(args) > 0 {
				target = args[0]
			}
			tasks := cfg.ShipTasks("deploy", target)
			if len(tasks) == 0 {
				return fmt.Errorf("ship.deploy.%s not configured", target)
			}
			return runner.Run("ship", tasks, jsonOutput)
		},
	})

	return cmd
}

// ── verify ──

func verifyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "verify",
		Short: "Run health checks and smoke tests",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			tasks := cfg.VerifyTasks(args...)
			if len(tasks) == 0 {
				return fmt.Errorf("no verify tasks configured")
			}
			return runner.Run("verify", tasks, jsonOutput)
		},
	}
}

// ── worktree ──

func worktreeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worktree",
		Short: "Manage isolated git worktrees",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "create <name>",
		Short: "Create an isolated worktree for parallel work",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			if cfg.Worktree == nil {
				return fmt.Errorf("worktree not configured in tidal.yaml")
			}
			name := args[0]
			dir := cfg.Worktree.Dir
			if dir == "" {
				dir = "/tmp/tidal-worktrees"
			}
			wtPath := filepath.Join(dir, name)
			branch := "tidal/" + name

			// git worktree add
			gitCmd := exec.Command("git", "worktree", "add", "-b", branch, wtPath)
			out, err := gitCmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("git worktree add failed: %s\n%s", err, string(out))
			}

			// run setup command if configured
			if cfg.Worktree.Setup != "" {
				setupCmd := exec.Command("sh", "-c", cfg.Worktree.Setup)
				setupCmd.Dir = wtPath
				if setupOut, err := setupCmd.CombinedOutput(); err != nil {
					fmt.Fprintf(os.Stderr, "setup warning: %s\n%s", err, string(setupOut))
				}
			}

			result := runner.TaskResult{
				Name:   name,
				Status: "pass",
				Structured: map[string]string{
					"path":   wtPath,
					"branch": branch,
				},
			}

			if jsonOutput {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				_ = enc.Encode(runner.Envelope{
					Command: "worktree",
					Tasks:   []runner.TaskResult{result},
				})
			} else {
				fmt.Printf("created worktree: %s → %s (branch: %s)\n", name, wtPath, branch)
			}
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List active worktrees",
		RunE: func(c *cobra.Command, args []string) error {
			out, err := exec.Command("git", "worktree", "list", "--porcelain").CombinedOutput()
			if err != nil {
				return fmt.Errorf("git worktree list failed: %s", err)
			}

			type wtInfo struct {
				Path   string `json:"path"`
				Branch string `json:"branch"`
			}
			var worktrees []wtInfo
			var current wtInfo
			for _, line := range strings.Split(string(out), "\n") {
				if strings.HasPrefix(line, "worktree ") {
					if current.Path != "" {
						worktrees = append(worktrees, current)
					}
					current = wtInfo{Path: strings.TrimPrefix(line, "worktree ")}
				} else if strings.HasPrefix(line, "branch ") {
					current.Branch = strings.TrimPrefix(line, "branch refs/heads/")
				}
			}
			if current.Path != "" {
				worktrees = append(worktrees, current)
			}

			if jsonOutput {
				var results []runner.TaskResult
				for _, wt := range worktrees {
					results = append(results, runner.TaskResult{
						Name:   filepath.Base(wt.Path),
						Status: "pass",
						Structured: map[string]string{
							"path":   wt.Path,
							"branch": wt.Branch,
						},
					})
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				_ = enc.Encode(runner.Envelope{
					Command: "worktree",
					Tasks:   results,
				})
			} else {
				fmt.Printf("%-20s %-40s %s\n", "NAME", "PATH", "BRANCH")
				fmt.Println(strings.Repeat("─", 70))
				for _, wt := range worktrees {
					fmt.Printf("%-20s %-40s %s\n", filepath.Base(wt.Path), wt.Path, wt.Branch)
				}
			}
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "destroy <name>",
		Short: "Remove a worktree and its branch",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			name := args[0]
			dir := "/tmp/tidal-worktrees"
			if cfg.Worktree != nil && cfg.Worktree.Dir != "" {
				dir = cfg.Worktree.Dir
			}
			wtPath := filepath.Join(dir, name)
			branch := "tidal/" + name

			// run cleanup if configured
			if cfg.Worktree != nil && cfg.Worktree.Cleanup != "" {
				cleanupCmd := exec.Command("sh", "-c", cfg.Worktree.Cleanup)
				cleanupCmd.Dir = wtPath
				if out, err := cleanupCmd.CombinedOutput(); err != nil {
					fmt.Fprintf(os.Stderr, "cleanup warning: %s\n%s", err, string(out))
				}
			}

			// git worktree remove
			rmCmd := exec.Command("git", "worktree", "remove", wtPath, "--force")
			if out, err := rmCmd.CombinedOutput(); err != nil {
				return fmt.Errorf("git worktree remove failed: %s\n%s", err, string(out))
			}

			// delete branch
			brCmd := exec.Command("git", "branch", "-D", branch)
			_ = brCmd.Run() // best effort

			if jsonOutput {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				_ = enc.Encode(runner.Envelope{
					Command: "worktree",
					Tasks: []runner.TaskResult{{
						Name:   name,
						Status: "pass",
						Structured: map[string]string{
							"path":   wtPath,
							"branch": branch,
							"action": "destroyed",
						},
					}},
				})
			} else {
				fmt.Printf("destroyed worktree: %s\n", name)
			}
			return nil
		},
	})

	return cmd
}

// ── grade ──

func gradeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "grade [name...]",
		Short: "Run quality scoring tasks",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			tasks := cfg.GradeTasks(args...)
			if len(tasks) == 0 {
				return fmt.Errorf("no grade tasks configured")
			}
			return runner.Run("grade", tasks, jsonOutput)
		},
	}
}

// ── status ──

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show configured capabilities for this repo",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			if jsonOutput {
				status := map[string]interface{}{
					"command": "status",
					"name":    cfg.Name,
					"lang":    cfg.Lang,
					"capabilities": map[string]interface{}{
						"test":        capInfo(len(cfg.Test) > 0, mapKeys(cfg.Test)),
						"lint":        capInfo(len(cfg.Lint) > 0, mapKeys(cfg.Lint)),
						"observe":     capInfo(len(cfg.Observe.Logs) > 0 || cfg.Observe.Issues != nil || cfg.Observe.CI != nil, nil),
						"ship:pr":     capInfo(cfg.Ship.PR != nil, nil),
						"ship:issue":  capInfo(cfg.Ship.Issue != nil, issueTypes(cfg.Ship.Issue)),
						"ship:deploy": capInfo(len(cfg.Ship.Deploy) > 0, mapKeysTask(cfg.Ship.Deploy)),
						"verify":      capInfo(cfg.Verify.Health != nil || len(cfg.Verify.Smoke) > 0, nil),
						"worktree":    capInfo(cfg.Worktree != nil, nil),
						"grade":       capInfo(len(cfg.Grade) > 0, mapKeys(cfg.Grade)),
					},
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(status)
			}
			cfg.PrintStatus()
			return nil
		},
	}
}

func capInfo(ready bool, details interface{}) map[string]interface{} {
	m := map[string]interface{}{"ready": ready}
	if details != nil {
		m["tasks"] = details
	}
	return m
}

func mapKeys(m map[string]config.Task) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func mapKeysTask(m map[string]*config.Task) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func issueTypes(ic *config.IssueConfig) []string {
	if ic == nil {
		return nil
	}
	var types []string
	for t := range ic.Types {
		types = append(types, t)
	}
	return types
}
