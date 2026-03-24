# Tidal — Positioning

## One sentence

**Tidal is the standard interface for AI agents to operate on any codebase, using minimal tokens.**

## The problem

AI agents can write code fast. But they can't efficiently:
- Figure out how to test a repo (read README? guess? try `npm test`?)
- Parse 500 lines of test output to find the one failure
- Set up CI/CD, deployment, monitoring
- Know what services exist, how they connect, where files are

This costs tokens. Lots of them.

```
Without tidal:
  Agent reads README             2000 tokens
  Guesses test command            500 tokens
  Runs test, reads raw output    3000 tokens
  Understands what failed        1000 tokens
  Total: ~6500 tokens per test cycle

With tidal:
  tidal test --json
  → {"status":"fail","failures":[{"file":"x.go","line":47}]}
  Total: ~50 tokens per test cycle
```

130x difference. For monitoring and debugging, the gap is even larger.

This isn't optimization. This is whether agents can economically operate at scale.

## What tidal is

A CLI tool that gives any repo a standard, structured interface for agents.

```bash
tidal init           # auto-detect language/tools/structure, generate config
tidal test --json    # run tests, return structured results
tidal lint --json    # check rules
tidal review --json  # analyze changes (diff, secrets, TODOs)
tidal ship pr        # create PR
tidal observe --json # view logs, issues, CI status
tidal verify --json  # health checks
tidal topology --json # project structure
tidal history --json # run trends over time
tidal grade --json   # quality metrics
```

Every command: JSON in (`--stdin`), JSON out (`--json`). Minimal tokens. Agent-native.

## What tidal is NOT

- Not an agent (no AI, no decisions, no loops)
- Not a CI/CD platform (uses existing tools: gh, kubectl, docker)
- Not an observability platform (uses existing: Datadog, CloudWatch, Grafana)
- Not a deployment service (uses existing: Vercel, Railway, k8s)

Tidal is a pickaxe. Agents are miners.

## Target users

### 1. Vibe coders (primary)

People who use AI to build apps but don't know how to set up testing, CI/CD, deployment, or monitoring.

```
What they can do:        What they can't do:
  "build me an app"        write tests
  "add a login page"       configure CI/CD
  "make it prettier"       deploy to production
                           read logs
                           debug failures
                           code review
```

For them, tidal doesn't make things "better." It makes things **exist**. Without tidal, they have no tests, no CI, no deployment pipeline. With `tidal init`, they have everything from Day 1.

```bash
npx create-next-app my-app
cd my-app
tidal init    # ← full harness generated, they don't need to understand any of it
```

### 2. AI agents (Claude Code, Codex, Devin)

Agents need a standard way to discover what they can do in any repo.

```bash
# Agent lands in unknown repo, first commands:
tidal status --json     # what can I do here?
tidal topology --json   # what is this project?
tidal test --json       # does it work?
```

Without tidal, every repo is a puzzle. With tidal, every repo speaks the same language.

### 3. Teams using AI-first development

Teams where agents write most of the code (per OpenAI's "Zero Lines of Code" approach). They need:
- Consistent agent interface across all repos
- Quality tracking over time (tidal grade, tidal history)
- Agent review capabilities (tidal review)

## Why Day 1 matters

Traditional path:
```
write code → write code → write code → maybe add tests → too late, tech debt
```

Tidal path:
```
tidal init → tests + lint + deploy exist from commit #1
agent runs tidal test after every change
there is never a "no tests" phase
```

The best time to add harness is before the first line of code. Tidal makes that free.

## Token economics

Tidal is a token optimization layer.

| Operation | Without tidal | With tidal | Savings |
|-----------|--------------|------------|---------|
| Run tests, understand result | ~6500 tokens | ~50 tokens | 130x |
| Check project structure | ~3000 tokens (read files) | ~100 tokens (topology) | 30x |
| Find open issues | ~2000 tokens (parse gh output) | ~80 tokens (JSON) | 25x |
| Monitor logs | ~5000 tokens (raw logs) | ~200 tokens (structured) | 25x |
| Review changes | ~4000 tokens (read diffs) | ~150 tokens (JSON summary) | 27x |

At scale (hundreds of agent runs per day), this is the difference between $10/day and $500/day.

## The auto in autoharness

`tidal init` scans the repo and generates a complete `tidal.yaml` automatically:

| Signal | Detected | Generated |
|--------|----------|-----------|
| `go.mod` | Go project | `go test`, `go vet`, `go build` |
| `package.json` | Node project | reads scripts, detects npm/yarn/pnpm |
| `pyproject.toml` | Python project | `pytest`, `ruff` |
| `Cargo.toml` | Rust project | `cargo test`, `cargo clippy` |
| `.github/workflows/` | CI provider | `gh run list` |
| `Dockerfile` | Containers | `docker compose up` |
| `k8s/` | Kubernetes | `kubectl apply` |
| git remote | Repo URL | `owner/repo` in vars |
| `cmd/` subdirs | Services | topology entries |
| `Makefile`, `README.md` | Refinement hints | `refine` block |

Zero manual configuration for standard repos. For non-standard repos, the `refine` block tells agents which files to read to fill in the gaps.

## Future: tidal as protocol

```
git solved:    how humans collaborate on code
tidal solves:  how agents operate on code
```

If `tidal.yaml` becomes the file every repo has (like `.gitignore`):

```
Phase 1: CLI tool (now)
  → tidal init / test / ship / verify
  → tidal.yaml in every repo

Phase 2: Community standard
  → 100+ repos using tidal.yaml
  → Agent platforms recognize tidal.yaml natively
  → Registry for sharing tidal.yaml templates

Phase 3: Protocol + Cloud
  → tidal protocol: standard API for agent-repo interaction
  → tidal cloud: dashboard, cross-repo analytics, agent readiness scores
  → Any agent speaks tidal, any repo answers
```

## Competitive landscape

| Product | What it does | What it doesn't do |
|---------|-------------|-------------------|
| GitHub Actions | CI/CD for humans | Not agent-native, no structured output |
| Makefile | Build automation | No structured output, no auto-detect |
| Docker | Container runtime | Doesn't know about tests/lint/deploy |
| AGENTS.md | Agent instructions | Prose, not executable, no JSON output |
| tidal | **Agent-native executable harness with structured I/O** | |

## Business model (future)

```
tidal CLI       → open source, free (like git)
tidal registry  → community templates, free (like Docker Hub)
tidal cloud     → SaaS: dashboard, analytics, scores (like GitHub)
```

The CLI creates the standard. The cloud monetizes the ecosystem.

## What to do now

1. Get `tidal.yaml` into 10 real repos
2. Measure: agent with tidal vs without tidal — token cost, success rate, speed
3. Publish the results
4. Let adoption drive the roadmap
