package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

/////////////////////////////// Webserver ///////////////////////////////

func runServer(config *Configuration) {

	// Initialize web handler
	webHandler := &http.ServeMux{}
	webHandler.HandleFunc("/", createRequesthandler(config))

	// Serve web application
	webServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", config.port),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
		Handler:      webHandler,
		// Errorlog:     logger.log.Getlogger(logger.logLevelError),
	}

	config.active = true

	// Allow graceful shutdown
	go func() {
		for config.active {
			time.Sleep(2 * time.Second)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		webServer.Shutdown(ctx)
	}()

	logStd("Serving \"%s\" on http://localhost:%d\n", config.serverDir, config.port)
	log.Fatalln(webServer.ListenAndServe())
}

func createRequesthandler(config *Configuration) func(w http.ResponseWriter, req *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		uri := req.URL.RequestURI()
		for path, proxy := range config.Proxies {
			if strings.Index(uri, path) == 0 {
				// Proxy path found
				// Route through proxy
				proxyRequest(proxy, w, req)
				return
			}
		}

		for path, plugin := range config.Plugins {
			if strings.Index(uri, path) == 0 {
				// Proxy path found
				// Route through proxy
				executePlugin(config, plugin, w, req)
				return
			}
		}

		// Handled by server
		http.ServeFile(w, req, filepath.Join(config.serverDir, req.URL.Path[1:]))
	}
}

/////////////////////////////// Logging ///////////////////////////////

func outputRoutes(config *Configuration) {
	logStd("Proxy routes:\n")
	pathLen := 0
	for path := range config.Proxies {
		if pathLen < len(path) {
			pathLen = len(path)
		}
	}
	for path := range config.Plugins {
		if pathLen < len(path) {
			pathLen = len(path)
		}
	}
	for path, proxy := range config.Proxies {
		logStd(fmt.Sprintf(" - %%-%ds => %%s\n", pathLen), path, proxy.URLTo)
	}
	for path, plugin := range config.Plugins {
		logStd(fmt.Sprintf(" - %%-%ds => %%s\n", pathLen), path, plugin.Executable)
	}
	logStd("\n")
}

func logDebug(format string, args ...interface{}) {
	if debugMode {
		fmt.Fprintf(os.Stdout, "[DEBUG] "+format, args...)
	}
}

func logStd(format string, args ...interface{}) {
	fmt.Fprintf(os.Stdout, format, args...)
}

func logError(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "[ERROR] "+format, args...)
}

func logFatal(exitCode int, format string, args ...interface{}) {
	logError(format, args...)
	os.Exit(exitCode)
}

func logJSON(data interface{}) {
	json, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		logError(err.Error())
	} else {
		fmt.Println(string(json))
	}
}
