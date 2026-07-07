package test

import (
	"tradebot/pkg/db"

	"github.com/BurntSushi/toml"
	"github.com/go-pg/pg/v10"
	"github.com/vmkteam/cron"
	"github.com/vmkteam/vfs"
)

var (
	Repo db.TradebotRepo
	err  error
	Cfg  *Config
)

type Config struct {
	Database *pg.Options
	Server   struct {
		Host      string
		Port      int
		IsDevel   bool
		EnableVFS bool
	}
	Sentry struct {
		Environment string
		DSN         string
	}
	Service struct {
		ChatGPTSrvURL string
	}
	Cron struct {
		OzonWriter     cron.Schedule
		YandexWriter   cron.Schedule
		WBWriter       cron.Schedule
		OrderCleaner   cron.Schedule
		FetchReviews   cron.Schedule
		ProcessReviews cron.Schedule
	}
	VFS vfs.Config
}

func Setup() (*db.DB, error) {
	if _, err = toml.DecodeFile("/Users/sergey/GolandProjects/tradebot/cfg/local.toml", &Cfg); err != nil {
		return nil, err
	}

	pgdb := pg.Connect(Cfg.Database)
	dbc := db.New(pgdb)

	return &dbc, nil
}
