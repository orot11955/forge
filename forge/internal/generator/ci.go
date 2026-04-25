package generator

import (
	"fmt"

	"github.com/orot/forge/internal/project"
)

func JenkinsfileFor(p *project.Config) string {
	switch p.Type {
	case "node", "next", "nest":
		pm := p.Runtime.PackageManager
		if pm == "" {
			pm = "npm"
		}
		install := pmInstall(pm)
		test := pmRun(pm, "test")
		build := pmRun(pm, "build")
		return fmt.Sprintf(`pipeline {
  agent any

  stages {
    stage('Install') {
      steps {
        sh '%s'
      }
    }
    stage('Test') {
      steps {
        sh '%s'
      }
    }
    stage('Build') {
      steps {
        sh '%s'
      }
    }
  }
}
`, install, test, build)
	case "go":
		return `pipeline {
  agent any

  stages {
    stage('Test') {
      steps {
        sh 'go test ./...'
      }
    }
    stage('Build') {
      steps {
        sh 'go build ./...'
      }
    }
  }
}
`
	case "java":
		return `pipeline {
  agent any

  stages {
    stage('Test') {
      steps {
        sh 'if [ -f mvnw ]; then ./mvnw test; elif [ -f gradlew ]; then ./gradlew test; else mvn test; fi'
      }
    }
    stage('Build') {
      steps {
        sh 'if [ -f mvnw ]; then ./mvnw package; elif [ -f gradlew ]; then ./gradlew build; else mvn package; fi'
      }
    }
  }
}
`
	default:
		return `pipeline {
  agent any

  stages {
    stage('Check') {
      steps {
        sh 'echo "Add project-specific CI steps here"'
      }
    }
  }
}
`
	}
}
