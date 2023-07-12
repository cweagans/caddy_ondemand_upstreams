# caddy ondemand upstreams

This plugin extends the dynamic upstreams functionality in Caddy. The goal is for Caddy to be able to start and stop backing processes when traffic to a particular vhost is received.

Right now, the plugin can start processes on demand. When there isn't traffic for some configurable amount of itme, the process is stopped.

This is useful for running a lot of applications on a server that doesn't have a lot of RAM, especially if the applications are infrequently used.

## Example caddyfile

```
:8085 {
    reverse_proxy {
		dynamic ondemand {
			command "./pocketbase serve --dir instance1_data --http :%d"
			startup_delay 1s
			idle_timeout 10s
		}
	}
}

:8086 {
    reverse_proxy {
		dynamic ondemand {
			command "./pocketbase serve --dir instance2_data --http :%d"
			startup_delay 1s
			idle_timeout 12s
		}
	}
}

:8087 {
    reverse_proxy {
		dynamic ondemand {
			command "./pocketbase serve --dir instance3_data --http :%d"
			startup_delay 1s
			idle_timeout 30s
		}
	}
}
```


## Things to do

* There are a number of commented out members of the `OndemandUpstreams` struct. Those should be uncommented, implemented in `UnmarshalCaddyfile`, and then handled properly by the `UpstreamProcess` struct methods.
* I'm not sure how to handle websocket connections. There are two different paths:
    * As long as a websocket connection is open, keep the process running.
    * Even if a connection is still open, still kill the process and rely on a client library to auto reconnect (which would start the process again)
* Documentation
