
# goproxy - Simple Webserver with Reverse Proxy and Plugin Support

`goproxy`'s main goal is used to host local web applications while acessing live data from a remote system via reverse proxy.

In order to make it more suitable for local development, it can be extended by simple plugins (any executable or script) using standard output.

## Execution

`goproxy` needs a configuration file in JSON format as argument:

```sh
goproxy -config path/to/config.json
```

The accepted command line arguments are:

```none
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

The remote target can have the following properties:

- `url` must contain the full target URL
- `auth` may contain username and password separated by ":" or the same string base64 encoded
- `parameters` may contain a map of arguments that are always appended to the request
- `insecure` may be set to true to disable the certificate validation for the target
- `log` may be set to true to enable request logging to standard output

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

This configuration would proxy all request starting with `http://localhost:8000/remote/` to `https://remote-server.invalid:12345/api/v1/` adding the basic authentication header for user "USER" and password "PASSWORD" and always adding the query-parameter "client-id=abc" to each request.  
A request to `http://localhost:8000/remote/list/something` would become `https://remote-server.invalid:12345/api/v1/list/something?client-id=abc`.

### Plugins

Entries in the `plugins` section describe an external program that is called when the registered URL is called. All output of the program is sent as response.

Example configuration:

```JSON
{
	"proxies": {},
	"plugins": {
		"/ext/simple": {
			"type": "",
			"executable": "plugin-simple",
			"arguments": [ "-arg", "abc", "-param", "xyz"],
			"content-type": "text/plain",
		},
		"/ext/cgi": {
			"type": "cgi",
			"executable": "plugin-cgi.{{extension}}",
			"arguments": [ "-arg", "xyz", "-param", "abc"],
			"log": true
		}
	}
}
```

#### Common plugin configuration properties

Plugin entries need at least the `executable`-property filled with an executable command, the optional `arguments` array is given to the executable program as command line arguments.

The following properties are supported by all plugin types:

- `type` can either be "" (empty string) to indicate a simple plugin or "cgi" to indicate a plugin using CGI.
- `executable` is the path of the executable file or script
- `arguments` is an optional array of arguments given to the exeutable

The `type` property is optional and defaults to "" (simple).

The `executable` and `arguments` can contain simple macros that are replaced on execution:

- `{{extension}}` is replaced with the executable extension for the current system - "exe" on windows and $OS.$ARCH on
  other systems, for example "linux.amd64" on 64 bit Linux or "darwin.amd64" on 64 bit MacOS.
- `{{path}}` is replaced with the path in the URL after the defined plugin path
- `{{query}}` is replaced with the request query/search

#### Simple plugins

If the type is empty, the program is treated as a simple plugin that cannot set headers or receive request bodies.
Simple plugins executable standard output is sent to the browser. The simple plugin can be used to expose any command line tool output via the proxy.

In case the program returns with a non-zero exit code, the response will have the status code 500 and the "X-Exit-Code"-header will be set to the exit code, the request body will contain the standard-error-output of the program.

The following properties are supported by the simple plugin type:

- `content-type` defines which content-type header to set for the returned data. Since the simple plugin has no access
  to headers, so the content-type must be set here and cannot change.  

Example:

```JSON
{
	"proxies": {},
	"plugins": {
		"/eslint/project-a": {
			"executable": "eslint",
			"arguments": [ "-f", "json", "/path/to/project-a-source-code"],
		}
	}
}
```

#### CGI plugins

If the type is "cgi", the program must implement Common Gateway Interface (CGI), see [RFC3875](https://tools.ietf.org/html/rfc3875).  
This way the program can read the request body via standard-input, set headers and return status codes other than 200.
The standard-error of a CGI plugin is logged to the console if `log` is set to true.

The following properties are supported by the CGI plugin type:

- `log` defines whether the standard error output is ignored. If set to true the plugin standard error output will be
  written to the proxy standard output
