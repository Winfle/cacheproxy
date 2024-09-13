package cacheproxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"go.uber.org/zap"
)

type HttpPayload struct {
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

func (h *HttpPayload) serialize() ([]byte, error) {
	p := HttpPayload{
		Method: h.Method,
		Body:   []byte(h.GetResponseBody()),
		Header: h.Header,
	}

	j, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}

	return j, nil

	// var compressedData bytes.Buffer
	// gz := gzip.NewWriter(&compressedData)
	// if _, err := gz.Write(json); err != nil {
	// 	return nil, err
	// }

	// if err := gz.Close(); err != nil {
	// 	return nil, err
	// }

	// return compressedData.Bytes(), nil
}

func UnserializeHttpPayload(data []byte) (HttpPayload, error) {
	// j, err := DecompressGzip(data)
	// if err != nil {
	// 	return HttpPayload{}, err
	// }

	var p HttpPayload
	err := json.Unmarshal([]byte(data), &p)
	if err != nil {
		return HttpPayload{}, err
	}

	return p, nil
}

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
}

func (f FSM) ServeHTTP(w http.ResponseWriter, r *http.Request) {

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

	var err error
	hash := HashBytes(f.req.Body)

	data, _ := f.rds.Get(hash)
	if len(data) > 0 {
		f.log.Info(fmt.Sprintf("HIT: %s", hash))
		f.res, err = UnserializeHttpPayload(data)
		if err == nil {
			f.Deliver()
			return
		}
		f.log.Error("Error deserealizing object: " + hash + err.Error())
	}

	f.BackendCall()
}

func (f *FSM) Deliver() {
	// for name, values := range f.res.Header {
	// 	for _, value := range values {
	// 		f.w.Header().Set(name, value)
	// 	}
	// }

	f.w.Header().Set("Content-Type", "application/json")
	f.w.Header().Set("Server", "graphrunner-cacheproxy")

	f.w.WriteHeader(200)
	f.w.Write(f.res.Body)
	return
}

func (f *FSM) BackendCall() {
	beCtx := &HttpResponseCtx{
		w: f.w,
		h: make(http.Header),
	}

	f.next.ServeHTTP(beCtx, f.r)

	f.res.Body = beCtx.body
	f.res.Header = beCtx.h

	f.log.Info("Body: " + string(beCtx.body))

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

	hash := HashBytes(f.req.Body)
	bytes, err := f.res.serialize()
	if err != nil {
		f.log.Error("Unable to serialize hash" + hash)
		return

	}
	f.rds.Set(hash, bytes)
	f.log.Info("PUT: " + hash)
}

var ww http.ResponseWriter
var rr *http.Request
