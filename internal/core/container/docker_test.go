package container

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseDockerfile(t *testing.T) {
	tmp := t.TempDir()
	dockerfile := filepath.Join(tmp, "Dockerfile")
	content := "" +
		"FROM ubuntu:22.04\n" +
		"EXPOSE 80 443\n" +
		"ENV INSTALL_NODE 18\n" +
		"ENV INSTALL_PYTHON 3\n" +
		"CMD [\"npm\", \"start\"]\n"
	if err := os.WriteFile(dockerfile, []byte(content), 0644); err != nil {
		t.Fatalf("write dockerfile: %v", err)
	}

	cfg, err := parseDockerfile(dockerfile)
	if err != nil {
		t.Fatalf("parseDockerfile error: %v", err)
	}

	if cfg.From != "ubuntu:22.04" {
		t.Fatalf("from mismatch: %q", cfg.From)
	}
	if len(cfg.Expose) != 2 || cfg.Expose[0] != "80" || cfg.Expose[1] != "443" {
		t.Fatalf("expose mismatch: %#v", cfg.Expose)
	}
	if got := cfg.Env["INSTALL_NODE"]; got != "18" {
		t.Fatalf("env INSTALL_NODE mismatch: %q", got)
	}
	if got := cfg.Env["INSTALL_PYTHON"]; got != "3" {
		t.Fatalf("env INSTALL_PYTHON mismatch: %q", got)
	}
	if len(cfg.Cmd) != 2 || cfg.Cmd[0] != "npm" || cfg.Cmd[1] != "start" {
		t.Fatalf("cmd mismatch: %#v", cfg.Cmd)
	}
}

func TestGenerateInstallCommands(t *testing.T) {
	cmds := generateInstallCommands(map[string]string{
		"INSTALL_NODE":   "16",
		"INSTALL_RUST":   "latest",
		"INSTALL_PYTHON": "3",
		"INSTALL_GOLANG": "1.20.0",
		"INSTALL_JAVA":   "11",
	})

	expectContains := []string{
		"curl -fsSL https://deb.nodesource.com/setup_lts.x | bash -",
		"apt-get install -y nodejs",
		"npm install -g n && n 16",
		"curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y",
		"apt-get update && apt-get install -y python3 python3-pip",
		"wget https://go.dev/dl/go1.20.0.linux-amd64.tar.gz",
		"apt-get update && apt-get install -y openjdk-11-jdk",
	}

	for _, want := range expectContains {
		found := false
		for _, cmd := range cmds {
			if cmd == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected command not found: %q\ncmds=%#v", want, cmds)
		}
	}
}

func TestConvertToQubeYaml(t *testing.T) {
	cfg := &DockerfileConfig{
		From:   "alpine:3.19",
		Expose: []string{"8080"},
		Cmd:    []string{"npm", "start"},
		Env: map[string]string{
			"FOO": "bar",
		},
	}
	cfg.InstallCmds = []string{"echo install"}

	qube := convertToQubeYaml(cfg)

	if qube.System != "alpine:3.19" {
		t.Fatalf("system mismatch: %q", qube.System)
	}
	if len(qube.Ports) != 1 || qube.Ports[0] != "8080" {
		t.Fatalf("ports mismatch: %#v", qube.Ports)
	}
	cmdSlice, ok := qube.Cmd.([]string)
	if !ok {
		t.Fatalf("cmd type unexpected: %T", qube.Cmd)
	}
	if len(cmdSlice) != 3 || cmdSlice[0] != "echo install" || cmdSlice[1] != "npm" || cmdSlice[2] != "start" {
		t.Fatalf("cmd content mismatch: %#v", cmdSlice)
	}
	envStr, ok := qube.Environment.(string)
	if !ok || envStr != "FOO=bar" {
		t.Fatalf("env mismatch: %#v", qube.Environment)
	}
	if !qube.Isolated {
		t.Fatalf("expected isolated true")
	}
}
