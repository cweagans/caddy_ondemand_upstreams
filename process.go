package caddy_ondemand_upstreams

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/caddyserver/caddy/v2"
)

type UpstreamProcess struct {
	cmd                    *exec.Cmd
	command                string
	port                   int
	dir                    string
	env                    map[string]string
	startupDelay           time.Duration
	idleTimeout            time.Duration
	terminationGracePeriod time.Duration
	lastActivity           time.Time
	mu                     sync.Mutex
}

func NewUpstreamProcess(command string, port int, dir string, env map[string]string, startup_delay time.Duration, idle_timeout time.Duration, termination_grace_period time.Duration) *UpstreamProcess {
	return &UpstreamProcess{
		command:                command,
		port:                   port,
		dir:                    dir,
		env:                    env,
		startupDelay:           startup_delay,
		idleTimeout:            idle_timeout,
		terminationGracePeriod: termination_grace_period,
		lastActivity:           time.Now(),
	}
}

func (u *UpstreamProcess) GetPort() int {
	return u.port
}

func (u *UpstreamProcess) IsRunning() bool {
	// TODO: This is not working as expected.
	// return u.cmd != nil && !u.cmd.ProcessState.Exited()
	return u.cmd != nil
}

func (u *UpstreamProcess) LogActivity() {
	u.lastActivity = time.Now()
}

func (u *UpstreamProcess) Start() error {
	u.mu.Lock()
	defer u.mu.Unlock()

	// If it's already running, nothing needs to happen.
	if u.IsRunning() {
		caddy.Log().Named(CHANNEL).Info("upstream process is already running")
		return nil
	}

	// Assign a port if needed.
	if u.port == -1 {
		port, err := getAvailablePort()
		if err != nil {
			return err
		}
		u.port = port
	}

	// Create the exec command.
	c := u.getFormattedCommand()
	u.cmd = exec.Command("sh", "-c", c)
	u.cmd.Stdout = os.Stdout
	u.cmd.Stderr = os.Stderr
	u.cmd.Dir = u.dir
	for k, v := range u.env {
		u.cmd.Env = append(u.cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	caddy.Log().Named(CHANNEL).Info("starting upstream process")
	err := u.cmd.Start()
	if err != nil {
		caddy.Log().Named(CHANNEL).Info("error while starting upstream process: " + fmt.Sprint(err))
		return err
	}
	caddy.Log().Named(CHANNEL).Info("started upstream process")

	// Wait for the startup delay if needed.
	if u.startupDelay > 0 {
		caddy.Log().Named(CHANNEL).Info("waiting for upstream process to start")
		time.Sleep(u.startupDelay)
		caddy.Log().Named(CHANNEL).Info("startup delay complete; continuing")
	}

	// Log activity to reset the counter for idle timeout.
	u.LogActivity()

	// Watch for idle timeout.
	go func() {
		for {
			time.Sleep(time.Second)
			caddy.Log().Named(CHANNEL).Info("tick for service on port " + fmt.Sprint(u.GetPort()))

			if u.lastActivity.Add(u.idleTimeout).After(time.Now()) {
				continue
			}

			caddy.Log().Named(CHANNEL).Info("idle timeout reached; stopping upstream process on port " + fmt.Sprint(u.GetPort()))
			u.Stop()
			break
		}
	}()

	return nil
}

func (u *UpstreamProcess) Stop() {
	u.mu.Lock()
	defer u.mu.Unlock()

	if !u.IsRunning() {
		return
	}

	caddy.Log().Named(CHANNEL).Info("sending SIGINT to gracefully stop the process")
	err := u.cmd.Process.Signal(os.Interrupt)
	if err == nil {
		go func() {
			// Wait for the termination grace period.
			time.Sleep(u.terminationGracePeriod)
			if u.IsRunning() {
				caddy.Log().Named(CHANNEL).Info("grace period expired; sending SIGKILL to stop the process")
				u.cmd.Process.Kill()
			}
		}()
		u.cmd.Wait()
	} else {
		caddy.Log().Named(CHANNEL).Info("error while sending SIGINT to process: " + fmt.Sprint(err))
		return
	}

	if u.IsRunning() {
		caddy.Log().Named(CHANNEL).Info("grace period expired and process is still running; sending SIGKILL to stop the process")
		u.cmd.Process.Kill()
	}

	caddy.Log().Named(CHANNEL).Info("upstream process stopped")

	u.cmd = nil
}

func (u *UpstreamProcess) getFormattedCommand() string {
	command := u.command
	if strings.Contains(command, "%d") {
		command = fmt.Sprintf(command, u.port)
	}
	caddy.Log().Named(CHANNEL).Info("formatted command for upstream: " + command)

	return command
}

// getAvailablePort returns an available port number.
func getAvailablePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()

	return l.Addr().(*net.TCPAddr).Port, nil
}
