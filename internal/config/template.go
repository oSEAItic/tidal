package config

import (
	"fmt"
	"os"
)

const yamlTemplate = `harness: v2
name: my-project
lang: go

# ── observe: see what's happening ──
# tidal orchestrates existing CLI tools — no new SDKs needed.
# API keys come from environment variables ($VAR or ${VAR}).
observe:
  logs:
    - name: app
      cmd: "tail -100 /var/log/app.log"
    - name: git
      cmd: "git log --oneline -20"
    # ── uncomment the providers you use ──
    # - name: datadog
    #   cmd: "curl -s 'https://api.datadoghq.com/api/v1/logs-queries/list' -H 'DD-API-KEY: $DD_API_KEY' -H 'DD-APPLICATION-KEY: $DD_APP_KEY' -d '{\"query\":\"service:{{service}} status:error\",\"time\":{\"from\":\"now-1h\"},\"limit\":20}'"
    # - name: cloudwatch
    #   cmd: "aws logs filter-log-events --log-group-name /ecs/{{service}} --filter-pattern ERROR --limit 20 --output json"
    # - name: loki
    #   cmd: "curl -s '{{loki_url}}/loki/api/v1/query_range?query={app=\"{{service}}\"}&limit=50'"
    # - name: vercel
    #   cmd: "vercel logs {{service}} --limit 20 --token $VERCEL_TOKEN"

  # metrics:
    # - name: prometheus
    #   cmd: "curl -s '{{prom_url}}/api/v1/query?query=rate(http_requests_total{service=\"{{service}}\"}[5m])'"
    # - name: datadog
    #   cmd: "curl -s 'https://api.datadoghq.com/api/v1/query?query=avg:system.cpu.user{service:{{service}}}&from=$(date -v-1H +%s)&to=$(date +%s)' -H 'DD-API-KEY: $DD_API_KEY' -H 'DD-APPLICATION-KEY: $DD_APP_KEY'"
    # - name: cloudwatch
    #   cmd: "aws cloudwatch get-metric-statistics --namespace AWS/ECS --metric-name CPUUtilization --dimensions Name=ServiceName,Value={{service}} --period 300 --statistics Average --start-time $(date -u -v-1H +%Y-%m-%dT%H:%M:%S) --end-time $(date -u +%Y-%m-%dT%H:%M:%S) --output json"

  # traces:
    # - name: jaeger
    #   cmd: "curl -s '{{jaeger_url}}/api/traces?service={{service}}&limit=10'"

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

# ── review: analyze changes before shipping ──
review:
  diff:
    cmd: "git diff --stat"
  secrets:
    cmd: "git diff HEAD | grep -inE '(password|secret|api.?key|token)\\s*[:=]' || echo 'none detected'"
  todos:
    cmd: "git diff HEAD | grep -c TODO || echo 0"

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
      # ── uncomment your deploy tool ──
      # cmd: "kubectl rollout restart deploy/{{service}} -n staging"
      # cmd: "vercel deploy --token $VERCEL_TOKEN"
      # cmd: "fly deploy --app {{service}}"
      # cmd: "aws ecs update-service --cluster {{cluster}} --service {{service}} --force-new-deployment"
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

# ── variables ──
# Use {{var}} for tidal vars, $VAR for environment variables.
# API keys should ALWAYS use $ENV_VAR, never hardcode them.
vars:
  repo: "owner/repo"
  service: my-project
  base_url: "http://localhost:8080"
  env: staging
  # loki_url: "http://localhost:3100"
  # prom_url: "http://localhost:9090"
  # jaeger_url: "http://localhost:16686"

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
