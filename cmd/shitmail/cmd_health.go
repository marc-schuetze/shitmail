package main

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

// runHealth is the container HEALTHCHECK: the runtime image is distroless,
// so no wget/curl exists there. Exit 0 on HTTP 200 from /api/v1/health.
func runHealth(_ []string) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	c := &http.Client{Timeout: 3 * time.Second}
	resp, err := c.Get("http://127.0.0.1:" + port + "/api/v1/health")
	if err != nil || resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "unhealthy:", err)
		os.Exit(1)
	}
	resp.Body.Close()
}
