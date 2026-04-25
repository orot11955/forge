package generator

import (
	"fmt"
	"strings"

	"github.com/orot/forge/internal/project"
)

// DockerfileFor builds a Dockerfile content based on project type/runtime.
func DockerfileFor(p *project.Config) (string, error) {
	switch p.Type {
	case "next":
		return nextDockerfile(p), nil
	case "nest":
		return nestDockerfile(p), nil
	case "node":
		return nodeDockerfile(p), nil
	case "go":
		return goDockerfile(p), nil
	case "java":
		return javaDockerfile(p), nil
	case "generic":
		return genericDockerfile(), nil
	}
	return "", fmt.Errorf("unsupported project type: %s", p.Type)
}

// DockerignoreFor returns a sensible .dockerignore for the project type.
func DockerignoreFor(p *project.Config) string {
	common := []string{".forge/", ".git/", ".env", "*.log", "node_modules"}
	switch p.Type {
	case "next", "nest", "node":
		common = append(common, ".next", "dist", "build", "coverage")
	case "go":
		common = append(common, "bin", "*.test")
	case "java":
		common = append(common, "target", "build")
	}
	return strings.Join(common, "\n") + "\n"
}

func pmInstall(pm string) string {
	switch pm {
	case "yarn":
		return "yarn install --frozen-lockfile"
	case "pnpm":
		return "pnpm install --frozen-lockfile"
	case "npm":
		return "npm ci"
	case "bun":
		return "bun install --frozen-lockfile"
	default:
		return "npm ci"
	}
}

func pmRun(pm, script string) string {
	switch pm {
	case "npm":
		return "npm run " + script
	default:
		return pm + " " + script
	}
}

func nodeBaseImage() string { return "node:22-alpine" }

func nextDockerfile(p *project.Config) string {
	pm := p.Runtime.PackageManager
	if pm == "" {
		pm = "npm"
	}
	return fmt.Sprintf(`# syntax=docker/dockerfile:1.7
# --- deps ---
FROM %s AS deps
WORKDIR /app
COPY package.json yarn.lock* package-lock.json* pnpm-lock.yaml* ./
RUN %s

# --- builder ---
FROM %s AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN %s

# --- runner ---
FROM %s AS runner
WORKDIR /app
ENV NODE_ENV=production
COPY --from=builder /app/.next ./.next
COPY --from=builder /app/public ./public
COPY --from=builder /app/package.json ./package.json
COPY --from=builder /app/node_modules ./node_modules
EXPOSE 3000
CMD ["%s", "start"]
`, nodeBaseImage(), pmInstall(pm), nodeBaseImage(), pmRun(pm, "build"), nodeBaseImage(), pm)
}

func nestDockerfile(p *project.Config) string {
	pm := p.Runtime.PackageManager
	if pm == "" {
		pm = "npm"
	}
	return fmt.Sprintf(`# syntax=docker/dockerfile:1.7
FROM %s AS deps
WORKDIR /app
COPY package.json yarn.lock* package-lock.json* pnpm-lock.yaml* ./
RUN %s

FROM %s AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN %s

FROM %s AS runner
WORKDIR /app
ENV NODE_ENV=production
COPY --from=builder /app/dist ./dist
COPY --from=builder /app/package.json ./package.json
COPY --from=builder /app/node_modules ./node_modules
EXPOSE 3000
CMD ["node", "dist/main.js"]
`, nodeBaseImage(), pmInstall(pm), nodeBaseImage(), pmRun(pm, "build"), nodeBaseImage())
}

func nodeDockerfile(p *project.Config) string {
	pm := p.Runtime.PackageManager
	if pm == "" {
		pm = "npm"
	}
	return fmt.Sprintf(`# syntax=docker/dockerfile:1.7
FROM %s AS app
WORKDIR /app
COPY package.json yarn.lock* package-lock.json* pnpm-lock.yaml* ./
RUN %s
COPY . .
ENV NODE_ENV=production
EXPOSE 3000
CMD ["%s", "start"]
`, nodeBaseImage(), pmInstall(pm), pm)
}

func goDockerfile(p *project.Config) string {
	bin := p.Name
	if bin == "" {
		bin = "app"
	}
	return fmt.Sprintf(`# syntax=docker/dockerfile:1.7
FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download || true
COPY . .
RUN CGO_ENABLED=0 go build -o /out/%s ./...

FROM gcr.io/distroless/static-debian12
COPY --from=builder /out/%s /app
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app"]
`, bin, bin)
}

func javaDockerfile(p *project.Config) string {
	return `# syntax=docker/dockerfile:1.7
FROM eclipse-temurin:21-jdk AS builder
WORKDIR /src
COPY . .
RUN if [ -f mvnw ]; then ./mvnw -B -DskipTests package; \
    elif [ -f gradlew ]; then ./gradlew --no-daemon build -x test; \
    else echo "No build wrapper found" && exit 1; fi
RUN cp $(ls target/*.jar build/libs/*.jar 2>/dev/null | head -n1) /app.jar

FROM eclipse-temurin:21-jre
COPY --from=builder /app.jar /app.jar
EXPOSE 8080
ENTRYPOINT ["java", "-jar", "/app.jar"]
`
}

func genericDockerfile() string {
	return `# Generic placeholder Dockerfile.
# Replace with the runtime image and build steps for this project.
FROM alpine:3.20
WORKDIR /app
COPY . .
CMD ["sh"]
`
}
