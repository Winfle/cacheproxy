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
		f.log.Error("Error redis fetch: " + hKey + err.Error())
		f.BackendCall()
		return
	}

	if len(data) > 0 {
		f.log.Info("HIT: " + hKey)

		f.res, err = UnserializeHttpPayload(data)
		if err == nil {
			f.Deliver()
			return
		}
		f.log.Error("Error deserealizing object: " + hKey + err.Error())
	}

	f.BackendCall()
}

func (f *FSM) Deliver() {
	for name, values := range f.res.Header {
		if name == "Content-Length" || name == "Transfer-Encoding" || name == "Content-Encoding" ||
			name == "Connection" || name == "Date" {
			continue
		}

		if len(values) > 1 {
			concatenatedValues := strings.Join(values, ", ")
			f.w.Header().Set(name, concatenatedValues)
		} else if len(values) == 1 {
			f.w.Header().Set(name, values[0])
		}
	}

	f.w.Header().Set("X-Server", "graphrunner-cacheproxy")
	f.w.Header().Set("X-Cache", "HIT")
	f.w.Header().Set("X-Elapsed", fmt.Sprintf("%vms", time.Since(f.startTime).Milliseconds()))

	f.w.WriteHeader(200)
	f.w.Write(f.res.Body)
	return
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
	if f.req.Method == "POST" {
		f.cacheable = true
	} else {
		f.cacheable = false
	}

	f.Cache()
}

func (f *FSM) Cache() {
	if !f.cacheable {
		return
	}

	hKey := f.req.HashKey()
	bytes, err := f.res.serialize()
	if err != nil {
		f.log.Error("Unable to serialize hash" + hKey)
		return
	}

	f.log.Info("PUT: " + hKey)
	f.rds.Set(hKey, bytes)
}
