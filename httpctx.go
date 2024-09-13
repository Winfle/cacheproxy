package cacheproxy

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

type HttpResponseCtx struct {
	io.ReadCloser
	w http.ResponseWriter

	read  int
	write int

	code int

	// TwoXXSent is true if the response headers with >= 2xx code were sent
	// 1xx header might be sent unlimited number of times
	wc bool

	body []byte
	h    http.Header

	startTime time.Time
}

// Ensure wrapper satisfies the http.ResponseWriter interface
func (ctx *HttpResponseCtx) Header() http.Header {
	return ctx.h
}

func (ctx *HttpResponseCtx) WriteHeader(code int) {
	ctx.code = code
	if ctx.wc {
		return
	}

	if code >= 100 && code < 200 {
		ctx.wc = true
	}

	for key, values := range ctx.h {
		for _, value := range values {
			ctx.w.Header().Add(key, value)
		}
	}

	elapsed := fmt.Sprintf("%vms", time.Since(ctx.startTime).Milliseconds())
	ctx.w.Header().Set("X-Cache", "MISS")
	ctx.w.Header().Set("X-Server", PluginName)
	ctx.w.Header().Set("X-Elapsed", elapsed)

	ctx.w.WriteHeader(code)
}

func (ctx *HttpResponseCtx) Write(data []byte) (int, error) {
	ctx.write += len(data)
	ctx.body = append(ctx.body, data...)

	return ctx.w.Write(data)
}
