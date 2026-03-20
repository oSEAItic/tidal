package main

import (
	"fmt"
	"os"

	"github.com/oSEAItic/tidal/internal/config"
	"github.com/oSEAItic/tidal/internal/runner"
	"github.com/spf13/cobra"
)

var (
	cfgFile    string
	env        string
	jsonOutput bool
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

	root.AddCommand(initCmd())
	root.AddCommand(testCmd())
	root.AddCommand(observeCmd())
	root.AddCommand(shipCmd())
	root.AddCommand(verifyCmd())
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
			return runner.Run(tasks, jsonOutput)
		},
	}
}

// ── observe ──

func observeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "observe",
		Short: "Observe logs, traces, errors",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "logs [name]",
		Short: "View logs",
		RunE: func(c *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			tasks := cfg.ObserveTasks("logs", args...)
			return runner.Run(tasks, jsonOutput)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "errors",
		Short: "View errors",
		RunE: func(c *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			tasks := cfg.ObserveTasks("errors", args...)
			return runner.Run(tasks, jsonOutput)
		},
	})
	return cmd
}

// ── ship ──

func shipCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ship",
		Short: "Ship code: PR, deploy",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "pr",
		Short: "Create a pull request",
		RunE: func(c *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			tasks := cfg.ShipTasks("pr")
			return runner.Run(tasks, jsonOutput)
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
			return runner.Run(tasks, jsonOutput)
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
			return runner.Run(tasks, jsonOutput)
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
			cfg.PrintStatus()
			return nil
		},
	}
}
