package cacheproxy

import (
	"fmt"
)

func ProcessCtx(hash string, ctx *HttpResponseCtx) {
	cache.Set(hash, getResponseBody(ctx))
	fmt.Println("PUT:", hash)
}

func getResponseBody(ctx *HttpResponseCtx) string {
	var bodyStr string
	if (ctx.h.Get("Content-Encoding") == "gzip") {
		bodyStr, _ = DecompressGzip(ctx.body)
		return bodyStr
	} 

	return string(ctx.body)	
}