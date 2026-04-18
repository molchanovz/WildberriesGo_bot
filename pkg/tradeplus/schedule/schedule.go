package schedule

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/vmkteam/embedlog"
	"tradebot/pkg/bot"
	"tradebot/pkg/db"
	"tradebot/pkg/tradeplus"
	"tradebot/pkg/tradeplus/ozon"
	"tradebot/pkg/tradeplus/wb"
	"tradebot/pkg/tradeplus/yandex"
)

type Manager struct {
	embedlog.Logger
	tm *tradeplus.Manager
	bs *bot.Service
}

func NewManager(dbc db.DB, logger embedlog.Logger, bs *bot.Service) Manager {
	return Manager{tm: tradeplus.NewManager(dbc), Logger: logger, bs: bs}
}

func (s *Manager) WriteWB(ctx context.Context) error {
	cabinets, err := s.tm.GetCabinetsByMp(ctx, db.MarketWB)
	if err != nil {
		return fmt.Errorf("fetch cabinet failed: %w", err)
	}

	if cabinets[0].SheetLink == nil {
		return errors.New("sheet link is null")
	}

	manager := wb.NewOrdersManager(cabinets[0].Key, *cabinets[0].SheetLink)

	err = manager.Write()
	if err != nil {
		return fmt.Errorf("write orders failed: %w", err)
	}

	return nil
}

func (s *Manager) WriteOzon(ctx context.Context) error {
	cabinets, err := s.tm.GetCabinetsByMp(ctx, db.MarketOzon)
	if err != nil {
		return err
	}

	titleRange := "!A1"
	fbsRange := "!A2:B1000"
	fboRange := "!D2:E1000"
	returnsRange := "!G2:H1000"

	maxValuesCount, err := ozon.NewService(cabinets[0]).GetOrdersAndReturnsManager().WriteToGoogleSheets(titleRange, fbsRange, fboRange, returnsRange)
	if err != nil {
		return err
	}

	maxValuesCount += 3
	titleRange = fmt.Sprintf("!A%v", maxValuesCount)

	maxValuesCount++
	fbsRange = fmt.Sprintf("!A%v:B%v", maxValuesCount, maxValuesCount+1000)
	fboRange = fmt.Sprintf("!D%v:E%v", maxValuesCount, maxValuesCount+1000)
	returnsRange = fmt.Sprintf("!G%v:H%v", maxValuesCount, maxValuesCount+1000)

	_, err = ozon.NewService(cabinets[1]).GetOrdersAndReturnsManager().WriteToGoogleSheets(titleRange, fbsRange, fboRange, returnsRange)
	if err != nil {
		return err
	}

	return nil
}

func (s *Manager) WriteOzonShipments(ctx context.Context) error {
	cabinets, err := s.tm.GetCabinetsByMp(ctx, db.MarketOzon)
	if err != nil {
		return err
	}

	msk := time.FixedZone("MSK", 3*3600)
	now := time.Now().In(msk).AddDate(0, 0, -tradeplus.OrdersDaysAgo)
	yesterday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, msk)

	var failed []string
	for _, cab := range cabinets {
		if cab.Settings.ShipmentsSheetID == "" {
			s.Print(ctx, fmt.Sprintf("ozonShipments: cabinet=%d skipped (no shipmentsSheetId)", cab.ID))
			continue
		}
		m, err := ozon.NewShipmentsManager(cab)
		if err != nil {
			failed = append(failed, fmt.Sprintf("cab=%d init: %v", cab.ID, err))
			continue
		}
		if err := m.WriteForDate(ctx, yesterday); err != nil {
			failed = append(failed, err.Error())
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("ozonShipments: %s", strings.Join(failed, "; "))
	}
	return nil
}

func (s *Manager) WriteWBShipments(ctx context.Context) error {
	cabinets, err := s.tm.GetCabinetsByMp(ctx, db.MarketWB)
	if err != nil {
		return err
	}

	msk := time.FixedZone("MSK", 3*3600)
	now := time.Now().In(msk).AddDate(0, 0, -tradeplus.OrdersDaysAgo)
	yesterday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, msk)

	var failed []string
	for _, cab := range cabinets {
		if cab.Settings.ShipmentsSheetID == "" {
			s.Print(ctx, fmt.Sprintf("wbShipments: cabinet=%d skipped (no shipmentsSheetId)", cab.ID))
			continue
		}
		m, err := wb.NewShipmentsManager(cab)
		if err != nil {
			failed = append(failed, fmt.Sprintf("cab=%d init: %v", cab.ID, err))
			continue
		}
		if err := m.WriteForDate(ctx, yesterday); err != nil {
			failed = append(failed, err.Error())
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("wbShipments: %s", strings.Join(failed, "; "))
	}
	return nil
}

func (s *Manager) WriteYandex(ctx context.Context) error {
	cabinets, err := s.tm.GetCabinetsByMp(ctx, db.MarketYandex)
	if err != nil {
		return err
	}

	err = yandex.NewService(cabinets...).GetOrdersAndReturnsManager().Write()
	if err != nil {
		return err
	}

	return nil
}

func (s *Manager) ClearOrders(ctx context.Context) error {
	return s.tm.DeleteOrders(ctx)
}

func (s *Manager) SendNewReviews(ctx context.Context) error {
	return s.bs.Manager().SendNewReviews(ctx)
}
