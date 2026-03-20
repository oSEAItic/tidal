# tidal.yaml Schema v2 — Design

## 问题

v1 schema 有几个逻辑问题：

1. `observe.errors` 本质是 `gh issue list --label bug` — 太窄，且把 "issue" 藏在了 "observe" 下面
2. `ship.issue` 只能打固定 label — 没有 feat/bug/chore 区分
3. issue 和 PR 没有关联机制
4. `observe` 混了两类完全不同的东西：基础设施日志 vs GitHub issue
5. 缺少 worktree、lint、grade 的位置

## Dog-fooding Insight（Claude Code 实际使用 tidal 的痛点）

作为一个真正在用 tidal 的 agent，以下是实际体验中发现的问题：

### Pain 1: 输入方式不是 agent-native

我实际执行的命令：
```bash
tidal ship issue "feat: add worktree" "## Motivation\n\nAgents need isolated..."
```
整个 body 塞进 shell 参数，转义地狱。Agent 最自然的输入方式是 **JSON stdin**：
```bash
echo '{"type":"feat","title":"...","body":"..."}' | tidal ship issue
```
**原则：输入也要 JSON，不只是输出。**

### Pain 2: 输出不够结构化

`tidal observe errors --json` 返回：
```json
[{"name":"errors","status":"pass","output":""}]
```
`output` 是原始文本。Agent 没法 parse。应该返回：
```json
[{"name":"issues","status":"pass","items":[{"number":1,"title":"...","labels":["enhancement"]}]}]
```
**原则：output 字段能结构化的就结构化，不要让 agent 二次 parse 文本。**

### Pain 3: 只告诉"怎样了"，没告诉"怎么办"

test fail 时返回 `"error":"exit status 1"`。Agent 需要的是：
```json
{"name":"unit","status":"fail","failures":[{"file":"config_test.go","line":47,"msg":"expected 3, got 0"}]}
```
**原则：失败结果要带 actionable context — 文件、行号、消息。**

### Pain 4: ship 的输入和 observe 的输出没有打通

我用 `tidal observe issues` 看到 issue #1，想用 `tidal ship pr --closes 1` 自动关联。
但现在 observe 返回的是纯文本，我得自己 parse 出 issue number。

**原则：tidal 内部的命令之间应该有数据流通的可能。observe 的输出格式应该能直接喂给 ship。**

### Pain 5: 没有 --stdin flag

所有写操作（ship issue, ship pr）都应该支持 `--stdin`，从 stdin 读 JSON 输入：
```bash
# agent 自然的工作方式
echo '{"type":"feat","title":"add worktree","body":"..."}' | tidal ship issue --stdin
echo '{"title":"feat: worktree","body":"...","closes":1}' | tidal ship pr --stdin
```

---

## 设计原则

1. **每个顶层 block 对应一个动词** — agent 调用时直觉清晰
2. **block 内部用名字区分，不用类型** — 灵活但统一
3. **JSON in, JSON out** — 输入输出都是 JSON，不只是输出（`--stdin` + `--json`）
4. **输出要 actionable** — 失败要带文件/行号/消息，不只是 exit code
5. **声明式，不含逻辑** — tidal 不做判断，只执行
6. **命令间数据可流通** — observe 的输出格式能直接喂给 ship
7. **可选渐进** — 新 repo 可以只写 `test`，其他后补

## v2 Schema

