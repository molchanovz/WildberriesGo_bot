package wb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"tradebot/pkg/client/google"
	"tradebot/pkg/tradeplus"
)

const (
	wbSupplierURL          = "https://marketplace-api.wildberries.ru"
	wbHTTPTimeout          = 30 * time.Second
	wbSuppliesLimit        = 1000
	wbOrdersLimit          = 1000
	wbStickerBatch         = 100
	wbStatusBatch          = 1000
	wbOrdersWindowDaysBack = 60
	wbOrdersWindowFw       = 1
)

var wbSupplierStatusRu = map[string]string{
	"new":       "Новое",
	"confirm":   "На сборке",
	"complete":  "В доставке",
	"cancel":    "Отменено",
	"deliver":   "В доставке",
	"receive":   "Получено",
	"reject":    "Отказ",
	"waiting":   "Ожидает",
	"sorted":    "Отсортировано",
	"sold":      "Продано",
	"canceled":  "Отменено",
	"declined":  "Отклонено",
	"defect":    "Брак",
	"ready":     "Готово к выдаче",
	"delivered": "Доставлено",
}

var wbHeader = []string{
	"№ задания", "QR-код поставки", "Стикер", "Дата создания", "Дата сканирования ШК ТТН",
	"Наименование", "Размер", "Цвет", "Баркод", "Стоимость", "Валюта",
	"Артикул Wildberries", "Артикул продавца", "Склад продавца", "Статус задания",
}

type wbShipmentsAPI struct {
	token string
	hc    *http.Client
}

func newWBShipmentsAPI(token string) wbShipmentsAPI {
	return wbShipmentsAPI{token: token, hc: &http.Client{Timeout: wbHTTPTimeout}}
}

func (a wbShipmentsAPI) do(ctx context.Context, method, path string, params url.Values, body any) ([]byte, error) {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rdr = bytes.NewReader(b)
	}
	u := wbSupplierURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rdr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", a.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return data, fmt.Errorf("wb %s %s: %d %s", method, path, resp.StatusCode, string(data))
	}
	return data, nil
}

type wbSupply struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	CreatedAt time.Time  `json:"createdAt"`
	ClosedAt  *time.Time `json:"closedAt"`
	ScanDt    *time.Time `json:"scanDt"`
	Done      bool       `json:"done"`
}

type wbSuppliesResp struct {
	Next     int        `json:"next"`
	Supplies []wbSupply `json:"supplies"`
}

type wbOrderIDsResp struct {
	IDs []int `json:"orderIds"`
}

type wbOrderFBS struct {
	SupplyID     string    `json:"supplyId"`
	OrderUID     string    `json:"orderUid"`
	Article      string    `json:"article"`
	RID          string    `json:"rid"`
	CreatedAt    time.Time `json:"createdAt"`
	Skus         []string  `json:"skus"`
	ID           int       `json:"id"`
	WarehouseID  int       `json:"warehouseId"`
	NmID         int       `json:"nmId"`
	Price        float64   `json:"price"`
	CurrencyCode int       `json:"currencyCode"`
}

type wbOrdersResp struct {
	Next   int          `json:"next"`
	Orders []wbOrderFBS `json:"orders"`
}

type wbStickersResp struct {
	Stickers []struct {
		OrderID int    `json:"orderId"`
		PartA   string `json:"partA"`
		PartB   string `json:"partB"`
		Barcode string `json:"barcode"`
	} `json:"stickers"`
}

type wbStatusesResp struct {
	Orders []struct {
		ID             int    `json:"id"`
		SupplierStatus string `json:"supplierStatus"`
		WbStatus       string `json:"wbStatus"`
	} `json:"orders"`
}

type wbWarehouse struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	OfficeID int    `json:"officeId"`
}

