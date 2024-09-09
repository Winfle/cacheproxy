package middleware

import (
	"net/http"

	"go.uber.org/zap"
)

const PluginName = "middleware"

var logger *zap.Logger

type Plugin struct{}

func (p *Plugin) Init() error {
	logger := zap.Must(zap.NewProduction())

	defer logger.Sync()

	logger.Info("Hello from Zap logger!")
	return nil
}

func (p *Plugin) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.Info("Request")

		next.ServeHTTP(w, r)
	})
}

func (p *Plugin) Name() string {
	return PluginName
}
