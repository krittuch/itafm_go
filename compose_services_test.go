package main

import (
	"os"
	"strings"
	"testing"
)

type stackConfig struct {
	name             string
	composeFile      string
	kafkaServiceName string
}

var stackConfigs = []stackConfig{
	{
		name:             "production",
		composeFile:      "production.yml",
		kafkaServiceName: "itafm_kafka",
	},
	{
		name:             "development",
		composeFile:      "development.yml",
		kafkaServiceName: "itafm_kafka",
	},
	{
		name:             "local",
		composeFile:      "local.yml",
		kafkaServiceName: "kafka",
	},
}

func TestKafkaServiceUsesHostPort9099(t *testing.T) {
	for _, stack := range stackConfigs {
		stack := stack
		t.Run(stack.name, func(t *testing.T) {
			composeContent := readFile(t, stack.composeFile)
			kafkaService := serviceBlock(t, composeContent, stack.kafkaServiceName)

			mustContain(t, kafkaService, "- \"9099:9092\"")
			mustContain(t, kafkaService, "--kafka-addr=PLAINTEXT://0.0.0.0:9092")
			mustContain(t, kafkaService, "--advertise-kafka-addr=PLAINTEXT://localhost:9099")
		})
	}
}

func TestGatewayAndJavaBrokerReadDotEnv(t *testing.T) {
	for _, file := range []string{"production.yml", "development.yml", "local.yml", "stack.development.yml"} {
		composeContent := readFile(t, file)
		if strings.Contains(composeContent, ".envs") {
			t.Fatalf("%s must not use .envs", file)
		}
		for _, service := range []string{"gateway", "java_broker"} {
			mustContain(t, serviceBlock(t, composeContent, service), "env_file:\n      - ./.env\n")
		}
	}

	if got := readEnvValue(readFile(t, ".env.example"), "KAFKA_BROKERS"); got != "localhost:9099" {
		t.Fatalf("expected KAFKA_BROKERS=localhost:9099 in .env.example, got %q", got)
	}
}

func TestProductionGatewayRunsSurveillanceAndIDEPOnly(t *testing.T) {
	gatewayService := serviceBlock(t, readFile(t, "production.yml"), "gateway")

	mustContain(t, gatewayService, `GATEWAY_FLIGHT_MOVEMENT_ENABLED: "false"`)
	if strings.Contains(gatewayService, "AODS_ARCHIVE_DB_") {
		t.Fatal("production gateway must not configure the AODS archive DB")
	}
}

func TestDevDeployWritesDotEnvFromEnvironmentSecrets(t *testing.T) {
	workflow := readFile(t, ".github/workflows/dev.yml")
	mustContain(t, workflow, "environment: development")

	// Variable values show in the public Actions logs, so anything that looks
	// like a host, login or key must be a secret.
	sensitive := []string{"HOST", "USER", "PASSWORD", "KEYS", "URL", "IP_ADDRESS"}

	source := ""
	keys := map[string]int{}
	for _, line := range strings.Split(readFile(t, ".env.example"), "\n") {
		switch {
		case strings.HasPrefix(line, "# === Secrets"):
			source = "secrets"
			continue
		case strings.HasPrefix(line, "# === Variables"):
			source = "vars"
			continue
		}
		key, _, ok := strings.Cut(line, "=")
		if !ok || strings.HasPrefix(key, "#") {
			continue
		}
		if source == "" {
			t.Fatalf("%s is outside the Secrets and Variables blocks of .env.example", key)
		}
		keys[source]++
		mustContain(t, workflow, key+": ${{ "+source+"."+key+" }}")
		if source == "vars" {
			for _, part := range sensitive {
				if strings.Contains(key, part) {
					t.Fatalf("%s looks sensitive and must be in the Secrets block", key)
				}
			}
		}
	}
	if keys["secrets"] == 0 || keys["vars"] == 0 {
		t.Fatalf(".env.example must list both secrets and variables, got %v", keys)
	}

	for ignoreFile, entries := range map[string][]string{
		".gitignore":    {".env", ".envs/"},
		".dockerignore": {".env", ".envs"},
	} {
		content := "\n" + readFile(t, ignoreFile) + "\n"
		for _, entry := range entries {
			if !strings.Contains(content, "\n"+entry+"\n") {
				t.Fatalf("%s must ignore %s", ignoreFile, entry)
			}
		}
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}

	return string(content)
}

func serviceBlock(t *testing.T, composeContent string, serviceName string) string {
	t.Helper()

	lines := strings.Split(composeContent, "\n")
	var builder strings.Builder
	inTargetService := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") && strings.HasSuffix(trimmed, ":") {
			name := strings.TrimSuffix(trimmed, ":")

			if inTargetService && name != serviceName {
				break
			}

			if name == serviceName {
				inTargetService = true
				builder.WriteString(line)
				builder.WriteString("\n")
				continue
			}
		}

		if inTargetService {
			builder.WriteString(line)
			builder.WriteString("\n")
		}
	}

	if !inTargetService {
		t.Fatalf("service %q not found in compose file", serviceName)
	}

	return builder.String()
}

func readEnvValue(content string, key string) string {
	lines := strings.Split(content, "\n")
	prefix := key + "="

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if strings.HasPrefix(trimmed, prefix) {
			return strings.TrimPrefix(trimmed, prefix)
		}
	}

	return ""
}

func mustContain(t *testing.T, content string, expected string) {
	t.Helper()

	if !strings.Contains(content, expected) {
		t.Fatalf("expected to find %q in service block:\n%s", expected, content)
	}
}
