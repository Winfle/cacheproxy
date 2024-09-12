package cacheproxy

import (
	"context"
	"net/http"

	"github.com/roadrunner-server/errors"
	"go.uber.org/zap"
)

const PluginName = "cacheproxy"

type Plugin struct {
	log       *zap.Logger
	cfg       *Config
	cancelCtx *context.CancelFunc
	fsm       *FSM
}

type Logger interface {
	NamedLogger(name string) *zap.Logger
}

type Configurer interface {
	// UnmarshalKey takes a single key and unmarshal it into a Struct.
	UnmarshalKey(name string, out any) error

	// Has checks if a config section exists.
	Has(name string) bool
}

func (p *Plugin) Init(l Logger, cfg Configurer) error {
	logger := l.NamedLogger(PluginName)
	p.log = logger

	if !cfg.Has(PluginName) {
		p.log.Warn("middleware is disabled")
		return errors.E(errors.Disabled)
	}

	err := cfg.UnmarshalKey(PluginName, &p.cfg)
	if err != nil {
		p.log.Error("config is not set")
		return errors.E(errors.Disabled)
	}

	p.log.Info("connecting to redis: " + p.cfg.RedisAddr)

	ctx := context.Background()

	rdsClient, initErr := initRedisConnection(p.cfg.RedisAddr, ctx)
	if initErr != nil {
		p.log.Error(initErr.Error())
		return errors.E(errors.Disabled)
	}

	// Init FSM
	p.fsm = &FSM{
		rds: rdsClient,
		log: logger,
	}

	return nil
}

func (p *Plugin) Stop() error {
	(*p.cancelCtx)()

	return nil
}

func (p *Plugin) Middleware(next http.Handler) http.Handler {
	p.fsm.next = next

	return p.fsm
}

func (p *Plugin) Name() string {
	return PluginName
}
