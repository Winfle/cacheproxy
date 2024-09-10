package cacheproxy

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

type ReverseProxy struct {
	Proxy  *httputil.ReverseProxy
	Target string
	Cache  *RedisClient
}

func (p *ReverseProxy) HttpHandler(w http.ResponseWriter, req *http.Request) {
	backend, err := url.Parse(p.Target)
	if err != nil {
		http.Error(w, "Invalid backend URL", http.StatusInternalServerError)
		return
	}

	bodyBytes, err := io.ReadAll(req.Body)
	req.Body.Close()

	hash := HashBytes(bodyBytes)

	if cacheBody, _ := p.Cache.Get(hash); cacheBody != "" {
		fmt.Printf("Cache HIT: %s\n", hash)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(cacheBody))
		return
	}

	fmt.Printf("Cache MISS: %s\n", hash)

	bodyReader := bytes.NewReader(bodyBytes)

	// Create a new request for the backend
	proxyReq, err := http.NewRequest(req.Method, backend.ResolveReference(req.URL).String(), bodyReader)
	if err != nil {
		http.Error(w, "Error creating proxy request", http.StatusInternalServerError)
		return
	}

	proxyReq.Header = req.Header
	resp, err := http.DefaultTransport.RoundTrip(proxyReq)

	if err != nil {
		http.Error(w, "Error contacting backend", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Set(key, value)
		}
	}

	bodyBytes, err = io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Error reading response from backend", http.StatusInternalServerError)
		return
	}

	var bodyString string
	if strings.Contains(resp.Header.Get("Content-Encoding"), "gzip") {
		bodyString, _ = DecompressGzip(bodyBytes)
	} else {
		bodyString = string(bodyBytes)
	}

	p.Cache.Set(hash, bodyString)

	// Set the status code and write the response body
	w.WriteHeader(resp.StatusCode)
	w.Write(bodyBytes)
}
