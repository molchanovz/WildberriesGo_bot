package ozon

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"tradebot/pkg/client/google"
	"tradebot/pkg/client/ozon"
	"tradebot/pkg/tradeplus"
)

const (
	shipmentsReportLookback      = 30 * 24 * time.Hour
	shipmentsReportTail          = 2 * 24 * time.Hour
	shipmentsReportTimeout       = 10 * time.Minute
	shipmentsParallelism         = 4
	shipmentsDateColumn          = "Фактическая дата передачи в доставку"
	shipmentsInProcessColumn     = "Принят в обработку"
	shipmentsStatusColumn        = "Статус"
	shipmentsWarehouseColumn     = "ID склада"
	shipmentsPostingNumberColumn = "Номер отправления"
	// aggregatedLookbackDays: на сколько суток назад от day перечитываем CSV-отчёт
	// для агрегированного листа. Нужно, чтобы заказ, отменённый через 1–7 дней
	// после создания, был удалён с листа.
	aggregatedLookbackDays = 7
)

// ozonReportAPI — лёгкая обёртка вокруг Ozon report endpoints.
// Отдельная от pkg/client/ozon.Client т.к. нам важно видеть полное тело
// при ошибке и не привязываться к существующим хелперам.
type ozonReportAPI struct {
	clientID string
	apiKey   string
	hc       *http.Client
}

func newOzonReportAPI(clientID, apiKey string) ozonReportAPI {
	return ozonReportAPI{
		clientID: clientID,
		apiKey:   apiKey,
		hc:       &http.Client{Timeout: 5 * time.Minute},
	}
}

func (a ozonReportAPI) post(path string, body, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, "https://api-seller.ozon.ru"+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Client-Id", a.clientID)
	req.Header.Set("Api-Key", a.apiKey)

	resp, err := a.hc.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ozon %s %d: %s", path, resp.StatusCode, string(data))
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode %s: %w; body=%s", path, err, string(data))
	}
	return nil
}

func (a ozonReportAPI) createPostingsReport(sinceUTC, toUTC time.Time, warehouseID int) (string, error) {
	body := map[string]any{
		"filter": map[string]any{
			"processed_at_from": sinceUTC.Format("2006-01-02T15:04:05.000Z"),
			"processed_at_to":   toUTC.Format("2006-01-02T15:04:05.000Z"),
			"delivery_schema":   []string{"fbs"},
			"warehouse_id":      []int{warehouseID},
		},
		"language": "DEFAULT",
	}
	var r struct {
		Result struct {
			Code string `json:"code"`
		} `json:"result"`
	}
	if err := a.post("/v1/report/postings/create", body, &r); err != nil {
		return "", err
	}
	return r.Result.Code, nil
}

type reportInfo struct {
	Code   string `json:"code"`
	Status string `json:"status"`
	Error  string `json:"error"`
	File   string `json:"file"`
}

func (a ozonReportAPI) reportInfo(code string) (reportInfo, error) {
	var r struct {
		Result reportInfo `json:"result"`
	}
	err := a.post("/v1/report/info", map[string]any{"code": code}, &r)
	return r.Result, err
}

func (a ozonReportAPI) waitReport(ctx context.Context, code string, timeout time.Duration) (reportInfo, error) {
	deadline := time.Now().Add(timeout)
	delay := 2 * time.Second
	for {
		info, err := a.reportInfo(code)
		if err != nil {
			return info, err
		}
		switch info.Status {
		case "success":
			return info, nil
		case "failed":
			return info, fmt.Errorf("report %s failed: %s", code, info.Error)
		}
		if time.Now().After(deadline) {
			return info, fmt.Errorf("report %s: timeout (status=%s)", code, info.Status)
		}
		select {
		case <-ctx.Done():
			return info, ctx.Err()
		case <-time.After(delay):
		}
		if delay < 10*time.Second {
			delay += time.Second
		}
	}
}

func (a ozonReportAPI) downloadReport(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Client-Id", a.clientID)
	req.Header.Set("Api-Key", a.apiKey)
	resp, err := a.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %d: %s", resp.StatusCode, string(data))
	}
	return data, nil
}

