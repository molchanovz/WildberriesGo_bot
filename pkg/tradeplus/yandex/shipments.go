package yandex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"tradebot/pkg/client/google"
	"tradebot/pkg/tradeplus"
)

const (
	ymBaseURL       = "https://api.partner.market.yandex.ru"
	ymHTTPTimeout   = 30 * time.Second
	ymOrdersLimit   = 50
	ymStatsOrderCap = 200 // Yandex stats/orders: orders[] array hard cap
)

// ymCancelBeforeShipment — stats-status'ы, которые означают, что заказ отменили
// ДО физической отгрузки. Такие дропаем из отчёта.
var ymCancelBeforeShipment = map[string]struct{}{
	"CANCELLED_BEFORE_PROCESSING": {},
	"CANCELLED_IN_PROCESSING":     {},
}

var ymShipmentsHeader = []string{
	"Номер заказа", "Ваш номер заказа", "Дата оформления", "Ваш SKU",
	"Название товара", "Количество", "Ваша цена (за шт.)", "Статус заказа",
	"Статус изменён", "Способ оплаты", "Склад отгрузки", "Дата отгрузки",
	"Грузоместа", "Регион доставки",
}

var ymStatusRu = map[string]string{
	"PLACING":             "Оформление",
	"RESERVED":            "Резерв",
	"UNPAID":              "Ожидает оплаты",
	"PROCESSING":          "В обработке",
	"DELIVERY":            "Доставляется",
	"PICKUP":              "Доставлен в пункт выдачи",
	"DELIVERED":           "Доставлен",
	"CANCELLED":           "Отменён",
	"PENDING":             "Ожидает",
	"PARTIALLY_DELIVERED": "Частично доставлен",
	"PARTIALLY_RETURNED":  "Частично возвращён",
	"RETURNED":            "Возвращён",
	"LOST":                "Утерян",
	"UNKNOWN":             "Неизвестно",
}

var ymPaymentRu = map[string]string{
	"PREPAID":          "предоплата",
	"POSTPAID":         "оплата при получении",
	"CASH_ON_DELIVERY": "наличные при получении",
	"CARD_ON_DELIVERY": "картой при получении",
}

type ymOrder struct {
	OrderID         int64    `json:"orderId"`
	CampaignID      int64    `json:"campaignId"`
	ProgramType     string   `json:"programType"`
	CreationDate    string   `json:"creationDate"`
	UpdateDate      string   `json:"updateDate"`
	Status          string   `json:"status"`
	PaymentType     string   `json:"paymentType"`
	ExternalOrderID string   `json:"externalOrderId"`
	Items           []ymItem `json:"items"`
	Delivery        struct {
		WarehouseID int64 `json:"warehouseId"`
		Shipment    struct {
			ID           int64  `json:"id"`
			ShipmentDate string `json:"shipmentDate"`
		} `json:"shipment"`
		Courier struct {
			Region ymRegion `json:"region"`
		} `json:"courier"`
		Pickup struct {
			Region ymRegion `json:"region"`
		} `json:"pickup"`
		BoxesLayout []struct {
			BoxIndex int `json:"boxIndex"`
		} `json:"boxesLayout"`
	} `json:"delivery"`
}

type ymRegion struct {
	Name   string `json:"name"`
	Parent struct {
		Name string `json:"name"`
	} `json:"parent"`
}

type ymItem struct {
	OfferID   string `json:"offerId"`
	OfferName string `json:"offerName"`
	Count     int    `json:"count"`
	Prices    struct {
		Payment struct {
			Value float64 `json:"value"`
		} `json:"payment"`
	} `json:"prices"`
}

type ymOrdersReq struct {
	Dates        ymOrdersReqDates `json:"dates"`
	ProgramTypes []string         `json:"programTypes,omitempty"`
}

type ymOrdersReqDates struct {
	ShipmentDateFrom string `json:"shipmentDateFrom"`
	ShipmentDateTo   string `json:"shipmentDateTo"`
}

type ymOrdersResp struct {
	Paging struct {
		NextPageToken string `json:"nextPageToken"`
	} `json:"paging"`
	Orders []ymOrder `json:"orders"`
}

