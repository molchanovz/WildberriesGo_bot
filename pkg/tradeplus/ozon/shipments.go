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
	"strings"
	"sync"
	"time"

	"tradebot/pkg/client/google"
	"tradebot/pkg/client/ozon"
	"tradebot/pkg/tradeplus"
)

const (
	shipmentsReportLookback = 30 * 24 * time.Hour
	shipmentsReportTail     = 2 * 24 * time.Hour
	shipmentsReportTimeout  = 10 * time.Minute
	shipmentsParallelism    = 4
	shipmentsDateColumn     = "Фактическая дата передачи в доставку"
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

// parseShipmentCSV парсит скачанный CSV отчёта и возвращает заголовок и строки,
// у которых «Фактическая дата передачи в доставку» ∈ [from, to).
func parseShipmentCSV(raw []byte, from, to time.Time, loc *time.Location) (header []string, rows [][]string, err error) {
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

	header = all[0]
	shipCol := -1
	for i, h := range header {
		name := strings.TrimPrefix(strings.TrimSpace(h), "\ufeff")
		if name == shipmentsDateColumn {
			shipCol = i
			break
		}
	}
	if shipCol == -1 {
		return nil, nil, fmt.Errorf("колонка %q не найдена; заголовки=%v", shipmentsDateColumn, header)
	}

	for _, row := range all[1:] {
		if shipCol >= len(row) {
			continue
		}
		v := strings.TrimSpace(row[shipCol])
		if v == "" {
			continue
		}
		t, ok := parseOzonDate(v, loc)
		if !ok {
			continue
		}
		if !t.Before(from) && t.Before(to) {
			rows = append(rows, row)
		}
	}
	return header, rows, nil
}

// ShipmentsManager заливает FBS-отгрузки кабинета за выбранную дату в Google Sheets,
// разделяя строки по складам: main → один лист, остальные → ФФ-лист.
type ShipmentsManager struct {
	api           ozonReportAPI
	ozonClient    ozon.Client
	sheets        google.SheetsService
	spreadsheetID string
	mainIDs       map[int]struct{}
	cabinetID     int
	cabinetName   string
}

func NewShipmentsManager(cabinet tradeplus.Cabinet) (*ShipmentsManager, error) {
	if cabinet.ClientID == nil || *cabinet.ClientID == "" {
		return nil, fmt.Errorf("cabinet %d: empty Ozon Client-Id", cabinet.ID)
	}
	if cabinet.Settings.ShipmentsSheetID == "" {
		return nil, fmt.Errorf("cabinet %d: empty shipmentsSheetId", cabinet.ID)
	}

	mains := make(map[int]struct{}, len(cabinet.Settings.MainWarehouseIDs))
	for _, id := range cabinet.Settings.MainWarehouseIDs {
		mains[id] = struct{}{}
	}

	return &ShipmentsManager{
		api:           newOzonReportAPI(*cabinet.ClientID, cabinet.Key),
		ozonClient:    ozon.NewClient(*cabinet.ClientID, cabinet.Key),
		sheets:        google.NewSheetsService("pkg/client/google/token.json", "pkg/client/google/credentials.json"),
		spreadsheetID: cabinet.Settings.ShipmentsSheetID,
		mainIDs:       mains,
		cabinetID:     cabinet.ID,
		cabinetName:   cabinet.Name,
	}, nil
}

// WriteForDate качает по отчёту на каждый склад кабинета за окно [day, day+1) MSK
// и пишет результат в листы main / ФФ.
func (m *ShipmentsManager) WriteForDate(ctx context.Context, day time.Time) error {
	msk := day.Location()
	shipFrom := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, msk)
	shipTo := shipFrom.AddDate(0, 0, 1)
	reportFromUTC := shipFrom.Add(-shipmentsReportLookback).UTC()
	reportToUTC := shipTo.Add(shipmentsReportTail).UTC()

	wl, err := m.ozonClient.Warehouses("")
	if err != nil {
		return fmt.Errorf("warehouses: %w", err)
	}
	if len(wl.Warehouses) == 0 {
		log.Printf("ozonShipments: cabinet=%d no warehouses, skip", m.cabinetID)
		return nil
	}

	type result struct {
		warehouseID   int
		warehouseName string
		header        []string
		rows          [][]string
		err           error
	}
	results := make([]result, len(wl.Warehouses))

	sem := make(chan struct{}, shipmentsParallelism)
	var wg sync.WaitGroup
	for i, w := range wl.Warehouses {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, whID int, whName string) {
			defer wg.Done()
			defer func() { <-sem }()

			res := result{warehouseID: whID, warehouseName: whName}
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
			header, rows, err := parseShipmentCSV(data, shipFrom, shipTo, msk)
			if err != nil {
				res.err = fmt.Errorf("warehouse %d %q: parse csv: %w", whID, whName, err)
				results[i] = res
				return
			}
			res.header = header
			res.rows = rows
			log.Printf("ozonShipments: cabinet=%d warehouse=%d %q rows=%d", m.cabinetID, whID, whName, len(rows))
			results[i] = res
		}(i, w.WarehouseID, w.Name)
	}
	wg.Wait()

	var (
		header []string
		mainBs []warehouseBlock
		ffBs   []warehouseBlock
		errs   []string
	)
	for _, r := range results {
		if r.err != nil {
			errs = append(errs, r.err.Error())
			continue
		}
		if header == nil && r.header != nil {
			header = r.header
		}
		if len(r.rows) == 0 {
			continue
		}
		block := warehouseBlock{name: r.warehouseName, rows: r.rows}
		if _, isMain := m.mainIDs[r.warehouseID]; isMain {
			mainBs = append(mainBs, block)
		} else {
			ffBs = append(ffBs, block)
		}
	}

	dayLabel := shipFrom.Day()
	if header != nil {
		if err := m.upload(fmt.Sprintf("Отправлено на Ozon FBS-%d", dayLabel), header, mainBs); err != nil {
			errs = append(errs, fmt.Sprintf("main upload: %v", err))
		}
		if err := m.upload(fmt.Sprintf("Отправлено на Ozon ФФ FBS-%d", dayLabel), header, ffBs); err != nil {
			errs = append(errs, fmt.Sprintf("ff upload: %v", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("cabinet=%d: %s", m.cabinetID, strings.Join(errs, "; "))
	}
	return nil
}

type warehouseBlock struct {
	name string
	rows [][]string
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
	log.Printf("ozonShipments: cabinet=%d sheet=%q blocks=%d rows=%d", m.cabinetID, title, len(blocks), totalRows)
	return nil
}

func toIface(row []string) []interface{} {
	out := make([]interface{}, len(row))
	for i, v := range row {
		out[i] = v
	}
	return out
}
