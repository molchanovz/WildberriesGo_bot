package bot

import (
	"context"
	"fmt"
	"github.com/vmkteam/embedlog"
	"log"
	"math"
	"os"
	"strconv"
	"sync"
	"tradebot/pkg/client/chatgptsrv"
	"tradebot/pkg/tradeplus/ozon"

	"tradebot/pkg/db"
	"tradebot/pkg/tradeplus"

	botlib "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/xuri/excelize/v2"
)

const (
	CallbackStartHandler                 = "start"
	MessageStartHandler                  = "/start"
	CallbackSettingsSelectCabinetHandler = "SETTINGS-CABINET_"
)

type Manager struct {
	dbc          db.DB
	sl           embedlog.Logger
	b            *botlib.Bot
	tm           *tradeplus.Manager
	chatgpt      *chatgptsrv.Client
	reviewChatID int
	SheetMap     *sync.Map
	APIMap       *sync.Map
	ReviewMap    *sync.Map
}

func NewManager(dbc db.DB, cfg Config, chatgpt *chatgptsrv.Client, logger embedlog.Logger) *Manager {
	return &Manager{
		dbc:          dbc,
		tm:           tradeplus.NewManager(dbc),
		chatgpt:      chatgpt,
		reviewChatID: cfg.ReviewChatID,
		SheetMap:     new(sync.Map),
		APIMap:       new(sync.Map),
		ReviewMap:    new(sync.Map),
		sl:           logger,
	}
}

func (m *Manager) SetBot(bot *botlib.Bot) {
	m.b = bot
}
func (m *Manager) GetBot() *botlib.Bot {
	return m.b
}

