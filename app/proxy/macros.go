package main

import (
	"fmt"
	"net/http"
	"runtime"
	"strings"
)

/////////////////////////////// Macros ///////////////////////////////

func replacePluginMacrosSingle(str string, plugin *Plugin, req *http.Request, config *Configuration) string {
	return replacePluginMacros([]string{str}, plugin, req, config)[0]
}

func replacePluginMacros(strs []string, plugin *Plugin, req *http.Request, config *Configuration) []string {
	replaced := make([]string, len(strs))

	// {{extension}}
	var extension string
	if runtime.GOOS == "windows" {
		extension = "exe"
	} else {
		extension = fmt.Sprintf("%s.%s", runtime.GOOS, runtime.GOARCH)
	}

	path := strings.Replace(req.URL.Path, plugin.URLFrom, "", 1)

	for i, str := range strs {
		replaced[i] = str
		replaced[i] = strings.ReplaceAll(replaced[i], "{{extension}}", extension)
		replaced[i] = strings.ReplaceAll(replaced[i], "{{path}}", path)
		replaced[i] = strings.ReplaceAll(replaced[i], "{{query}}", req.URL.RawQuery)

		// TODO: Maybe add the following as variables:
		//
		// replaced[i] = strings.ReplaceAll(replaced[i], "{{host}}", req.Host)
		// replaced[i] = strings.ReplaceAll(replaced[i], "{{remote_addr}}", req.RemoteAddr)
		// replaced[i] = strings.ReplaceAll(replaced[i], "{{request_method}}", req.Method)
		// replaced[i] = strings.ReplaceAll(replaced[i], "{{url}}", plugin.URLFrom)
		// replaced[i] = strings.ReplaceAll(replaced[i], "{{port}}", fmt.Sprintf("%d", config.port))
		// replaced[i] = strings.ReplaceAll(replaced[i], "{{app_name}}", AppName)
		// replaced[i] = strings.ReplaceAll(replaced[i], "{{app_version}}", AppVersion)
		// replaced[i] = strings.ReplaceAll(replaced[i], "{{user_agent}}", req.Header.Get("User-Agent"))
		// if user, _, ok := req.BasicAuth(); ok {
		// 	replaced[i] = strings.ReplaceAll(replaced[i], "{{user}}", user)
		// } else {
		// 	replaced[i] = strings.ReplaceAll(replaced[i], "{{user}}", "anonymous")
		// }
	}

	return replaced
}