func (a wbShipmentsAPI) listSuppliesOnDate(ctx context.Context, loc *time.Location, day time.Time) ([]wbSupply, error) {
	dayStart := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc)
	dayEnd := dayStart.Add(24 * time.Hour)

	var out []wbSupply
	next := 0
	for {
		params := url.Values{}
		params.Set("limit", strconv.Itoa(wbSuppliesLimit))
		params.Set("next", strconv.Itoa(next))
		data, err := a.do(ctx, http.MethodGet, "/api/v3/supplies", params, nil)
		if err != nil {
			return nil, err
		}
		var r wbSuppliesResp
		if err := json.Unmarshal(data, &r); err != nil {
			return nil, fmt.Errorf("decode supplies: %w; body=%s", err, string(data))
		}
		var newest time.Time
		for _, s := range r.Supplies {
			if s.ScanDt != nil && !s.ScanDt.IsZero() {
				t := s.ScanDt.In(loc)
				if !t.Before(dayStart) && t.Before(dayEnd) {
					out = append(out, s)
				}
			}
			if s.CreatedAt.After(newest) {
				newest = s.CreatedAt
			}
		}
		if len(r.Supplies) == 0 || r.Next == 0 || r.Next == next {
			break
		}
		next = r.Next
		// pages go oldest → newest; once newest is well past target, stop.
		if !newest.IsZero() && newest.In(loc).After(dayEnd.Add(7*24*time.Hour)) {
			break
		}
	}
	return out, nil
}

func (a wbShipmentsAPI) orderIDsBySupply(ctx context.Context, supplyID string) ([]int, error) {
	data, err := a.do(ctx, http.MethodGet, "/api/marketplace/v3/supplies/"+supplyID+"/order-ids", nil, nil)
	if err != nil {
		return nil, err
	}
	var r wbOrderIDsResp
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("decode order-ids %s: %w; body=%s", supplyID, err, string(data))
	}
	return r.IDs, nil
}

func (a wbShipmentsAPI) listOrdersInWindow(ctx context.Context, from, to time.Time, wanted map[int]struct{}) (map[int]wbOrderFBS, error) {
	out := make(map[int]wbOrderFBS, len(wanted))
	const chunk = 29 * 24 * time.Hour // WB caps range at 30 days
	chunkTo := to
	for chunkTo.After(from) && len(out) < len(wanted) {
		chunkFrom := chunkTo.Add(-chunk)
		if chunkFrom.Before(from) {
			chunkFrom = from
		}
		next := 0
		for {
			params := url.Values{}
			params.Set("limit", strconv.Itoa(wbOrdersLimit))
			params.Set("next", strconv.Itoa(next))
			params.Set("dateFrom", strconv.FormatInt(chunkFrom.Unix(), 10))
			params.Set("dateTo", strconv.FormatInt(chunkTo.Unix(), 10))
			data, err := a.do(ctx, http.MethodGet, "/api/v3/orders", params, nil)
			if err != nil {
				return nil, err
			}
			var r wbOrdersResp
			if err := json.Unmarshal(data, &r); err != nil {
				return nil, fmt.Errorf("decode orders: %w; body=%s", err, string(data))
			}
			for _, o := range r.Orders {
				if _, ok := wanted[o.ID]; ok {
					out[o.ID] = o
				}
			}
			if len(r.Orders) < wbOrdersLimit || r.Next == 0 || r.Next == next {
				break
			}
			next = r.Next
			if len(out) == len(wanted) {
				break
			}
		}
		chunkTo = chunkFrom
	}
	return out, nil
}

func (a wbShipmentsAPI) stickers(ctx context.Context, ids []int) (map[int]string, error) {
	out := make(map[int]string, len(ids))
	for i := 0; i < len(ids); i += wbStickerBatch {
		end := i + wbStickerBatch
		if end > len(ids) {
			end = len(ids)
		}
		params := url.Values{}
		params.Set("type", "png")
		params.Set("width", "58")
		params.Set("height", "40")
		body := map[string]any{"orders": ids[i:end]}
		data, err := a.do(ctx, http.MethodPost, "/api/v3/orders/stickers", params, body)
		if err != nil {
			return nil, err
		}
		var r wbStickersResp
		if err := json.Unmarshal(data, &r); err != nil {
			return nil, fmt.Errorf("decode stickers: %w; body=%s", err, string(data))
		}
		for _, s := range r.Stickers {
			out[s.OrderID] = s.PartA + s.PartB
		}
	}
	return out, nil
}

func (a wbShipmentsAPI) statuses(ctx context.Context, ids []int) (map[int]string, error) {
	out := make(map[int]string, len(ids))
	for i := 0; i < len(ids); i += wbStatusBatch {
		end := i + wbStatusBatch
		if end > len(ids) {
			end = len(ids)
		}
		body := map[string]any{"orders": ids[i:end]}
		data, err := a.do(ctx, http.MethodPost, "/api/v3/orders/status", nil, body)
		if err != nil {
			return nil, err
		}
		var r wbStatusesResp
		if err := json.Unmarshal(data, &r); err != nil {
			return nil, fmt.Errorf("decode statuses: %w; body=%s", err, string(data))
		}
		for _, o := range r.Orders {
			if ru, ok := wbSupplierStatusRu[o.SupplierStatus]; ok {
				out[o.ID] = ru
			} else {
				out[o.ID] = o.SupplierStatus
			}
		}
	}
	return out, nil
}

