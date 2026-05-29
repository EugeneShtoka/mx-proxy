package main

import (
	"fmt"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Upstream  UpstreamConfig  `toml:"upstream"`
	Listen    ListenConfig    `toml:"listen"`
	Bridges   []BridgeConfig  `toml:"bridges"`
	Processor ProcessorConfig `toml:"processor"`
}

type UpstreamConfig struct {
	Homeserver string `toml:"homeserver"`
	// ASToken is used to impersonate any user when re-injecting events whose
	// sender token was never cached (e.g. messages from remote Matrix clients
	// that connect directly to the homeserver, bypassing the CS proxy).
	// Set this to a doublepuppet appservice token that covers all users.
	ASToken string `toml:"as_token"`
}

type ListenConfig struct {
	CS string `toml:"cs"`
	AS string `toml:"as"`
}

type BridgeConfig struct {
	Name    string `toml:"name"`
	URL     string `toml:"url"`
	HSToken string `toml:"hs_token"`
}

type ProcessorConfig struct {
	Transport      string            `toml:"transport"`
	Endpoint       string            `toml:"endpoint"`
	SendTemplate   string            `toml:"send_template"`
	ReceiveMapping map[string]string `toml:"receive_mapping"`
}

func loadConfig(path string) (*Config, error) {
	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	if err := validateConfig(&cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return &cfg, nil
}

func validateConfig(cfg *Config) error {
	if cfg.Upstream.Homeserver == "" {
		return fmt.Errorf("upstream.homeserver must not be empty")
	}
	if cfg.Listen.CS == "" {
		return fmt.Errorf("listen.cs must not be empty")
	}
	if cfg.Listen.AS == "" {
		return fmt.Errorf("listen.as must not be empty")
	}
	for i, b := range cfg.Bridges {
		if b.URL == "" {
			return fmt.Errorf("bridges[%d].url must not be empty", i)
		}
		if b.HSToken == "" {
			return fmt.Errorf("bridges[%d].hs_token must not be empty", i)
		}
	}
	switch cfg.Processor.Transport {
	case "unix", "websocket", "http":
	default:
		return fmt.Errorf("processor.transport must be one of unix, websocket, http")
	}
	if cfg.Processor.Endpoint == "" {
		return fmt.Errorf("processor.endpoint must not be empty")
	}
	return nil
}

func (c *Config) bridgeByHSToken(token string) *BridgeConfig {
	for i := range c.Bridges {
		if c.Bridges[i].HSToken == token {
			return &c.Bridges[i]
		}
	}
	return nil
}

func (c *Config) bridgeByName(name string) *BridgeConfig {
	for i := range c.Bridges {
		if c.Bridges[i].Name == name {
			return &c.Bridges[i]
		}
	}
	return nil
}