// parseOzonDate понимает оба формата из отчёта: ISO ("2026-04-14 19:01:23")
// и UI-экспортный ("14.04.2026 19:01").
func parseOzonDate(s string, loc *time.Location) (time.Time, bool) {
	for _, layout := range []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
		"02.01.2006 15:04:05",
		"02.01.2006 15:04",
		"02.01.2006",
	} {
		if t, err := time.ParseInLocation(layout, s, loc); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// parseShipmentCSV парсит скачанный CSV отчёта и возвращает заголовок и все
// строки данных. Фильтрация по дате выполняется отдельно через
// filterRowsByDateColumn.
func parseShipmentCSV(raw []byte) (header []string, rows [][]string, err error) {
	body := bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})

	r := csv.NewReader(bytes.NewReader(body))
	r.Comma = ';'
	r.LazyQuotes = true
	r.FieldsPerRecord = -1

	all, err := r.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("parse csv: %w", err)
	}
	if len(all) == 0 {
		return nil, nil, nil
	}
	return all[0], all[1:], nil
}

// findColumn возвращает индекс колонки по имени (учитывая BOM/пробелы) или -1.
func findColumn(header []string, name string) int {
	for i, h := range header {
		n := strings.TrimPrefix(strings.TrimSpace(h), "\ufeff")
		if n == name {
			return i
		}
	}
	return -1
}

// splitCancelledNotShipped делит строки CSV-отчёта на две группы:
//   - kept: всё, что не «отменён + не отгружен»;
//   - cancelledKeys: номера отправлений (значения колонки «Номер отправления»)
//     строк, у которых статус содержит «отмен», а колонка фактической передачи
//     в доставку пустая.
//
// Если ключевые колонки в header не найдены, всё уходит в kept.
func splitCancelledNotShipped(header []string, rows [][]string) (kept [][]string, cancelledKeys []string) {
	statusCol := findColumn(header, shipmentsStatusColumn)
	shipCol := findColumn(header, shipmentsDateColumn)
	postingCol := findColumn(header, shipmentsPostingNumberColumn)
	if statusCol == -1 || shipCol == -1 {
		return rows, nil
	}
	kept = make([][]string, 0, len(rows))
	for _, row := range rows {
		status := ""
		if statusCol < len(row) {
			status = strings.ToLower(strings.TrimSpace(row[statusCol]))
		}
		ship := ""
		if shipCol < len(row) {
			ship = strings.TrimSpace(row[shipCol])
		}
		if ship == "" && strings.Contains(status, "отмен") {
			if postingCol >= 0 && postingCol < len(row) {
				if k := strings.TrimSpace(row[postingCol]); k != "" {
					cancelledKeys = append(cancelledKeys, k)
				}
			}
			continue
		}
		kept = append(kept, row)
	}
	return kept, cancelledKeys
}

// filterRowsByDateColumn оставляет строки, у которых значение в колонке colName
// парсится как время и лежит в [from, to).
func filterRowsByDateColumn(header []string, rows [][]string, colName string, from, to time.Time, loc *time.Location) ([][]string, error) {
	col := findColumn(header, colName)
	if col == -1 {
		return nil, fmt.Errorf("колонка %q не найдена; заголовки=%v", colName, header)
	}
	var out [][]string
	for _, row := range rows {
		if col >= len(row) {
			continue
		}
		v := strings.TrimSpace(row[col])
		if v == "" {
			continue
		}
		t, ok := parseOzonDate(v, loc)
		if !ok {
			continue
		}
		if !t.Before(from) && t.Before(to) {
			out = append(out, row)
		}
	}
	return out, nil
}

// ShipmentsManager заливает FBS-отгрузки кабинета за выбранную дату в один
// лист Google Sheets. ID склада пишется в отдельный столбец в каждой строке.
type ShipmentsManager struct {
	api              ozonReportAPI
	ozonClient       ozon.Client
	sheets           google.SheetsService
	spreadsheetID    string
	allSpreadsheetID string
	excludedWHs      map[int]struct{}
	cabinetID        int
	cabinetName      string
}

