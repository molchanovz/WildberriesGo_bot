package db

import (
	"context"
	"log"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/go-pg/pg/v10"
	"github.com/stretchr/testify/require"
)

type Config struct {
	Database *pg.Options
	Server   struct {
		Host      string
		Port      int
		IsDevel   bool
		EnableVFS bool
	}
}

var (
	cfg    Config
	testDB *pg.DB
	repo   TradebotRepo
	ctx    = context.Background()
)

func TestMain(m *testing.M) {
	_, err := toml.DecodeFile("/Users/sergey/GolandProjects/tradebot/cfg/local.toml", &cfg)
	if err != nil {
		log.Println(err)
		return
	}
	testDB = pg.Connect(cfg.Database)
	d := New(testDB)
	repo = NewTradebotRepo(d)
	m.Run()
}

func TestTradebotRepo_DeleteOrdersByFilter(t *testing.T) {
	got, err := repo.DeleteOrdersLastWeek(context.Background())
	require.NoError(t, err)
	t.Log(got)
}
