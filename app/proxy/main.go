package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type Proxy struct {
	URLTo      string            `json:"url"`
	URLFrom    string            `json:"-"`
	Parameters map[string]string `json:"parameters"`
	Auth       string            `json:"auth"`
	Log        bool              `json:"log"`
	client     *http.Client
}

type Plugin struct {
	Executable  string   `json:"executable"`
	Arguments   []string `json:"arguments"`
	ContentType string   `json:"content-type"`
}

type Configuration struct {
	Proxies   map[string]*Proxy  `json:"proxies"`
	Plugins   map[string]*Plugin `json:"plugins"`
	serverDir string
	port      int
	active    bool
}

const (
	ExitcodeConfigPath  = 1
	ExitcodeReadConfig  = 2
	ExitcodeParseConfig = 3
	ExitcodeServerDir   = 4
)

var debugMode bool

func main() {
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
		fmt.Println()
		flag.PrintDefaults()
		os.Exit(0)
	}

	if configPath == "" {
		logFatal(ExitcodeConfigPath, "Please provide a configuration path")
	} else {
		configPath = filepath.Clean(configPath)
		_, err := os.Stat(configPath)
		if err != nil {
			logFatal(ExitcodeReadConfig, "Configuration file not found: \"%s\"", configPath)
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

	config.port = port

	logStd("Proxy routes:\n")
	pathLen := 0
	for path := range config.Proxies {
		if pathLen < len(path) {
			pathLen = len(path)
		}
	}
	for path, proxy := range config.Proxies {
		logStd(fmt.Sprintf(" - %%-%ds => %%s\n", pathLen), path, proxy.URLTo)
	}
	logStd("\n")

	// Initialize proxies
	for path, proxy := range config.Proxies {
		proxy.client = createClient()
		proxy.URLFrom = path
	}

	runServer(config)
}

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

	logDebug("Serving \"%s\" on http://localhost:%d\n", config.serverDir, config.port)
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
				executePlugin(plugin, w, req)
				return
			}
		}

		// Handled by server
		http.ServeFile(w, req, filepath.Join(config.serverDir, req.URL.Path[1:]))
	}
}

/////////////////////////////// Plugins ///////////////////////////////

func executePlugin(plugin *Plugin, w http.ResponseWriter, req *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	executable := replaceMacros(plugin.Executable)

	cmd := exec.CommandContext(ctx, executable, plugin.Arguments...)

	cmd.Stdout = w
	cmd.Stderr = os.Stderr

	w.Header().Set("Content-Type", plugin.ContentType)
	w.WriteHeader(200)

	// TODO: handle errors
	err := cmd.Run()
	if err != nil {
		logError("Running Plugin \"%s\": %s", plugin.Executable, err.Error())
	}
}

/////////////////////////////// Proxy Client ///////////////////////////////

