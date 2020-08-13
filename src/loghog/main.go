package main

import (
	"flag"
	"log"
	"loghog/config"
	"loghog/monitor"
)

const VERSION = "1.1.1"
const CONFIG_FILE = "/etc/loghog.yml"

func main() {
	config_fn := flag.String("config", CONFIG_FILE, "Config file path")
	flag.Parse()
	log.Printf("loghog v%s starting", VERSION)
	c, err := config.New(*config_fn)
	if err != nil {
		log.Fatalf("Unable to get configuration: %s", err.Error())
	}
	log.Printf("cfg: %#v", c)
	m, err := monitor.New(c)
	if err != nil {
		log.Fatalf("Unable to start monitor: %s", err.Error())
	}
	err = m.Run()
	if err != nil {
		log.Fatalf("Unable to run monitor: %s", err.Error())
	}
}
