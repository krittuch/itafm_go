package main

import (
	"os"
	"strings"
	"testing"
)

type stackConfig struct {
	name             string
	composeFile      string
	kafkaEnvFile     string
	kafkaServiceName string
}

var stackConfigs = []stackConfig{
	{
		name:             "production",
		composeFile:      "production.yml",
		kafkaEnvFile:     ".envs/.production/.kafka",
		kafkaServiceName: "itafm_kafka",
	},
	{
		name:             "development",
		composeFile:      "development.yml",
		kafkaEnvFile:     ".envs/.development/.kafka",
		kafkaServiceName: "itafm_kafka",
	},
	{
		name:             "local",
		composeFile:      "local.yml",
		kafkaEnvFile:     ".envs/.local/.kafka",
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

func TestJavaBrokerServiceUsesKafkaEnvFile(t *testing.T) {
	for _, stack := range stackConfigs {
		stack := stack
		t.Run(stack.name, func(t *testing.T) {
			composeContent := readFile(t, stack.composeFile)
			javaBrokerService := serviceBlock(t, composeContent, "java_broker")

			mustContain(t, javaBrokerService, stack.kafkaEnvFile)

			kafkaEnvContent := readFile(t, stack.kafkaEnvFile)
			if got := readEnvValue(kafkaEnvContent, "KAFKA_BROKERS"); got != "localhost:9099" {
				t.Fatalf("expected KAFKA_BROKERS=localhost:9099 in %s, got %q", stack.kafkaEnvFile, got)
			}
		})
	}
}

func TestGatewayServiceUsesKafkaEnvFile(t *testing.T) {
	for _, stack := range stackConfigs {
		stack := stack
		t.Run(stack.name, func(t *testing.T) {
			composeContent := readFile(t, stack.composeFile)
			gatewayService := serviceBlock(t, composeContent, "gateway")

			mustContain(t, gatewayService, stack.kafkaEnvFile)

			kafkaEnvContent := readFile(t, stack.kafkaEnvFile)
			if got := readEnvValue(kafkaEnvContent, "KAFKA_BROKERS"); got != "localhost:9099" {
				t.Fatalf("expected KAFKA_BROKERS=localhost:9099 in %s, got %q", stack.kafkaEnvFile, got)
			}
		})
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
