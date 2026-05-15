package config

import (
	"log"

	"github.com/spf13/viper"
)

type Config struct {
	App struct {
		Name string
		Port string
	}

	Postgres struct {
		DSN string
	}

	Redis struct {
		Addr string
	}
}

var Cfg *Config

func Init() {
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")

	err := viper.ReadInConfig()
	if err != nil {
		log.Fatal("read config failed:", err)
	}

	Cfg = &Config{}

	Cfg.App.Name = viper.GetString("APP_NAME")
	Cfg.App.Port = viper.GetString("APP_PORT")

	Cfg.Postgres.DSN = viper.GetString("POSTGRES_DSN")
	Cfg.Redis.Addr = viper.GetString("REDIS_ADDR")
}