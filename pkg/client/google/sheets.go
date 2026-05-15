package google

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

type SheetsService struct {
	tokenPath       string
	credentialsPath string
}

func NewSheetsService(tokenPath, credentialsPath string) SheetsService {
	return SheetsService{
		tokenPath:       tokenPath,
		credentialsPath: credentialsPath,
	}
}

// Retrieve a token, saves the token, then returns the generated client.
func (gs SheetsService) getClient(config *oauth2.Config) (*http.Client, error) {
	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	tok, err := gs.tokenFromFile(gs.tokenPath)
	if err != nil {
		tok, err = gs.getTokenFromWeb(config)
		if err != nil {
			return nil, err
		}

		err = gs.saveToken(gs.tokenPath, tok)
		if err != nil {
			return nil, err
		}
	}
	return config.Client(context.Background(), tok), nil
}

// Request a token from the web, then returns the retrieved token.
func (SheetsService) getTokenFromWeb(config *oauth2.Config) (*oauth2.Token, error) {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	log.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		return nil, fmt.Errorf("unable to read authorization code: %w", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		fmt.Errorf("unable to retrieve token from web: %w", err)
	}
	return tok, err
}

// Retrieves a token from a local file.
func (SheetsService) tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

func (SheetsService) saveToken(path string, token *oauth2.Token) error {
	log.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("unable to cache oauth token: %v", err)
	}
	defer f.Close()

	err = json.NewEncoder(f).Encode(token)
	if err != nil {
		return fmt.Errorf("encode token failed: %v", err)
	}

	return nil
}

// func (gs SheetsService) read(spreadsheetId, readRange string) [][]interface{} {
//	ctx := context.Background()
//	b, err := os.ReadFile(gs.credentialsPath)
//	if err != nil {
//		log.Fatalf("Unable to read client secret file: %v", err)
//	}
//
//	// If modifying these scopes, delete your previously saved token.json.
//	config, err := google.ConfigFromJSON(b, "https://www.googleapis.com/auth/spreadsheets.readonly")
//	if err != nil {
//		log.Fatalf("Unable to parse client secret file to config: %v", err)
//	}
//	client := gs.getClient(config)
//
//	srv, err := sheets.NewService(ctx, option.WithHTTPClient(client))
//	if err != nil {
//		log.Fatalf("Unable to retrieve Sheets client: %v", err)
//	}
//
//	// Prints the names and majors of students in a sample spreadsheet:
//
//	// https://docs.google.com/spreadsheets/d/1_vD7wEx4ZaRdYn5pjJNelKAtzH7JA61TO2Q5QlVs0kQ/edit?usp=sharing
//	resp, err := srv.Spreadsheets.Values.Get(spreadsheetId, readRange).Do()
//	if err != nil {
//		log.Fatalf("Unable to retrieve data from sheet: %v", err)
//	}
//
//	if len(resp.Values) == 0 {
//		fmt.Println("No data found.")
//		return nil
//	} else {
//		return resp.Values
//	}
// }

func (gs SheetsService) service(ctx context.Context) (*sheets.Service, error) {
	b, err := os.ReadFile(gs.credentialsPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read client secret file: %w", err)
	}

	config, err := google.ConfigFromJSON(b, "https://www.googleapis.com/auth/spreadsheets")
	if err != nil {
		return nil, fmt.Errorf("unable to parse client secret file to config: %w", err)
	}

	client, err := gs.getClient(config)
	if err != nil {
		return nil, fmt.Errorf("unable to get oauth client: %w", err)
	}

	srv, err := sheets.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve Sheets client: %w", err)
	}
	return srv, nil
}

func (gs SheetsService) Write(spreadsheetID, writeRange string, values [][]interface{}) error {
	ctx := context.Background()
	srv, err := gs.service(ctx)
	if err != nil {
		return err
	}

	body := &sheets.ValueRange{
		Values: values,
	}

	_, err = srv.Spreadsheets.Values.Update(spreadsheetID, writeRange, body).
		ValueInputOption("RAW").Do()
	if err != nil {
		return fmt.Errorf("unable to update data in sheet: %w", err)
	}

	return nil
}

// EnsureSheet creates a sheet with the given title in the spreadsheet if it does not exist.
// Returns true if the sheet was newly created, false if it already existed.
func (gs SheetsService) EnsureSheet(spreadsheetID, title string) (bool, error) {
	ctx := context.Background()
	srv, err := gs.service(ctx)
	if err != nil {
		return false, err
	}

	ss, err := srv.Spreadsheets.Get(spreadsheetID).Fields("sheets.properties.title").Do()
	if err != nil {
		return false, fmt.Errorf("get spreadsheet: %w", err)
	}
	for _, s := range ss.Sheets {
		if s.Properties != nil && s.Properties.Title == title {
			return false, nil
		}
	}

	_, err = srv.Spreadsheets.BatchUpdate(spreadsheetID, &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{{
			AddSheet: &sheets.AddSheetRequest{
				Properties: &sheets.SheetProperties{Title: title},
			},
		}},
	}).Do()
	if err != nil {
		if isSheetAlreadyExists(err) {
			return false, nil
		}
		return false, fmt.Errorf("add sheet %q: %w", title, err)
	}
	return true, nil
}

