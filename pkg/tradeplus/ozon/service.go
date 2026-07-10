package ozon

import (
	"errors"

	"tradebot/pkg/tradeplus"
)

const (
	StocksDaysAgo = 14
)

var ErrNoRows = errors.New("no rows in result set")

type Service struct {
	tradeplus.Authorization
	spreadsheetID string
}

func NewService(cabinet tradeplus.Cabinet) Service {
	service := Service{
		Authorization: tradeplus.Authorization{
			Token: cabinet.Key,
		},
	}

	if cabinet.ClientID != nil {
		service.ClientID = *cabinet.ClientID
	}

	if cabinet.SheetLink != nil {
		service.spreadsheetID = *cabinet.SheetLink
	}

	return service
}

func (s Service) GetOrdersAndReturnsManager() OrdersManager {
	return NewOrdersManager(s.ClientID, s.Token, s.spreadsheetID)
}

func (s Service) GetStocksManager() AnalyzeManager {
	return NewAnalyzeManager(s.ClientID, s.Token, StocksDaysAgo)
}

func (s Service) GetStickersFBSManager(printedOrders map[string]struct{}, warehouseID int64) StickerManager {
	return NewStickerManager(s.ClientID, s.Token, printedOrders, warehouseID)
}
