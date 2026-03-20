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
	Cmd     string
	Timeout int  // seconds, 0 = no limit
	Retries int  // 0 = no retry
	Confirm bool // require user confirmation
}

// Result holds the outcome of running a task.
type Result struct {
	Name   string `json:"name"`
	Status string `json:"status"` // "pass" or "fail"
	TimeMs int64  `json:"time_ms"`
	Output string `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
}

// Run executes tasks sequentially and prints results.
func Run(tasks []Task, jsonOut bool) error {
	var results []Result
	var failed bool

	for _, t := range tasks {
		r := execute(t)
		results = append(results, r)
		if r.Status == "fail" {
			failed = true
		}
	}

	if jsonOut {
		printJSON(results)
	} else {
		printTable(results)
	}

	if failed {
		return fmt.Errorf("one or more tasks failed")
	}
	return nil
}

func execute(t Task) Result {
	start := time.Now()

	timeout := time.Duration(t.Timeout) * time.Second
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", t.Cmd)
	out, err := cmd.CombinedOutput()
	elapsed := time.Since(start).Milliseconds()

	r := Result{
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

func printTable(results []Result) {
	fmt.Printf("%-16s %-8s %-10s %s\n", "TASK", "STATUS", "TIME", "DETAIL")
	fmt.Println(strings.Repeat("─", 60))

	for _, r := range results {
		icon := "✅"
		if r.Status == "fail" {
			icon = "❌"
		}
		detail := r.Error
		if len(detail) > 40 {
			detail = detail[:40] + "..."
		}
		fmt.Printf("%-16s %s %-5s %-10s %s\n", r.Name, icon, r.Status, fmtMs(r.TimeMs), detail)
	}
}

func printJSON(results []Result) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(results)
}

func fmtMs(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	return fmt.Sprintf("%.1fs", float64(ms)/1000)
}
