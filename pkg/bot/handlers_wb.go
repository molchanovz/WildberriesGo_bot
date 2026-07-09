package bot

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"tradebot/pkg/db"
	"tradebot/pkg/tradeplus"
	"tradebot/pkg/tradeplus/wb"

	botlib "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/xuri/excelize/v2"
)

const (
	CallbackWbHandler        = "WB"
	CallbackWbFbsHandler     = "WB-FBS"
	CallbackWbOrdersHandler  = "WB-ORDERS"
	CallbackWbStocksHandler  = "WB-STOCKS"
	CallbackWbReturnsHandler = "WB-RETURNS"

	CallbackWbAnswerReview     = "WB-ANSWER-REVIEW"
	CallbackWbEditReview       = "WB-EDIT-REVIEW"
	CallbackWbCancelEditReview = "WB-CANCEL-EDIT-REVIEW"
	CallbackWbDeleteReview     = "WB-DELETE-REVIEW"
)

// reviewEdit is stored in ReviewMap while the operator is typing a new answer.
// It keeps the review being edited and the ID of the "Отправь новый ответ"
// prompt message so it can be cleaned up once the new answer arrives.
type reviewEdit struct {
	reviewID    string
	promptMsgID int
}

func wbHandler(ctx context.Context, bot *botlib.Bot, update *models.Update) {
	chatID := update.CallbackQuery.From.ID
	messageID := update.CallbackQuery.Message.Message.ID

	text := "Кабинет ВБ"

	var buttonsRow []models.InlineKeyboardButton
	var allButtons [][]models.InlineKeyboardButton

	buttonsRow = append(buttonsRow, models.InlineKeyboardButton{Text: "Этикетки FBS", CallbackData: fmt.Sprintf("%v", CallbackWbFbsHandler)})
	allButtons = append(allButtons, buttonsRow)
	buttonsRow = []models.InlineKeyboardButton{}

	buttonsRow = append(buttonsRow, models.InlineKeyboardButton{Text: "Анализ заказов", CallbackData: fmt.Sprintf("%v", CallbackWbStocksHandler)})
	allButtons = append(allButtons, buttonsRow)
	buttonsRow = []models.InlineKeyboardButton{}

	buttonsRow = append(buttonsRow, models.InlineKeyboardButton{Text: "Возвраты в ПВЗ", CallbackData: fmt.Sprintf("%v", CallbackWbReturnsHandler)})
	allButtons = append(allButtons, buttonsRow)
	buttonsRow = []models.InlineKeyboardButton{}

	buttonsRow = append(buttonsRow, models.InlineKeyboardButton{Text: "Назад", CallbackData: CallbackStartHandler})
	allButtons = append(allButtons, buttonsRow)

	markup := models.InlineKeyboardMarkup{InlineKeyboard: allButtons}

	_, err := bot.EditMessageText(ctx, &botlib.EditMessageTextParams{ChatID: chatID, MessageID: messageID, Text: text, ReplyMarkup: markup})
	if err != nil {
		log.Printf("%v", err)
		return
	}
}

func (m *Manager) stickersHandler(ctx context.Context, bot *botlib.Bot, update *models.Update) {
	chatID := update.CallbackQuery.From.ID

	user, err := m.tm.UserByChatID(ctx, chatID)
	if err != nil {
		log.Println("Ошибка получения пользователя: ", err)
		return
	}

	_, err = m.tm.SetUserStatus(ctx, user, db.StatusWaitingWbState)
	if err != nil {
		log.Println("Ошибка обновления WaitingWbState пользователя: ", err)
		return
	}

	text := "Отправь мне номер отгрузки"

	var buttonBack []models.InlineKeyboardButton

	buttonBack = append(buttonBack, models.InlineKeyboardButton{Text: "Назад", CallbackData: CallbackStartHandler})

	allButtons := [][]models.InlineKeyboardButton{buttonBack}
	markup := models.InlineKeyboardMarkup{InlineKeyboard: allButtons}

	_, err = bot.EditMessageText(ctx, &botlib.EditMessageTextParams{MessageID: update.CallbackQuery.Message.Message.ID, ChatID: chatID, Text: text, ReplyMarkup: markup})
	if err != nil {
		log.Printf("%v", err)
		return
	}
}

