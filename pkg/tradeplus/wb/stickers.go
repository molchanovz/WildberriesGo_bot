package wb

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"time"

	"tradebot/pkg/client/google"
	"tradebot/pkg/client/wb"
	"tradebot/pkg/tradeplus"

	"github.com/fogleman/gg"
	"github.com/jung-kurt/gofpdf"
)

type StickerManager struct {
	client       wb.Client
	googleSheets google.SheetsService
}

func NewStickerManager(token string) StickerManager {
	return StickerManager{
		client:       wb.NewClient(token),
		googleSheets: google.NewSheetsService("token.json", "credentials.json"),
	}
}

func (m StickerManager) GetReadyFile(supplyID string, progressChan chan tradeplus.Progress) ([]string, error) {
	tradeplus.CreateDirectories()

	orders, err := m.orders(supplyID)
	if err != nil {
		return nil, err
	}

	totalOrders := len(orders)
	var resultFiles []string
	var ordersSlice []string
	batchCount := 0

	for i, order := range orders {
		stickers, err := m.client.GetStickersFbs(order.ID)
		if err != nil {
			fmt.Println("Ошибка GetStickersFbs")
			return nil, fmt.Errorf("ошибка получения стикеров: %w", err)
		}

		if len(stickers.Stickers) == 0 {
			continue
		}

		err = decodeToPDF(stickers.Stickers[0].File, stickers.Stickers[0].OrderID, order)
		if err != nil {
			return nil, err
		}
		ordersSlice = append(ordersSlice, tradeplus.ReadyPath+strconv.Itoa(order.ID)+".pdf")

		// Батчи по 300 заказов
		if (i+1)%300 == 0 || i == totalOrders-1 {
			batchCount++

			if len(ordersSlice) == 0 {
				continue // Пропускаем пустые батчи
			}

			// Создаем PDF для текущего батча
			batchFilePath := fmt.Sprintf("%s%s_%d.pdf", tradeplus.DirectoryPath, supplyID, batchCount)
			err = mergePDFsInDirectory(ordersSlice, batchFilePath)
			if err != nil {
				return nil, fmt.Errorf("ошибка объединения PDF для батча %d: %w", batchCount, err)
			}

			if !fileExists(batchFilePath) {
				return nil, fmt.Errorf("итоговый PDF для батча %d не создан", batchCount)
			}

			resultFiles = append(resultFiles, batchFilePath)
			ordersSlice = []string{} // Сбрасываем для следующего батча
		}

		if i%5 == 0 {
			progressChan <- tradeplus.Progress{Current: i, Total: totalOrders}
		}

		time.Sleep(200 * time.Millisecond)
	}

	if len(resultFiles) == 0 {
		return nil, errors.New("не было создано ни одного PDF файла")
	}

	return resultFiles, nil
}

func (m StickerManager) orders(supplyID string) ([]wb.OrderFBS, error) {
	var filteredOrders []wb.OrderFBS

	ordersIDs, err := m.client.GetOrderIDsFbs(supplyID)
	if err != nil {
		return nil, err
	}

	dateFrom := int(wb.GetUnix(time.Now().AddDate(0, 0, -7)))
	dateTo := int(wb.GetUnix(time.Now()))

	allOrders, err := m.client.GetOrdersFBS(dateFrom, dateTo)
	if err != nil {
		return nil, err
	}

	// Создаем map для быстрого поиска заказов по ID
	orderMap := make(map[int]wb.OrderFBS, len(allOrders.OrdersFBS))
	for i := range allOrders.OrdersFBS {
		orderMap[allOrders.OrdersFBS[i].ID] = allOrders.OrdersFBS[i]
	}

	// Отбираем только нужные заказы
	for _, id := range ordersIDs.IDs {
		if order, exists := orderMap[id]; exists {
			filteredOrders = append(filteredOrders, order)
		}
	}
	sortOrdersByArticle(filteredOrders)

	return filteredOrders, nil
}

