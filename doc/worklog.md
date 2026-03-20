# Tidal — Worklog

## 2026-03-20 · Project Genesis

### 命名

**Tidal** = Tide + AI。数据/context 像潮汐一样循环流动，对应产品的核心闭环：
`observe → test → ship → verify`

属于 oSEAItic（ocean + SEA + AI）产品线。

---

### 核心洞察

AI agent 开发的瓶颈不是写代码，是 **observe → test → ship → verify** 这个闭环。

任何 repo 要被 AI agent 高效开发，都需要：能看日志、能跑测试、能提 PR、能部署、能验证。
区别只在于每个 repo 的具体命令不同。

**Tidal 做的事** = 统一接口 + 每个 repo 的声明式配置。
就像 Docker — 不管什么语言什么框架，Dockerfile 声明怎么 build，docker run 统一运行。
Tidal 也是：不管 Go/Python/Node/Rust，`tidal.yaml` 声明能力，`tidal` CLI 统一调用。

---

### 设计：tidal.yaml — 通用 Schema

```yaml
# 任何 repo 都能用的通用格式
harness: v1
name: my-project
lang: go                    # 提示信息用，不影响逻辑

# ---- observe: 看到发生了什么 ----
observe:
  logs:
    - name: app
      cmd: "kubectl logs deploy/{{service}} -n {{env}} --tail={{lines}}"
    - name: structured
      api: "{{base_url}}/internal/debug/logs"
  traces:
    api: "{{base_url}}/internal/debug/trace/{{trace_id}}"
  errors:
    cmd: "gh issue list --label bug --state open"

# ---- test: 验证改动 ----
test:
  unit:
    cmd: "go test ./... -short"
    timeout: 120
  lint:
    cmd: "golangci-lint run"
  build:
    cmd: "go build ./cmd/..."
  integration:
    cmd: "go test ./... -run Integration -tags=integration"
    requires: [docker]

# ---- ship: 交付 ----
ship:
  pr:
    base: main
    prefix: "tidal/"
    auto_test: true          # 提 PR 前自动跑 test
    template: |
      ## Summary
      {{summary}}
      ## Test
      {{test_output}}
  deploy:
    staging:
      cmd: "kubectl rollout restart deploy/{{service}} -n staging"
      wait: 30
    production:
      cmd: "kubectl rollout restart deploy/{{service}} -n production"
      confirm: true          # 必须人确认

# ---- verify: 确认结果 ----
verify:
  health:
    cmd: "curl -sf {{base_url}}/health"
    retries: 3
    interval: 10
  smoke:
    - name: "core api"
      cmd: "curl -sf {{base_url}}/api/v1/ping"
    - name: "turbo"
      cmd: "curl -sf {{base_url}}/api/turbo/health"
  rollback:
    cmd: "kubectl rollout undo deploy/{{service}} -n {{env}}"

# ---- 变量 ----
vars:
  service: folder
  base_url: "https://api-staging.kuse.ai"
  env: staging
  lines: 200

# ---- 环境覆盖 ----
envs:
  production:
    vars:
      base_url: "https://api.kuse.ai"
      env: production
```

---

### 关键设计点

1. **纯声明式，不绑定语言/框架** — Go 写 `go test`，Python 写 `pytest`，Node 写 `pnpm test`，tidal 不在乎，只执行 `cmd`
2. **模板变量 `{{...}}`** — 同一套配置，`--env staging` 和 `--env production` 自动切换
3. **每个 block 都是可选的** — 新 repo 接入可以从只写 `test.unit` 开始，逐步补全。CLI 遇到没定义的能力就报 `not configured`
4. **AI agent 友好的输出** — 表格给人看，`--json` 给 agent parse

---

### CLI 命令

```
tidal init              # 在当前 repo 生成 tidal.yaml 模板
tidal test [name...]    # 跑测试（全部或指定）
tidal observe logs      # 看日志
tidal observe errors    # 看错误
tidal ship pr           # 提 PR（自动跑 test）
tidal ship deploy       # 部署
tidal verify            # health + smoke 检查
tidal status            # 当前 repo 的 tidal 能力概览
tidal run-loop          # 全自动: observe → test → ship → verify
```

