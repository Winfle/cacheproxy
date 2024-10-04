package cacheproxy

type Config struct {
	RedisAddr string `mapstructure:"redis_addr"`
	DB        string `mapstructure:"db"`
}