func (m *Manager) getWbStickers(ctx context.Context, bot *botlib.Bot, chatID int64, supplyID string) error {
	done := make(chan []string)
	progressChan := make(chan tradeplus.Progress)
	errChan := make(chan error)

	defer tradeplus.CleanFiles()

	cabinets, err := m.tm.GetCabinetsByMp(ctx, db.MarketWB)
	if err != nil {
		return err
	}

	go func() {
		filePath, err := wb.NewStickerManager(cabinets[0].Key).GetReadyFile(supplyID, progressChan)
		if err != nil {
			log.Println("Ошибка при получении файла:", err)
			errChan <- err
			return
		}
		done <- filePath
	}()

	err = WaitReadyFile(ctx, bot, chatID, progressChan, done, errChan)
	if err != nil {
		return err
	}
	return nil
}

func (m *Manager) wbOrdersHandler(ctx context.Context, bot *botlib.Bot, update *models.Update) {
	chatID := update.CallbackQuery.From.ID

	cabinets, err := m.tm.GetCabinetsByMp(ctx, db.MarketWB)
	if err != nil {
		log.Println(err)
		return
	}

	if cabinets[0].SheetLink == nil {
		log.Println(errors.New("sheet link is null"))
		return
	}

	manager := wb.NewOrdersManager(cabinets[0].Key, *cabinets[0].SheetLink)

	err = manager.Write()
	if err != nil {
		_, err = SendTextMessage(ctx, bot, chatID, err.Error())
		if err != nil {
			log.Println(err)
			return
		}
	}

	date := time.Now().AddDate(0, 0, -tradeplus.OrdersDaysAgo)
	_, err = SendTextMessage(ctx, bot, chatID, fmt.Sprintf("Заказы вб за %v были внесены", date))
	if err != nil {
		log.Println(err)
		return
	}

}
func (m *Manager) wbStocksHandler(ctx context.Context, bot *botlib.Bot, update *models.Update) {
	daysAgo := 14

	chatID := update.CallbackQuery.From.ID

	cabinets, err := m.tm.GetCabinetsByMp(ctx, db.MarketWB)
	if err != nil {
		m.sl.Errorf("%v", err)
		return
	}

	manager := wb.NewStockManager(cabinets[0].Key)

	orders, err := manager.GetOrders(daysAgo)
	if err != nil {
		m.sl.Errorf("%v", err)
		return
	}

	stocks, lostWarehouses, err := manager.GetStocks()
	if err != nil {
		_, err = SendTextMessage(ctx, bot, chatID, fmt.Sprintf("Ошибка при анализе остатков: %v", err))
		if err != nil {
			m.sl.Errorf("send msg failed: %v", err)
			return
		}
		return
	}

	filePath, err := generateExcelWB(orders, stocks, db.MarketWB)
	if err != nil {
		_, err = SendTextMessage(ctx, bot, chatID, fmt.Sprintf("Ошибка при генерации экселя: %v", err))
		if err != nil {
			m.sl.Errorf("send msg failed: %v", err)
			return
		}
		return
	}

	err = SendMediaMessage(ctx, bot, chatID, filePath)
	if err != nil {
		m.sl.Errorf("send media failed: %v", err)
		return
	}
	os.Remove(filePath)

	if len(lostWarehouses) > 0 {
		warehousesStr := strings.Builder{}

		for warehouse := range lostWarehouses {
			warehousesStr.WriteString(warehouse + "\n")
		}
		_, err = SendTextMessage(ctx, bot, chatID, fmt.Sprintf("Нужно добавить: %v\n", warehousesStr.String()))
		if err != nil {
			return
		}
	}
}

func (m *Manager) returnsHandler(ctx context.Context, bot *botlib.Bot, update *models.Update) {
	chatID := update.CallbackQuery.From.ID

	cabinets, err := m.tm.GetCabinetsByMp(ctx, db.MarketWB)
	if err != nil {
		m.sl.Errorf("%v", err)
		return
	}

	filePath, err := wb.NewReturnsManager(cabinets[0].Key).WriteReturns()
	if err != nil {
		_, err = SendTextMessage(ctx, bot, chatID, fmt.Sprintf("Ошибка при анализе остатков: %v", err))
		if err != nil {
			m.sl.Errorf("send msg failed: %v", err)
			return
		}
		return
	}

	defer os.Remove(filePath)

	err = SendMediaMessage(ctx, bot, chatID, filePath)
	if err != nil {
		m.sl.Errorf("send media failed: %v", err)
		return
	}
}

