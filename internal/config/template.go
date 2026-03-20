package config

import (
	"fmt"
	"os"
)

const yamlTemplate = `harness: v2
name: my-project
lang: go

# ── observe: see what's happening ──
observe:
  logs:
    - name: app
      cmd: "tail -100 /var/log/app.log"
    - name: git
      cmd: "git log --oneline -20"
  ci:
    cmd: "gh run list --repo {{repo}} --limit 5"
  issues:
    cmd: "gh issue list --repo {{repo}} --state open"

# ── test: validate changes ──
test:
  build:
    cmd: "go build ./cmd/..."
  unit:
    cmd: "go test ./... -short"
    timeout: 120

# ── lint: check rules and constraints ──
lint:
  vet:
    cmd: "go vet ./..."
  # add your linters here

# ── ship: deliver ──
ship:
  pr:
    base: main
    prefix: "tidal/"
    auto_test: true
  issue:
    repo: "{{repo}}"
    types:
      feat:
        labels: [enhancement]
      bug:
        labels: [bug]
      chore:
        labels: [chore]
  deploy:
    staging:
      cmd: "echo 'deploy to staging'"
      wait: 30
    production:
      cmd: "echo 'deploy to production'"
      confirm: true

# ── verify: confirm results ──
verify:
  health:
    cmd: "curl -sf {{base_url}}/health"
    retries: 3
    interval: 10
  smoke:
    - name: ping
      cmd: "curl -sf {{base_url}}/api/v1/ping"

# ── worktree: isolated execution ──
worktree:
  dir: "/tmp/tidal-worktrees"
  setup: ""
  cleanup: ""

# ── grade: quality scoring ──
grade:
  test_count:
    cmd: "go test ./... -v -short 2>&1 | grep -c PASS"

# ── variables (use {{var_name}} in commands) ──
vars:
  repo: "owner/repo"
  service: my-project
  base_url: "http://localhost:8080"
  env: staging

# ── environment overrides ──
envs:
  production:
    vars:
      base_url: "https://api.example.com"
      env: production
`

// WriteTemplate writes a tidal.yaml template to the given path.
func WriteTemplate(path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists", path)
	}
	if err := os.WriteFile(path, []byte(yamlTemplate), 0644); err != nil {
		return err
	}
	fmt.Printf("Created %s — edit it for your project.\n", path)
	return nil
}