func NewShipmentsManager(cabinet tradeplus.Cabinet) (*ShipmentsManager, error) {
	if cabinet.ClientID == nil || *cabinet.ClientID == "" {
		return nil, fmt.Errorf("cabinet %d: empty Ozon Client-Id", cabinet.ID)
	}
	if cabinet.Settings.ShipmentsSheetID == "" {
		return nil, fmt.Errorf("cabinet %d: empty shipmentsSheetId", cabinet.ID)
	}

	excluded := map[int]struct{}{}
	for _, s := range cabinet.Settings.ExcludedShipmentsWarehouseIDs {
		if id, err := strconv.Atoi(s); err == nil {
			excluded[id] = struct{}{}
		}
	}
	return &ShipmentsManager{
		api:              newOzonReportAPI(*cabinet.ClientID, cabinet.Key),
		ozonClient:       ozon.NewClient(*cabinet.ClientID, cabinet.Key),
		sheets:           google.NewSheetsService("pkg/client/google/token.json", "pkg/client/google/credentials.json"),
		spreadsheetID:    cabinet.Settings.ShipmentsSheetID,
		allSpreadsheetID: cabinet.Settings.ShipmentsAllSheetID,
		excludedWHs:      excluded,
		cabinetID:        cabinet.ID,
		cabinetName:      cabinet.Name,
	}, nil
}

// warehouseCSV — сырой результат скачивания отчёта по одному складу.
type warehouseCSV struct {
	warehouseID   int
	warehouseName string
	header        []string
	rows          [][]string
	err           error
}

// fetchWarehouseCSVs параллельно генерирует, ждёт и скачивает отчёт постингов
// по каждому складу кабинета за указанное окно processed_at_from/to (UTC).
// Возвращает по одному элементу на склад; ошибки конкретных складов лежат в .err.
func (m *ShipmentsManager) fetchWarehouseCSVs(ctx context.Context, reportFromUTC, reportToUTC time.Time) ([]warehouseCSV, error) {
	wl, err := m.ozonClient.Warehouses("")
	if err != nil {
		return nil, fmt.Errorf("warehouses: %w", err)
	}
	if len(wl.Warehouses) == 0 {
		return nil, nil
	}

	results := make([]warehouseCSV, len(wl.Warehouses))
	sem := make(chan struct{}, shipmentsParallelism)
	var wg sync.WaitGroup
	for i, w := range wl.Warehouses {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, whID int, whName string) {
			defer wg.Done()
			defer func() { <-sem }()

			res := warehouseCSV{warehouseID: whID, warehouseName: whName}
			code, err := m.api.createPostingsReport(reportFromUTC, reportToUTC, whID)
			if err != nil {
				res.err = fmt.Errorf("warehouse %d %q: create report: %w", whID, whName, err)
				results[i] = res
				return
			}
			info, err := m.api.waitReport(ctx, code, shipmentsReportTimeout)
			if err != nil {
				res.err = fmt.Errorf("warehouse %d %q: wait report: %w", whID, whName, err)
				results[i] = res
				return
			}
			data, err := m.api.downloadReport(ctx, info.File)
			if err != nil {
				res.err = fmt.Errorf("warehouse %d %q: download report: %w", whID, whName, err)
				results[i] = res
				return
			}
			header, rows, err := parseShipmentCSV(data)
			if err != nil {
				res.err = fmt.Errorf("warehouse %d %q: parse csv: %w", whID, whName, err)
				results[i] = res
				return
			}
			res.header = header
			res.rows = rows
			results[i] = res
		}(i, w.WarehouseID, w.Name)
	}
	wg.Wait()
	return results, nil
}

