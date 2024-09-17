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

const DEFAULT_TTL = 3600

type FSM struct {
	rds  *RedisClient
	next http.Handler
	log  *zap.Logger

	w http.ResponseWriter
	r *http.Request

	cacheable bool
	ttl       int

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

	f.res.Status = beCtx.code
	f.res.Body = beCtx.body
	f.res.Header = beCtx.h

	f.Hash()
}

func (f FSM) Hash() {
	f.SetTTL()
	f.Cacheable()

	f.Cache()
}

func (f *FSM) Cacheable() {
	f.cacheable = false

	if f.ttl == 0 {
		return
	}

	if f.res.Status != 200 {
		return
	}

	if f.req.Method != "POST" {
		return
	}

	if bytes.Contains(f.req.Body, []byte("mutation")) {
		return
	}

	cacheControl := f.res.Header.Get("Cache-Control")
	if cacheControl != "" {
		if strings.Contains(cacheControl, "no-store") || strings.Contains(cacheControl, "no-cache") {
			return
		}
	}

	expires := f.res.Header.Get("Expires")
	if expires != "" {
		expireTime, err := http.ParseTime(expires)
		if err == nil && time.Now().After(expireTime) {
			return
		}
	}

	f.cacheable = true
}

func (f *FSM) SetTTL() {
	f.ttl = f.res.GetTTL()
}

func (f *FSM) Cache() {
	if !f.cacheable || f.ttl == 0 {
		return
	}

	hKey := f.req.HashKey()

	f.res.RemovePayloadHeaders()
	bytes, err := f.res.Serialize()
	if err != nil {
		f.log.Error("Unable to serialize hash" + hKey)
		return
	}

	f.log.Info("PUT: " + hKey)
	f.rds.Set(hKey, bytes, f.ttl)
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
