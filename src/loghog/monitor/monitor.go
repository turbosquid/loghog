package monitor

import (
	"bufio"
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"log"
	"loghog/config"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime/debug"
	"strings"
	"syscall"
	"time"
)

type Monitor struct {
	cli         *client.Client
	hostname    string
	listeners   map[string]context.CancelFunc
	cfg         *config.Config
	chContainer chan string
}

func New(c *config.Config) (m *Monitor, err error) {
	m = &Monitor{cfg: c}
	m.hostname, err = os.Hostname()
	if err != nil {
		return
	}
	m.cli, err = client.NewEnvClient()
	if err != nil {
		return
	}
	m.listeners = make(map[string]context.CancelFunc)
	m.chContainer = make(chan string)
	return
}

func (m *Monitor) Run() (err error) {
	log.Printf("Starting docker monitor...")
	// See if we need to add records since we've just come up
	containers, err := m.cli.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		return err
	}
	for _, container := range containers {
		m.addListener(container.ID)
	}
	// Delete containers that no onger exist
	// Event loop time:
events:
	for {
		// Handle panics here
		defer func() {
			if r := recover(); r != nil {
				handlePanic(r)
			}
		}()
		ev, ev_err := m.cli.Events(context.Background(), types.EventsOptions{})
		for {
			select {
			case event := <-ev:
				var err error
				log.Printf("Got event: %s %s %s %s", event.Type, event.Action, event.Status, event.Actor.ID[:10])
				if event.Type == "container" && event.Action == "start" && event.Status == "start" {
					err = m.addListener(event.Actor.ID)
				} else if event.Type == "container" && event.Action == "die" && event.Status == "die" {
					err = m.removeListener(event.Actor.ID)
				}
				if err != nil {
					log.Printf("Unable to process %s event: %s", event.Action, err.Error())
				}
			case err := <-ev_err:
				log.Printf("Got error event: %s", err.Error())
				break events
			case id := <-m.chContainer:
				log.Printf("Logging stopped for monitored container: %s", id[:10])
				if m.isRunning(id) {
					log.Printf("Container is still running -- will re-add monitor")
				}
			}
		}
	}
	return
}

func (m *Monitor) isRunning(id string) bool {
	container_json, err := m.cli.ContainerInspect(context.Background(), id)
	if err != nil {
		log.Printf("Unable o recheck container. Probably no longer exists:  %s - %s", id[:10], err.Error())
		return false
	}
	if container_json.State.Status != "running" {
		log.Printf("Container %s is no longer running", id[:10])
		return false
	}
	return true
}