func (m *Manager) RegisterBotHandlers() {
	m.b.RegisterHandler(botlib.HandlerTypeMessageText, MessageStartHandler, botlib.MatchTypePrefix, m.startHandler)
	m.b.RegisterHandler(botlib.HandlerTypeCallbackQueryData, CallbackStartHandler, botlib.MatchTypePrefix, m.startHandler)
	m.b.RegisterHandler(botlib.HandlerTypeMessageText, MessageSettingsHandler, botlib.MatchTypePrefix, m.settingsHandler)
	m.b.RegisterHandler(botlib.HandlerTypeCallbackQueryData, MessageSettingsHandler, botlib.MatchTypePrefix, m.settingsHandler)
	m.b.RegisterHandler(botlib.HandlerTypeCallbackQueryData, CallbackSettingsHandler, botlib.MatchTypePrefix, m.selectMpSettingsHandler)
	m.b.RegisterHandler(botlib.HandlerTypeCallbackQueryData, CallbackSettingsSelectCabinetHandler, botlib.MatchTypePrefix, m.settingsMPHandler)
	m.b.RegisterHandler(botlib.HandlerTypeCallbackQueryData, CallbackChangeAPIHandler, botlib.MatchTypePrefix, m.ChangeApiHandler)
	m.b.RegisterHandler(botlib.HandlerTypeCallbackQueryData, CallbackChangeSheetHandler, botlib.MatchTypePrefix, m.ChangeSheetHandler)

	m.b.RegisterHandler(botlib.HandlerTypeCallbackQueryData, CallbackWbHandler, botlib.MatchTypeExact, wbHandler)
	m.b.RegisterHandler(botlib.HandlerTypeCallbackQueryData, CallbackYandexHandler, botlib.MatchTypeExact, m.yandexHandler)
	m.b.RegisterHandler(botlib.HandlerTypeCallbackQueryData, CallbackOzonHandler, botlib.MatchTypeExact, m.ozonHandler)

	m.b.RegisterHandler(botlib.HandlerTypeCallbackQueryData, CallbackWbFbsHandler, botlib.MatchTypeExact, m.stickersHandler)

	m.b.RegisterHandler(botlib.HandlerTypeCallbackQueryData, CallbackWbAnswerReview, botlib.MatchTypePrefix, m.wbAnswerReview)
	m.b.RegisterHandler(botlib.HandlerTypeCallbackQueryData, CallbackWbRegenReview, botlib.MatchTypePrefix, m.wbRegenReview)
	m.b.RegisterHandler(botlib.HandlerTypeCallbackQueryData, CallbackWbEditReview, botlib.MatchTypePrefix, m.wbEditReview)
	m.b.RegisterHandler(botlib.HandlerTypeCallbackQueryData, CallbackWbDeleteReview, botlib.MatchTypePrefix, m.wbDeleteReview)

	m.b.RegisterHandler(botlib.HandlerTypeCallbackQueryData, CallbackYandexStickersHandler, botlib.MatchTypePrefix, m.yandexFbsHandler)
	m.b.RegisterHandler(botlib.HandlerTypeCallbackQueryData, CallbackWbOrdersHandler, botlib.MatchTypePrefix, m.wbOrdersHandler)
	m.b.RegisterHandler(botlib.HandlerTypeCallbackQueryData, CallbackYandexOrdersHandler, botlib.MatchTypePrefix, m.yandexOrdersHandler)
	m.b.RegisterHandler(botlib.HandlerTypeCallbackQueryData, CallbackOzonStocksHandler, botlib.MatchTypePrefix, m.ozonStocksHandler)
	m.b.RegisterHandler(botlib.HandlerTypeCallbackQueryData, CallbackWbStocksHandler, botlib.MatchTypePrefix, m.wbStocksHandler)
	m.b.RegisterHandler(botlib.HandlerTypeCallbackQueryData, CallbackWbReturnsHandler, botlib.MatchTypePrefix, m.returnsHandler)
	m.b.RegisterHandler(botlib.HandlerTypeCallbackQueryData, CallbackOzonStickersHandler, botlib.MatchTypePrefix, m.ozonStickersHandler)
	m.b.RegisterHandler(botlib.HandlerTypeCallbackQueryData, CallbackOzonPrintStickersHandler, botlib.MatchTypePrefix, m.ozonPrintStickers)

	m.b.RegisterHandler(botlib.HandlerTypeCallbackQueryData, CallbackSelectOzonCabinetHandler, botlib.MatchTypePrefix, m.ozonCabinetHandler)
	m.b.RegisterHandler(botlib.HandlerTypeCallbackQueryData, CallbackSelectYandexCabinetHandler, botlib.MatchTypePrefix, m.yandexCabinetHandler)

	//b.RegisterHandler(botlib.HandlerTypeCallbackQueryData, "YANDEX_FBS", botlib.MatchTypePrefix, wbOrdersHandler)
}

// DefaultHandler ловит сообщения без команд, проверяет статус пользователя, после обновляет статус на enabled
func (m *Manager) DefaultHandler(ctx context.Context, bot *botlib.Bot, update *models.Update) {
	chatUserID := update.Message.From.ID
	chatID := update.Message.Chat.ID
	message := update.Message.Text

	user, err := m.tm.UserByChatID(ctx, chatUserID)
	if err != nil {
		log.Println(err)
		return
	} else if user == nil {
		log.Println("user not found", chatUserID)
		return
	}

	switch user.StatusID {
	case db.StatusEnabled:
		{
			if chatUserID != chatID {
				return
			}
		}
	case db.StatusWaitingWbState:
		{
			err = m.getWbStickers(ctx, bot, chatUserID, message)
			if err != nil {
				return
			}
		}
	case db.StatusWaitingYaState:
		{
			m.getYandexFbs(ctx, bot, chatUserID, message)
		}
	case db.StatusWaitingAPI:
		{
			m.changeAPI(ctx, bot, chatUserID, update.Message)
		}
	case db.StatusWaitingSheet:
		{
			m.changeSheet(ctx, bot, chatUserID, update.Message)
		}
	case db.StatusWaitingReview:
		{
			m.updateReview(ctx, bot, chatID, update.Message)
		}
	default:
		log.Println("Такого статуса пользователя нет")
	}

	_, err = m.tm.SetUserStatus(ctx, user, db.StatusEnabled)
	if err != nil {
		log.Println("Ошибка обновления статуса пользователя: ", err)
		return
	}
}