// WriteForDate качает по отчёту на каждый склад кабинета за окно [day, day+1) MSK
// и пишет все строки в один лист, добавляя колонку с ID склада.
func (m *ShipmentsManager) WriteForDate(ctx context.Context, day time.Time) error {
	msk := day.Location()
	shipFrom := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, msk)
	shipTo := shipFrom.AddDate(0, 0, 1)
	reportFromUTC := shipFrom.Add(-shipmentsReportLookback).UTC()
	reportToUTC := shipTo.Add(shipmentsReportTail).UTC()

	results, err := m.fetchWarehouseCSVs(ctx, reportFromUTC, reportToUTC)
	if err != nil {
		return err
	}
	if len(results) == 0 {
		log.Printf("ozonShipments: cabinet=%d no warehouses, skip", m.cabinetID)
		return nil
	}

	var (
		header  []string
		allRows [][]string
		errs    []string
	)
	for _, r := range results {
		if r.err != nil {
			errs = append(errs, r.err.Error())
			continue
		}
		shipmentRows, err := filterRowsByDateColumn(r.header, r.rows, shipmentsDateColumn, shipFrom, shipTo, msk)
		if err != nil {
			errs = append(errs, fmt.Sprintf("warehouse %d %q: filter shipment: %v", r.warehouseID, r.warehouseName, err))
			continue
		}
		log.Printf("ozonShipments: cabinet=%d warehouse=%d %q shipped=%d", m.cabinetID, r.warehouseID, r.warehouseName, len(shipmentRows))
		if header == nil && r.header != nil {
			header = r.header
		}
		whID := "'" + strconv.Itoa(r.warehouseID)
		for _, row := range shipmentRows {
			allRows = append(allRows, append([]string{whID}, row...))
		}
	}

	if header != nil {
		title := fmt.Sprintf("Отправлено на Ozon FBS-%d", shipFrom.Day())
		fullHeader := append([]string{shipmentsWarehouseColumn}, header...)
		if err := m.upload(title, fullHeader, allRows); err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("cabinet=%d: %s", m.cabinetID, strings.Join(errs, "; "))
	}
	return nil
}

// WriteAggregatedForDate синхронизирует общий лист «Все заказы Ozon»:
//   - новые отправления за вчерашние сутки [day, day+1) MSK дописываются в конец;
//   - отменённые-неотгруженные за окно [day-6д, day+1) MSK удаляются с листа.
//
// day обычно = yesterday 00:00 MSK. Склад из excludedWHs пропускается.
func (m *ShipmentsManager) WriteAggregatedForDate(ctx context.Context, day time.Time) error {
	if m.allSpreadsheetID == "" {
		return nil
	}
	msk := day.Location()
	newFrom := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, msk)
	// newTo = момент запуска (cron или нажатие кнопки), чтобы захватить
	// заказы, появившиеся уже сегодня. Идемпотентность держит дедуп
	// по «Номеру отправления» в ClearRowsAndAppendColored.
	newTo := time.Now().In(msk)
	if newTo.Before(newFrom.AddDate(0, 0, 1)) {
		newTo = newFrom.AddDate(0, 0, 1)
	}
	lookbackFrom := newFrom.AddDate(0, 0, -(aggregatedLookbackDays - 1))
	reportFromUTC := lookbackFrom.Add(-shipmentsReportLookback).UTC()
	reportToUTC := newTo.Add(shipmentsReportTail).UTC()

	results, err := m.fetchWarehouseCSVs(ctx, reportFromUTC, reportToUTC)
	if err != nil {
		return err
	}
	if len(results) == 0 {
		log.Printf("ozonShipmentsAll: cabinet=%d no warehouses, skip", m.cabinetID)
		return nil
	}

	var (
		aggregatedRows [][]string
		clearKeys      []string
		errs           []string
	)
	for _, r := range results {
		if r.err != nil {
			errs = append(errs, r.err.Error())
			continue
		}
		if _, skip := m.excludedWHs[r.warehouseID]; skip {
			continue
		}
		// За весь lookback-период: отменённые-неотгруженные → clearKeys,
		// остальные → kept (кандидаты на append). Дубли отсекает
		// DeleteRowsAndAppendColored по «Номеру отправления». Это восстанавливает
		// пропуски, если cron не сработал в прошлые сутки.
		lookbackRows, err := filterRowsByDateColumn(r.header, r.rows, shipmentsInProcessColumn, lookbackFrom, newTo, msk)
		if err != nil {
			errs = append(errs, fmt.Sprintf("warehouse %d %q: filter lookback: %v", r.warehouseID, r.warehouseName, err))
			continue
		}
		kept, cancelledKeys := splitCancelledNotShipped(r.header, lookbackRows)
		clearKeys = append(clearKeys, cancelledKeys...)

		log.Printf("ozonShipmentsAll: cabinet=%d warehouse=%d %q lookback=%d candidates=%d cancelled_unshipped=%d",
			m.cabinetID, r.warehouseID, r.warehouseName, len(lookbackRows), len(kept), len(cancelledKeys))
		whID := "'" + strconv.Itoa(r.warehouseID)
		for _, row := range kept {
			aggregatedRows = append(aggregatedRows, append([]string{whID}, row...))
		}
	}

	// keyColumn в строке листа = 1 (после "ID склада") + позиция «Номер отправления» в CSV header.
	keyCol := -1
	for _, r := range results {
		if r.err == nil && r.header != nil {
			if idx := findColumn(r.header, shipmentsPostingNumberColumn); idx >= 0 {
				keyCol = 1 + idx
				break
			}
		}
	}
	if keyCol < 0 {
		errs = append(errs, fmt.Sprintf("колонка %q не найдена в CSV", shipmentsPostingNumberColumn))
	} else if err := m.uploadAggregated(aggregatedRows, clearKeys, keyCol, newFrom); err != nil {
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return fmt.Errorf("cabinet=%d: %s", m.cabinetID, strings.Join(errs, "; "))
	}
	return nil
}

