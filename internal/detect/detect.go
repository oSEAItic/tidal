package detect

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Result holds everything auto-detected about a repo.
type Result struct {
	Name     string
	Lang     string
	Test     map[string]string // name → cmd
	Lint     map[string]string
	Build    string
	CI       string
	Issues   string
	Deploy   map[string]string
	Topology []Service
	Paths    map[string]string
	External map[string]string
	Repo   string
	Refine []Hint
}

type Hint struct {
	File string
	Hint string
}

type Service struct {
	Name string
	Lang string
	Path string
}

// Scan analyzes the given directory and returns detected capabilities.
func Scan(dir string) Result {
	r := Result{
		Name:     filepath.Base(dir),
		Test:     make(map[string]string),
		Lint:     make(map[string]string),
		Deploy:   make(map[string]string),
		Paths:    make(map[string]string),
		External: make(map[string]string),
	}

	r.Repo = detectRepo(dir)

	// language detection (order matters — first match wins for primary lang)
	if exists(dir, "go.mod") {
		r.Lang = "go"
		r.Build = detectGoBuild(dir)
		r.Test["build"] = r.Build
		r.Test["unit"] = "go test ./... -short"
		r.Lint["vet"] = "go vet ./..."
	}
	if exists(dir, "package.json") {
		if r.Lang == "" {
			r.Lang = "typescript"
		}
		r.Test["unit"] = detectNPMScript(dir, "test")
		if lint := detectNPMScript(dir, "lint"); lint != "" {
			r.Lint["lint"] = lint
		}
		if build := detectNPMScript(dir, "build"); build != "" {
			r.Test["build"] = build
		}
	}
	if exists(dir, "pyproject.toml") || exists(dir, "requirements.txt") || exists(dir, "setup.py") {
		if r.Lang == "" {
			r.Lang = "python"
		}
		r.Test["unit"] = "pytest"
		if exists(dir, "ruff.toml") || existsInFile(dir, "pyproject.toml", "ruff") {
			r.Lint["ruff"] = "ruff check ."
		} else {
			r.Lint["flake8"] = "flake8 ."
		}
	}
	if exists(dir, "Cargo.toml") {
		if r.Lang == "" {
			r.Lang = "rust"
		}
		r.Test["unit"] = "cargo test"
		r.Test["build"] = "cargo build"
		r.Lint["clippy"] = "cargo clippy -- -D warnings"
	}
	if exists(dir, "Gemfile") {
		if r.Lang == "" {
			r.Lang = "ruby"
		}
		r.Test["unit"] = "bundle exec rspec"
		r.Lint["rubocop"] = "bundle exec rubocop"
	}

	// CI detection
	if dirExists(dir, ".github/workflows") {
		r.CI = "gh run list --repo {{repo}} --limit 5"
		r.Issues = "gh issue list --repo {{repo}} --state open"
		r.External["ci"] = "GitHub Actions"
		r.External["issues"] = "GitHub Issues"
		r.Paths["ci"] = ".github/workflows/"
	}
	if exists(dir, ".gitlab-ci.yml") {
		r.External["ci"] = "GitLab CI"
	}
	if exists(dir, "Jenkinsfile") {
		r.External["ci"] = "Jenkins"
	}

	// deploy detection
	if exists(dir, "Dockerfile") || exists(dir, "docker-compose.yml") || exists(dir, "docker-compose.yaml") || exists(dir, "compose.yaml") {
		r.Deploy["local"] = "docker compose up -d"
		r.External["container"] = "Docker"
	}
	if dirExists(dir, "k8s") || dirExists(dir, "kubernetes") || dirExists(dir, "deploy/k8s") {
		r.Deploy["staging"] = "kubectl apply -k k8s/overlays/staging"
		r.External["orchestration"] = "Kubernetes"
		kdir := "k8s/"
		if dirExists(dir, "kubernetes") {
			kdir = "kubernetes/"
		}
		r.Paths["k8s"] = kdir
	}
	if exists(dir, "vercel.json") || exists(dir, ".vercel") {
		r.Deploy["production"] = "vercel deploy --prod --token $VERCEL_TOKEN"
		r.External["hosting"] = "Vercel"
	}
	if exists(dir, "fly.toml") {
		r.Deploy["production"] = "fly deploy"
		r.External["hosting"] = "Fly.io"
	}
	if exists(dir, "terraform") || dirExists(dir, "terraform") {
		r.External["infra"] = "Terraform"
		r.Paths["terraform"] = "terraform/"
	}

	// path detection
	for _, p := range []string{"src/", "lib/", "pkg/", "internal/", "app/", "cmd/"} {
		if dirExists(dir, p) {
			r.Paths["source"] = p
			break
		}
	}
	for _, p := range []string{"tests/", "test/", "spec/", "__tests__/"} {
		if dirExists(dir, p) {
			r.Paths["tests"] = p
			break
		}
	}
	for _, p := range []string{"docs/", "doc/", "documentation/"} {
		if dirExists(dir, p) {
			r.Paths["docs"] = p
			break
		}
	}
	for _, p := range []string{"migrations/", "db/migrations/", "prisma/"} {
		if dirExists(dir, p) {
			r.Paths["migrations"] = p
			break
		}
	}
	if exists(dir, "README.md") {
		r.Paths["readme"] = "README.md"
	}

	// topology: scan for cmd/ subdirs (Go), or known service patterns
	if r.Lang == "go" && dirExists(dir, "cmd") {
		entries, _ := os.ReadDir(filepath.Join(dir, "cmd"))
		for _, e := range entries {
			if e.IsDir() {
				r.Topology = append(r.Topology, Service{
					Name: e.Name(),
					Lang: "go",
					Path: "cmd/" + e.Name(),
				})
			}
		}
	}
	if len(r.Topology) == 0 && r.Name != "" {
		r.Topology = append(r.Topology, Service{
			Name: r.Name,
			Lang: r.Lang,
			Path: ".",
		})
	}

	// refine: tell agents which files to read to improve this config
	refineTargets := []struct {
		file string
		hint string
	}{
		{"Makefile", "may contain actual build/test/deploy targets that override defaults"},
		{"README.md", "may describe setup steps, architecture, and deploy process"},
		{"CONTRIBUTING.md", "may describe dev workflow and testing requirements"},
		{"docker-compose.yml", "may define services, ports, and dependencies"},
		{"docker-compose.yaml", "may define services, ports, and dependencies"},
		{"compose.yaml", "may define services, ports, and dependencies"},
		{".env.example", "may list required environment variables"},
		{".github/workflows/ci.yml", "may show actual CI test/build/deploy commands"},
		{".github/workflows/ci.yaml", "may show actual CI test/build/deploy commands"},
		{".github/workflows/deploy.yml", "may show deploy process and environments"},
		{".github/workflows/deploy.yaml", "may show deploy process and environments"},
		{"AGENTS.md", "may contain agent-specific instructions and constraints"},
		{"CLAUDE.md", "may contain agent-specific instructions and constraints"},
		{"package.json", "scripts section may have additional commands beyond test/lint/build"},
		{"pyproject.toml", "may contain tool configs, extras, and script definitions"},
		{"Taskfile.yml", "may contain task definitions that replace Makefile targets"},
		{"justfile", "may contain task definitions that replace Makefile targets"},
	}
	for _, t := range refineTargets {
		if exists(dir, t.file) {
			r.Refine = append(r.Refine, Hint{File: t.file, Hint: t.hint})
		}
	}

	// also scan for CI workflow files dynamically
	if dirExists(dir, ".github/workflows") {
		entries, _ := os.ReadDir(filepath.Join(dir, ".github/workflows"))
		for _, e := range entries {
			name := ".github/workflows/" + e.Name()
			// skip ones we already added
			alreadyAdded := false
			for _, h := range r.Refine {
				if h.File == name {
					alreadyAdded = true
					break
				}
			}
			if !alreadyAdded && (strings.HasSuffix(e.Name(), ".yml") || strings.HasSuffix(e.Name(), ".yaml")) {
				r.Refine = append(r.Refine, Hint{
					File: name,
					Hint: "CI workflow — may contain test/build/deploy commands",
				})
			}
		}
	}

	return r
}

