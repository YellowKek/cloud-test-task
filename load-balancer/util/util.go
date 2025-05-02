package util

import (
	"log"
	"net/http"
	"net/url"
)

func IsBackendAlive(url url.URL) bool {
	resp, err := http.Get(url.String() + "/health")
	if err != nil {
		log.Printf("Backend %v is down: %v", url, err)
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