// createStartAdminMarkup создает клавиатуру с кнопками для авторизованного пользователя
func createStartAdminMarkup() (string, models.InlineKeyboardMarkup) {
	startMessage := "Выбери маркетплейс для работы"
	var buttonsRow []models.InlineKeyboardButton
	buttonsRow = append(buttonsRow, models.InlineKeyboardButton{Text: "ВБ", CallbackData: CallbackWbHandler})
	buttonsRow = append(buttonsRow, models.InlineKeyboardButton{Text: "ЯНДЕКС", CallbackData: CallbackYandexHandler})
	buttonsRow = append(buttonsRow, models.InlineKeyboardButton{Text: "ОЗОН", CallbackData: CallbackOzonHandler})
	allButtons := [][]models.InlineKeyboardButton{buttonsRow}
	markup := models.InlineKeyboardMarkup{InlineKeyboard: allButtons}
	return startMessage, markup
}

// createStartUserMarkup создает клавиатуру с кнопками для неавторизованного пользователя
func createStartUserMarkup() (string, models.InlineKeyboardMarkup) {
	startMessage := "Для доступа к функционалу бота пиши @molchanovz. А пока можешь перейти в наши магазины"
	var buttonsRow []models.InlineKeyboardButton
	buttonsRow = append(buttonsRow, models.InlineKeyboardButton{Text: "ВБ", URL: "https://www.wildberries.ru/seller/27566"})
	buttonsRow = append(buttonsRow, models.InlineKeyboardButton{Text: "ЯНДЕКС", URL: "https://market.yandex.ru/business--metr-v-kube/3697903"})
	buttonsRow = append(buttonsRow, models.InlineKeyboardButton{Text: "ОЗОН", URL: "https://www.ozon.ru/seller/metr-v-kube-259267"})
	allButtons := [][]models.InlineKeyboardButton{buttonsRow}
	markup := models.InlineKeyboardMarkup{InlineKeyboard: allButtons}
	return startMessage, markup
}

func (m *Manager) startHandler(ctx context.Context, bot *botlib.Bot, update *models.Update) {
	var chatID int64
	var text string

	if update.Message != nil {
		chatID = update.Message.From.ID
		fmt.Println(update.Message.Chat.ID)
	} else {
		chatID = update.CallbackQuery.From.ID
	}

	_, err := bot.SetMyCommands(ctx, &botlib.SetMyCommandsParams{
		Commands: []models.BotCommand{
			{Command: MessageStartHandler, Description: "Перезапуск бота"},
			{Command: MessageSettingsHandler, Description: "Настройки"},
		},
	})
	if err != nil {
		log.Println("Ошибка регистрации команд:", err)
		return
	}

	_, err = bot.SetChatMenuButton(ctx, &botlib.SetChatMenuButtonParams{
		ChatID: chatID,
		MenuButton: models.MenuButtonCommands{
			Type: models.MenuButtonTypeCommands,
		},
	})
	if err != nil {
		log.Println("Ошибка создания меню: ", err)
		return
	}

	user, err := m.tm.CreateUser(ctx, chatID)
	if err != nil {
		log.Println("Ошибка создания меню: ", err)
		return
	}

	var startMessage string
	var markup models.InlineKeyboardMarkup

	if user.IsAdmin {
		startMessage, markup = createStartAdminMarkup()
	} else {
		startMessage, markup = createStartUserMarkup()
	}

	if update.Message != nil {
		name := update.Message.From.FirstName
		text = fmt.Sprintf("Привет, %v. %v", name, startMessage)
		_, err = bot.SendMessage(ctx, &botlib.SendMessageParams{ChatID: chatID, Text: text, ReplyMarkup: markup})
		if err != nil {
			log.Printf("ошибка отправки сообщения %v", err)
			return
		}
	} else {
		messageID := update.CallbackQuery.Message.Message.ID
		text = startMessage
		_, err = bot.EditMessageText(ctx, &botlib.EditMessageTextParams{MessageID: messageID, ChatID: chatID, Text: text, ReplyMarkup: markup})
		if err != nil {
			log.Printf("ошибка редактирования сообщения %v", err)
			return
		}
	}
}