type ymShipmentResp struct {
	Result struct {
		Warehouse struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"warehouse"`
	} `json:"result"`
}

type ymCampaignResp struct {
	Campaign struct {
		Business struct {
			ID int64 `json:"id"`
		} `json:"business"`
	} `json:"campaign"`
}

type ymStatsOrdersReq struct {
	Orders []int64 `json:"orders"`
}

type ymStatsOrdersResp struct {
	Result struct {
		Orders []struct {
			ID     int64  `json:"id"`
			Status string `json:"status"`
		} `json:"orders"`
		Paging struct {
			NextPageToken string `json:"nextPageToken"`
		} `json:"paging"`
	} `json:"result"`
}

type ymShipmentsAPI struct {
	token string
	hc    *http.Client
}

func newYMShipmentsAPI(token string) ymShipmentsAPI {
	return ymShipmentsAPI{token: token, hc: &http.Client{Timeout: ymHTTPTimeout}}
}

func (a ymShipmentsAPI) do(ctx context.Context, method, path string, params url.Values, body any) ([]byte, error) {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rdr = bytes.NewReader(b)
	}
	u := ymBaseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rdr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Api-Key", a.token)
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
		return data, fmt.Errorf("yandex %s %s: %d %s", method, path, resp.StatusCode, string(data))
	}
	return data, nil
}

func (a ymShipmentsAPI) businessID(ctx context.Context, campaignID string) (int64, error) {
	data, err := a.do(ctx, http.MethodGet, "/campaigns/"+campaignID, nil, nil)
	if err != nil {
		return 0, err
	}
	var r ymCampaignResp
	if err := json.Unmarshal(data, &r); err != nil {
		return 0, fmt.Errorf("decode campaign %s: %w; body=%s", campaignID, err, string(data))
	}
	if r.Campaign.Business.ID == 0 {
		return 0, fmt.Errorf("campaign %s: empty business.id", campaignID)
	}
	return r.Campaign.Business.ID, nil
}

func (a ymShipmentsAPI) listOrdersByShipmentDate(ctx context.Context, businessID int64, day time.Time) ([]ymOrder, error) {
	dayStr := day.Format("2006-01-02")
	body := ymOrdersReq{
		Dates:        ymOrdersReqDates{ShipmentDateFrom: dayStr, ShipmentDateTo: dayStr},
		ProgramTypes: []string{"FBS"},
	}
	var out []ymOrder
	var pageToken string
	for {
		params := url.Values{}
		params.Set("limit", strconv.Itoa(ymOrdersLimit))
		if pageToken != "" {
			params.Set("pageToken", pageToken)
		}
		data, err := a.do(ctx, http.MethodPost, fmt.Sprintf("/v1/businesses/%d/orders", businessID), params, body)
		if err != nil {
			return nil, err
		}
		var r ymOrdersResp
		if err := json.Unmarshal(data, &r); err != nil {
			return nil, fmt.Errorf("decode orders: %w; body=%s", err, string(data))
		}
		for _, o := range r.Orders {
			if o.ProgramType != "" && o.ProgramType != "FBS" {
				continue
			}
			out = append(out, o)
		}
		if r.Paging.NextPageToken == "" || r.Paging.NextPageToken == pageToken {
			break
		}
		pageToken = r.Paging.NextPageToken
	}
	return out, nil
}

// cancelPhases запрашивает детализованные статусы отмены у /v2/campaigns/{cid}/stats/orders.
// Входной набор — (campaignId, orderId) ТОЛЬКО отменённых заказов. Возвращает orderId → status
// (например CANCELLED_BEFORE_PROCESSING, CANCELLED_IN_PROCESSING, CANCELLED_IN_DELIVERY).
func (a ymShipmentsAPI) cancelPhases(ctx context.Context, byCampaign map[int64][]int64) (map[int64]string, error) {
	out := map[int64]string{}
	for campaignID, ids := range byCampaign {
		for start := 0; start < len(ids); start += ymStatsOrderCap {
			end := start + ymStatsOrderCap
			if end > len(ids) {
				end = len(ids)
			}
			body := ymStatsOrdersReq{Orders: ids[start:end]}
			var pageToken string
			for {
				params := url.Values{}
				params.Set("limit", strconv.Itoa(ymStatsOrderCap))
				if pageToken != "" {
					params.Set("pageToken", pageToken)
				}
				data, err := a.do(ctx, http.MethodPost, fmt.Sprintf("/v2/campaigns/%d/stats/orders", campaignID), params, body)
				if err != nil {
					return nil, err
				}
				var r ymStatsOrdersResp
				if err := json.Unmarshal(data, &r); err != nil {
					return nil, fmt.Errorf("decode stats campaign=%d: %w; body=%s", campaignID, err, string(data))
				}
				for _, o := range r.Result.Orders {
					out[o.ID] = o.Status
				}
				if r.Result.Paging.NextPageToken == "" || r.Result.Paging.NextPageToken == pageToken {
					break
				}
				pageToken = r.Result.Paging.NextPageToken
			}
		}
	}
	return out, nil
}

func (a ymShipmentsAPI) shipmentWarehouses(ctx context.Context, orders []ymOrder) (map[int64]string, error) {
	type key struct{ campaign, shipment int64 }
	seen := map[key]struct{}{}
	out := map[int64]string{}
	for _, o := range orders {
		shipID := o.Delivery.Shipment.ID
		if shipID == 0 || o.CampaignID == 0 {
			continue
		}
		k := key{campaign: o.CampaignID, shipment: shipID}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		path := fmt.Sprintf("/v2/campaigns/%d/first-mile/shipments/%d", o.CampaignID, shipID)
		data, err := a.do(ctx, http.MethodGet, path, nil, nil)
		if err != nil {
			return nil, err
		}
		var r ymShipmentResp
		if err := json.Unmarshal(data, &r); err != nil {
			return nil, fmt.Errorf("decode shipment %d: %w; body=%s", shipID, err, string(data))
		}
		if r.Result.Warehouse.Name != "" {
			out[shipID] = r.Result.Warehouse.Name
		}
	}
	return out, nil
}

// ShipmentsManager заливает FBS-отгрузки Yandex кабинета за выбранную дату
// в один лист Google Sheets (без разделения main/ФФ).
type ShipmentsManager struct {
	api           ymShipmentsAPI
	sheets        google.SheetsService
	spreadsheetID string
	campaignID    string
	businessID    int64
	cabinetID     int
	cabinetName   string
}

func NewShipmentsManager(ctx context.Context, cabinet tradeplus.Cabinet) (*ShipmentsManager, error) {
	if cabinet.Key == "" {
		return nil, fmt.Errorf("cabinet %d: empty Yandex Api-Key", cabinet.ID)
	}
	if cabinet.ClientID == nil || *cabinet.ClientID == "" {
		return nil, fmt.Errorf("cabinet %d: empty campaignId (ClientID)", cabinet.ID)
	}
	if cabinet.Settings.ShipmentsSheetID == "" {
		return nil, fmt.Errorf("cabinet %d: empty shipmentsSheetId", cabinet.ID)
	}
	api := newYMShipmentsAPI(cabinet.Key)
	businessID, err := api.businessID(ctx, *cabinet.ClientID)
	if err != nil {
		return nil, fmt.Errorf("cabinet %d: resolve businessId: %w", cabinet.ID, err)
	}
	return &ShipmentsManager{
		api:           api,
		sheets:        google.NewSheetsService("pkg/client/google/token.json", "pkg/client/google/credentials.json"),
		spreadsheetID: cabinet.Settings.ShipmentsSheetID,
		campaignID:    *cabinet.ClientID,
		businessID:    businessID,
		cabinetID:     cabinet.ID,
		cabinetName:   cabinet.Name,
	}, nil
}

func (m *ShipmentsManager) WriteForDate(ctx context.Context, day time.Time) error {
	orders, err := m.api.listOrdersByShipmentDate(ctx, m.businessID, day)
	if err != nil {
		return fmt.Errorf("list orders: %w", err)
	}
	if len(orders) == 0 {
		log.Printf("ymShipments: cabinet=%d no orders, skip", m.cabinetID)
		return nil
	}

	// Для CANCELLED заказов дёргаем /v2/campaigns/{cid}/stats/orders — только там есть
	// различение CANCELLED_BEFORE_PROCESSING / CANCELLED_IN_PROCESSING / CANCELLED_IN_DELIVERY.
	// Первые две фазы = не отгружали, исключаем из отчёта.
	cancelledByCampaign := map[int64][]int64{}
	for _, o := range orders {
		if o.Status == "CANCELLED" && o.CampaignID != 0 {
			cancelledByCampaign[o.CampaignID] = append(cancelledByCampaign[o.CampaignID], o.OrderID)
		}
	}
	var phases map[int64]string
	if len(cancelledByCampaign) > 0 {
		phases, err = m.api.cancelPhases(ctx, cancelledByCampaign)
		if err != nil {
			log.Printf("ymShipments: cabinet=%d cancel phases fetch failed: %v (keeping all cancellations)", m.cabinetID, err)
			phases = map[int64]string{}
		}
	}

	filtered := make([]ymOrder, 0, len(orders))
	dropped := 0
	for _, o := range orders {
		if o.Status == "CANCELLED" {
			if _, pre := ymCancelBeforeShipment[phases[o.OrderID]]; pre {
				dropped++
				continue
			}
		}
		filtered = append(filtered, o)
	}
	if dropped > 0 {
		log.Printf("ymShipments: cabinet=%d dropped %d pre-shipment cancellations", m.cabinetID, dropped)
	}

	whNames, err := m.api.shipmentWarehouses(ctx, filtered)
	if err != nil {
		log.Printf("ymShipments: cabinet=%d warehouses fetch failed: %v (leaving column empty)", m.cabinetID, err)
		whNames = map[int64]string{}
	}

	rows := buildYMShipmentRows(filtered, whNames, day)
	title := fmt.Sprintf("Отправлено на Яндекс FBS-%d", day.Day())
	if _, err := m.sheets.EnsureSheet(m.spreadsheetID, title); err != nil {
		return fmt.Errorf("ensure sheet %q: %w", title, err)
	}

	values := make([][]interface{}, 0, len(rows)+2)
	values = append(values, toIfaceYM([]string{m.cabinetName}))
	values = append(values, toIfaceYM(ymShipmentsHeader))
	for _, r := range rows {
		values = append(values, toIfaceYM(r))
	}
	if err := m.sheets.Append(m.spreadsheetID, title+"!A1", values); err != nil {
		return fmt.Errorf("append %q: %w", title, err)
	}
	log.Printf("ymShipments: cabinet=%d sheet=%q rows=%d", m.cabinetID, title, len(rows))
	return nil
}

func buildYMShipmentRows(orders []ymOrder, whNames map[int64]string, day time.Time) [][]string {
	var rows [][]string
	for _, o := range orders {
		created := ymFmtDay(ymParseDate(o.CreationDate))
		updated := ymFmtDay(ymParseDate(o.UpdateDate))
		status := ymTranslate(ymStatusRu, o.Status)
		payment := ymTranslate(ymPaymentRu, o.PaymentType)
		extID := o.ExternalOrderID
		if extID == "" {
			extID = strconv.FormatInt(o.OrderID, 10)
		}
		cargo := 1
		if n := len(o.Delivery.BoxesLayout); n > 0 {
			cargo = n
		}
		reg := ymRegionOf(o)
		orderID := strconv.FormatInt(o.OrderID, 10)
		wh := ""
		if name, ok := whNames[o.Delivery.Shipment.ID]; ok {
			wh = name
		}
		shipDate := day.Format("02.01.2006")
		if s := o.Delivery.Shipment.ShipmentDate; s != "" {
			if t := ymParseDate(s); !t.IsZero() {
				shipDate = ymFmtDay(t)
			}
		}

		for _, it := range o.Items {
			price := ""
			if it.Count > 0 && it.Prices.Payment.Value > 0 {
				price = strconv.FormatFloat(it.Prices.Payment.Value/float64(it.Count), 'f', -1, 64)
			}
			rows = append(rows, []string{
				orderID,
				extID,
				created,
				it.OfferID,
				it.OfferName,
				strconv.Itoa(it.Count),
				price,
				status,
				updated,
				payment,
				wh,
				shipDate,
				strconv.Itoa(cargo),
				reg,
			})
		}
	}
	return rows
}

func ymTranslate(m map[string]string, key string) string {
	if v, ok := m[key]; ok {
		return v
	}
	return key
}

func ymParseDate(s string) time.Time {
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func ymFmtDay(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("02.01.2006")
}

func ymRegionOf(o ymOrder) string {
	if o.Delivery.Courier.Region.Name != "" {
		return o.Delivery.Courier.Region.Name
	}
	if o.Delivery.Pickup.Region.Name != "" {
		return o.Delivery.Pickup.Region.Name
	}
	if o.Delivery.Courier.Region.Parent.Name != "" {
		return o.Delivery.Courier.Region.Parent.Name
	}
	return o.Delivery.Pickup.Region.Parent.Name
}

func toIfaceYM(row []string) []interface{} {
	out := make([]interface{}, len(row))
	for i, v := range row {
		out[i] = v
	}
	return out
}
