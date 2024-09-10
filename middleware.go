package cacheproxy

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	// "compress/gzip"

	"go.uber.org/zap"
)

const PluginName = "cacheproxy"

var logger *zap.Logger

type Plugin struct{}

var redisDns = "redis:6379"
var cache *RedisClient

var reqBuffer []byte

func (p *Plugin) Init() error {
	fmt.Println("Instantiating ProxyCache middleware")
	cache = InitRedisConnection(redisDns)

	return nil
}

func (p *Plugin) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		r.Body.Close()

		hash := HashBytes(bodyBytes)

		if cacheBody, _ := cache.Get(hash); cacheBody != "" {
			fmt.Printf("HIT: %s\n", hash)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			w.Write([]byte(cacheBody))
			return
		}
		fmt.Printf("MISS: %s\n", hash)

		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		httpCtx := &HttpResponseCtx{
			w: w,
			h: make(http.Header),
		}

		next.ServeHTTP(httpCtx, r)
		
		ProcessCtx(hash, httpCtx)
	})
}


func (p *Plugin) Name() string {
	return PluginName
}