```yaml
harness: v2
name: my-project
lang: go

# ──────────────────────────────────────
# observe — 获取当前状态的只读操作
# ──────────────────────────────────────
# agent 用这些来了解"现在发生了什么"
observe:
  logs:                           # 日志源（可以有多个）
    - name: app
      cmd: "kubectl logs deploy/{{service}} -n {{env}} --tail={{lines}}"
    - name: build
      cmd: "go build ./cmd/... 2>&1"
    - name: git
      cmd: "git log --oneline -20"

  metrics:                        # 指标查询
    - name: latency
      cmd: "curl -s '{{prom_url}}/api/v1/query?query=http_request_duration_seconds{service=\"{{service}}\"}'  "

  traces:                         # 分布式追踪
    - name: recent
      cmd: "curl -s '{{trace_url}}/api/traces?service={{service}}&limit=10'"

  ci:                             # CI/CD 状态
    cmd: "gh run list --repo {{repo}} --limit 5"

  issues:                         # GitHub issues（替代原来的 errors）
    cmd: "gh issue list --repo {{repo}} --state open"

# ──────────────────────────────────────
# test — 验证代码正确性
# ──────────────────────────────────────
# agent 用这些来回答"改动是否安全"
test:
  build:
    cmd: "go build ./cmd/..."
  vet:
    cmd: "go vet ./..."
  unit:
    cmd: "go test ./... -short"
    timeout: 120
  integration:
    cmd: "go test ./... -run Integration -tags=integration"
    requires: [docker]

# ──────────────────────────────────────
# lint — 检查规则和约束（与 test 分开）
# ──────────────────────────────────────
# test 验证"能不能跑"，lint 验证"符不符合规范"
# agent 用 lint 结果来判断代码质量
lint:
  golangci:
    cmd: "golangci-lint run --out-format json"
  filesize:
    cmd: "find . -name '*.go' -size +500l | head -20"
  # 用户可以定义任意规则
  imports:
    cmd: "go vet -vettool=$(which importcheck) ./..."

# ──────────────────────────────────────
# ship — 交付改动
# ──────────────────────────────────────
ship:
  pr:
    base: main
    prefix: "tidal/"              # 分支前缀
    auto_test: true               # 提 PR 前自动跑 test
    template: |
      ## Summary
      {{summary}}

      ## Test
      {{test_output}}

      {{closes}}

  issue:
    repo: "{{repo}}"
    types:                        # 不同类型的 issue 用不同 label
      feat:
        labels: [enhancement]
      bug:
        labels: [bug]
      chore:
        labels: [chore]
      doc:
        labels: [documentation]

  deploy:
    staging:
      cmd: "kubectl rollout restart deploy/{{service}} -n staging"
      wait: 30
    production:
      cmd: "kubectl rollout restart deploy/{{service}} -n production"
      confirm: true

# ──────────────────────────────────────
# verify — 确认交付结果
# ──────────────────────────────────────
verify:
  health:
    cmd: "curl -sf {{base_url}}/health"
    retries: 3
    interval: 10
  smoke:
    - name: "core api"
      cmd: "curl -sf {{base_url}}/api/v1/ping"
  rollback:
    cmd: "kubectl rollout undo deploy/{{service}} -n {{env}}"

# ──────────────────────────────────────
# worktree — 隔离执行环境
# ──────────────────────────────────────
# agent 用这个来并行处理多个任务
worktree:
  dir: "/tmp/tidal-worktrees"     # worktree 存放目录
  setup: "go mod download"        # 创建后执行
  cleanup: ""                     # 销毁前执行

# ──────────────────────────────────────
# grade — 质量评分（只读）
# ──────────────────────────────────────
# agent 用这个来量化"代码库有多健康"
grade:
  coverage:
    cmd: "go test ./... -coverprofile=cover.out -short && go tool cover -func=cover.out | tail -1"
  lint_score:
    cmd: "golangci-lint run --out-format json 2>/dev/null | jq '.Issues | length'"
  doc_coverage:
    cmd: "find . -name '*.go' -exec grep -L '// ' {} \\; | wc -l"
  test_count:
    cmd: "go test ./... -v -short 2>&1 | grep -c 'PASS\\|FAIL'"

# ──────────────────────────────────────
# vars + envs
# ──────────────────────────────────────
vars:
  repo: "oSEAItic/tidal"
  service: my-service
  base_url: "https://api-staging.example.com"
  prom_url: "http://localhost:9090"
  trace_url: "http://localhost:16686"
  env: staging
  lines: 200
  summary: ""
  test_output: ""
  closes: ""

envs:
  production:
    vars:
      base_url: "https://api.example.com"
      env: production
```

