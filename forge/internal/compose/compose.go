package compose

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
)

var candidates = []string{
	"docker-compose.yml",
	"compose.yaml",
	"compose.yml",
}

// DetectFile returns the path to a compose file in projectRoot, or empty string if none.
func DetectFile(projectRoot string) string {
	for _, c := range candidates {
		p := filepath.Join(projectRoot, c)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

// HasEnvFile returns true if .env exists in projectRoot.
func HasEnvFile(projectRoot string) bool {
	_, err := os.Stat(filepath.Join(projectRoot, ".env"))
	return err == nil
}

// Cmd returns the binary + leading args to invoke "docker compose" (v2)
// or "docker-compose" (v1) depending on what is available.
func Cmd() (string, []string, error) {
	if _, err := exec.LookPath("docker"); err == nil {
		// Prefer `docker compose` plugin.
		if err := exec.Command("docker", "compose", "version").Run(); err == nil {
			return "docker", []string{"compose"}, nil
		}
	}
	if _, err := exec.LookPath("docker-compose"); err == nil {
		return "docker-compose", nil, nil
	}
	return "", nil, errors.New("neither 'docker compose' nor 'docker-compose' is available")
}
