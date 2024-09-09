package middleware

import (
	"net/http"
)

const PluginName = "middleware"

type Plugin struct{}

func (p *Plugin) Init() error {
	return nil
}

func (p *Plugin) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// p.log.Info("Request")

		next.ServeHTTP(w, r)
	})
}

func (p *Plugin) Name() string {
	return PluginName
}