## v1 → v2 变化总结

| 变化 | 原因 |
|------|------|
| `observe.errors` → `observe.issues` | 不只是 bug，feat/chore issue 也需要被 observe |
| `observe` 新增 `metrics`、`traces`、`ci` | agent 需要更多维度的可观测信号 |
| 新增 `lint` 顶层 block | test 和 lint 是不同的事：test = 能不能跑，lint = 符不符合规范 |
| `ship.issue` 支持 `types` | `tidal ship issue --type feat` vs `--type bug`，自动打对应 label |
| `ship.pr.template` 加 `{{closes}}` | PR 自动关联 issue（`closes #N`） |
| 新增 `worktree` 顶层 block | Phase 2：隔离并行执行 |
| 新增 `grade` 顶层 block | Phase 4：量化代码质量 |

## Agent-Native 交互设计

### 核心理念：JSON in, JSON out

```
Agent ──JSON stdin──→ tidal ──JSON stdout──→ Agent
                        │
                   tidal.yaml（声明能力）
```

Agent 不应该构造 shell 参数。所有写操作支持 `--stdin`：

```bash
# ❌ v1: shell 参数（转义地狱）
tidal ship issue "feat: add worktree" "## Motivation\n\nAgents need..."

# ✅ v2: JSON stdin（agent 原生）
echo '{"type":"feat","title":"add worktree","body":"## Motivation\n\n..."}' | tidal ship issue --stdin

# ✅ v2: pipe 联动（observe 输出喂给 ship）
tidal observe issues --json | jq '.[0].items[0]' | tidal ship pr --stdin --closes
```

### 所有命令的统一接口

```bash
# 读操作：只有 --json flag
tidal <verb> [subcommand] [name...] --json

# 写操作：支持 --stdin 读 JSON 输入
tidal ship issue --stdin --json < input.json
tidal ship pr --stdin --json < input.json
```

## CLI 命令映射

```bash
# observe（只读）
tidal observe logs [name]         --json
tidal observe metrics [name]      --json
tidal observe traces [name]       --json
tidal observe ci                  --json
tidal observe issues              --json

# test（只读）
tidal test [name...]              --json

# lint（只读）
tidal lint [name...]              --json

# ship（写操作，支持 --stdin）
tidal ship pr       --stdin       --json
tidal ship issue    --stdin       --json
tidal ship deploy [env]           --json

# verify（只读）
tidal verify [name...]            --json

# worktree（写操作）
tidal worktree create <name>      --json
tidal worktree list               --json
tidal worktree destroy <name>     --json

# grade（只读）
tidal grade [name...]             --json

# meta
tidal init
tidal status                      --json
```

## JSON 输出格式

### 统一 Result 结构

每个命令返回同一种结构，agent 不需要按命令分别处理：

```jsonc
{
  "command": "test",              // 哪个命令
  "tasks": [                     // 结果数组（统一）
    {
      "name": "unit",
      "status": "pass",           // "pass" | "fail" | "skip"
      "time_ms": 1200,
      "output": "...",            // 原始输出（给人看）
      "structured": { ... }       // 结构化数据（给 agent 用）
    }
  ],
  "summary": {
    "total": 3,
    "passed": 2,
    "failed": 1
  }
}
```

### 每个命令的 structured 字段

