package bot

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"tradebot/pkg/db"
	"tradebot/pkg/tradeplus"
	"tradebot/pkg/tradeplus/ozon"
	"tradebot/pkg/tradeplus/wb"

	botlib "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const CallbackShipmentsAllHandler = "SHIPMENTS-ALL"

func (m *Manager) shipmentsAllHandler(ctx context.Context, bot *botlib.Bot, update *models.Update) {
	chatID := update.CallbackQuery.From.ID

	_, err := bot.AnswerCallbackQuery(ctx, &botlib.AnswerCallbackQueryParams{CallbackQueryID: update.CallbackQuery.ID})
	if err != nil {
		log.Printf("%v", err)
	}

	startMsg, err := SendTextMessage(ctx, bot, chatID, "Заполняю листы «Все заказы WB/Ozon» за последние 7 дней...")
	if err != nil {
		log.Printf("%v", err)
	}

	msk := time.FixedZone("MSK", 3*3600)
	now := time.Now().In(msk).AddDate(0, 0, -tradeplus.OrdersDaysAgo)
	yesterday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, msk)

	var failed []string

	wbCabinets, err := m.tm.GetCabinetsByMp(ctx, db.MarketWB)
	if err != nil {
		failed = append(failed, fmt.Sprintf("WB cabinets: %v", err))
	}
	for _, cab := range wbCabinets {
		if cab.Settings.ShipmentsAllSheetID == "" {
			continue
		}
		sm, err := wb.NewShipmentsManager(cab)
		if err != nil {
			failed = append(failed, fmt.Sprintf("WB cab=%d init: %v", cab.ID, err))
			continue
		}
		if err := sm.WriteAggregatedForDate(ctx, yesterday); err != nil {
			failed = append(failed, err.Error())
		}
	}

	ozonCabinets, err := m.tm.GetCabinetsByMp(ctx, db.MarketOzon)
	if err != nil {
		failed = append(failed, fmt.Sprintf("Ozon cabinets: %v", err))
	}
	for _, cab := range ozonCabinets {
		if cab.Settings.ShipmentsAllSheetID == "" {
			continue
		}
		sm, err := ozon.NewShipmentsManager(cab)
		if err != nil {
			failed = append(failed, fmt.Sprintf("Ozon cab=%d init: %v", cab.ID, err))
			continue
		}
		if err := sm.WriteAggregatedForDate(ctx, yesterday); err != nil {
			failed = append(failed, err.Error())
		}
	}

	if startMsg != nil {
		_, err = bot.DeleteMessage(ctx, &botlib.DeleteMessageParams{ChatID: chatID, MessageID: startMsg.ID})
		if err != nil {
			log.Printf("%v", err)
		}
	}

	text := "Заказы заполнены в листы «Все заказы WB/Ozon»"
	if len(failed) > 0 {
		text = fmt.Sprintf("Завершено с ошибками:\n%s", strings.Join(failed, "\n"))
	}
	_, err = SendTextMessage(ctx, bot, chatID, text)
	if err != nil {
		log.Printf("%v", err)
	}
}
