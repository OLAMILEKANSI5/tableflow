package util

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"tableflow/go/pkg/tf"
	"time"
)

func IsValidURL(urlStr string) bool {
	_, err := url.ParseRequestURI(urlStr)
	return err == nil
}

func IsValidDomain(domainStr string) bool {
	if len(domainStr) == 0 {
		return false
	}
	if domainStr == "localhost" {
		return true
	}
	// Regular expression pattern for domain validation
	// This pattern allows domains of the form "example.com", "www.example.com", etc.
	// It does not validate IP addresses or special characters in domain names
	pattern := `^(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+(?:[a-zA-Z]{2,})$`
	regex := regexp.MustCompile(pattern)
	return regex.MatchString(domainStr)
}

// ParseBaseURL Returns the scheme and host without the path of a provided URL
func ParseBaseURL(urlStr string) (string, error) {
	u, err := url.ParseRequestURI(urlStr)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s://%s", u.Scheme, u.Host), nil
}

func GetPort(urlStr string) (int, error) {
	u, err := url.ParseRequestURI(urlStr)
	if err != nil {
		return 0, err
	}
	colon := strings.LastIndexByte(u.Host, ':')
	if colon != -1 && validPortFromURL(u.Host[colon:]) {
		return strconv.Atoi(u.Host[colon+1:])
	}
	switch u.Scheme {
	case "http":
		return 80, nil
	case "https":
		return 443, nil
	}
	return 0, errors.New("could not determine port")
}

func ParsePort(port string) (int, error) {
	if len(port) == 0 {
		return -1, errors.New("empty port string")
	}
	portInt, err := strconv.Atoi(port)
	if err == nil && (portInt < 0 || portInt > 65536) {
		return -1, errors.New("port out of range")
	}
	if err != nil {
		return -1, err
	}
	return portInt, nil
}

func validPortFromURL(port string) bool {
	if port[0] != ':' {
		return false
	}
	for _, b := range port[1:] {
		if b < '0' || b > '9' {
			return false
		}
	}
	return true
}

func HTTPRequest(url, method string, body interface{}, headers map[string]string) error {
	client := &http.Client{
		Timeout: time.Second * 10,
	}
	requestBytes, err := json.Marshal(&body)
	if err != nil {
		tf.Log.Errorw("Could not marshal HTTP request body", "error", err, "url", url)
		return err
	}
	req, err := http.NewRequest(method, url, bytes.NewBuffer(requestBytes))
	if err != nil {
		tf.Log.Errorw("Could not create HTTP request", "error", err, "url", url)
		return err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	response, err := client.Do(req)
	if err != nil {
		tf.Log.Errorw("Error executing HTTP request", "error", err, "url", url)
		return err
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		bodyBytes, err := io.ReadAll(response.Body)
		if err != nil {
			tf.Log.Errorw("Could not read response body on unsuccessful status during HTTP request", "error", err, "url", url)
		}
		tf.Log.Warnw("Received non-200 status code while executing http request", "status", response.StatusCode, "body", string(bodyBytes))
		return errors.New("received non-200 status code while executing http request")
	}
	defer response.Body.Close()
	return nil
}
