package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

// Configuration contains all data needed for the proxy to run
type Configuration struct {
	Proxies   map[string]*Proxy  `json:"proxies"`
	Plugins   map[string]*Plugin `json:"plugins"`
	serverDir string
	port      int
	active    bool
}

func initConfiguration() *Configuration {
	var configPath, serverDir string
	var port int
	var showHelp bool

	flag.StringVar(&configPath, "config", configPath, "Path to proxy configuration")
	flag.StringVar(&serverDir, "server-dir", ".", "Directory served by the webserver")
	flag.BoolVar(&debugMode, "debug-mode", false, "Enable debug logging to stdout")
	flag.IntVar(&port, "port", 8000, "Port the webserver is listening on")

	flag.BoolVar(&showHelp, "help", false, "Show this help")
	flag.Parse()

	if showHelp {
		fmt.Printf("\n%s v%s\n%s\n", AppName, AppVersion, AppDescription)
		fmt.Print("Command line arguments:\n")
		flag.PrintDefaults()
		fmt.Printf("%s\n", AppExitCodesDescription)
		os.Exit(0)
	}

	if configPath == "" {
		logFatal(ExitcodeConfigPath, "Please provide a configuration path\n")
	} else {
		configPath = filepath.Clean(configPath)
		_, err := os.Stat(configPath)
		if err != nil {
			logFatal(ExitcodeReadConfig, "Configuration file not found: \"%s\"\n", configPath)
		}
	}

	config := &Configuration{}
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		logFatal(ExitcodeReadConfig, err.Error())
	}

	err = json.Unmarshal(data, &config)
	if err != nil {
		logFatal(ExitcodeParseConfig, err.Error())
	}

	config.serverDir, err = filepath.Abs(serverDir)
	if err != nil {
		logFatal(ExitcodeServerDir, err.Error())
	}

	dir, err := os.Stat(config.serverDir)
	if err != nil {
		logFatal(ExitcodeServerDir, err.Error())
	}
	if !dir.IsDir() {
		logFatal(ExitcodeServerDir, err.Error())
	}

	config.port = port

	urls := make(map[string]bool, len(config.Proxies)+len(config.Plugins))

	for path := range config.Proxies {
		if urls[path] {
			logFatal(ExitcodeURLNotUnique, fmt.Sprintf("Proxy URL is not unique: \"%s\"", path))
		}
		urls[path] = true
	}
	for path := range config.Plugins {
		if urls[path] {
			logFatal(ExitcodeURLNotUnique, fmt.Sprintf("Plugin URL is not unique: \"%s\"", path))
		}
		urls[path] = true
	}

	// Initialize proxies
	for path, proxy := range config.Proxies {
		proxy.client = createClient(proxy.Insecure)
		proxy.URLFrom = path
	}

	// Initialize plugin
	for path, plugin := range config.Plugins {
		plugin.URLFrom = path
	}

	return config
}