// FetchReviews (cron A) pulls unanswered reviews from WB into the DB as Created.
func (m *Manager) FetchReviews(ctx context.Context) error {
	cabinets, err := m.tm.GetCabinetsByMp(ctx, db.MarketWB)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	manager := wb.NewReviewManager(m.dbc, &cabinets[0], m.chatgpt)
	return manager.Fetch(ctx)
}

// ProcessReviews (cron B) generates answers for Created reviews and routes them:
// positives are auto-posted to WB, problems go to the operator in Telegram.
func (m *Manager) ProcessReviews(ctx context.Context) error {
	cabinets, err := m.tm.GetCabinetsByMp(ctx, db.MarketWB)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	manager := wb.NewReviewManager(m.dbc, &cabinets[0], m.chatgpt)
	return manager.ProcessPending(ctx, m.sendReview)
}

// reviewMarkup builds the review card keyboard (answer / edit / delete).
func reviewMarkup(reviewID string) models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{
			{Text: "Ответить", CallbackData: fmt.Sprintf("%v_%v", CallbackWbAnswerReview, reviewID)},
			{Text: "Редактировать", CallbackData: fmt.Sprintf("%v_%v", CallbackWbEditReview, reviewID)},
		},
		{
			{Text: "Удалить", CallbackData: CallbackWbDeleteReview},
		},
	}}
}

func (m *Manager) sendReview(ctx context.Context, review tradeplus.Review) error {
	_, err := m.b.SendMessage(ctx, &botlib.SendMessageParams{
		ChatID:      int64(m.reviewChatID),
		Text:        review.ToMessage(),
		ReplyMarkup: reviewMarkup(review.ExternalID),
		ParseMode:   models.ParseModeHTML,
	})
	if err != nil {
		return fmt.Errorf("review#%d send failed: %v", review.ID, err)
	}
	return nil
}

func (m *Manager) wbAnswerReview(ctx context.Context, bot *botlib.Bot, update *models.Update) {
	parts := strings.Split(update.CallbackQuery.Data, "_")

	if len(parts) != 2 {
		m.sl.Error(ctx, "wbAnswerReview неверное кол-во parts")
		return
	}

	reviewId := parts[1]

	cabinets, err := m.tm.GetCabinetsByMp(ctx, db.MarketWB)
	if err != nil {
		m.sl.Errorf("%v", err)
		return
	}

	manager := wb.NewReviewManager(m.dbc, &cabinets[0], m.chatgpt)

	err = manager.AnswerReview(ctx, reviewId)
	if err != nil {
		m.sl.Errorf("%v", err)
		_, _ = bot.AnswerCallbackQuery(ctx, &botlib.AnswerCallbackQueryParams{Text: err.Error(), ShowAlert: true, CallbackQueryID: update.CallbackQuery.ID})
		return
	}

	m.wbDeleteReview(ctx, bot, update)
}

func (m *Manager) wbEditReview(ctx context.Context, bot *botlib.Bot, update *models.Update) {
	chatUserID := update.CallbackQuery.From.ID
	chatID := update.CallbackQuery.Message.Message.Chat.ID

	parts := strings.Split(update.CallbackQuery.Data, "_")

	if len(parts) != 2 {
		log.Println("wbAnswerReview неверное кол-во parts")
		return
	}

	reviewId := parts[1]
	promptMsgID := update.CallbackQuery.Message.Message.ID

	m.ReviewMap.Store(chatID, reviewEdit{reviewID: reviewId, promptMsgID: promptMsgID})

	user, err := m.tm.UserByChatID(ctx, chatUserID)
	if err != nil {
		log.Println("Ошибка получения User")
		return
	}

	if user == nil {
		log.Println("Ошибка получения User")
		return
	}

	_, err = m.tm.SetUserStatus(ctx, user, db.StatusWaitingReview)
	if err != nil {
		m.sl.Errorf("Ошибка обновления статуса:%v", err)
		return
	}

	// Show the current answer (copyable) so the operator can grab and tweak it.
	text := "Отправь новый ответ"
	if review, err := m.tm.GetReviewByID(ctx, reviewId); err == nil && review != nil && review.Answer != "" {
		text = "Текущий ответ (можно скопировать и поправить):\n<pre>" + review.Answer + "</pre>\n\nОтправь новый ответ"
	}

	// "Назад" aborts the edit and restores the review card so the current
	// answer can still be posted.
	backMarkup := models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{{Text: "Назад", CallbackData: fmt.Sprintf("%v_%v", CallbackWbCancelEditReview, reviewId)}},
	}}

	_, err = bot.EditMessageText(ctx, &botlib.EditMessageTextParams{
		MessageID:   promptMsgID,
		ChatID:      chatID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: backMarkup,
	})
	if err != nil {
		log.Println("Ошибка отправки сообщения")
		return
	}
}

