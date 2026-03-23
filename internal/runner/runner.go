package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Task is a single executable unit.
type Task struct {
	Name    string
	Cmd     string   // shell command (run via sh -c)
	Args    []string // direct exec args (bypasses shell, preferred for user content)
	Timeout int      // seconds, 0 = no limit
	Retries int      // 0 = no retry
	Confirm bool     // require user confirmation
}

// TaskResult holds the outcome of running a single task.
type TaskResult struct {
	Name       string      `json:"name"`
	Status     string      `json:"status"` // "pass", "fail", "skip"
	TimeMs     int64       `json:"time_ms"`
	Output     string      `json:"output,omitempty"`     // raw output (for humans)
	Error      string      `json:"error,omitempty"`      // error message if failed
	Structured interface{} `json:"structured,omitempty"` // structured data (for agents)
}

// Envelope is the unified JSON output for all tidal commands.
type Envelope struct {
	Command string       `json:"command"`
	Tasks   []TaskResult `json:"tasks"`
	Summary *Summary     `json:"summary,omitempty"`
}

// Summary provides counts for quick agent parsing.
type Summary struct {
	Total  int `json:"total"`
	Passed int `json:"passed"`
	Failed int `json:"failed"`
}

// Run executes tasks, prints results, and returns the envelope.
func Run(command string, tasks []Task, jsonOut bool) (*Envelope, error) {
	var results []TaskResult
	var failed bool

	for _, t := range tasks {
		r := execute(t)
		results = append(results, r)
		if r.Status == "fail" {
			failed = true
		}
	}

	env := buildEnvelope(command, results)

	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(env)
	} else {
		printTable(results)
	}

	if failed {
		return &env, fmt.Errorf("one or more tasks failed")
	}
	return &env, nil
}

func buildEnvelope(command string, results []TaskResult) Envelope {
	passed := 0
	failed := 0
	for _, r := range results {
		if r.Status == "pass" {
			passed++
		} else if r.Status == "fail" {
			failed++
		}
	}
	return Envelope{
		Command: command,
		Tasks:   results,
		Summary: &Summary{
			Total:  len(results),
			Passed: passed,
			Failed: failed,
		},
	}
}

// RunSingle executes a single task and returns its result directly.
func RunSingle(t Task) TaskResult {
	return execute(t)
}

func execute(t Task) TaskResult {
	start := time.Now()

	timeout := time.Duration(t.Timeout) * time.Second
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var cmd *exec.Cmd
	if len(t.Args) > 0 {
		// direct exec: no shell interpretation, safe for user content
		cmd = exec.CommandContext(ctx, t.Args[0], t.Args[1:]...)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", t.Cmd)
	}
	out, err := cmd.CombinedOutput()
	elapsed := time.Since(start).Milliseconds()

	r := TaskResult{
		Name:   t.Name,
		TimeMs: elapsed,
		Output: strings.TrimSpace(string(out)),
	}

	if err != nil {
		r.Status = "fail"
		r.Error = err.Error()
	} else {
		r.Status = "pass"
	}

	return r
}

func printTable(results []TaskResult) {
	fmt.Printf("%-16s %-8s %-10s %s\n", "TASK", "STATUS", "TIME", "DETAIL")
	fmt.Println(strings.Repeat("─", 60))

	for _, r := range results {
		icon := "✅"
		if r.Status == "fail" {
			icon = "❌"
		} else if r.Status == "skip" {
			icon = "⏭️"
		}
		detail := r.Error
		if len(detail) > 40 {
			detail = detail[:40] + "..."
		}
		fmt.Printf("%-16s %s %-5s %-10s %s\n", r.Name, icon, r.Status, fmtMs(r.TimeMs), detail)
	}
}


func fmtMs(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	return fmt.Sprintf("%.1fs", float64(ms)/1000)
}
