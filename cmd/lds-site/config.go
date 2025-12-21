package main

import (
	"os"

	"gopkg.in/yaml.v3"
)

type SiteConfig struct {
	CanonicalHost string                  `yaml:"canonical_host"`
	Modules       map[string]ModuleConfig `yaml:"modules"`
	Webfinger     []WebfingerLink         `yaml:"webfinger"`
}

// ModuleConfig represents metadata for a Go module
type ModuleConfig struct {
	Path       string `yaml:"path" json:"Path"`             // e.g., "lds.li/oauth2ext"
	GitURL     string `yaml:"git_url" json:"GitURL"`        // e.g., "https://github.com/lstoll/oauth2ext"
	RedirectTo string `yaml:"redirect_to" json:"RedirectTo"` // Optional, e.g., "https://github.com/lstoll/oidccli"
}

// WebfingerLink represents a link in a webfinger response
type WebfingerLink struct {
	Rel  string `yaml:"rel" json:"rel"`
	Href string `yaml:"href" json:"href"`
}

func LoadConfig(path string) (*SiteConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cfg SiteConfig
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
