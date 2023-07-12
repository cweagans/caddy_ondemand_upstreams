package caddy_ondemand_upstreams

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
)

// Interface guards.
var (
	_ caddyfile.Unmarshaler       = (*OndemandUpstreams)(nil)
	_ caddy.Provisioner           = (*OndemandUpstreams)(nil)
	_ caddy.Validator             = (*OndemandUpstreams)(nil)
	_ caddy.CleanerUpper          = (*OndemandUpstreams)(nil)
	_ reverseproxy.UpstreamSource = (*OndemandUpstreams)(nil)
)

const CHANNEL = "ondemand_upstream"

func init() {
	caddy.RegisterModule(OndemandUpstreams{})
}

// OndemandUpstreams provides upstreams from processes that are started on demand.
// This is useful for running short-lived processes, such as for serverless, or
// for starting backend services that don't always need to be running (such as
// infrequently used applications)
type OndemandUpstreams struct {
	// Required. A command to run to start the upstream process. The command
	// can include a %d placeholder, which will be replaced with the value of the
	// Port setting. If no %d placeholder is present, the Port setting must be
	// set so that Caddy knows where to proxy requests.
	Command string `json:"command,omitempty"`

	// StartupDelay is the amount of time to wait after starting the process
	// before attempting to connect to it. This is useful for processes that
	// take some time to start up. Default: 0.
	StartupDelay caddy.Duration `json:"startup_delay,omitempty"`

	// Optional. A fixed port number to use for the upstream. If this is not set
	// in your configuration, an available port will be chosen automatically.
	// Default: -1 (automatic port assignment)
	Port int `json:"port,omitempty"`

	// Optional. The working directory to use for the upstream process. If not
	// set, the current working directory will be used.
	// Dir string `json:"dir,omitempty"`

	// Optional. The user to run the process as. If not set, the process will be
	// run as the current user.
	// User string `json:"user,omitempty"`

	// Optional. A list of environment variables to set for the process.
	// Env map[string]string `json:"env,omitempty"`

	// Optional. The number of seconds that the process should continue running
	// if no traffic is received. Set to -1 to disable process termination.
	// Default: 300 seconds.
	IdleTimeout caddy.Duration `json:"idle_timeout,omitempty"`

	// Optional. The amount of time to wait for the application to gracefully
	// shut down before killing it (after idle_timeout). Default: 10 seconds.
	// TerminationGracePeriod caddy.Duration `json:"termination_grace_period,omitempty"`

	// Optional. Redirect stdout to a file. If not set, stdout will be sent to
	// Caddy's stdout.
	// StdoutFile string `json:"stdout_file,omitempty"`

	// Optional. Redirect stderr to a file. If not set, stderr will be sent to
	// Caddy's stderr.
	// StderrFile string `json:"stderr_file,omitempty"`

	// The managed upstream process.
	upstreamProcess *UpstreamProcess
}

// CaddyModule returns the Caddy module information.
func (OndemandUpstreams) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.reverse_proxy.upstreams.ondemand",
		New: func() caddy.Module { return new(OndemandUpstreams) },
	}
}

// UnmarshalCaddyfile implements caddyfile.Unmarshaler.
func (o *OndemandUpstreams) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	caddy.Log().Named(CHANNEL).Info("ondemand_upstream unmarshal caddyfile")

	for d.Next() {
		if d.NextArg() {
			return d.ArgErr()
		}

		for d.NextBlock(0) {
			switch d.Val() {
			case "command":
				caddy.Log().Named(CHANNEL).Info("parsing command")
				if !d.NextArg() {
					return d.ArgErr()
				}
				if o.Command != "" {
					return d.Err("command has already been specified")
				}
				o.Command = d.Val()
				caddy.Log().Named(CHANNEL).Info("command: " + o.Command)

			case "port":
				caddy.Log().Named(CHANNEL).Info("parsing port")
				if !d.NextArg() {
					return d.ArgErr()
				}
				if o.Port != 0 {
					return d.Err("port has already been specified")
				}
				i, err := strconv.Atoi(d.Val())
				if err != nil {
					return d.Errf("invalid port number: %v", err)
				}
				o.Port = i
				caddy.Log().Named(CHANNEL).Info("port: " + d.Val())

			case "startup_delay":
				caddy.Log().Named(CHANNEL).Info("parsing startup_delay")
				if !d.NextArg() {
					return d.ArgErr()
				}
				if o.StartupDelay != 0 {
					return d.Err("startup_delay has already been specified")
				}
				dur, err := caddy.ParseDuration(d.Val())
				if err != nil {
					return d.Errf("invalid duration: %v", err)
				}
				o.StartupDelay = caddy.Duration(dur)
				caddy.Log().Named(CHANNEL).Info("startup_delay: " + d.Val())

			case "idle_timeout":
				caddy.Log().Named(CHANNEL).Info("parsing idle_timeout")
				if !d.NextArg() {
					return d.ArgErr()
				}
				if o.IdleTimeout != 0 {
					return d.Err("idle_timeout has already been specified")
				}
				dur, err := caddy.ParseDuration(d.Val())
				if err != nil {
					return d.Errf("invalid duration: %v", err)
				}
				o.IdleTimeout = caddy.Duration(dur)
				caddy.Log().Named(CHANNEL).Info("idle_timeout: " + d.Val())
			}
		}
	}

	return nil
}

// Provision implements caddy.Provisioner.
func (o *OndemandUpstreams) Provision(ctx caddy.Context) error {
	caddy.Log().Named(CHANNEL).Info("ondemand_upstream provisioned")
	return nil
}

// Validate implements caddy.Validator.
func (o *OndemandUpstreams) Validate() error {
	caddy.Log().Named(CHANNEL).Info("ondemand_upstream validate")

	if o.Command == "" {
		return fmt.Errorf("command is required")
	}

	if o.IdleTimeout == caddy.Duration(0) {
		o.IdleTimeout = caddy.Duration(300 * time.Second)
		caddy.Log().Named(CHANNEL).Info("idle_timeout: " + fmt.Sprint(o.IdleTimeout))
	}

	if o.Port == 0 {
		o.Port = -1
	}

	caddy.Log().Named(CHANNEL).Info("port: " + strconv.Itoa(o.Port))

	return nil
}

// GetUpstreams implements reverseproxy.UpstreamSource.
func (o *OndemandUpstreams) GetUpstreams(r *http.Request) ([]*reverseproxy.Upstream, error) {
	caddy.Log().Named(CHANNEL).Info("ondemand_upstream get upstreams")

	if o.upstreamProcess == nil {
		// Create a new upstream process.
		o.upstreamProcess = NewUpstreamProcess(o.Command, o.Port, time.Duration(o.StartupDelay), time.Duration(o.IdleTimeout))
	}

	o.upstreamProcess.Start()

	if o.upstreamProcess.IsRunning() {
		o.upstreamProcess.LogActivity()
		caddy.Log().Named(CHANNEL).Info("sending req to port " + fmt.Sprint(o.upstreamProcess.GetPort()))
		return []*reverseproxy.Upstream{
			{
				Dial: net.JoinHostPort("localhost", strconv.Itoa(o.upstreamProcess.GetPort())),
			},
		}, nil
	}

	return nil, fmt.Errorf("no upstreams available")
}

// Cleanup implements caddy.CleanerUpper.
func (o *OndemandUpstreams) Cleanup() error {
	if o.upstreamProcess != nil && o.upstreamProcess.IsRunning() {
		o.upstreamProcess.Stop()
	}

	return nil
}