---

### 输出格式

人类友好表格：
```
┌─────────┬────────┬─────────┬──────────────────────┐
│ name    │ status │ time    │ detail               │
├─────────┼────────┼─────────┼──────────────────────┤
│ build   │ ✅ pass │ 3.2s   │                      │
│ lint    │ ✅ pass │ 1.8s   │                      │
│ unit    │ ❌ fail │ 12.4s  │ turbo_test.go:47 ... │
└─────────┴────────┴─────────┴──────────────────────┘
```

JSON（`--json`）给 agent：
```json
{"name":"unit","status":"fail","time_ms":12400,
 "failures":[{"file":"turbo_test.go","line":47,"msg":"expected 3, got 0"}]}
```

---

### 目标用户

| 用户 | 场景 |
|------|------|
| Claude Code | 改代码后 `tidal test`，提 PR `tidal ship pr` |
| Codex / Devin | 同样的 tidal.yaml，同样的接口 |
| 人类开发者 | `tidal deploy staging` 比记一堆 kubectl 命令方便 |
| CI/CD | GitHub Actions 里 `tidal test && tidal ship deploy` |

---

### 定位澄清（2026-03-20）

**Tidal 是工具，不是 agent。**

```
Agent (Claude Code / Codex / Devin)     ← 有脑子，做决策
  │
  ├── tidal test          ← 执行，返回 JSON
  ├── tidal observe       ← 执行，返回 JSON
  ├── tidal ship          ← 执行，返回 JSON
  ├── tidal verify        ← 执行，返回 JSON
  ├── tidal lint          ← 执行，返回 JSON
  ├── tidal worktree      ← 创建/销毁隔离环境
  └── tidal grade         ← 量化，返回 JSON
```

类比：
- `kubectl` 是工具，Kubernetes operator 是 agent
- `gh` 是工具，GitHub bot 是 agent
- `tidal` 是工具，Claude Code / Codex 是 agent

**Tidal 不做决策。** Agent 调用 tidal 拿到结构化结果，自己决定下一步。
要不要循环、要不要 escalate、要不要 review —— 都是 agent 的事。

Tidal 的职责边界：
- ✅ 执行声明在 tidal.yaml 里的命令
- ✅ 返回结构化结果（`--json`）
- ✅ 提供隔离执行环境
- ✅ 量化代码质量指标
- ❌ 自己做 review loop
- ❌ 自己决定要不要 refactor
- ❌ 自己决定要不要 escalate

---

### 实施路线（修订版）

基于 OpenAI "Zero Lines of Code" 文章的 insight 重新梳理。
只增加**工具能力**，不增加 agent 逻辑。

| Phase | 内容 | 给 agent 提供的能力 | 状态 |
|-------|------|-------------------|------|
| 0 | CLI 骨架 + tidal.yaml schema | 基础框架 | ✅ Done |
| 1 | `tidal test/observe/ship/verify` 全部跑通 | 完整的 observe→test→ship→verify 工具链 | ✅ Done |
| 2 | `tidal worktree create/destroy` | Agent 可以并行在隔离环境工作 | |
| 3 | `tidal lint` — 声明式规则引擎 | Agent 可以检查架构约束、命名、文件大小、import 方向 | |
| 4 | `tidal grade` — 质量评分 | Agent 可以量化当前代码质量，决定要不要 refactor | |
| 5 | `tidal observe` 扩展 metrics/traces | Agent 可以查 PromQL/TraceQL，不只是看日志 | |
| 6 | `tidal drive start/screenshot/record` | Agent 可以启动 app、截图、录视频来验证 UI | |

每个 phase 的产出都是：**agent 调一个命令，拿到 JSON 结果。**

---

### 技术栈

- **语言**: Go
- **CLI 框架**: cobra
- **配置**: tidal.yaml (gopkg.in/yaml.v3)
- **Repo**: https://github.com/oSEAItic/tidal