```jsonc
// tidal test --json → 失败时带 actionable context
{
  "command": "test",
  "tasks": [{
    "name": "unit",
    "status": "fail",
    "time_ms": 1200,
    "structured": {
      "failures": [
        {"file": "config_test.go", "line": 47, "func": "TestLoad", "msg": "expected 3, got 0"}
      ]
    }
  }]
}

// tidal observe issues --json → 结构化 issue 列表
{
  "command": "observe",
  "tasks": [{
    "name": "issues",
    "status": "pass",
    "structured": {
      "items": [
        {"number": 1, "title": "feat: worktree support", "labels": ["enhancement"], "state": "open", "url": "https://..."}
      ]
    }
  }]
}

// tidal ship issue --stdin --json → 返回创建结果
{
  "command": "ship",
  "tasks": [{
    "name": "issue",
    "status": "pass",
    "structured": {
      "number": 2,
      "url": "https://github.com/oSEAItic/tidal/issues/2"
    }
  }]
}

// tidal ship pr --stdin --json → 返回 PR 结果
{
  "command": "ship",
  "tasks": [{
    "name": "pr",
    "status": "pass",
    "structured": {
      "number": 5,
      "url": "https://github.com/oSEAItic/tidal/pull/5",
      "closes": [1]
    }
  }]
}

// tidal worktree create feat-123 --json
{
  "command": "worktree",
  "tasks": [{
    "name": "feat-123",
    "status": "pass",
    "structured": {
      "path": "/tmp/tidal-worktrees/feat-123",
      "branch": "tidal/feat-123"
    }
  }]
}

// tidal grade --json
{
  "command": "grade",
  "tasks": [
    {"name": "coverage", "status": "pass", "structured": {"value": 72.3, "unit": "percent"}},
    {"name": "lint_score", "status": "pass", "structured": {"value": 3, "unit": "issues"}},
    {"name": "test_count", "status": "pass", "structured": {"value": 47, "unit": "tests"}}
  ],
  "summary": {"total": 3, "passed": 3, "failed": 0}
}

// tidal status --json
{
  "command": "status",
  "name": "tidal",
  "lang": "go",
  "capabilities": {
    "test": {"ready": true, "tasks": ["build", "vet", "unit"]},
    "lint": {"ready": true, "tasks": ["golangci", "filesize"]},
    "observe": {"ready": true, "tasks": ["logs", "ci", "issues"]},
    "ship:pr": {"ready": true},
    "ship:issue": {"ready": true, "types": ["feat", "bug", "chore", "doc"]},
    "ship:deploy": {"ready": false},
    "verify": {"ready": true, "tasks": ["health", "smoke"]},
    "worktree": {"ready": true},
    "grade": {"ready": true, "tasks": ["coverage", "lint_score", "test_count"]}
  }
}
```

### --stdin 输入格式

```jsonc
// tidal ship issue --stdin
{"type": "feat", "title": "add worktree support", "body": "## Motivation\n\n..."}

// tidal ship pr --stdin
{"title": "feat: worktree support", "body": "## Summary\n\n...", "closes": [1], "branch": "tidal/worktree"}
```

## Agent 工作流示例

一个 agent 使用 tidal 的完整 dog-food 流程：

```bash
# 1. 了解当前状态
STATE=$(tidal status --json)
ISSUES=$(tidal observe issues --json)

# 2. 创建隔离环境
WT=$(tidal worktree create fix-issue-1 --json)
cd $(echo $WT | jq -r '.tasks[0].structured.path')

# 3. ... agent 在 worktree 里改代码 ...

# 4. 验证改动
tidal test --json
tidal lint --json

# 5. 检查质量
tidal grade --json

# 6. 提 PR（JSON stdin，关联 issue）
echo '{"title":"fix: resolve issue #1","body":"...","closes":[1]}' | tidal ship pr --stdin --json

# 7. 验证部署
tidal verify --json

# 8. 清理
tidal worktree destroy fix-issue-1
```

## 向后兼容

- `harness: v1` 的 yaml 继续能用（旧的 Result 数组格式）
- `harness: v2` 启用新格式（统一 Result 结构 + structured 字段）
- `observe.errors` 自动映射到 `observe.issues`（deprecation warning）
- 不带 `--stdin` 时仍支持 CLI 参数（人类友好）
