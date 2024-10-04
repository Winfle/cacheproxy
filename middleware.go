package cacheproxy

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/roadrunner-server/errors"
	"go.uber.org/zap"
)

const PluginName = "cacheproxy"

type Plugin struct {
	log       *zap.Logger
	cfg       *Config
	cancelCtx *context.CancelFunc
	rds       *RedisClient
}

type Logger interface {
	NamedLogger(name string) *zap.Logger
}

type Configurer interface {
	UnmarshalKey(name string, out any) error

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

	ctx := context.Background()

	var initErr error

	if p.cfg.DB == "" {
		p.log.Error("redis db is not set")
		return errors.E(errors.Disabled)
	}

	db, err := strconv.Atoi(p.cfg.DB)
	if err != nil {
		fmt.Println("error parsing DB number:", err)
		return errors.E(errors.Disabled)
	}

	p.log.Info(fmt.Sprintf("connecting to redis: %s[%d]", p.cfg.RedisAddr, db))
	p.rds, initErr = InitRedisConnection(p.cfg.RedisAddr, db, ctx)
	if initErr != nil {
		p.log.Error(initErr.Error())
		return errors.E(errors.Disabled)
	}

	p.log.Info("connected")

	return nil
}

func (p *Plugin) Stop() error {
	(*p.cancelCtx)()

	return nil
}

func (p *Plugin) Middleware(next http.Handler) http.Handler {
	fsm := FSM{
		rds:  p.rds,
		log:  p.log,
		next: next,
	}

	return fsm
}

func (p *Plugin) Name() string {
	return PluginName
}