// wbCancelEditReview aborts an in-progress edit: it clears the waiting-for-answer
// state and restores the original review card (with its answer/edit/delete
// buttons) so the operator can still post the current answer.
func (m *Manager) wbCancelEditReview(ctx context.Context, bot *botlib.Bot, update *models.Update) {
	chatUserID := update.CallbackQuery.From.ID
	chatID := update.CallbackQuery.Message.Message.Chat.ID
	messageID := update.CallbackQuery.Message.Message.ID

	parts := strings.Split(update.CallbackQuery.Data, "_")
	if len(parts) != 2 {
		log.Println("wbCancelEditReview неверное кол-во parts")
		return
	}
	reviewId := parts[1]

	// Drop the waiting state so the operator's next message is not captured as a
	// new answer.
	m.ReviewMap.Delete(chatID)
	if user, err := m.tm.UserByChatID(ctx, chatUserID); err == nil && user != nil {
		if _, err = m.tm.SetUserStatus(ctx, user, db.StatusEnabled); err != nil {
			m.sl.Errorf("Ошибка обновления статуса: %v", err)
		}
	}

	review, err := m.tm.GetReviewByID(ctx, reviewId)
	if err != nil || review == nil {
		m.sl.Errorf("wbCancelEditReview: не удалось получить отзыв %s: %v", reviewId, err)
		return
	}

	_, err = bot.EditMessageText(ctx, &botlib.EditMessageTextParams{
		MessageID:   messageID,
		ChatID:      chatID,
		Text:        review.ToMessage(),
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: reviewMarkup(reviewId),
	})
	if err != nil {
		log.Println("wbCancelEditReview: ошибка восстановления карточки:", err)
		return
	}
}

func (m *Manager) wbDeleteReview(ctx context.Context, bot *botlib.Bot, update *models.Update) {
	_, err := bot.DeleteMessage(ctx,
		&botlib.DeleteMessageParams{
			ChatID:    update.CallbackQuery.Message.Message.Chat.ID,
			MessageID: update.CallbackQuery.Message.Message.ID,
		})
	if err != nil {
		log.Println(err)
		return
	}
}

func (m *Manager) updateReview(ctx context.Context, bot *botlib.Bot, chatID int64, message *models.Message) {
	var (
		review *tradeplus.Review
		err    error
	)

	if v, ok := m.ReviewMap.Load(chatID); ok {
		edit, _ := v.(reviewEdit)

		review, err = m.tm.GetReviewByID(ctx, edit.reviewID)
		if err != nil {
			return
		}
		review.Answer = message.Text
		review, err = m.tm.UpdateReview(ctx, review)
		if err != nil {
			log.Println("Ошибка получения кабинета")
			return
		}

		// Remove the "Отправь новый ответ" prompt now that the answer is in.
		if edit.promptMsgID != 0 {
			_, err = bot.DeleteMessage(ctx, &botlib.DeleteMessageParams{
				ChatID:    chatID,
				MessageID: edit.promptMsgID,
			})
			if err != nil {
				log.Println("Ошибка удаления сообщения 'Отправь новый ответ': ", err)
			}
		}
	}

	defer m.ReviewMap.Delete(chatID)

	_, err = bot.DeleteMessage(ctx, &botlib.DeleteMessageParams{
		ChatID:    chatID,
		MessageID: message.ID,
	})
	if err != nil {
		log.Println("Ошибка удаления сообщения с API: ", err)
		return
	}

	if review == nil {
		m.sl.Error(ctx, "review is null")
		return
	}

	err = m.sendReview(ctx, *review)
	if err != nil {
		return
	}
}

