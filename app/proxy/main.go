package main

// debugMode is the only global variable
var debugMode bool

func main() {
	config := initConfiguration()
	outputRoutes(config)
	runServer(config)
}