func isSheetAlreadyExists(err error) bool {
	var ae *googleapi.Error
	if errors.As(err, &ae) {
		msg := strings.ToLower(ae.Message)
		if strings.Contains(msg, "already exists") || strings.Contains(msg, "уже существует") {
			return true
		}
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "already exists") || strings.Contains(msg, "уже существует")
}

// Append adds rows after the last non-empty row in the given range.
func (gs SheetsService) Append(spreadsheetID, writeRange string, values [][]interface{}) error {
	ctx := context.Background()
	srv, err := gs.service(ctx)
	if err != nil {
		return err
	}

	body := &sheets.ValueRange{Values: values}

	_, err = srv.Spreadsheets.Values.Append(spreadsheetID, writeRange, body).
		ValueInputOption("USER_ENTERED").
		InsertDataOption("INSERT_ROWS").
		Do()
	if err != nil {
		return fmt.Errorf("unable to append data to sheet: %w", err)
	}

	return nil
}

// ClearRowsAndAppendColored:
//  1. Reads the keyColumn (0-based) of sheetTitle, building map[key] -> 1-based row index.
//  2. For rows whose key is in clearKeys: clears all cells in the row EXCEPT
//     keyColumn и preserveColumns (например колонки с ручными формулами).
//     BatchClearValues — формат, включая фон, не трогаем; строка не удаляется,
//     нижние строки не сдвигаются.
//  3. From values keeps only rows whose key (at keyColumn) is NOT already present
//     on the sheet — appends those, painting the new block with r/g/b and
//     copying formulas from the row above via PASTE_FORMULA.
//
// Returns (cleared, appended, err).
func (gs SheetsService) ClearRowsAndAppendColored(
	spreadsheetID, sheetTitle string,
	keyColumn int,
	preserveColumns []int,
	skipColorColumns []int,
	values [][]interface{},
	clearKeys []string,
	r, g, b float64,
) (cleared, appended int, err error) {
	ctx := context.Background()
	srv, err := gs.service(ctx)
	if err != nil {
		return 0, 0, err
	}

	ss, err := srv.Spreadsheets.Get(spreadsheetID).Do()
	if err != nil {
		return 0, 0, fmt.Errorf("get spreadsheet: %w", err)
	}
	var sheetID int64 = -1
	var sheetColumns int64 = 26
	for _, s := range ss.Sheets {
		if s.Properties != nil && s.Properties.Title == sheetTitle {
			sheetID = s.Properties.SheetId
			if s.Properties.GridProperties != nil && s.Properties.GridProperties.ColumnCount > 0 {
				sheetColumns = s.Properties.GridProperties.ColumnCount
			}
			break
		}
	}
	if sheetID == -1 {
		return 0, 0, fmt.Errorf("sheet %q not found", sheetTitle)
	}

	keyColA1 := columnIndexToLetter(keyColumn)
	readRange := fmt.Sprintf("%s!%s:%s", sheetTitle, keyColA1, keyColA1)
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).
		MajorDimension("COLUMNS").Do()
	if err != nil {
		return 0, 0, fmt.Errorf("read key column %s: %w", readRange, err)
	}

	existing := map[string]int{} // key -> 1-based row index
	if len(resp.Values) > 0 {
		for i, v := range resp.Values[0] {
			s, _ := v.(string)
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			if _, ok := existing[s]; !ok {
				existing[s] = i + 1
			}
		}
	}

	clearSet := make(map[string]struct{}, len(clearKeys))
	for _, k := range clearKeys {
		clearSet[strings.TrimSpace(k)] = struct{}{}
	}
	var rowsToClear []int
	for k, idx := range existing {
		if _, ok := clearSet[k]; ok {
			rowsToClear = append(rowsToClear, idx)
		}
	}
	sort.Ints(rowsToClear)

	if len(rowsToClear) > 0 {
		// Чистим всё, кроме keyColumn и preserveColumns: для каждой строки
		// собираем непрерывные сегменты колонок между «защищёнными» индексами.
		// Значения стираются, формат (включая фон) остаётся.
		protected := map[int]struct{}{keyColumn: {}}
		for _, c := range preserveColumns {
			protected[c] = struct{}{}
		}
		segments := rowSegmentsExcluding(protected, int(sheetColumns))
		ranges := make([]string, 0, len(rowsToClear)*len(segments))
		for _, row := range rowsToClear {
			for _, seg := range segments {
				ranges = append(ranges, fmt.Sprintf("%s!%s%d:%s%d",
					sheetTitle,
					columnIndexToLetter(seg[0]), row,
					columnIndexToLetter(seg[1]), row))
			}
		}
		if len(ranges) > 0 {
			_, err = srv.Spreadsheets.Values.BatchClear(spreadsheetID, &sheets.BatchClearValuesRequest{
				Ranges: ranges,
			}).Do()
			if err != nil {
				return 0, 0, fmt.Errorf("clear rows: %w", err)
			}
		}
		cleared = len(rowsToClear)
	}

	toAppend := make([][]interface{}, 0, len(values))
	for _, row := range values {
		if keyColumn >= len(row) {
			continue
		}
		key, _ := row[keyColumn].(string)
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, ok := existing[key]; ok {
			continue
		}
		if _, ok := clearSet[key]; ok {
			continue
		}
		toAppend = append(toAppend, row)
	}

	if len(toAppend) == 0 {
		log.Printf("ClearRowsAndAppendColored: sheet=%q cleared=%d appended=0", sheetTitle, cleared)
		return cleared, 0, nil
	}

	body := &sheets.ValueRange{Values: toAppend}
	appendResp, err := srv.Spreadsheets.Values.Append(spreadsheetID, sheetTitle+"!A1", body).
		ValueInputOption("USER_ENTERED").
		InsertDataOption("INSERT_ROWS").
		Do()
	if err != nil {
		return cleared, 0, fmt.Errorf("append data: %w", err)
	}
	appended = len(toAppend)

	if appendResp.Updates == nil || appendResp.Updates.UpdatedRange == "" {
		log.Printf("ClearRowsAndAppendColored: sheet=%q cleared=%d appended=%d (no color)", sheetTitle, cleared, appended)
		return cleared, appended, nil
	}
	startRow, endRow, ok := parseUpdatedRange(appendResp.Updates.UpdatedRange)
	if !ok {
		return cleared, appended, nil
	}

	// Покраска: разбиваем диапазон колонок на сегменты, пропуская skipColorColumns.
	skipColorSet := make(map[int]struct{}, len(skipColorColumns))
	for _, c := range skipColorColumns {
		skipColorSet[c] = struct{}{}
	}
	colorSegments := rowSegmentsExcluding(skipColorSet, int(sheetColumns))
	postRequests := make([]*sheets.Request, 0, len(colorSegments)+1)
	for _, seg := range colorSegments {
		postRequests = append(postRequests, &sheets.Request{
			RepeatCell: &sheets.RepeatCellRequest{
				Range: &sheets.GridRange{
					SheetId:          sheetID,
					StartRowIndex:    int64(startRow - 1),
					EndRowIndex:      int64(endRow),
					StartColumnIndex: int64(seg[0]),
					EndColumnIndex:   int64(seg[1] + 1),
					ForceSendFields:  []string{"SheetId", "StartRowIndex", "StartColumnIndex"},
				},
				Cell: &sheets.CellData{
					UserEnteredFormat: &sheets.CellFormat{
						BackgroundColor: &sheets.Color{Red: r, Green: g, Blue: b},
					},
				},
				Fields: "userEnteredFormat.backgroundColor",
			},
		})
	}
	// Для каждой колонки с ручной формулой (preserveColumns) копируем формулу
	// из строки выше новых строк на весь свежедобавленный диапазон —
	// точечно по одной колонке, чтобы не задеть значения в остальных.
	if startRow > 1 {
		for _, col := range preserveColumns {
			if col < 0 || int64(col) >= sheetColumns {
				continue
			}
			postRequests = append(postRequests, &sheets.Request{
				CopyPaste: &sheets.CopyPasteRequest{
					Source: &sheets.GridRange{
						SheetId:          sheetID,
						StartRowIndex:    int64(startRow - 2),
						EndRowIndex:      int64(startRow - 1),
						StartColumnIndex: int64(col),
						EndColumnIndex:   int64(col + 1),
						ForceSendFields:  []string{"SheetId", "StartRowIndex", "StartColumnIndex"},
					},
					Destination: &sheets.GridRange{
						SheetId:          sheetID,
						StartRowIndex:    int64(startRow - 1),
						EndRowIndex:      int64(endRow),
						StartColumnIndex: int64(col),
						EndColumnIndex:   int64(col + 1),
						ForceSendFields:  []string{"SheetId", "StartColumnIndex"},
					},
					PasteType:        "PASTE_NORMAL",
					PasteOrientation: "NORMAL",
				},
			})
		}
	}
	_, err = srv.Spreadsheets.BatchUpdate(spreadsheetID, &sheets.BatchUpdateSpreadsheetRequest{
		Requests: postRequests,
	}).Do()
	if err != nil {
		return cleared, appended, fmt.Errorf("post-append batch: %w", err)
	}
	log.Printf("ClearRowsAndAppendColored: sheet=%q cleared=%d appended=%d range=%q color=(%.2f,%.2f,%.2f)",
		sheetTitle, cleared, appended, appendResp.Updates.UpdatedRange, r, g, b)
	return cleared, appended, nil
}

