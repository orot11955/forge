package generator

import (
	"fmt"

	"github.com/orot/forge/internal/project"
)

// ComposeFor returns a single-service docker-compose.yml for the project.
func ComposeFor(p *project.Config) string {
	port := defaultPort(p.Type)
	return fmt.Sprintf(`version: "3.9"

services:
  app:
    build:
      context: .
      dockerfile: Dockerfile
    image: %s:local
    container_name: %s
    restart: unless-stopped
    env_file:
      - .env
    ports:
      - "%d:%d"
`, p.ID, p.ID, port, port)
}

func defaultPort(t string) int {
	switch t {
	case "next", "node", "nest":
		return 3000
	case "go":
		return 8080
	case "java":
		return 8080
	}
	return 8080
}
