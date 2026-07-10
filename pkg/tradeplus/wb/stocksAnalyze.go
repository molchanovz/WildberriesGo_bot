package wb

import (
	"fmt"

	"tradebot/pkg/client/google"
	"tradebot/pkg/client/wb"
)

var warehousesMap = map[string]string{
	"Санкт-Петербург Уткина Заводь":  "Северо-Западный федеральный округ",
	"Екатеринбург - Испытателей 14г": "Уральский федеральный округ",
	"Невинномысск":                   "Северо-Кавказский федеральный округ",
	"Новосибирск":                    "Сибирский федеральный округ",
	"Краснодар":                      "Южный федеральный округ",
	"Волгоград":                      "Южный федеральный округ",
	"Подольск":                       "Центральный федеральный округ",
	"Подольск 3":                     "Центральный федеральный округ",
	"Подольск 4":                     "Центральный федеральный округ",
	"Белые столбы":                   "Центральный федеральный округ",
	"Коледино":                       "Центральный федеральный округ",
	"Белая дача":                     "Центральный федеральный округ",
	"Электросталь":                   "Центральный федеральный округ",
	"Чашниково":                      "Центральный федеральный округ",
	"Тула":                           "Центральный федеральный округ",
	"Котовск":                        "Центральный федеральный округ",
	"Владимир":                       "Центральный федеральный округ",
	"Казань":                         "Приволжский федеральный округ",
	"Самара (Новосемейкино)":         "Приволжский федеральный округ",
	"Хабаровск":                      "Дальневосточный федеральный округ",
	"Рязань (Тюшевское)":             "Центральный федеральный округ",
}

type StockManager struct {
	client       wb.Client
	googleSheets google.SheetsService
}

func NewStockManager(token string) StockManager {
	return StockManager{
		client:       wb.NewClient(token),
		googleSheets: google.NewSheetsService("token.json", "credentials.json"),
	}
}

func (m StockManager) GetOrders(daysAgo int) (map[string]map[string]int, error) {
	orders, err := m.client.GetAllOrders(daysAgo, 0)
	if err != nil {
		return nil, fmt.Errorf("wb GetAllorders failed: %w", err)
	}

	ordersMap := make(map[string]map[string]int)

	for _, order := range orders {
		cluster := order.OblastOkrugName
		if cluster == "" {
			cluster = order.CountryName
		}
		if _, exists := ordersMap[cluster]; !exists {
			ordersMap[cluster] = make(map[string]int)
		}
		ordersMap[cluster][order.SupplierArticle] += 1
	}

	return ordersMap, nil
}

func (m StockManager) GetStocks() (map[string]map[string]int, map[string]int, error) {
	stocks, err := m.client.GetStockFbo()
	if err != nil {
		return nil, nil, err
	}

	stocksMap := make(map[string]map[string]int)

	lostWarehouses := make(map[string]int)

	for _, stock := range stocks {
		if federalRegion, ok := warehousesMap[stock.WarehouseName]; !ok && stock.Quantity > 0 {
			// Ищем потерянные склады с округах
			lostWarehouses[stock.WarehouseName] = 1
		} else {
			stock.WarehouseName = federalRegion
		}

		if _, exists := stocksMap[stock.WarehouseName]; !exists {
			stocksMap[stock.WarehouseName] = make(map[string]int)
		}

		stocksMap[stock.WarehouseName][stock.SupplierArticle] += stock.Quantity
	}
	return stocksMap, lostWarehouses, nil
}
