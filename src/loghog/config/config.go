package config

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"path/filepath"
)

type HostConfig struct {
	Name    string            `yaml:"name"`
	Command string            `yaml:"command"`
	Envars  map[string]string `yaml:"envars"`
}

type Config struct {
	ExcludeHosts []string     `yaml:"exclude_hosts"`
	Defaults     HostConfig   `yaml:"defaults"`
	Hosts        []HostConfig `yaml:"hosts"`
}

func New(fn string) (c *Config, err error) {
	dat, err := ioutil.ReadFile(fn)
	if err != nil {
		return
	}
	c = &Config{}
	err = yaml.Unmarshal(dat, c)
	if err != nil {
		return
	}
	return
}

func (c *Config) HostInfo(name string) (h *HostConfig) {
	// Check to see if container is excluded
	for _, host_match := range c.ExcludeHosts {
		matched, _ := filepath.Match(host_match, name)
		if matched {
			log.Printf("Excluding host %s -- matches %s", name, host_match)
			return // Excluded host
		}
	}
	// See if we have a match. Match on the FIRST matchinng record
	for _, hc := range c.Hosts {
		matched, _ := filepath.Match(hc.Name, name)
		if matched {
			log.Printf("Found matching host for %s (%s)", name, hc.Name)
			h = &HostConfig{}
			*h = hc
			if h.Command == "" {
				h.Command = c.Defaults.Command
			}
			// Set or merge in envars from defaults
			if h.Envars == nil {
				h.Envars = c.Defaults.Envars
			} else {
				for k, v := range c.Defaults.Envars {
					if h.Envars[k] == "" {
						h.Envars[k] = v
					}
				}
			}
			return
		}
	}
	return
}
