
# goproxy - Simple Webserver with Reverse Proxy and Plugin Support 

`goproxy`'s main goal is used to host local web applications while acessing live data from a remote system via reverse proxy.

In order to make it more suitable for local development, it can be extended by simple plugins (any executable or script) using standard output.

## Execution

`goproxy` needs a configuration file in JSON format as argument:

```sh
goproxy -config path/to/config.json
```

The accepted command line arguments are:

```
-config string
	Path to proxy configuration
-debug-mode
	Enable debug logging to stdout
-help
	Show this help
-port int
	Port the webserver is listening on (default 8000)
-server-dir string
	Directory served by the webserver (default ".")
```

## Configuration

The configuration file consists of a JSON object with two properties "proxies" and "plugin" which are again objects/maps.

```JSON
{
	"proxies": {},
	"plugins": {}
}
```

### Proxies

Entries in the `proxies` object have their local url-prefix as their key and the remote target information as their value.

The remote target must have at least the property `url` (string) filled and can have the additional properties `auth` (string) and parameters (object/map).
Example:

```JSON
{
	"proxies": {
		"/remote/": {
			"url": "https://remote-server.invalid:12345/api/v1/",
			"auth": "USER:PASSWORD",
			"parameters": {
				"client-id": "abc"
			}
		}
	},
	"plugins": {}
}
```

This configuration would proxy all request starting with `http://localhost:8000/remote/` to `https://remote-server.invalid:12345/api/v1/` adding the basic authentication header for user "USER" and password "PASSWORD" and always adding the query-parameter "client-id=abc" to each request.<br>
A request to `http://localhost:8000/remote/list/something` would become `https://remote-server.invalid:12345/api/v1/list/something?client-id=abc`.

### Plugins

Entries in the `plugins` section describe an external program that is called when the registered URL is called. All output of the program is sent as response.

The executable string supports very minimalistic macro replacement for executable extensions. The macro `{{executable}}` is replaced with "exe" on windows and OS.ARCH on other systems, for example "linux.amd64" on 64 bit Linux or "darwin.amd64" on 64 bit MacOS.

Plugin entries need at least the "executable"-property filled with an executable command, the optional arguments array is given to the executable program as command line arguments.
The "content-type" entry defines which content-type header to set for the returned data.

(This is not very sophisticated and currently lacks input to the external program or error handling. IT will be improved in later versions.)

Example:

```JSON
{
	"proxies": {},
	"plugins": {
		"/ext/abc": {
			"executable": "plugin-one.{{extension}}",
			"arguments": [ "-arg", "abc", "-param", "xyz"],
			"content-type": "text/plain"
		}
	}
}
```
