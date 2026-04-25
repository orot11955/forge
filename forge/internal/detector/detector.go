package detector

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Result struct {
	Type           string
	PackageManager string
	HasGit         bool
	HasDockerfile  bool
	HasCompose     bool
	NodeScripts    map[string]string
	Warnings       []string
	Paths          map[string]string
}

func Detect(root string) Result {
	r := Result{Type: "generic", Paths: map[string]string{}}
	r.HasGit = exists(filepath.Join(root, ".git"))
	r.HasDockerfile = exists(filepath.Join(root, "Dockerfile"))
	r.HasCompose = exists(filepath.Join(root, "docker-compose.yml")) ||
		exists(filepath.Join(root, "compose.yaml")) ||
		exists(filepath.Join(root, "compose.yml"))

	// Go
	if exists(filepath.Join(root, "go.mod")) {
		r.Type = "go"
		return r
	}
	// Java
	if exists(filepath.Join(root, "pom.xml")) ||
		exists(filepath.Join(root, "build.gradle")) ||
		exists(filepath.Join(root, "build.gradle.kts")) {
		r.Type = "java"
		return r
	}
	// Node family
	pkgPath := filepath.Join(root, "package.json")
	if exists(pkgPath) {
		r.Type = "node"
		r.PackageManager = detectPackageManager(root)
		nextSeen, nestSeen, scripts := readPackageJSON(pkgPath)
		r.NodeScripts = scripts
		switch {
		case nextSeen && nestSeen:
			r.Type = "node"
			r.Warnings = append(r.Warnings, "multiple framework signals detected (next, nest); falling back to type=node")
		case nextSeen:
			r.Type = "next"
		case nestSeen:
			r.Type = "nest"
		}
		if exists(filepath.Join(root, "next.config.js")) ||
			exists(filepath.Join(root, "next.config.mjs")) ||
			exists(filepath.Join(root, "next.config.ts")) {
			if r.Type == "node" {
				r.Type = "next"
			}
		}
		if exists(filepath.Join(root, "nest-cli.json")) && r.Type == "node" {
			r.Type = "nest"
		}
	}
	return r
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func detectPackageManager(root string) string {
	switch {
	case exists(filepath.Join(root, "yarn.lock")):
		return "yarn"
	case exists(filepath.Join(root, "pnpm-lock.yaml")):
		return "pnpm"
	case exists(filepath.Join(root, "package-lock.json")):
		return "npm"
	case exists(filepath.Join(root, "bun.lockb")), exists(filepath.Join(root, "bun.lock")):
		return "bun"
	}
	return "npm"
}

type pkgJSON struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
	Scripts         map[string]string `json:"scripts"`
}

func readPackageJSON(path string) (nextSeen, nestSeen bool, scripts map[string]string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, false, nil
	}
	var p pkgJSON
	if json.Unmarshal(data, &p) != nil {
		return false, false, nil
	}
	if _, ok := p.Dependencies["next"]; ok {
		nextSeen = true
	}
	if _, ok := p.DevDependencies["next"]; ok {
		nextSeen = true
	}
	if _, ok := p.Dependencies["@nestjs/core"]; ok {
		nestSeen = true
	}
	if _, ok := p.DevDependencies["@nestjs/core"]; ok {
		nestSeen = true
	}
	return nextSeen, nestSeen, p.Scripts
}
