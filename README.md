# Tidal

**Universal dev harness for AI agents and humans.**

Declare your repo's observe/test/ship/verify capabilities once in `tidal.yaml`, then run them with a single CLI — whether you're Claude Code, Codex, a human, or CI/CD.

```
tidal init     →  generate tidal.yaml template
tidal test     →  run all tests
tidal observe  →  view logs, errors
tidal ship     →  create PR, deploy
tidal verify   →  health checks, smoke tests
tidal status   →  show what's configured
```

## Install

```bash
go install github.com/oSEAItic/tidal/cmd/tidal@latest
```

## Quick Start

```bash
# In any repo:
tidal init          # creates tidal.yaml
vim tidal.yaml      # configure your commands
tidal test          # run tests
tidal test unit     # run specific test
tidal ship pr       # create a PR (auto-runs tests first if configured)
tidal verify        # health + smoke checks
```

## How it works

`tidal.yaml` is a declarative config that maps capabilities to shell commands:

```yaml
harness: v1
name: my-project

test:
  unit:
    cmd: "go test ./... -short"
  lint:
    cmd: "golangci-lint run"

ship:
  pr:
    base: main
    auto_test: true

verify:
  health:
    cmd: "curl -sf https://my-app.com/health"

vars:
  service: my-app
```

Tidal doesn't care about your language or framework — it just executes the `cmd` you declare.

### Template variables

Use `{{var}}` in commands. Switch environments with `--env`:

```yaml
vars:
  base_url: "https://staging.example.com"

envs:
  production:
    vars:
      base_url: "https://example.com"
```

```bash
tidal verify                  # uses staging URL
tidal verify --env production # uses production URL
```

### JSON output for AI agents

```bash
tidal test --json
# {"name":"unit","status":"pass","time_ms":3200}
# {"name":"lint","status":"fail","time_ms":1800,"detail":"..."}
```

## Dog-fooding: Tidal on Tidal

This repo uses itself. Here's our own `tidal.yaml`:

```yaml
harness: v1
name: tidal
lang: go

test:
  build:
    cmd: "go build ./cmd/tidal"
  vet:
    cmd: "go vet ./..."
  unit:
    cmd: "go test ./... -short"

observe:
  errors:
    cmd: "gh issue list --label bug --state open --repo oSEAItic/tidal"

ship:
  pr:
    base: main
    prefix: "tidal/"
    auto_test: true
    template: |
      ## Summary
      {{summary}}
      ## Test
      {{test_output}}

verify:
  health:
    cmd: "go build ./cmd/tidal && echo 'binary builds OK'"

vars:
  summary: ""
  test_output: ""
```

After cloning this repo:

```bash
# Build and test tidal using tidal
go build -o tidal ./cmd/tidal
./tidal test              # runs build + vet + unit
./tidal test build        # just build
./tidal status            # show configured capabilities
./tidal observe errors    # check open bugs
./tidal ship pr           # open a PR
```

## CLI Reference

| Command | Description | Flags |
|---------|-------------|-------|
| `tidal init` | Generate `tidal.yaml` template | |
| `tidal test [name...]` | Run test tasks (all or by name) | `--json` |
| `tidal observe logs [name]` | View logs | `--json` |
| `tidal observe errors` | View errors | `--json` |
| `tidal ship pr` | Create a pull request | `--json` |
| `tidal ship deploy [env]` | Deploy to environment | `--json` |
| `tidal verify` | Run health + smoke checks | `--json` |
| `tidal status` | Show configured capabilities | |

**Global flags:**

| Flag | Description |
|------|-------------|
| `-c, --config` | Config file path (default: `tidal.yaml`) |
| `-e, --env` | Environment override (e.g. `production`) |
| `--json` | Structured JSON output |

## Who is this for?

| User | How they use Tidal |
|------|-------------------|
| **AI Agents** (Claude Code, Codex, Devin) | `tidal test --json` → parse results → fix → `tidal ship pr` |
| **Human devs** | `tidal deploy staging` instead of remembering kubectl commands |
| **CI/CD** | `tidal test && tidal ship deploy` in GitHub Actions |

## Roadmap

- [x] Phase 0: CLI scaffold + tidal.yaml schema
- [ ] Phase 1: `tidal test` — full structured output
- [ ] Phase 2: `tidal observe` — log/trace formatting
- [ ] Phase 3: `tidal ship` — gh pr + deploy integration
- [ ] Phase 4: `tidal verify` — health/smoke/rollback
- [ ] Phase 5: `tidal run-loop` — full autonomous loop

## License

MIT — [oSEAItic](https://github.com/oSEAItic)