func sortOrdersByArticle(orders []wb.OrderFBS) {
	sort.SliceStable(orders, func(i, j int) bool {
		return orders[i].Article < orders[j].Article
	})
}

func decodeToPNG(base64String string, orderID int) (string, error) {
	base64Data := base64String

	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "", fmt.Errorf("ошибка при декодировании base64:%w", err)
	}

	filePath := tradeplus.CodesPath + strconv.Itoa(orderID) + ".png" // Замените на желаемое имя файла и расширение

	file, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("ошибка при создании файла:%w", err)
	}
	defer file.Close()

	_, err = file.Write(data)
	if err != nil {
		return "", fmt.Errorf("ошибка при записи в файл:%w", err)
	}

	return filePath, nil
}

func decodeToPDF(base64String string, orderID int, order wb.OrderFBS) error {
	pageWidthMM := 75.0
	pageHeightMM := 120.0
	// Создание нового PDF-документа
	pdf := gofpdf.New("P", "mm", "", "")
	// Добавление страницы с заданными размерами
	pdf.AddPageFormat("P", gofpdf.SizeType{Wd: pageWidthMM, Ht: pageHeightMM})
	imgPath1, err := decodeToPNG(base64String, orderID)
	if err != nil {
		return err
	}
	// Добавление первого изображения в PDF (без изменения размера изображения)
	pdf.ImageOptions(imgPath1, (75-58)/2, 13, 58, 40, false, gofpdf.ImageOptions{ImageType: "PNG"}, 0, "")

	var skuImageURL string

	skuImageURL = tradeplus.BarcodesPath + order.Article + ".png"

	if !fileExists(skuImageURL) {
		skuImageURL = ""
	}

	if skuImageURL == "" {
		// Путь к пустому баркоду с артикулом
		skuImageURL = tradeplus.GeneratedPath + order.Article + "_generated.png"
		err = createBarcodeWithSKU(order.Article, skuImageURL, 40)
		if err != nil {
			log.Printf("Ошибка при создании изображения с артикулом: %v", err)
			skuImageURL = tradeplus.BarcodesPath + "0.png" // Резервный пустой баркод
		}
	}

	pdf.ImageOptions(skuImageURL, (75-58)/2, 67, 58, 40, false, gofpdf.ImageOptions{ImageType: "PNG"}, 0, "")
	// Сохранение PDF-документа
	err = pdf.OutputFileAndClose(tradeplus.ReadyPath + strconv.Itoa(orderID) + ".pdf")
	if err != nil {
		return err
	}
	return nil
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir() // Проверяем, что это файл, а не директория
}

func mergePDFsInDirectory(orderSlice []string, outputFile string) error {
	// Проверяем, есть ли PDF-файлы для объединения
	if len(orderSlice) == 0 {
		return errors.New("нет PDF-файлов в директории")
	}

	// Формируем команду для выполнения merge PDF через pdfcpu
	args := append([]string{"merge", outputFile}, orderSlice...)
	cmd := exec.Command("pdfcpu", args...)

	// Запуск команды
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ошибка выполнения pdfcpu: %w, %s", err, string(output))
	}
	return nil
}

// Функция для создания изображения с текстом (артикул товара) и сохранения в PNG
func createBarcodeWithSKU(sku string, outputPath string, fontSize float64) error {
	const imgWidth = 580
	const imgHeight = 400

	// Создание нового изображения
	dc := gg.NewContext(imgWidth, imgHeight)

	// Установка фона (белый цвет)
	dc.SetRGB(1, 1, 1)
	dc.Clear()

	// Загрузка шрифта и установка его размера
	if err := dc.LoadFontFace(tradeplus.FontPath, fontSize); err != nil {
		return err
	}

	// Настройка текста
	dc.SetRGB(0, 0, 0) // Цвет текста (черный)
	text := sku

	// Добавление текста на изображение
	dc.DrawStringAnchored(text, float64(imgWidth)/2, float64(imgHeight)/2, 0.5, 0.5)

	// Сохранение изображения в формате PNG
	return dc.SavePNG(outputPath)
}