// ToYAML generates the tidal.yaml content from detection results.
func (r Result) ToYAML() string {
	var b strings.Builder

	b.WriteString("harness: v2\n")
	b.WriteString("name: " + r.Name + "\n")
	b.WriteString("lang: " + r.Lang + "\n")

	// observe
	b.WriteString("\nobserve:\n")
	b.WriteString("  logs:\n")
	b.WriteString("    - name: git\n")
	b.WriteString("      cmd: \"git log --oneline -20\"\n")
	if r.CI != "" {
		b.WriteString("  ci:\n")
		b.WriteString("    cmd: \"" + r.CI + "\"\n")
	}
	if r.Issues != "" {
		b.WriteString("  issues:\n")
		b.WriteString("    cmd: \"" + r.Issues + "\"\n")
	}

	// test
	if len(r.Test) > 0 {
		b.WriteString("\ntest:\n")
		for name, cmd := range r.Test {
			b.WriteString("  " + name + ":\n")
			b.WriteString("    cmd: \"" + cmd + "\"\n")
		}
	}

	// lint
	if len(r.Lint) > 0 {
		b.WriteString("\nlint:\n")
		for name, cmd := range r.Lint {
			b.WriteString("  " + name + ":\n")
			b.WriteString("    cmd: \"" + cmd + "\"\n")
		}
	}

	// review
	b.WriteString("\nreview:\n")
	b.WriteString("  diff:\n")
	b.WriteString("    cmd: \"git diff --stat\"\n")
	b.WriteString("  secrets:\n")
	b.WriteString("    cmd: \"git diff HEAD | grep -inE '(password|secret|api.?key|token)\\\\s*[:=]' || echo 'none detected'\"\n")
	b.WriteString("  todos:\n")
	b.WriteString("    cmd: \"git diff HEAD | grep -c TODO || echo 0\"\n")

	// ship
	b.WriteString("\nship:\n")
	b.WriteString("  pr:\n")
	b.WriteString("    base: main\n")
	b.WriteString("    prefix: \"tidal/\"\n")
	b.WriteString("    auto_test: true\n")
	b.WriteString("  issue:\n")
	b.WriteString("    repo: \"{{repo}}\"\n")
	b.WriteString("    types:\n")
	b.WriteString("      feat:\n")
	b.WriteString("        labels: [enhancement]\n")
	b.WriteString("      bug:\n")
	b.WriteString("        labels: [bug]\n")
	b.WriteString("      chore:\n")
	b.WriteString("        labels: [chore]\n")
	if len(r.Deploy) > 0 {
		b.WriteString("  deploy:\n")
		for name, cmd := range r.Deploy {
			b.WriteString("    " + name + ":\n")
			b.WriteString("      cmd: \"" + cmd + "\"\n")
		}
	}

	// verify
	b.WriteString("\nverify:\n")
	b.WriteString("  health:\n")
	if r.Build != "" {
		b.WriteString("    cmd: \"" + r.Build + " && echo 'build OK'\"\n")
	} else {
		b.WriteString("    cmd: \"echo 'no health check configured'\"\n")
	}

	// worktree
	b.WriteString("\nworktree:\n")
	b.WriteString("  dir: \"/tmp/tidal-worktrees\"\n")
	b.WriteString("  setup: \"\"\n")

	// grade
	b.WriteString("\ngrade:\n")
	b.WriteString("  file_count:\n")
	switch r.Lang {
	case "go":
		b.WriteString("    cmd: \"find . -name '*.go' -not -path './vendor/*' | wc -l | tr -d ' '\"\n")
	case "typescript", "javascript":
		b.WriteString("    cmd: \"find . -name '*.ts' -o -name '*.tsx' -o -name '*.js' -o -name '*.jsx' | grep -v node_modules | wc -l | tr -d ' '\"\n")
	case "python":
		b.WriteString("    cmd: \"find . -name '*.py' -not -path './.venv/*' | wc -l | tr -d ' '\"\n")
	case "rust":
		b.WriteString("    cmd: \"find . -name '*.rs' -not -path './target/*' | wc -l | tr -d ' '\"\n")
	default:
		b.WriteString("    cmd: \"find . -type f | wc -l | tr -d ' '\"\n")
	}

	// topology
	if len(r.Topology) > 0 {
		b.WriteString("\ntopology:\n")
		b.WriteString("  services:\n")
		for _, s := range r.Topology {
			b.WriteString("    - name: " + s.Name + "\n")
			b.WriteString("      lang: " + s.Lang + "\n")
			b.WriteString("      path: " + s.Path + "\n")
		}
	}

	// paths
	if len(r.Paths) > 0 {
		b.WriteString("\npaths:\n")
		for k, v := range r.Paths {
			b.WriteString("  " + k + ": \"" + v + "\"\n")
		}
	}

	// external
	if len(r.External) > 0 {
		b.WriteString("\nexternal:\n")
		for k, v := range r.External {
			b.WriteString("  " + k + ": \"" + v + "\"\n")
		}
	}

	// history
	b.WriteString("\nhistory:\n")
	b.WriteString("  dir: \".tidal\"\n")

	// refine
	if len(r.Refine) > 0 {
		b.WriteString("\n# ── refine: agent should read these files to improve this config ──\n")
		b.WriteString("refine:\n")
		for _, h := range r.Refine {
			b.WriteString("  - file: \"" + h.File + "\"\n")
			b.WriteString("    hint: \"" + h.Hint + "\"\n")
		}
	}

	// vars
	b.WriteString("\nvars:\n")
	repo := r.Repo
	if repo == "" {
		repo = "owner/" + r.Name
	}
	b.WriteString("  repo: \"" + repo + "\"\n")

	return b.String()
}

