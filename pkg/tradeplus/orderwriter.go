package tradeplus

import (
	"tradebot/pkg/client/google"
)

const OrdersDaysAgo = 1

type GoogleSheetWriter interface {
	Write() error
}

type OrderManager struct {
	DaysAgo       int
	SpreadsheetID string
	GoogleService google.SheetsService
}

func NewOrdersManager(spreadsheetID string) OrderManager {
	ordersManager := OrderManager{
		DaysAgo:       OrdersDaysAgo,
		SpreadsheetID: spreadsheetID,
		GoogleService: google.NewSheetsService("pkg/client/google/token.json", "pkg/client/google/credentials.json"),
	}

	return ordersManager
}