func (a wbShipmentsAPI) warehouses(ctx context.Context) ([]wbWarehouse, error) {
	data, err := a.do(ctx, http.MethodGet, "/api/v3/warehouses", nil, nil)
	if err != nil {
		return nil, err
	}
	var ws []wbWarehouse
	if err := json.Unmarshal(data, &ws); err != nil {
		return nil, fmt.Errorf("decode warehouses: %w; body=%s", err, string(data))
	}
	return ws, nil
}

// ShipmentsManager заливает FBS-отгрузки WB кабинета за выбранную дату в Google Sheets,
// разделяя строки по складам: main → один лист, остальные → ФФ-лист.
type ShipmentsManager struct {
	api           wbShipmentsAPI
	sheets        google.SheetsService
	spreadsheetID string
	mainIDs       map[int]struct{}
	cabinetID     int
	cabinetName   string
}

func NewShipmentsManager(cabinet tradeplus.Cabinet) (*ShipmentsManager, error) {
	if cabinet.Key == "" {
		return nil, fmt.Errorf("cabinet %d: empty WB token", cabinet.ID)
	}
	if cabinet.Settings.ShipmentsSheetID == "" {
		return nil, fmt.Errorf("cabinet %d: empty shipmentsSheetId", cabinet.ID)
	}
	mains := make(map[int]struct{}, len(cabinet.Settings.MainWarehouseIDs))
	for _, id := range cabinet.Settings.MainWarehouseIDs {
		mains[id] = struct{}{}
	}
	return &ShipmentsManager{
		api:           newWBShipmentsAPI(cabinet.Key),
		sheets:        google.NewSheetsService("pkg/client/google/token.json", "pkg/client/google/credentials.json"),
		spreadsheetID: cabinet.Settings.ShipmentsSheetID,
		mainIDs:       mains,
		cabinetID:     cabinet.ID,
		cabinetName:   cabinet.Name,
	}, nil
}

type warehouseBlock struct {
	name string
	rows [][]string
}

// WriteForDate берёт supplies со scanDt = day (MSK), достаёт заказы/стикеры/статусы,
// группирует по warehouseId и пишет в листы main / ФФ.
func (m *ShipmentsManager) WriteForDate(ctx context.Context, day time.Time) error {
	msk := day.Location()

	supplies, err := m.api.listSuppliesOnDate(ctx, msk, day)
	if err != nil {
		return fmt.Errorf("list supplies: %w", err)
	}
	if len(supplies) == 0 {
		log.Printf("wbShipments: cabinet=%d no supplies, skip", m.cabinetID)
		return nil
	}

	orderToSupply := map[int]*wbSupply{}
	var allOrderIDs []int
	for i := range supplies {
		ids, err := m.api.orderIDsBySupply(ctx, supplies[i].ID)
		if err != nil {
			return fmt.Errorf("order-ids %s: %w", supplies[i].ID, err)
		}
		for _, id := range ids {
			orderToSupply[id] = &supplies[i]
			allOrderIDs = append(allOrderIDs, id)
		}
	}
	if len(allOrderIDs) == 0 {
		log.Printf("wbShipments: cabinet=%d no orders, skip", m.cabinetID)
		return nil
	}

	wanted := make(map[int]struct{}, len(allOrderIDs))
	for _, id := range allOrderIDs {
		wanted[id] = struct{}{}
	}
	from := day.AddDate(0, 0, -wbOrdersWindowDaysBack)
	to := day.AddDate(0, 0, wbOrdersWindowFw)
	orders, err := m.api.listOrdersInWindow(ctx, from, to, wanted)
	if err != nil {
		return fmt.Errorf("list orders: %w", err)
	}

	stickerMap, err := m.api.stickers(ctx, allOrderIDs)
	if err != nil {
		return fmt.Errorf("stickers: %w", err)
	}
	statusMap, err := m.api.statuses(ctx, allOrderIDs)
	if err != nil {
		return fmt.Errorf("statuses: %w", err)
	}

	whs, err := m.api.warehouses(ctx)
	if err != nil {
		log.Printf("wbShipments: cabinet=%d warehouses fetch failed: %v (using IDs)", m.cabinetID, err)
	}
	nameByID := make(map[int]string, len(whs))
	for _, w := range whs {
		nameByID[w.ID] = w.Name
	}

	byWh := map[int][]int{}
	for _, id := range allOrderIDs {
		wh := orders[id].WarehouseID
		byWh[wh] = append(byWh[wh], id)
	}
	whIDs := make([]int, 0, len(byWh))
	for wh := range byWh {
		whIDs = append(whIDs, wh)
	}
	sort.Ints(whIDs)

	var mainBs, ffBs []warehouseBlock
	for _, wh := range whIDs {
		ids := byWh[wh]
		rows := buildWBRows(ids, orderToSupply, orders, stickerMap, statusMap, msk)
		name := nameByID[wh]
		if name == "" {
			name = strconv.Itoa(wh)
		}
		block := warehouseBlock{name: name, rows: rows}
		if _, isMain := m.mainIDs[wh]; isMain {
			mainBs = append(mainBs, block)
		} else {
			ffBs = append(ffBs, block)
		}
	}

	dayLabel := day.Day()
	var errs []string
	if err := m.upload(fmt.Sprintf("Отгружено на WB FBS-%d", dayLabel), wbHeader, mainBs); err != nil {
		errs = append(errs, fmt.Sprintf("main upload: %v", err))
	}
	if err := m.upload(fmt.Sprintf("Отгружено на WB ФФ FBS-%d", dayLabel), wbHeader, ffBs); err != nil {
		errs = append(errs, fmt.Sprintf("ff upload: %v", err))
	}
	if len(errs) > 0 {
		return fmt.Errorf("cabinet=%d: %s", m.cabinetID, strings.Join(errs, "; "))
	}
	return nil
}

