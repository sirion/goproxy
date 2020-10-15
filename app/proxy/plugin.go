package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// PluginType can be either "" or "cgi" - see PluginType*-constants
type PluginType string

// Plugin describes a plugin entry in the server
type Plugin struct {
	Type        PluginType `json:"type"`
	Executable  string     `json:"executable"`
	Arguments   []string   `json:"arguments"`
	ContentType string     `json:"content-type"`
	Log         bool       `json:"log"`
	URLFrom     string     `json:"-"`
}

func executePlugin(config *Configuration, plugin *Plugin, w http.ResponseWriter, req *http.Request) {
	switch plugin.Type {

	case PluginTypeCGI:
		executePluginCGI(config, plugin, w, req)
		break

	default:
		logError("Invalig plugin type for plugin %s", plugin.Executable)
		// fall through
	case PluginTypeSimple:
		executePluginSimple(config, plugin, w, req)
		break
	}
}

func executePluginCGI(config *Configuration, plugin *Plugin, w http.ResponseWriter, req *http.Request) {
	// TODO: Should we add real environment variables?
	// env := os.Environ()
	env := make([]string, 0, 21) // 21 - maximum filled
	path := strings.Replace(req.URL.Path, plugin.URLFrom, "", 1)

	env = append(env, []string{
		"GATEWAY_INTERFACE=CGI/1.1",
		fmt.Sprintf("AUTH_TYPE=%s", req.Header.Get("auth-scheme")),
		fmt.Sprintf("PATH_INFO=%s", path),
		fmt.Sprintf("PATH_TRANSLATED=%s", filepath.Join(config.serverDir, path)),
		fmt.Sprintf("QUERY_STRING=%s", req.URL.RawQuery),
		fmt.Sprintf("REMOTE_ADDR=%s", req.RemoteAddr),
		fmt.Sprintf("REMOTE_HOST=%s", req.RemoteAddr), // No support for looking up FQDN
		fmt.Sprintf("REQUEST_METHOD=%s", req.Method),
		fmt.Sprintf("SCRIPT_NAME=%s", plugin.URLFrom),
		fmt.Sprintf("SERVER_NAME=%s", req.Host),
		fmt.Sprintf("SERVER_PORT=%d", config.port),
		fmt.Sprintf("SERVER_PROTOCOL=%s", "HTTP/1.0"),
		fmt.Sprintf("SERVER_SOFTWARE=%s/%s", AppName, AppVersion),
		// fmt.Sprintf("REMOTE_IDENT=%s", ""), // Not supported
	}...)

	// HTTP Request Headers:
	var value string
	value = req.Header.Get("Accept")
	if value != "" {
		env = append(env, fmt.Sprintf("HTTP_ACCEPT=%s", value))
	}
	value = req.Header.Get("Accept-Charset")
	if value != "" {
		env = append(env, fmt.Sprintf("HTTP_ACCEPT_CHARSET=%s", value))
	}
	value = req.Header.Get("Accept-Encoding")
	if value != "" {
		env = append(env, fmt.Sprintf("HTTP_ACCEPT_ENCODING=%s", value))
	}
	value = req.Header.Get("Accept-Language")
	if value != "" {
		env = append(env, fmt.Sprintf("HTTP_ACCEPT_LANGUAGE=%s", value))
	}
	value = req.Header.Get("User-Agent")
	if value != "" {
		env = append(env, fmt.Sprintf("HTTP_USER_AGENT=%s", value))
	}

	if user, _, ok := req.BasicAuth(); ok {
		env = append(env, fmt.Sprintf("REMOTE_USER=%s", user))
	}

	// Only if there is a message body sent from the client
	if req.ContentLength > 0 {
		env = append(env, fmt.Sprintf("CONTENT_LENGTH=%d", req.ContentLength))

		ct := req.Header.Get("Content-Type")
		if ct != "" {
			// TODO: Content type detection
			env = append(env, fmt.Sprintf("CONTENT_TYPE=%s", ct))
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	executable := replacePluginMacrosSingle(plugin.Executable, plugin, req, config)
	args := replacePluginMacros(plugin.Arguments, plugin, req, config)

	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Env = env

	errorBuffer := bytes.Buffer{}
	cmd.Stderr = &errorBuffer
	cmd.Stdin = req.Body
	output, err := cmd.Output()

	if plugin.Log && errorBuffer.Len() > 0 {
		fmt.Fprintf(os.Stderr, "CGI Error Output: \n |%s\n\n", strings.ReplaceAll(errorBuffer.String(), "\n", "\n |"))
	}

	if err != nil {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(500)
		w.Write([]byte(fmt.Sprintf("CGI invocation error: %s\n", err.Error())))
	} else {
		lines := bytes.Split(output, []byte("\n"))
		header := true
		headerWritten := false
		status := 200

		for _, line := range lines {
			validHeaderLine := bytes.Index(line, []byte(":")) > -1

			if header {
				if len(line) == 0 {
					header = false
					continue
				}

				if !validHeaderLine {
					logError("Invalid Header Line: %s\n", line)
				}

				lineParts := bytes.SplitN(line, []byte(":"), 2)
				key := strings.TrimSpace(string(lineParts[0]))
				value := strings.TrimSpace(string(lineParts[1]))

				if strings.ToLower(key) == "status" {
					parts := strings.SplitN(value, " ", 2)
					status64, err := strconv.ParseInt(parts[0], 10, 0)
					if err != nil {
						logError("Could not parse status header from CGI: %s - \n%#v\n", err.Error(), parts)
					} else {
						status = int(status64)
					}
				} else {
					w.Header().Add(key, value)
				}

			} else {
				if !headerWritten {
					w.WriteHeader(status)
					headerWritten = true
				}
				w.Write(line)
				w.Write([]byte("\n"))
			}
		}
	}
}

func executePluginSimple(config *Configuration, plugin *Plugin, w http.ResponseWriter, req *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	outputBuffer := bytes.Buffer{}

	executable := replacePluginMacrosSingle(plugin.Executable, plugin, req, config)
	args := replacePluginMacros(plugin.Arguments, plugin, req, config)

	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Stdout = &outputBuffer

	// TODO: Change content type in case of error?
	w.Header().Set("Content-Type", plugin.ContentType)

	err := cmd.Run()
	if err != nil {
		exit, ok := err.(*exec.ExitError)
		if ok {
			// Set X-Exit-Code header to exitcode of plugin
			w.Header().Set("X-Exit-Code", fmt.Sprintf("%d", exit.ExitCode()))
			w.WriteHeader(500)
			w.Write(exit.Stderr)

		} else {
			w.WriteHeader(500)
			w.Write([]byte(err.Error()))
		}

		logError("Running Plugin \"%s\": %s", plugin.Executable)
	} else {
		w.WriteHeader(200)
		w.Write(outputBuffer.Bytes())
	}
}
