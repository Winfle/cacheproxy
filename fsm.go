package cacheproxy

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

type FSM struct {
	rds  *RedisClient
	next http.Handler
	log  *zap.Logger

	w http.ResponseWriter
	r *http.Request

	cacheable bool

	req HttpPayload
	res HttpPayload

	beRespBody   []byte
	beRespHeader http.Header

	startTime time.Time
}

func (f FSM) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.startTime = time.Now()
	reqBody, _ := io.ReadAll(r.Body)

	r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(reqBody))

	f.req = HttpPayload{
		Body:   reqBody,
		Method: r.Method,
		Header: r.Header,
	}

	f.w = w
	f.r = r

	f.Recv()
}

func (f *FSM) Recv() {
	hKey := f.req.HashKey()

	data, err := f.rds.Get(hKey)
	if err != nil {
		f.log.Error(fmt.Sprintf("Error fetching from Redis for key %s: %v", hKey, err))
		f.BackendCall() // Perform backend call if Redis fetch fails
		return
	}

	if len(data) == 0 {
		f.log.Info("MISS: " + hKey)
		f.BackendCall() // Perform backend call if data is empty
		return
	}

	f.log.Info(fmt.Sprintf("HIT: %s", hKey))
	f.res, err = UnserializeHttpPayload(data)
	if err != nil {
		f.log.Error(fmt.Sprintf("Error deserializing data for key %s: %v", hKey, err))
		f.BackendCall() // Perform backend call if deserialization fails
		return
	}

	f.DeliverCache()
}

func (f *FSM) BackendCall() {
	beCtx := &HttpResponseCtx{
		startTime: f.startTime,
		w:         f.w,
		h:         make(http.Header),
	}

	f.next.ServeHTTP(beCtx, f.r)

	f.res.Body = beCtx.body
	f.res.Header = beCtx.h

	f.Hash()
}

func (f FSM) Hash() {
	f.IsCache()

	if f.cacheable {
		f.Cache()
	}
}

func (f *FSM) IsCache() {
	if bytes.Contains(f.req.Body, []byte("mutation")) {
		f.cacheable = false
		return
	}

	if f.req.Method != "POST" {
		f.cacheable = false
		return
	}

	f.cacheable = true
}

func (f *FSM) Cache() {
	hKey := f.req.HashKey()
	for name, _ := range f.res.Header {
		switch name {
		case "Content-Length", "Transfer-Encoding", "Content-Encoding", "Connection", "Date":
			f.res.Header.Del(name) // Remove auto-generated headers
		}
	}

	bytes, err := f.res.serialize()
	if err != nil {
		f.log.Error("Unable to serialize hash" + hKey)
		return
	}

	f.log.Info("PUT: " + hKey)
	f.rds.Set(hKey, bytes)
}

func (f *FSM) DeliverCache() {
	for name, values := range f.res.Header {
		f.w.Header().Set(name, strings.Join(values, ", "))
	}

	xHeaders := map[string]string{
		"X-Server":         "cacheproxy",
		"X-Cache":          "HIT",
		"Content-Encoding": "gzip",
		"X-Elapsed":        fmt.Sprintf("%vms", time.Since(f.startTime).Milliseconds()),
	}

	for key, value := range xHeaders {
		f.w.Header().Set(key, value)
	}

	f.w.WriteHeader(200)
	f.w.Write(f.res.Body)
}
