package ozon

import (
	"context"
	"testing"
	"time"

	"tradebot/pkg/db"
	"tradebot/pkg/tradeplus/test"

	"github.com/BurntSushi/toml"
	"github.com/go-pg/pg/v10"
	"github.com/stretchr/testify/require"
)

var (
	testRepo db.TradebotRepo
	cabinet  *db.Cabinet
	err      error
	cfg      = test.Cfg
	ctx      = context.Background()
)

func TestMain(m *testing.M) {
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

func TestOrdersManager_GetReturnsMap(t *testing.T) {
	m := NewOrdersManager(*cabinet.ClientID, cabinet.Key, *cabinet.SheetLink)
	since := time.Now().AddDate(0, 0, -2).Format("2006-01-02") + "T21:00:00.000Z"
	to := time.Now().AddDate(0, 0, -1).Format("2006-01-02") + "T21:00:00.000Z"

	got, err := m.GetReturnsMap(*cabinet.ClientID, cabinet.Key, since, to)
	require.NoError(t, err)
	t.Log(got)
}
