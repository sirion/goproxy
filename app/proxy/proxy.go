package main

import (
	"crypto/tls"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"time"
)

// Proxy describes a proxy entry in the server
type Proxy struct {
	URLTo      string            `json:"url"`
	Parameters map[string]string `json:"parameters"`
	Auth       string            `json:"auth"`
	Log        bool              `json:"log"`
	Insecure   bool              `json:"insecure"`
	URLFrom    string            `json:"-"`
	client     *http.Client
}

/////////////////////////////// Proxy Client ///////////////////////////////

func proxyRequest(proxy *Proxy, w http.ResponseWriter, req *http.Request) {
	method := req.Method

	// If req.URL.Path contains escaped characters they will be replaced and we get the wrong path
	// In that case req.URL.RawPath will be filled. See https://golang.org/pkg/net/url/#URL for details
	path := req.URL.Path
	if req.URL.RawPath != "" {
		path = req.URL.RawPath
	}

	targetURL := strings.Replace(path, proxy.URLFrom, proxy.URLTo, 1)

	target, err := url.Parse(targetURL)
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
	query := req.URL.Query()
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

	cookieNames := make([]string, 0)
	for _, cookie := range cookies {
		cookie.Secure = false
		cookie.Domain = ""
		cookie.SameSite = http.SameSiteLaxMode

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

	// if resp.StatusCode >= 400 {
	// 	log.Printf("HTTP Err: %s:\n %#v\n\n", newReq.URL.String(), resp.Header)
	// }

	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	cookies = proxy.client.Jar.Cookies(target)
	for _, cookie := range cookies {
		cookie.Secure = false
		cookie.Domain = ""
		http.SetCookie(w, cookie)
	}

	w.WriteHeader(resp.StatusCode)
	written, err := io.Copy(w, resp.Body)
	if err != nil {
		logError("Proxy: %d of %d - %s", written, resp.ContentLength, err.Error())
	}
	defer resp.Body.Close()
}

func createClient(insecure bool) *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{
		Jar:     jar,
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			// Proxy: http.ProxyURL(proxyURL),
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: insecure,
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