func (m *Manager) AnalyzeStocks(apiKey string, ctx context.Context, b *botlib.Bot) error {
	//stocksFBO, err := wbc.GetStockFbo(apiKey)
	//if err != nil {
	//	return err
	//}
	//
	//if stocksFBO == nil {
	//	return errors.New("newStocks nil")
	//}
	//
	//type customStock struct {
	//	stockFBO int
	//	stockFBS int
	//}
	//
	//stocksMap := make(map[string]customStock)
	//
	//// Заполнение мапы артикулов
	//for i := range stocksFBO {
	//	if stock, hasArticle := stocksMap[stocksFBO[i].SupplierArticle]; hasArticle {
	//		stock.stockFBO += stocksFBO[i].Quantity
	//		stocksMap[stocksFBO[i].SupplierArticle] = stock
	//	} else {
	//		stock := customStock{
	//			stockFBO: stocksFBO[i].Quantity,
	//		}
	//		stocksMap[stocksFBO[i].SupplierArticle] = stock
	//	}
	//}
	//
	//if len(stocksMap) == 0 {
	//	return errors.New("stocksMap nil")
	//}
	//
	//for article, newStocks := range stocksMap {
	//	// Смотрим есть ли артикул в бд
	//	stocks, err := m.repo.GetStocks(article, "wildberries")
	//	if err != nil {
	//		return err
	//	}
	//
	//	// если артикула нет - заполняем бд
	//	if len(stocks) == 0 {
	//		stock := db.Stock{Article: article, CountFbo: &newStocks.stockFBO, UpdatedAt: time.Now(), CabinetID: 0}
	//		err = m.repo.CreateStock(stock)
	//		if err != nil {
	//			return err
	//		}
	//
	//		continue
	//	}
	//
	//	if newStocks.stockFBO == *stocks[0].CountFbo {
	//		continue
	//	}
	//
	//	// Если стало нулем
	//	if newStocks.stockFBO == 0 && *stocks[0].CountFbo != 0 {
	//		// Отправляем уведомление
	//		_, err = b.SendMessage(ctx, &botlib.SendMessageParams{
	//			ChatID:    m.reviewChatID,
	//			Text:      fmt.Sprintf("На складе <b>WB</b> закончились <code>%v</code>. Проверьте FBS", article),
	//			ParseMode: models.ParseModeHTML,
	//		})
	//		if err != nil {
	//			return err
	//		}
	//	}
	//
	//	log.Println("Обновляем ", stocks[0].Article)
	//
	//	err = m.repo.UpdateStock(db.Stock{
	//		Article:   stocks[0].Article,
	//		CountFbo:  &newStocks.stockFBO,
	//		UpdatedAt: time.Now(),
	//	})
	//	if err != nil {
	//		return err
	//	}
	//}

	log.Println("НЕ РАБОТАЕТ")

	return nil
}

func generateExcelWB(postings map[string]map[string]int, stocks map[string]map[string]int, mp string) (string, error) {
	file := excelize.NewFile()
	sheetName := "StocksFBO Analysis"
	err := file.SetSheetName("Sheet1", sheetName)
	if err != nil {
		return "", err
	}

	// Заголовки
	headers := []string{"Кластер", "Артикул", "Заказано", "Остатки"}
	for i, h := range headers {
		cell := string(rune('A'+i)) + "1"
		err = file.SetCellValue(sheetName, cell, h)
		if err != nil {
			return "", err
		}
	}

	articles := make(map[string]struct{})

	// Собираем все уникальные артикулы
	for _, postingsMap := range postings {
		for article := range postingsMap {
			articles[article] = struct{}{}
		}
	}
	for _, stocksMap := range stocks {
		for article := range stocksMap {
			articles[article] = struct{}{}
		}
	}

	row := 2
	for cluster, postingsMap := range postings {
		for article := range articles {
			postingCount := postingsMap[article]
			stock := 0
			if clusterStocks, stocksExists := stocks[cluster]; stocksExists {
				stock = clusterStocks[article]
			}

			err = file.SetCellValue(sheetName, "A"+strconv.Itoa(row), cluster)
			if err != nil {
				return "", err
			}
			err = file.SetCellValue(sheetName, "B"+strconv.Itoa(row), article)
			if err != nil {
				return "", err
			}
			err = file.SetCellValue(sheetName, "C"+strconv.Itoa(row), postingCount)
			if err != nil {
				return "", err
			}
			err = file.SetCellValue(sheetName, "D"+strconv.Itoa(row), stock)
			if err != nil {
				return "", err
			}
			row++
		}
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

	// Сохраняем файл
	filePath := mp + "_stock_analysis.xlsx"
	if err = file.SaveAs(filePath); err != nil {
		return "", err
	}
	return filePath, nil
}