func (m *ShipmentsManager) upload(title string, header []string, rows [][]string) error {
	if len(rows) == 0 {
		return nil
	}
	if _, err := m.sheets.EnsureSheet(m.spreadsheetID, title); err != nil {
		return fmt.Errorf("ensure sheet %q: %w", title, err)
	}

	values := make([][]interface{}, 0, len(rows)+2)
	values = append(values, toIface([]string{m.cabinetName}))
	values = append(values, toIface(header))
	for _, r := range rows {
		values = append(values, toIface(r))
	}
	if err := m.sheets.Append(m.spreadsheetID, title+"!A1", values); err != nil {
		return fmt.Errorf("append %q: %w", title, err)
	}
	log.Printf("ozonShipments: cabinet=%d sheet=%q rows=%d", m.cabinetID, title, len(rows))
	return nil
}

func (m *ShipmentsManager) uploadAggregated(rows [][]string, clearKeys []string, keyColumn int, day time.Time) error {
	if m.allSpreadsheetID == "" {
		return nil
	}
	if len(rows) == 0 && len(clearKeys) == 0 {
		return nil
	}
	const title = "Все заказы Ozon"
	if _, err := m.sheets.EnsureSheet(m.allSpreadsheetID, title); err != nil {
		return fmt.Errorf("ensure aggregated %q: %w", title, err)
	}
	values := make([][]interface{}, 0, len(rows))
	for _, r := range rows {
		values = append(values, toIface(r))
	}
	r, g, b := aggregatedDayColor(day)
	// preserveColumns = [30] (AE) — ручная формула, не стираем при отмене.
	// skipColorColumns = [0,3,16,21,30] (A,D,Q,V,AE) — фон не красим.
	cleared, appended, err := m.sheets.ClearRowsAndAppendColored(m.allSpreadsheetID, title, keyColumn, []int{30}, []int{0, 3, 16, 21, 30}, values, clearKeys, r, g, b)
	if err != nil {
		return fmt.Errorf("sync aggregated %q: %w", title, err)
	}
	log.Printf("ozonShipments: cabinet=%d aggregated cleared=%d appended=%d", m.cabinetID, cleared, appended)
	return nil
}

func aggregatedDayColor(day time.Time) (r, g, b float64) {
	if day.Day()%2 == 0 {
		return 0.80, 0.80, 0.80
	}
	return 0.93, 0.93, 0.93
}

func toIface(row []string) []interface{} {
	out := make([]interface{}, len(row))
	for i, v := range row {
		out[i] = v
	}
	return out
}
