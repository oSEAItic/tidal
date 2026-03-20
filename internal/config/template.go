package config

import (
	"fmt"
	"os"
)

const yamlTemplate = `harness: v1
name: my-project
lang: go

# ── observe: see what's happening ──
observe:
  logs:
    - name: app
      cmd: "tail -100 /var/log/app.log"
  errors:
    cmd: "gh issue list --label bug --state open"

# ── test: validate changes ──
test:
  build:
    cmd: "go build ./cmd/..."
  lint:
    cmd: "golangci-lint run"
  unit:
    cmd: "go test ./... -short"
    timeout: 120

# ── ship: deliver ──
ship:
  pr:
    base: main
    prefix: "tidal/"
    auto_test: true
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
    cmd: "curl -sf http://localhost:8080/health"
    retries: 3
    interval: 10
  smoke:
    - name: ping
      cmd: "curl -sf http://localhost:8080/api/v1/ping"
  rollback:
    cmd: "echo 'rollback not configured'"

# ── variables (use {{var_name}} in commands) ──
vars:
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
