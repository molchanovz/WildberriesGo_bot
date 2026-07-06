package ozon

import (
	"context"
	"testing"

	"tradebot/pkg/db"

	"github.com/BurntSushi/toml"
	"github.com/go-pg/pg/v10"
	"github.com/vmkteam/vfs"
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
	VFS vfs.Config
}

var (
	testRepo db.TradebotRepo
	cabinet  *db.Cabinet
	cfg      Config
	ctx      = context.Background()
)

func TestMain(m *testing.M) {
	var err error

	if _, err = toml.DecodeFile("/Users/sergey/GolandProjects/tradebot/cfg/local.toml", &cfg); err != nil {
		return
	}

	pgdb := pg.Connect(cfg.Database)
	dbc := db.New(pgdb)
	testRepo = db.NewTradebotRepo(dbc)

	cabinet, err = testRepo.CabinetByID(ctx, 3)
	if err != nil {
		return
	}
	m.Run()
}
