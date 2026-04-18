package app

import (
	"context"
	"fmt"
	"github.com/vmkteam/cron"
	"net"
	"net/http"
	"time"
	"tradebot/pkg/bot"
	"tradebot/pkg/client/chatgptsrv"
	"tradebot/pkg/db"
	"tradebot/pkg/tradeplus/schedule"

	"github.com/go-pg/pg/v10"
	monitor "github.com/hypnoglow/go-pg-monitor"
	"github.com/labstack/echo/v4"
	"github.com/vmkteam/embedlog"
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
	Bot    bot.Config
	Sentry struct {
		Environment string
		DSN         string
	}
	Service struct {
		ChatGPTSrvURL string
	}
	Cron struct {
		OzonWriter     cron.Schedule
		OzonShipments  cron.Schedule
		YandexWriter   cron.Schedule
		WBWriter       cron.Schedule
		WBShipments    cron.Schedule
		OrderCleaner   cron.Schedule
		SendNewReviews cron.Schedule
	}
	OpenAI struct {
		Token string
	}
	VFS vfs.Config
}

type App struct {
	embedlog.Logger
	appName string
	cfg     Config
	db      db.DB
	dbc     *pg.DB
	mon     *monitor.Monitor
	echo    *echo.Echo
	//vtsrv   zenrpc.Server
	bs              *bot.Service
	scheduleManager schedule.Manager
	c               *cron.Manager
	gptSrv          *chatgptsrv.Client
}

func New(appName string, sl embedlog.Logger, cfg Config, db db.DB, dbc *pg.DB) *App {
	a := &App{
		appName: appName,
		cfg:     cfg,
		db:      db,
		dbc:     dbc,
		echo:    echo.New(),
		Logger:  sl,
	}

	a.c = a.newCron()

	a.gptSrv = chatgptsrv.NewClient(a.cfg.Service.ChatGPTSrvURL, &http.Client{Timeout: time.Second * 30})

	a.bs = bot.NewService(cfg.Bot, db, a.gptSrv, a.Logger)

	a.scheduleManager = schedule.NewManager(db, a.Logger, a.bs)

	// setup echo
	a.echo.HideBanner = true
	a.echo.HidePort = true
	_, mask, _ := net.ParseCIDR("0.0.0.0/0")
	a.echo.IPExtractor = echo.ExtractIPFromRealIPHeader(echo.TrustIPRange(mask))

	// add services
	//a.vtsrv = vt.New(a.db, a.Logger, a.cfg.Server.IsDevel)
	return a
}

// Run is a function that runs application.
func (a *App) Run(ctx context.Context) error {
	a.registerMetrics()
	a.registerHandlers()
	a.registerDebugHandlers()
	//a.registerAPIHandlers()
	//a.registerVTApiHandlers()
	a.bs.Start()

	// run cron
	if err := a.c.Run(ctx); err != nil {
		return err
	} else {
		a.Logger.Print(ctx, "open this url for cron ui", "url", fmt.Sprintf("http://%v:%v/debug/cron", a.cfg.Server.Host, a.cfg.Server.Port))
		a.Logger.Print(ctx, "open this url for metrics", "url", fmt.Sprintf("http://%v:%v/metrics", a.cfg.Server.Host, a.cfg.Server.Port))
	}

	return a.runHTTPServer(ctx, a.cfg.Server.Host, a.cfg.Server.Port)
}

// VTTypeScriptClient returns TypeScript client for VT.
//func (a *App) VTTypeScriptClient() ([]byte, error) {
//	gen := rpcgen.FromSMD(a.vtsrv.SMD())
//	tsSettings := typescript.Settings{ExcludedNamespace: []string{NSVFS}, WithClasses: true}
//	return gen.TSCustomClient(tsSettings).Generate()
//}

// Shutdown is a function that gracefully stops HTTP server.
func (a *App) Shutdown(timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	a.mon.Close()

	if err := a.echo.Shutdown(ctx); err != nil {
		a.Error(ctx, "shutting down server", "err", err)
	}
}