func buildWBRows(orderIDs []int, orderToSupply map[int]*wbSupply, orders map[int]wbOrderFBS, stickerMap, statusMap map[int]string, loc *time.Location) [][]string {
	fmtTime := func(t time.Time) string { return t.In(loc).Format("15:04:05 02.01.2006") }
	rows := make([][]string, 0, len(orderIDs))
	for _, id := range orderIDs {
		s := orderToSupply[id]
		o := orders[id]
		barcode := ""
		if len(o.Skus) > 0 {
			barcode = o.Skus[0]
		}
		currency := ""
		switch o.CurrencyCode {
		case 643:
			currency = "₽"
		case 0:
			currency = ""
		default:
			currency = strconv.Itoa(o.CurrencyCode)
		}
		price := ""
		if o.Price != 0 {
			price = strconv.FormatFloat(o.Price/100.0, 'f', -1, 64)
		}
		createdAt := ""
		if !o.CreatedAt.IsZero() {
			createdAt = fmtTime(o.CreatedAt)
		}
		scanDt := ""
		supplyID := ""
		if s != nil {
			supplyID = s.ID
			if s.ScanDt != nil {
				scanDt = fmtTime(*s.ScanDt)
			}
		}
		warehouse := ""
		if o.WarehouseID != 0 {
			warehouse = strconv.Itoa(o.WarehouseID)
		}
		nmID := ""
		if o.NmID != 0 {
			nmID = strconv.Itoa(o.NmID)
		}
		rows = append(rows, []string{
			strconv.Itoa(id),
			supplyID,
			stickerMap[id],
			createdAt,
			scanDt,
			"", "", "",
			barcode,
			price,
			currency,
			nmID,
			o.Article,
			warehouse,
			statusMap[id],
		})
	}
	return rows
}

func (m *ShipmentsManager) upload(title string, header []string, blocks []warehouseBlock) error {
	if len(blocks) == 0 {
		return nil
	}
	if _, err := m.sheets.EnsureSheet(m.spreadsheetID, title); err != nil {
		return fmt.Errorf("ensure sheet %q: %w", title, err)
	}
	var values [][]interface{}
	totalRows := 0
	for _, b := range blocks {
		values = append(values, toIface([]string{m.cabinetName, b.name}))
		values = append(values, toIface(header))
		for _, r := range b.rows {
			values = append(values, toIface(r))
		}
		totalRows += len(b.rows)
	}
	if err := m.sheets.Append(m.spreadsheetID, title+"!A1", values); err != nil {
		return fmt.Errorf("append %q: %w", title, err)
	}
	log.Printf("wbShipments: cabinet=%d sheet=%q blocks=%d rows=%d", m.cabinetID, title, len(blocks), totalRows)
	return nil
}

func toIface(row []string) []interface{} {
	out := make([]interface{}, len(row))
	for i, v := range row {
		out[i] = v
	}
	return out
}
