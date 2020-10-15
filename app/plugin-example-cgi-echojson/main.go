package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {

	var input []byte

	cl := os.Getenv("CONTENT_LENGTH")
	if cl != "" {
		length, err := strconv.ParseInt(cl, 10, 0)
		if err != nil {
			outputCGIError(err)
		}

		read := make(chan bool)
		errd := make(chan bool)
		timeout := make(chan bool)

		go func() {
			input = make([]byte, length)
			_, err = io.ReadFull(os.Stdin, input)
			if err != nil {
				errd <- true
				return
			}
			read <- true
		}()

		go func() {
			time.Sleep(5 * time.Second)
			timeout <- true
		}()

		select {

		case <-timeout:
			outputCGIError(errors.New("Timout reading Stdin"))
		case <-errd:
			outputCGIError(err)
		case <-read:
		}
	}

	env := map[string]string{}

	for _, val := range os.Environ() {
		parts := strings.SplitN(val, "=", 2)
		env[parts[0]] = parts[1]
	}

	content, err := json.MarshalIndent(struct {
		Envorinment map[string]string
		Body        string
	}{
		Envorinment: env,
		Body:        string(input),
	}, "", "  ")
	if err != nil {
		outputCGIError(err)
	}

	contentLength := len(content)

	// Header
	fmt.Print("Status: 200 Ok\n")
	fmt.Print("Content-Type: application/json\n")
	fmt.Printf("Content-length: %d\n", contentLength)

	fmt.Print("\n")

	// Body
	fmt.Print(string(content))

	os.Exit(0)
}

func outputCGIError(err error) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())

	content := fmt.Sprintf("\"%s\"", strings.ReplaceAll(err.Error(), "\"", "\\\""))
	contentLength := len(content)

	// Header
	fmt.Print("Status: 500 Internal Server Error\n")
	fmt.Print("Content-Type: application/json\n")
	fmt.Printf("Content-length: %d\n", contentLength)

	fmt.Print("\n")

	// Body
	fmt.Print(content)

	os.Exit(0)
}
