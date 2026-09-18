package main

import (
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = os.Getenv("SYNCD_PORT")
	}
	if port == "" {
		port = "8000"
	}
	resp, err := http.Get("http://127.0.0.1:" + port + "/health")
	if err != nil || resp.StatusCode != http.StatusOK {
		os.Exit(1)
	}
}
