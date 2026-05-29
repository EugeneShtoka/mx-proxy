package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "*.toml")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(content)
	f.Close()
	return f.Name()
}

func TestLoadConfig_valid(t *testing.T) {
	path := writeConfig(t, `
[upstream]
  homeserver = "http://127.0.0.1:8008"
[listen]
  cs = "127.0.0.1:8900"
  as = "127.0.0.1:8901"
[[bridges]]
  name     = "test"
  url      = "http://127.0.0.1:9000"
  hs_token = "token123"
[processor]
  transport = "http"
  endpoint  = "http://127.0.0.1:9100/events"
`)
	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Upstream.Homeserver != "http://127.0.0.1:8008" {
		t.Errorf("homeserver: got %q", cfg.Upstream.Homeserver)
	}
	if len(cfg.Bridges) != 1 || cfg.Bridges[0].Name != "test" {
		t.Errorf("bridges: %+v", cfg.Bridges)
	}
	if cfg.Processor.Transport != "http" {
		t.Errorf("transport: got %q", cfg.Processor.Transport)
	}
}

func TestLoadConfig_missingHomeserver(t *testing.T) {
	path := writeConfig(t, `
[listen]
  cs = "127.0.0.1:8900"
  as = "127.0.0.1:8901"
[processor]
  transport = "http"
  endpoint  = "http://127.0.0.1:9100"
`)
	_, err := loadConfig(path)
	if err == nil {
		t.Error("expected error for missing homeserver")
	}
}

func TestLoadConfig_badTransport(t *testing.T) {
	path := writeConfig(t, `
[upstream]
  homeserver = "http://127.0.0.1:8008"
[listen]
  cs = "127.0.0.1:8900"
  as = "127.0.0.1:8901"
[processor]
  transport = "grpc"
  endpoint  = "127.0.0.1:9100"
`)
	_, err := loadConfig(path)
	if err == nil {
		t.Error("expected error for invalid transport")
	}
}

func TestLoadConfig_fileNotFound(t *testing.T) {
	_, err := loadConfig(filepath.Join(t.TempDir(), "nonexistent.toml"))
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestBridgeByHSToken(t *testing.T) {
	cfg := &Config{
		Bridges: []BridgeConfig{
			{Name: "a", HSToken: "tok-a"},
			{Name: "b", HSToken: "tok-b"},
		},
	}
	b := cfg.bridgeByHSToken("tok-b")
	if b == nil || b.Name != "b" {
		t.Errorf("got %+v", b)
	}
	if cfg.bridgeByHSToken("unknown") != nil {
		t.Error("expected nil for unknown token")
	}
}

func TestBridgeByName(t *testing.T) {
	cfg := &Config{
		Bridges: []BridgeConfig{
			{Name: "whatsapp", URL: "http://wa"},
		},
	}
	b := cfg.bridgeByName("whatsapp")
	if b == nil || b.URL != "http://wa" {
		t.Errorf("got %+v", b)
	}
	if cfg.bridgeByName("signal") != nil {
		t.Error("expected nil for unknown name")
	}
}