// rowSegmentsExcluding возвращает непрерывные сегменты колонок [start,end]
// (0-based, включительно), которые не входят в protected, в пределах [0,totalCols).
func rowSegmentsExcluding(protected map[int]struct{}, totalCols int) [][2]int {
	var segs [][2]int
	start := -1
	for c := 0; c < totalCols; c++ {
		if _, p := protected[c]; p {
			if start >= 0 {
				segs = append(segs, [2]int{start, c - 1})
				start = -1
			}
			continue
		}
		if start < 0 {
			start = c
		}
	}
	if start >= 0 {
		segs = append(segs, [2]int{start, totalCols - 1})
	}
	return segs
}

// columnIndexToLetter converts a 0-based column index to A1 letters (0 -> "A", 25 -> "Z", 26 -> "AA").
func columnIndexToLetter(idx int) string {
	if idx < 0 {
		idx = 0
	}
	var out []byte
	idx++
	for idx > 0 {
		idx--
		out = append([]byte{byte('A' + idx%26)}, out...)
		idx /= 26
	}
	return string(out)
}

// AppendColored appends rows to the sheet identified by title and paints the
// just-inserted block with the given background color. r/g/b in [0..1].
func (gs SheetsService) AppendColored(spreadsheetID, sheetTitle string, values [][]interface{}, r, g, b float64) error {
	ctx := context.Background()
	srv, err := gs.service(ctx)
	if err != nil {
		return err
	}

	ss, err := srv.Spreadsheets.Get(spreadsheetID).Do()
	if err != nil {
		return fmt.Errorf("get spreadsheet: %w", err)
	}
	var sheetID int64 = -1
	for _, s := range ss.Sheets {
		if s.Properties != nil && s.Properties.Title == sheetTitle {
			sheetID = s.Properties.SheetId
			break
		}
	}
	if sheetID == -1 {
		return fmt.Errorf("sheet %q not found", sheetTitle)
	}

	body := &sheets.ValueRange{Values: values}
	resp, err := srv.Spreadsheets.Values.Append(spreadsheetID, sheetTitle+"!A1", body).
		ValueInputOption("USER_ENTERED").
		InsertDataOption("INSERT_ROWS").
		Do()
	if err != nil {
		return fmt.Errorf("append data: %w", err)
	}

	if resp.Updates == nil || resp.Updates.UpdatedRange == "" {
		return nil
	}
	startRow, endRow, ok := parseUpdatedRange(resp.Updates.UpdatedRange)
	if !ok {
		return nil
	}

	_, err = srv.Spreadsheets.BatchUpdate(spreadsheetID, &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{{
			RepeatCell: &sheets.RepeatCellRequest{
				Range: &sheets.GridRange{
					SheetId:         sheetID,
					StartRowIndex:   int64(startRow - 1),
					EndRowIndex:     int64(endRow),
					ForceSendFields: []string{"SheetId", "StartRowIndex"},
				},
				Cell: &sheets.CellData{
					UserEnteredFormat: &sheets.CellFormat{
						BackgroundColor: &sheets.Color{Red: r, Green: g, Blue: b},
					},
				},
				Fields: "userEnteredFormat.backgroundColor",
			},
		}},
	}).Do()
	if err != nil {
		return fmt.Errorf("color rows: %w", err)
	}
	log.Printf("AppendColored: sheet=%q range=%q rows=%d-%d color=(%.2f,%.2f,%.2f)", sheetTitle, resp.Updates.UpdatedRange, startRow, endRow, r, g, b)
	return nil
}

func parseUpdatedRange(r string) (start, end int, ok bool) {
	parts := strings.SplitN(r, "!", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	cells := strings.SplitN(parts[1], ":", 2)
	if len(cells) == 0 {
		return 0, 0, false
	}
	s := extractRowNum(cells[0])
	if s == 0 {
		return 0, 0, false
	}
	if len(cells) == 1 {
		return s, s, true
	}
	e := extractRowNum(cells[1])
	if e == 0 {
		return 0, 0, false
	}
	return s, e, true
}

func extractRowNum(cell string) int {
	var num strings.Builder
	for _, c := range cell {
		if c >= '0' && c <= '9' {
			num.WriteRune(c)
		}
	}
	if num.Len() == 0 {
		return 0
	}
	n, err := strconv.Atoi(num.String())
	if err != nil {
		return 0
	}
	return n
}