func WaitReadyFile(ctx context.Context, bot *botlib.Bot, chatID int64, progressChan chan tradeplus.Progress, done chan []string, errChan chan error) error {
	var progressMsgID int
	var lastReportedCurrent int
	var lastTotal int
	var err error
	for {
		select {
		case progress := <-progressChan:
			progressMsgID, err = sendProgress(ctx, bot, chatID, progress, lastReportedCurrent, lastTotal, progressMsgID)
			if err != nil {
				return err
			}

		case filePath := <-done:
			err = sendFiles(ctx, bot, chatID, filePath, progressMsgID)
			if err != nil {
				errChan <- err
			}
			return nil

		case err = <-errChan:
			_, err = bot.SendMessage(ctx, &botlib.SendMessageParams{ChatID: chatID, Text: err.Error()})
			return err
		}
	}
}

func sendFiles(ctx context.Context, bot *botlib.Bot, chatID int64, filePath []string, progressMsgID int) error {
	_, err := bot.SendChatAction(ctx, &botlib.SendChatActionParams{
		ChatID: chatID,
		Action: models.ChatActionUploadDocument,
	})
	if err != nil {
		return err
	}

	if len(filePath) == 0 {
		return ozon.ErrNoRows
	}

	for _, batchPath := range filePath {
		err = SendMediaMessage(ctx, bot, chatID, batchPath)
		if err != nil {
			log.Println(err)
			return err
		}
	}

	if progressMsgID != 0 {
		_, err = bot.DeleteMessage(ctx, &botlib.DeleteMessageParams{
			ChatID:    chatID,
			MessageID: progressMsgID,
		})
		if err != nil {
			return err
		}
	}

	text, markup := createStartAdminMarkup()
	_, err = bot.SendMessage(ctx, &botlib.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ReplyMarkup: markup,
	})
	if err != nil {
		return err
	}
	return nil
}

func sendProgress(ctx context.Context, bot *botlib.Bot, chatID int64, progress tradeplus.Progress, lastReportedCurrent int, lastTotal int, progressMsgID int) (int, error) {
	if progress.Current != lastReportedCurrent || progress.Total != lastTotal {
		lastReportedCurrent = progress.Current
		lastTotal = progress.Total

		text := fmt.Sprintf("Обработано заказов: %d из %d", progress.Current, progress.Total)

		if progressMsgID == 0 {
			msg, err := bot.SendMessage(ctx, &botlib.SendMessageParams{
				ChatID: chatID,
				Text:   text,
			})
			if err != nil {
				log.Println(err)
				return 0, err
			} else {
				progressMsgID = msg.ID
			}
		} else {
			_, err := bot.EditMessageText(ctx, &botlib.EditMessageTextParams{
				ChatID:    chatID,
				MessageID: progressMsgID,
				Text:      text,
			})
			if err != nil {
				log.Println(err)
				return 0, err
			}
		}
	}
	return progressMsgID, nil
}

type CallbacksForCabinetMarkup struct {
	PaginationCallback string
	SelectCallback     string
	BackCallback       string
}

func createCabinetsMarkup(cabinetsIds tradeplus.Cabinets, callbacks CallbacksForCabinetMarkup, page int, hasNext bool) models.InlineKeyboardMarkup {
	var keyboard [][]models.InlineKeyboardButton
	var row []models.InlineKeyboardButton
	var button models.InlineKeyboardButton
	for _, c := range cabinetsIds {
		row = []models.InlineKeyboardButton{}
		button = models.InlineKeyboardButton{Text: c.Name, CallbackData: fmt.Sprintf("%v%v", callbacks.SelectCallback, c.ID)}
		row = append(row, button)

		keyboard = append(keyboard, row)
	}

	//Добавление кнопок для пагинации
	row = []models.InlineKeyboardButton{}
	if page > 1 {
		button = models.InlineKeyboardButton{Text: "⬅️", CallbackData: callbacks.PaginationCallback + strconv.Itoa(page-1)}
		row = append(row, button)
	}

	if hasNext {
		button = models.InlineKeyboardButton{Text: "➡️", CallbackData: callbacks.PaginationCallback + strconv.Itoa(page+1)}
		row = append(row, button)
	}

	if row != nil {
		keyboard = append(keyboard, row)
	}

	//row = []models.InlineKeyboardButton{}
	//button = models.InlineKeyboardButton{Text: "Добавить аккаунт", CallbackData: addParserCallback}
	//row = append(row, button)
	//keyboard = append(keyboard, row)

	row = []models.InlineKeyboardButton{}
	button = models.InlineKeyboardButton{Text: "Назад", CallbackData: callbacks.BackCallback}
	row = append(row, button)
	keyboard = append(keyboard, row)

	markup := models.InlineKeyboardMarkup{
		InlineKeyboard: keyboard,
	}
	return markup
}

