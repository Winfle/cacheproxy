package cacheproxy

import (
	"fmt"
	"io"
	"net/http"
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

	ctx.w.WriteHeader(code)
}

func (ctx *HttpResponseCtx) Write(data []byte) (int, error) {
	ctx.write += len(data)
	fmt.Print("WRITING\n")
	ctx.body = append(ctx.body, data...)

	return ctx.w.Write(data)
}
