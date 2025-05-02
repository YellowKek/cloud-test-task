package util

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"
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

func StringToDuration(str string) (time.Duration, error) {
	var duration time.Duration

	duration, err := time.ParseDuration(str)
	if err != nil {
		seconds, err := strconv.Atoi(str)
		if err != nil {
			return time.Duration(0), fmt.Errorf("invalid duration format, expected string like '1h' or number of seconds: %v", err)
		}
		duration = time.Duration(seconds) * time.Second
	}
	return duration, nil
}