func (m *Monitor) addListener(id string) (err error) {
	container_json, err := m.cli.ContainerInspect(context.Background(), id)
	if err != nil {
		log.Printf("Unable to inspect container %s - %s", id[:10], err.Error())
		return
	}
	// Get hostname
	hostname := container_json.Config.Hostname
	env := make(map[string]string)
	// Get envars into a map
	for _, v := range container_json.Config.Env {
		parts := strings.SplitN(v, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}
	// Add os envars into map. We will override any values there already with os values
	for _, v := range os.Environ() {
		parts := strings.SplitN(v, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}
	// Override hostname if desired
	if env["LOGHOG_HOSTNAME"] != "" {
		hostname = env["LOGHOG_HOSTNAME"]
	} else {
		env["LOGHOG_HOSTNAME"] = hostname
	}

	env["LOGHOG_HOSTNAME"] = fmt.Sprintf("%s-%s", env["LOGHOG_HOSTNAME"], id[0:8]) // Qualify with container id

	env["LOGHOG_CONTAINER_ID"] = id

	tty := container_json.Config.Tty

	// If this container has an environment varaible defined with the
	// same name as EnvarMatch then we automatically set up logging using default settings
	if m.cfg.EnvarMatch != "" && env[m.cfg.EnvarMatch] != "" {
		log.Printf("Found envar match  for %s (%s) with %s=%s", hostname, id, m.cfg.EnvarMatch, env[m.cfg.EnvarMatch])
		return m.startListener(id, hostname, m.cfg.Defaults.Command, env, &m.cfg.Defaults, tty)
	}

	// Otherwise, decide if the container is eligible based on hostname, and get some
	// Additional settings from the config based on the hostname

	// Exclude ourselves and hosts named after containers (no explicit hostname)
	if strings.Index(id, hostname) == 0 || hostname == m.hostname {
		log.Printf("Ignoring host %s", hostname)
		return
	}
	// Get host configuration. Bail if none, merge in envars if we have one
	hc := m.cfg.HostInfo(hostname)
	if hc == nil {
		return
	}
	for k, v := range hc.Envars {
		if env[k] == "" {
			env[k] = v
		}
	}
	log.Printf("Adding log listener for %s (%s)", hostname, id)
	err = m.startListener(id, hostname, hc.Command, env, hc, tty)
	return
}

func (m *Monitor) removeListener(id string) (err error) {
	log.Printf("Removing listener for %s", id)
	return
}

func (m *Monitor) startListener(id, hostname, command string, env map[string]string, h *config.HostConfig, tty bool) (err error) {
	ctx, cancel := context.WithCancel(context.Background())
	httpc := http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", "/var/run/docker.sock")
			},
		},
	}
	go func() {
		req, err := http.NewRequest("GET", fmt.Sprintf("http://unix/containers/%s/logs?stdout=1&stderr=1&follow=1&tail=10", id), nil)
		if err != nil {
			log.Printf("Unable to create request: %s", err.Error())
			return
		}
		req = req.WithContext(ctx)
		resp, err := httpc.Do(req)
		if err != nil {
			log.Printf("Unable to make request: %s", err.Error())
			return
		}
		defer resp.Body.Close()
		// Turn env map back into a list of strings
		var envars []string
		for k, v := range env {
			envars = append(envars, fmt.Sprintf("%s=%s", k, v))
		}
		log.Printf("Envars: %#v", envars)
		scanner := bufio.NewScanner(resp.Body)
		buf := make([]byte, 0, 1024*1024)
		scanner.Buffer(buf, 1024*1024*200) // 200 mb max size
		cmd := exec.Command(command)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		cmd.Env = envars
		stdin, err := cmd.StdinPipe()
		cmd.Stdout = os.Stdout
		err = cmd.Start()
		if err != nil {
			log.Printf("Unable to run command %s =>  %s", command, err.Error())
			return
		}
		go func() {
			cmd.Wait()
			log.Printf("Logging program finished for %s", hostname)
			cancel()
		}()
		for scanner.Scan() {
			buf := scanner.Text()
			if !tty && len(buf) >= 8 {
				buf = buf[8:]
			}
			txt := fmt.Sprintf("%s\n", buf)
			if h.FilterLine(txt) {
				continue
			}
			stdin.Write([]byte(txt))
			// log.Println(scanner.Text())
		}
		stdin.Close()
		if err := scanner.Err(); err != nil {
			log.Printf("Unable to read response body: %s for %s", err.Error(), hostname)
		}
		log.Printf("EOF reading logs")
		// Kill log process. Try TERM first, then kill it
		log.Printf("Terminating logginng program for %s", hostname)
		syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		time.Sleep(5 * time.Second)
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)

	}()
	m.listeners[id] = cancel
	return
}

func handlePanic(r interface{}) {
	var msg string
	switch v := r.(type) {
	case error:
		msg = v.Error()
	case string:
		msg = v
	default:
		msg = fmt.Sprintf("PANIC (unknown type %#v)", v)
	}
	log.Printf("PANIC: %s", msg)
	log.Printf("Stack:\n%s", debug.Stack())
}

func System(format string, v ...interface{}) (output string, err error) {
	cmd := fmt.Sprintf(format, v...)
	out, err := exec.Command("/bin/bash", "-c", cmd).CombinedOutput()
	output = string(out)
	if err != nil {
		log.Printf("Unable top execute %s (%s)", cmd, err.Error())
	}
	return
}