// helpers

func exists(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}

func dirExists(dir, name string) bool {
	info, err := os.Stat(filepath.Join(dir, name))
	return err == nil && info.IsDir()
}

func existsInFile(dir, name, substr string) bool {
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return false
	}
	return strings.Contains(string(data), substr)
}

func detectRepo(dir string) string {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	url := strings.TrimSpace(string(out))
	// https://github.com/owner/repo.git → owner/repo
	url = strings.TrimSuffix(url, ".git")
	if i := strings.Index(url, "github.com/"); i >= 0 {
		return url[i+len("github.com/"):]
	}
	if i := strings.Index(url, "github.com:"); i >= 0 {
		return url[i+len("github.com:"):]
	}
	return url
}

func detectGoBuild(dir string) string {
	if dirExists(dir, "cmd") {
		return "go build ./cmd/..."
	}
	return "go build ./..."
}

func detectNPMScript(dir, script string) string {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return ""
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if json.Unmarshal(data, &pkg) != nil {
		return ""
	}
	if _, ok := pkg.Scripts[script]; ok {
		// detect package manager
		pm := "npm"
		if exists(dir, "pnpm-lock.yaml") {
			pm = "pnpm"
		} else if exists(dir, "yarn.lock") {
			pm = "yarn"
		} else if exists(dir, "bun.lockb") {
			pm = "bun"
		}
		return pm + " run " + script
	}
	return ""
}
