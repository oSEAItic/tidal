package runner

import (
	"testing"
)

func TestExecutePass(t *testing.T) {
	r := execute(Task{Name: "echo", Cmd: "echo hello"})
	if r.Status != "pass" {
		t.Errorf("Status = %q, want pass", r.Status)
	}
	if r.Output != "hello" {
		t.Errorf("Output = %q, want %q", r.Output, "hello")
	}
	if r.TimeMs <= 0 {
		t.Error("TimeMs should be > 0")
	}
}

func TestExecuteFail(t *testing.T) {
	r := execute(Task{Name: "fail", Cmd: "exit 1"})
	if r.Status != "fail" {
		t.Errorf("Status = %q, want fail", r.Status)
	}
	if r.Error == "" {
		t.Error("Error should be set for failed task")
	}
}

func TestExecuteArgs(t *testing.T) {
	r := execute(Task{Name: "args", Args: []string{"echo", "hello world"}})
	if r.Status != "pass" {
		t.Errorf("Status = %q, want pass", r.Status)
	}
	if r.Output != "hello world" {
		t.Errorf("Output = %q, want %q", r.Output, "hello world")
	}
}

func TestExecuteArgsNoShell(t *testing.T) {
	// backticks and $() should NOT be interpreted when using Args
	r := execute(Task{Name: "safe", Args: []string{"echo", "hello `whoami` $(pwd)"}})
	if r.Status != "pass" {
		t.Errorf("Status = %q, want pass", r.Status)
	}
	if r.Output != "hello `whoami` $(pwd)" {
		t.Errorf("Output = %q — shell interpreted the content!", r.Output)
	}
}

func TestExecuteTimeout(t *testing.T) {
	r := execute(Task{Name: "slow", Cmd: "sleep 10", Timeout: 1})
	if r.Status != "fail" {
		t.Errorf("Status = %q, want fail (timeout)", r.Status)
	}
}

func TestRunPass(t *testing.T) {
	tasks := []Task{
		{Name: "a", Cmd: "echo a"},
		{Name: "b", Cmd: "echo b"},
	}
	err := Run("test", tasks, false)
	if err != nil {
		t.Errorf("Run returned error: %v", err)
	}
}

func TestRunFail(t *testing.T) {
	tasks := []Task{
		{Name: "ok", Cmd: "echo ok"},
		{Name: "bad", Cmd: "exit 1"},
	}
	err := Run("test", tasks, false)
	if err == nil {
		t.Error("Run should return error when task fails")
	}
}