func SendTextMessage(ctx context.Context, bot *botlib.Bot, chatID int64, text string) (*models.Message, error) {
	message, err := bot.SendMessage(ctx, &botlib.SendMessageParams{ChatID: chatID, Text: text})
	if err != nil {
		return nil, err
	}
	return message, nil
}

func SendMediaMessage(ctx context.Context, bot *botlib.Bot, chatID int64, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}

	inputFile := models.InputFileUpload{
		Filename: filePath,
		Data:     file,
	}

	_, err = bot.SendDocument(ctx, &botlib.SendDocumentParams{ChatID: chatID, Document: &inputFile})
	if err != nil {
		return err
	}
	return nil
}

// Функция для автоподбора ширины колонок
func autoFitColumns(f *excelize.File, sheet string, columns []string) error {
	for _, col := range columns {
		maxWidth := 8.0 // Минимальная ширина по умолчанию
		rows, err := f.GetRows(sheet)
		if err != nil {
			return err
		}

		// Находим максимальную длину содержимого в колонке
		for _, row := range rows {
			colIdx := int(col[0] - 'A')
			if colIdx < len(row) {
				cellValue := row[colIdx]
				// Учитываем длину текста + 2 символа для отступов
				width := float64(len(cellValue))*1.1 + 2
				if width > maxWidth {
					maxWidth = width
				}
			}
		}

		// Устанавливаем ширину
		if err := f.SetColWidth(sheet, col, col, maxWidth); err != nil {
			return err
		}
	}
	return nil
}

func calculateSmartDemandForecast(salesData []float64) float64 {
	if len(salesData) == 0 {
		return 0
	}

	// Настройки
	const (
		shortWindow = 4  // Анализ последних 4 дней для "горячего" тренда
		longWindow  = 14 // Анализ за 14 дней для базового уровня
	)

	// "горячий" тренд (последние 4 дня)
	hotTrend := 0.0
	if len(salesData) >= shortWindow {
		recent := salesData[len(salesData)-shortWindow:]
		first, last := recent[0], recent[len(recent)-1]
		if first > 0 {
			hotTrend = last / first // Рост в последние дни
		}
	}

	//Среднее за весь период (14 дней)
	fullPeriodAverage := mean(salesData)

	//Среднее за последние 4 дня
	var recentAverage float64
	if len(salesData) >= shortWindow {
		recentAverage = mean(salesData[len(salesData)-shortWindow:])
	} else {
		recentAverage = mean(salesData)
	}

	// Динамический вес для тренда
	trendWeight := 0.5  // Базовый вес тренда
	if hotTrend > 2.0 { // Если рост более 2x
		trendWeight = 0.8 // Сильнее учитываем тренд
	}

	// Комбинированный прогноз
	forecast := (recentAverage*trendWeight + fullPeriodAverage*(1-trendWeight)) * float64(longWindow)

	// Гарантия, что прогноз не ниже последних продаж
	if len(salesData) > 0 {
		lastDaySales := salesData[len(salesData)-1]
		minForecast := lastDaySales * float64(longWindow) * 0.7 // Не менее 70% от последнего дня
		if forecast < minForecast {
			forecast = minForecast
		}
	}

	return math.Round(forecast)
}

func mean(values []float64) float64 {
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}
