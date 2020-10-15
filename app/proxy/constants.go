package main

// AppName is the name of the program as exposed to the CGI plugins and shown on the console
const AppName = "gogroxy"

// AppVersion is the version of the program as exposed to the CGI plugins and shown on the console
const AppVersion = "0.2"

// AppDescription is the text shown in the help output
const AppDescription = `
goproxy is a simple webserver that support reverse proxy access to 
remote systems and additional plugins.
`

// AppExitCodesDescription is the list of exit codes in case of errors
const AppExitCodesDescription = `
Exit codes:
   1 - Configuration path not provided
   2 - Configuration file either not found or cannot be read
   3 - Configuration file cannot be parsed (invalid JSON)
   4 - Server directory is either not valid or not a directory
   5 - Not all proxy/plugin URLs are unique
`

const (
	// PluginTypeSimple are CLI programs exposed to the browser
	PluginTypeSimple = ""
	// PluginTypeCGI implement CGI v1.1 - https://tools.ietf.org/html/rfc3875
	PluginTypeCGI = "cgi"
)

// The following exit codes are possible in case of errors
const (
	ExitcodeConfigPath   = 1
	ExitcodeReadConfig   = 2
	ExitcodeParseConfig  = 3
	ExitcodeServerDir    = 4
	ExitcodeURLNotUnique = 5
)

// TODO: Document exit codes for the user
