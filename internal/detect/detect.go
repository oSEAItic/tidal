package detect

import (
	"encoding/json"
	"fmt"
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
	Name      string
	Lang      string
	Type      string   // e.g. "postgres", "redis"
	Path      string
	Port      int
	DependsOn []string
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

	// language detection — scan root AND immediate subdirs (monorepo/multi-service)
	detectLang(dir, &r, "")
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") && e.Name() != "node_modules" && e.Name() != "vendor" {
			detectLang(filepath.Join(dir, e.Name()), &r, e.Name())
		}
	}

	// Makefile parsing — overrides detected commands
	parseMakefile(dir, &r)

	// docker-compose parsing — extract services for topology
	parseDockerCompose(dir, &r)

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
	// k8s detection — check multiple common patterns
	for _, kp := range []struct{ dir, path string }{
		{"k8s", "k8s/"},
		{"kubernetes", "kubernetes/"},
		{"deploy/k8s", "deploy/k8s/"},
		{"deploy/kube", "deploy/kube/"},
		{"deploy", "deploy/"},
	} {
		if dirExists(dir, kp.dir) {
			// check if it actually contains k8s manifests
			hasYAML := false
			kEntries, _ := os.ReadDir(filepath.Join(dir, kp.dir))
			for _, ke := range kEntries {
				if strings.HasSuffix(ke.Name(), ".yml") || strings.HasSuffix(ke.Name(), ".yaml") {
					hasYAML = true
					break
				}
			}
			if hasYAML || kp.dir == "k8s" || kp.dir == "kubernetes" {
				r.Deploy["k8s"] = "kubectl apply -f " + kp.path
				r.External["orchestration"] = "Kubernetes"
				r.Paths["k8s"] = kp.path
				break
			}
		}
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
			if s.Lang != "" {
				b.WriteString("      lang: " + s.Lang + "\n")
			}
			if s.Type != "" {
				b.WriteString("      type: " + s.Type + "\n")
			}
			if s.Path != "" {
				b.WriteString("      path: " + s.Path + "\n")
			}
			if s.Port > 0 {
				b.WriteString(fmt.Sprintf("      port: %d\n", s.Port))
			}
			if len(s.DependsOn) > 0 {
				b.WriteString("      depends_on: [" + strings.Join(s.DependsOn, ", ") + "]\n")
			}
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

// GenerateCIWorkflow returns a GitHub Actions workflow that runs tidal.
func GenerateCIWorkflow() string {
	return `name: tidal
on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  harness:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install tidal
        run: |
          curl -sf https://raw.githubusercontent.com/oSEAItic/tidal/main/install.sh | sh
          echo "$HOME/.local/bin" >> $GITHUB_PATH

      - name: Test
        run: tidal test --json

      - name: Lint
        run: tidal lint --json
        continue-on-error: true

      - name: Review
        run: tidal review --json
        continue-on-error: true

      - name: Grade
        run: tidal grade --json
        continue-on-error: true
`
}

// ── deep detection functions ──

// detectLang scans a directory for language markers and adds to result.
// subdir is "" for root, or "backend"/"frontend" etc for subdirs.
func detectLang(dir string, r *Result, subdir string) {
	prefix := ""
	if subdir != "" {
		prefix = subdir + "/"
	}

	if exists(dir, "go.mod") {
		if r.Lang == "" {
			r.Lang = "go"
		}
		build := detectGoBuild(dir)
		if subdir == "" {
			r.Build = build
			r.Test["build"] = build
			r.Test["unit"] = "go test ./... -short"
			r.Lint["vet"] = "go vet ./..."
		} else {
			r.Test["build:"+subdir] = "cd " + subdir + " && go build ./..."
			r.Test["unit:"+subdir] = "cd " + subdir + " && go test ./... -short"
			r.Lint["vet:"+subdir] = "cd " + subdir + " && go vet ./..."
		}
		r.Paths[subdir+"_source"] = prefix
	}
	if exists(dir, "package.json") {
		if r.Lang == "" {
			r.Lang = "typescript"
		}
		if subdir == "" {
			if t := detectNPMScript(dir, "test"); t != "" {
				r.Test["unit"] = t
			}
			if l := detectNPMScript(dir, "lint"); l != "" {
				r.Lint["lint"] = l
			}
			if b := detectNPMScript(dir, "build"); b != "" {
				r.Test["build"] = b
			}
		} else {
			pm := detectPM(dir)
			if t := detectNPMScript(dir, "test"); t != "" {
				r.Test["unit:"+subdir] = "cd " + subdir + " && " + t
			}
			if l := detectNPMScript(dir, "lint"); l != "" {
				r.Lint["lint:"+subdir] = "cd " + subdir + " && " + l
			}
			if b := detectNPMScript(dir, "build"); b != "" {
				r.Test["build:"+subdir] = "cd " + subdir + " && " + b
			}
			_ = pm
		}
		r.Paths[subdir+"_source"] = prefix
	}
	if exists(dir, "pyproject.toml") || exists(dir, "requirements.txt") || exists(dir, "setup.py") {
		if r.Lang == "" {
			r.Lang = "python"
		}
		hasPoetry := exists(dir, "poetry.lock")
		if subdir == "" {
			if hasPoetry {
				r.Test["unit"] = "poetry run pytest"
			} else {
				r.Test["unit"] = "pytest"
			}
			if exists(dir, "ruff.toml") || existsInFile(dir, "pyproject.toml", "ruff") {
				if hasPoetry {
					r.Lint["ruff"] = "poetry run ruff check ."
				} else {
					r.Lint["ruff"] = "ruff check ."
				}
			}
		} else {
			if hasPoetry {
				r.Test["unit:"+subdir] = "cd " + subdir + " && poetry run pytest"
			} else {
				r.Test["unit:"+subdir] = "cd " + subdir + " && pytest"
			}
			if existsInFile(dir, "pyproject.toml", "ruff") {
				if hasPoetry {
					r.Lint["ruff:"+subdir] = "cd " + subdir + " && poetry run ruff check ."
				} else {
					r.Lint["ruff:"+subdir] = "cd " + subdir + " && ruff check ."
				}
			}
		}
		r.Paths[subdir+"_source"] = prefix
	}
	if exists(dir, "Cargo.toml") {
		if r.Lang == "" {
			r.Lang = "rust"
		}
		if subdir == "" {
			r.Test["unit"] = "cargo test"
			r.Test["build"] = "cargo build"
			r.Lint["clippy"] = "cargo clippy -- -D warnings"
		} else {
			r.Test["unit:"+subdir] = "cd " + subdir + " && cargo test"
			r.Test["build:"+subdir] = "cd " + subdir + " && cargo build"
			r.Lint["clippy:"+subdir] = "cd " + subdir + " && cargo clippy -- -D warnings"
		}
	}
}

// parseMakefile reads Makefile and extracts known targets.
// If Makefile has test/lint/format/deploy targets, they override detected commands.
func parseMakefile(dir string, r *Result) {
	data, err := os.ReadFile(filepath.Join(dir, "Makefile"))
	if err != nil {
		return
	}

	lines := strings.Split(string(data), "\n")
	targets := make(map[string]bool)
	for _, line := range lines {
		if len(line) > 0 && line[0] != '\t' && line[0] != '#' && line[0] != '.' {
			if idx := strings.Index(line, ":"); idx > 0 {
				target := strings.TrimSpace(line[:idx])
				if !strings.ContainsAny(target, " =$(){}") {
					targets[target] = true
				}
			}
		}
	}

	// override with make targets (they're the canonical commands)
	if targets["test"] {
		r.Test["unit"] = "make test"
	}
	if targets["lint"] {
		r.Lint["lint"] = "make lint"
	}
	if targets["format"] || targets["fmt"] {
		r.Lint["format"] = "make format"
	}
	if targets["build"] {
		r.Test["build"] = "make build"
		r.Build = "make build"
	}
	if targets["docker-build"] {
		r.Test["docker-build"] = "make docker-build"
	}
	if targets["docker-up"] {
		r.Deploy["local"] = "make docker-up"
	}
	if targets["docker-down"] {
		r.Deploy["local-down"] = "make docker-down"
	}
	if targets["deploy"] {
		r.Deploy["production"] = "make deploy"
	}
	if targets["install"] {
		r.Build = "make install"
	}
	if targets["dev"] {
		r.Deploy["dev"] = "make dev"
	}
}

// parseDockerCompose reads docker-compose.yml and extracts services for topology.
func parseDockerCompose(dir string, r *Result) {
	var data []byte
	var err error
	for _, name := range []string{"docker-compose.yml", "docker-compose.yaml", "compose.yaml", "compose.yml"} {
		data, err = os.ReadFile(filepath.Join(dir, name))
		if err == nil {
			break
		}
	}
	if err != nil {
		return
	}

	// simple YAML parsing for docker-compose services
	// we don't import a full YAML parser to keep detect lightweight
	type composeService struct {
		name      string
		image     string
		build     string
		ports     []string
		dependsOn []string
	}

	var services []composeService
	lines := strings.Split(string(data), "\n")
	inServices := false
	currentService := ""
	currentIndent := 0
	inDependsOn := false
	inPorts := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		indent := len(line) - len(strings.TrimLeft(line, " "))

		if trimmed == "services:" {
			inServices = true
			currentIndent = indent
			continue
		}

		if inServices && indent == currentIndent+2 && strings.HasSuffix(trimmed, ":") {
			// new service
			currentService = strings.TrimSuffix(trimmed, ":")
			services = append(services, composeService{name: currentService})
			inDependsOn = false
			inPorts = false
			continue
		}

		if currentService == "" {
			continue
		}

		svc := &services[len(services)-1]

		if indent == currentIndent+4 {
			inDependsOn = false
			inPorts = false

			if strings.HasPrefix(trimmed, "image:") {
				svc.image = strings.TrimSpace(strings.TrimPrefix(trimmed, "image:"))
			} else if strings.HasPrefix(trimmed, "build:") {
				svc.build = strings.TrimSpace(strings.TrimPrefix(trimmed, "build:"))
			} else if trimmed == "ports:" {
				inPorts = true
			} else if trimmed == "depends_on:" {
				inDependsOn = true
			}
		} else if indent > currentIndent+4 {
			if inPorts && strings.HasPrefix(trimmed, "- \"") {
				port := strings.Trim(trimmed[2:], "\"")
				svc.ports = append(svc.ports, port)
			} else if inPorts && strings.HasPrefix(trimmed, "- ") {
				port := strings.TrimPrefix(trimmed, "- ")
				svc.ports = append(svc.ports, port)
			} else if inDependsOn && strings.HasPrefix(trimmed, "- ") {
				dep := strings.TrimPrefix(trimmed, "- ")
				svc.dependsOn = append(svc.dependsOn, dep)
			} else if inDependsOn && strings.HasSuffix(trimmed, ":") {
				// depends_on with condition syntax
				dep := strings.TrimSuffix(trimmed, ":")
				svc.dependsOn = append(svc.dependsOn, dep)
			}
		}

		// top-level key other than services — stop parsing services
		if inServices && indent == currentIndent && trimmed != "services:" {
			break
		}
	}

	// convert to topology
	if len(services) > 0 {
		r.Topology = nil // clear default
		for _, svc := range services {
			s := Service{Name: svc.name}

			// detect lang/type from image or build context
			if svc.image != "" {
				img := strings.ToLower(svc.image)
				if strings.Contains(img, "postgres") {
					s.Type = "postgres"
				} else if strings.Contains(img, "redis") {
					s.Type = "redis"
				} else if strings.Contains(img, "mongo") {
					s.Type = "mongodb"
				} else if strings.Contains(img, "mysql") {
					s.Type = "mysql"
				} else if strings.Contains(img, "nginx") {
					s.Type = "nginx"
				} else if strings.Contains(img, "rabbitmq") {
					s.Type = "rabbitmq"
				}
			}
			if svc.build != "" {
				s.Path = svc.build
				// detect lang from build context
				buildDir := strings.TrimPrefix(svc.build, "./")
				if exists(filepath.Join(dir, buildDir), "go.mod") {
					s.Lang = "go"
				} else if exists(filepath.Join(dir, buildDir), "package.json") {
					s.Lang = "typescript"
				} else if exists(filepath.Join(dir, buildDir), "pyproject.toml") || exists(filepath.Join(dir, buildDir), "requirements.txt") {
					s.Lang = "python"
				} else if exists(filepath.Join(dir, buildDir), "Cargo.toml") {
					s.Lang = "rust"
				}
			}

			// extract host port from port mapping
			if len(svc.ports) > 0 {
				port := svc.ports[0]
				if parts := strings.SplitN(port, ":", 2); len(parts) == 2 {
					fmt.Sscanf(parts[0], "%d", &s.Port)
				}
			}

			s.DependsOn = svc.dependsOn
			r.Topology = append(r.Topology, s)
		}
	}
}

// helpers

func detectPM(dir string) string {
	if exists(dir, "pnpm-lock.yaml") {
		return "pnpm"
	} else if exists(dir, "yarn.lock") {
		return "yarn"
	} else if exists(dir, "bun.lockb") {
		return "bun"
	}
	return "npm"
}

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
