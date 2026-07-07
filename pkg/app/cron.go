package app

import (
	"github.com/vmkteam/cron"
)

func (a *App) newCron() *cron.Manager {
	cm := cron.NewManager()
	cm.Use(
		cron.WithMetrics(a.appName),
		cron.WithDevel(false),
		cron.WithSLog(a.Logger),
		cron.WithSkipActive(),
		cron.WithRecover(), // recover() inside
	)

	// add simple func
	cm.AddFunc("wbOrders", a.cfg.Cron.WBWriter, a.scheduleManager.WriteWB)
	cm.AddFunc("wbShipments", a.cfg.Cron.WBShipments, a.scheduleManager.WriteWBShipments)
	cm.AddFunc("wbShipmentsAll", a.cfg.Cron.WBShipmentsAll, a.scheduleManager.WriteWBShipmentsAll)
	cm.AddFunc("ozonOrders", a.cfg.Cron.OzonWriter, a.scheduleManager.WriteOzon)
	cm.AddFunc("ozonShipments", a.cfg.Cron.OzonShipments, a.scheduleManager.WriteOzonShipments)
	cm.AddFunc("ozonShipmentsAll", a.cfg.Cron.OzonShipmentsAll, a.scheduleManager.WriteOzonShipmentsAll)
	cm.AddFunc("yandexOrders", a.cfg.Cron.YandexWriter, a.scheduleManager.WriteYandex)
	cm.AddFunc("yandexShipments", a.cfg.Cron.YandexShipments, a.scheduleManager.WriteYandexShipments)
	cm.AddFunc("cleanOrders", a.cfg.Cron.OrderCleaner, a.scheduleManager.ClearOrders)
	cm.AddFunc("fetchReviews", a.cfg.Cron.FetchReviews, a.scheduleManager.FetchReviews)
	cm.AddFunc("processReviews", a.cfg.Cron.ProcessReviews, a.scheduleManager.ProcessReviews)

	return cm
}
