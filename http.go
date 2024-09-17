package cacheproxy

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

type HttpPayload struct {
	Status int
	Method string
	Body   []byte      `json:"body"`
	Header http.Header `json:"headers"`
}

func (h *HttpPayload) GetResponseBody() string {
	if h.Header.Get("Content-Encoding") == "gzip" {
		bodyStr, _ := DecompressGzip(h.Body)
		return string(bodyStr)
	}

	return string(h.Body)
}

func (h *HttpPayload) HashKey() string {
	store := h.Header.Get("store")
	if store == "" {
		store = "default"
	}
	hash := HashBytes(h.Body)

	return store + ":" + hash
}

func (h *HttpPayload) RemovePayloadHeaders() {
	for name, _ := range h.Header {
		switch name {
		case "Content-Length", "Transfer-Encoding", "Content-Encoding", "Connection", "Date":
			h.Header.Del(name) // Remove auto-generated headers
		}
	}
}

func (h *HttpPayload) Serialize() ([]byte, error) {

	compressedBody, err := CompressGzip([]byte(h.GetResponseBody()))
	if err != nil {
		return nil, err
	}

	p := HttpPayload{
		Method: h.Method,
		Body:   compressedBody,
		Header: h.Header,
	}

	return json.Marshal(p)
}

func UnserializeHttpPayload(data []byte) (HttpPayload, error) {
	var p HttpPayload
	err := json.Unmarshal([]byte(data), &p)
	if err != nil {
		return HttpPayload{}, err
	}

	// Decompress the body
	decompressedBody, err := DecompressGzip(p.Body)
	if err != nil {
		return HttpPayload{}, err
	}
	p.Body = decompressedBody

	return p, nil
}

func (p *HttpPayload) GetTTL() int {
	cacheControl := p.Header.Get("Cache-Control")
	if cacheControl == "" {
		return 0
	}

	directives := strings.Split(cacheControl, ",")
	for _, directive := range directives {
		directive = strings.TrimSpace(directive)
		if strings.HasPrefix(directive, "s-maxage=") {
			sMaxAgeStr := strings.TrimPrefix(directive, "s-maxage=")
			sMaxAge, err := strconv.Atoi(sMaxAgeStr)
			if err == nil {
				return sMaxAge
			}
		}

		if strings.HasPrefix(directive, "max-age=") {
			maxAgeStr := strings.TrimPrefix(directive, "max-age=")
			maxAge, err := strconv.Atoi(maxAgeStr)
			if err == nil {
				return maxAge
			}
		}
	}

	return 0
}