func proxyRequest(proxy *Proxy, w http.ResponseWriter, req *http.Request) {
	method := req.Method

	targetURL := req.URL.RequestURI()
	targetURL = strings.Replace(targetURL, proxy.URLFrom, proxy.URLTo, 1)

	target, err := url.Parse(targetURL)
	// target, err := url.Parse(config.serverHost)
	if err != nil {
		log.Println(err.Error())
		os.Exit(1)
	}

	newReq, err := http.NewRequest(method, target.String(), req.Body)
	if err != nil {
		w.WriteHeader(503)
		w.Write([]byte("Proxy Error: " + err.Error()))
		return
	}

	// Make sure forced parameters are added
	query := newReq.URL.Query()
	for key, value := range proxy.Parameters {
		query.Set(key, value)
	}
	newReq.URL.RawQuery = query.Encode()

	logDebug("Proxying: %s => %s...\n", req.URL.Path, newReq.URL.String())

	if proxy.Auth != "" {
		if strings.Index(proxy.Auth, ":") > -1 {
			auth := strings.SplitN(proxy.Auth, ":", 2)
			newReq.SetBasicAuth(auth[0], auth[1])
		} else {
			newReq.Header.Set("Authorization", "Basic "+proxy.Auth)
		}
	}

	newReq.URL.Scheme = target.Scheme
	newReq.URL.Host = target.Host

	for key, values := range req.Header {
		lowKey := strings.ToLower(key)

		// TODO: Write the following more elegantly... :/
		if req.Method != http.MethodPost && lowKey == "x-csrf-token" {
			continue
		}
		if strings.Index(lowKey, "sec-") == 0 {
			continue
		}

		// Make sure caching is disabled
		if lowKey == "if-none-match" {
			continue
		}
		if lowKey == "last-modified" {
			continue
		}
		if lowKey == "if-modified-since" {
			continue
		}

		for _, value := range values {
			newReq.Header.Add(key, value)
		}
	}
	if _, ok := newReq.Header["User-Agent"]; !ok { // Is this needed? Why?
		// explicitly disable User-Agent so it's not set to default value
		newReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:80.0) Gecko/20100101 Firefox/80.0")
	}

	// Replace security specific cookie parts
	cookies := req.Cookies()
	newReq.Header.Del("Set-Cookie") // TODO: Needed?

	cookieNames := make([]string, 0)
	for _, cookie := range cookies {
		cookie.Secure = false
		cookie.Domain = ""
		// newReq.Header.Add("Set-Cookie", cookie.String())
		newReq.AddCookie(cookie)
		cookieNames = append(cookieNames, cookie.Name)
	}

	// Make sure caching is disabled
	newReq.Header.Set("Cache-Control", "no-store")

	if proxy.Log {
		log.Printf("%s %s\n", newReq.Method, newReq.URL.String())
	}

	resp, err := proxy.client.Do(newReq)

	if err != nil {
		w.WriteHeader(503)
		w.Write([]byte("Proxy Error: " + err.Error()))
		return
	}

	// logStd("\n>>>>>>>>\n")
	// raw, _ := httputil.DumpRequestOut(newReq, false)
	// logStd("%s", raw)
	// logStd("\n<<<<<<<<\n")
	// raw, _ = httputil.DumpResponse(resp, false)
	// logStd("%s", raw)
	// logStd("------\n")

	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	cookieNames = make([]string, 0)
	cookies = proxy.client.Jar.Cookies(target)
	for _, cookie := range cookies {
		cookie.Secure = false
		cookie.Domain = ""

		cookieNames = append(cookieNames, cookie.Name)
		// w.Header().Add("Set-Cookie", cookie.String())
		http.SetCookie(w, cookie)
	}
	// logDebug("Cookies in: %s\n", strings.Join(cookieNames, ", "))

	w.WriteHeader(resp.StatusCode)
	written, err := io.Copy(w, resp.Body)
	if err != nil {
		logError("Proxy: %d of %d - %s", written, resp.ContentLength, err.Error())
	}
	defer resp.Body.Close()
}

func createClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{
		Jar:     jar,
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			// Proxy: http.ProxyURL(proxyURL),
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			Dial: (&net.Dialer{
				Timeout: 10 * time.Second,
			}).Dial,
			IdleConnTimeout:     30 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}
}

// func createProxyClient(proxyURI string) *http.Client {
// 	// os.Setenv("HTTP_PROXY", proxyURI)
// 	proxyURL, err := url.Parse(proxyURI)
// 	if err != nil {
// 		log.Println(err.Error())
// 		os.Exit(1)
// 	}

// 	return &http.Client{
// 		Transport: &http.Transport{
// 			Proxy: http.ProxyURL(proxyURL),
// 			TLSClientConfig: &tls.Config{
// 				InsecureSkipVerify: true,
// 			},
// 			IdleConnTimeout: 30 * time.Second,
// 		},
// 	}
// }

/////////////////////////////// Macros ///////////////////////////////

func replaceMacros(s string) string {
	// Extension
	extension := fmt.Sprintf("%s.%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		extension = "exe"
	}
	replaced := strings.ReplaceAll(s, "{{extension}}", extension)

	return replaced
}

/////////////////////////////// Logging ///////////////////////////////

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
