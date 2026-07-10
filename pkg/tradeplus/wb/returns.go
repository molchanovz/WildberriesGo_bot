package wb

import (
	"fmt"
	"time"
	"unicode/utf8"

	"tradebot/pkg/client/wb"
	"tradebot/pkg/tradeplus"

	"github.com/xuri/excelize/v2"
)

const StatusReadyForPickup = "Готов к выдаче"

type ReturnsManager struct {
	client wb.Client
}

func NewReturnsManager(token string) *ReturnsManager {
	return &ReturnsManager{client: wb.NewClient(token)}
}

type customCardData struct {
	volume float64
	weight float64
	count  int
}

func (m ReturnsManager) WriteReturns() (string, error) {
	dateFrom := time.Now().AddDate(0, 0, -30)
	dateTo := time.Now()
	dateFrom.Format("2006-01-02")
	dateTo.Format("2006-01-02")
	r, err := m.client.GetReturns(dateFrom.String(), dateTo.String())
	if err != nil {
		fmt.Println(err)
		return "", err
	}

	returns := tradeplus.NewReturns(r)

	if len(returns) == 0 {
		return "", nil
	}

	limit := 100
	allCards := make(tradeplus.Cards, 0, limit)

	var nmID *int
	var updatedAt *time.Time

	for {
		c, err := m.client.GetCards(nmID, updatedAt, tradeplus.Pointer(limit))
		if err != nil {
			return "", err
		}

		nmID = &c.Cursor.NmID
		updatedAt = &c.Cursor.UpdatedAt

		newCards := tradeplus.NewCardList(c)
		allCards = append(allCards, newCards...)

		if len(c.Cards) < limit {
			break
		}
	}

	cardsByNmID := allCards.IndexByNmID()

	result := make(map[string]map[string]customCardData)

	for _, item := range returns {
		if item.Status != StatusReadyForPickup {
			continue
		}
		if result[item.DstOfficeAddress] == nil {
			result[item.DstOfficeAddress] = make(map[string]customCardData)
		}

		if v, ok := cardsByNmID[item.NmId]; ok {
			data := result[item.DstOfficeAddress][v.VendorCode]

			data.count++
			data.volume += float64(v.Dimensions.Width*v.Dimensions.Length*v.Dimensions.Height) / 1000
			data.weight += v.Dimensions.WeightBrutto

			result[item.DstOfficeAddress][v.VendorCode] = data
		}
	}

	path, err := generateExcel(result)
	if err != nil {
		fmt.Println(err)
		return "", err
	}

	return path, nil
}

func generateExcel(returns map[string]map[string]customCardData) (string, error) {
	file := excelize.NewFile()
	sheetName := "Returns"
	err := file.SetSheetName("Sheet1", sheetName)
	if err != nil {
		return "", err
	}

	headers := []string{"Адрес ПВЗ", "Артикул", "Кол-во", "Общий объем, л", "Общий вес, кг"}
	for i, h := range headers {
		cell := string(rune('A'+i)) + "1"
		err = file.SetCellValue(sheetName, cell, h)
		if err != nil {
			return "", err
		}
	}

	style, err := file.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
	})
	if err != nil {
		return "", err
	}

	row := 2
	for address, articlesMap := range returns {
		startRow := row
		for article, data := range articlesMap {
			// Заполняем колонки B–E
			file.SetCellValue(sheetName, fmt.Sprintf("B%d", row), article)
			file.SetCellValue(sheetName, fmt.Sprintf("C%d", row), data.count)
			file.SetCellValue(sheetName, fmt.Sprintf("D%d", row), data.volume)
			file.SetCellValue(sheetName, fmt.Sprintf("E%d", row), data.weight)
			row++
		}
		endRow := row - 1
		if endRow > startRow {
			file.MergeCell(sheetName, fmt.Sprintf("A%d", startRow), fmt.Sprintf("A%d", endRow))
		}
		file.SetCellValue(sheetName, fmt.Sprintf("A%d", startRow), address)
		file.SetCellStyle(sheetName, fmt.Sprintf("A%d", startRow), fmt.Sprintf("A%d", endRow), style)
	}

	opt := []excelize.AutoFilterOptions{{
		Column:     "",
		Expression: "",
	}}

	rangeRef := fmt.Sprintf("A1:A%v", row)

	err = file.AutoFilter(sheetName, rangeRef, opt)
	if err != nil {
		return "", err
	}

	err = autoSizeColumns(file, sheetName)
	if err != nil {
		return "", err
	}

	filePath := "returns.xlsx"
	if err = file.SaveAs(filePath); err != nil {
		return "", err
	}
	return filePath, nil
}

func autoSizeColumns(f *excelize.File, sheetName string) error {
	cols, err := f.GetCols(sheetName)
	if err != nil {
		return err
	}
	for idx, col := range cols {
		largestWidth := 0
		for _, rowCell := range col {
			cellWidth := utf8.RuneCountInString(rowCell) + 2 // + 2 for margin
			if cellWidth > largestWidth {
				largestWidth = cellWidth
			}
		}
		name, err := excelize.ColumnNumberToName(idx + 1)
		if err != nil {
			return err
		}
		f.SetColWidth(sheetName, name, name, float64(largestWidth))
	}
	return nil
}
