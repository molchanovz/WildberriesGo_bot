# TradeBot — фичи и устройство

Полный обзор того, что умеет проект, как он устроен и где какая логика живёт.

---

## 1. Что это вообще

Go-приложение, которое:

- Тянет заказы, отгрузки, возвраты и отзывы из API трёх маркетплейсов: **Ozon**, **Wildberries**, **Yandex Market**.
- Пишет данные в **PostgreSQL** и **Google Sheets**.
- Печатает **FBS-этикетки** (PDF) по запросу.
- Отвечает на отзывы WB через **ChatGPT-сервис**.
- Общается с пользователем через **Telegram-бот**.
- Считает прогноз спроса по Ozon (анализ остатков).

---

## 2. Архитектура и стек

| Слой | Что использует |
|------|----------------|
| HTTP | `labstack/echo/v4` |
| ORM | `go-pg/pg/v10` (модели генерируются `mfd-generator`) |
| Telegram | `go-telegram/bot` |
| AI | вызов внешнего `chatgptsrv` по JSON-RPC 2.0 (использует `openai-go`) |
| PDF | `pdfcpu/pdfcpu` + `gen2brain/go-fitz` |
| Excel | `xuri/excelize/v2` |
| Observability | `prometheus/client_golang`, `getsentry/sentry-go`, `vmkteam/embedlog` |
| Cron | `vmkteam/cron` (поверх `robfig/cron/v3`) |
| Codegen | `vmkteam/zenrpc`, `vmkteam/appkit`, `mfd-generator`, `colgen` |

Зависимости вендорятся в `vendor/`.

### Пакеты

- `cmd/tradebot/` — точка входа, флаги, graceful shutdown.
- `pkg/app/` — HTTP-сервер (echo), регистрация cron-задач, Prometheus, Sentry, wiring.
- `pkg/bot/` — Telegram-бот, хендлеры по маркетплейсам и настройкам.
- `pkg/client/` — низкоуровневые HTTP-обёртки внешних API:
  - `ozon/`, `wb/`, `yandex/` — маркетплейсы;
  - `google/` — Google Sheets (OAuth);
  - `chatgptsrv/` — JSON-RPC к ChatGPT-сервису.
- `pkg/tradeplus/` — бизнес-логика. Корень — общие модели/менеджеры, подпапки `ozon/`, `wb/`, `yandex/` — фичи маркетплейсов, `schedule/` — оркестрация cron-задач.
- `pkg/db/` — go-pg слой (сгенерировано из `docs/model/tradebot.mfd`).
- `pkg/google/` — утилиты для Sheets, используемые из бизнес-логики.

---

## 3. Точка входа и конфиг

### `cmd/tradebot/main.go`

Флаги:

- `-config` (default `./cfg/local.toml`) — путь к TOML.
- `-verbose` (true) — debug-логирование.
- `-json` (true) — JSON-логи.
- `-dev` (true) — dev-режим (раскрывает все SQL).

Порядок старта:

1. `embedlog.NewLogger` / `NewDevLogger`, `slog.SetDefault`.
2. Парсинг TOML → `app.Config`.
3. Sentry init (если задан `Sentry.DSN`).
4. `pg.Connect(cfg.Database)`, проверка версии БД, query-hook в dev.
5. `app.NewApp(...)` и запуск в goroutine с recover() в Sentry.
6. `signal.Notify(SIGTERM, SIGINT)` → `a.Shutdown(5s)`.

### `cfg/local.toml`

```toml
[Database]
Addr = "localhost:5432"
User = "sergey"
Password = "..."
Database = "tradebot"

[Bot]
ReviewChatID = -5084065492    # куда падают новые отзывы
Token = "..."                 # Telegram bot token
ProxyURL = ""

[Server]
Host = "localhost"
Port = 8075
IsDevel = true
EnableVFS = true

[Sentry]
Environment = "..."
DSN = "..."

[Service]
ChatGPTSrvURL = "http://.../int/rpc/"

[Cron]
OzonWriter        = "0 5 * * *"
OzonShipments     = "0 0 * * *"
OzonShipmentsAll  = "0 18 * * *"
YandexWriter      = "0 5 * * *"
YandexShipments   = "0 0 * * *"
WBWriter          = "0 5 * * *"
WBShipments       = "0 0 * * *"
WBShipmentsAll    = "0 18 * * *"
OrderCleaner      = "0 5 * * *"
SendNewReviews    = "* * * * *"

[OpenAI]
Token = "..."
```

---

## 4. HTTP-сервер (`pkg/app/`)

- `echo` с CORS: `AllowOrigins=["*"]`, `AllowMethods=[GET,PUT,POST,DELETE]`.
- Middleware: Sentry tracking, request tracing, IP extraction (доверяет `X-Real-IP` для `0.0.0.0/0`).

Endpoints:

| Маршрут | Зачем |
|---------|-------|
| `GET /status` | health-check (PING к БД) |
| `GET /metrics` | Prometheus (HTTP-метрики + db connection pool) |
| `GET\|POST /debug/cron` | UI cron-менеджера |
| `GET\|POST /debug/pprof/*` | Профилирование Go |
| `GET /` (dev) | HTML со списком всех зарегистрированных routes |

Порт: `cfg.Server.Host:Port` (по умолчанию `localhost:8075`).

---

## 5. Cron-задачи (`pkg/app/cron.go`)

`cron.NewManager()` с middleware: `WithMetrics`, `WithSLog`, `WithSkipActive` (не запускать дубль если предыдущая ещё крутится), `WithRecover`.

| Имя | Расписание | Метод | Что делает |
|-----|-----------|-------|------------|
| `wbOrders` | `0 5 * * *` | `WriteWB` | Заказы WB → Google Sheets |
| `wbShipments` | `0 0 * * *` | `WriteWBShipments` | Отгрузки WB за вчера (по supplies) |
| `wbShipmentsAll` | `0 18 * * *` | `WriteWBShipmentsAll` | Все заказы WB в окне `[вчера 17:00, сегодня 17:00)` MSK |
| `ozonOrders` | `0 5 * * *` | `WriteOzon` | Заказы Ozon → Google Sheets |
| `ozonShipments` | `0 0 * * *` | `WriteOzonShipments` | Отгрузки Ozon за вчера |
| `ozonShipmentsAll` | `0 18 * * *` | `WriteOzonShipmentsAll` | Все заказы Ozon в окне `[вчера 17:00, сегодня 17:00)` MSK |
| `yandexOrders` | `0 5 * * *` | `WriteYandex` | Заказы Yandex Market |
| `yandexShipments` | `0 0 * * *` | `WriteYandexShipments` | Отгрузки Yandex (только кабинеты с `Type="fbs"`) |
| `cleanOrders` | `0 5 * * *` | `ClearOrders` | Удалить старые заказы из БД (старше недели) |
| `sendNewReviews` | `* * * * *` | `SendNewReviews` | Найти новые отзывы и кинуть в `ReviewChatID` |

**Замечание про таймзону cron:** `robfig/cron/v3` использует `time.Local` сервера. Расписание `0 18 * * *` сработает в 18:00 MSK, если сервер в MSK; если в UTC — нужно `0 15 * * *`.

### Окно «все заказы за сутки»

`OzonShipmentsAll` и `WBShipmentsAll` пишут в отдельный «общий» лист все заказы, которые пришли в окне `[day + 17h, day + 41h)` MSK (где `day` = вчерашняя дата 00:00 MSK). Константа `aggregatedCutoffHour = 17` в обоих `shipments.go`.

---

## 6. Telegram-бот (`pkg/bot/`)

### Жизненный цикл

- `pkg/bot/service.go` — `Service.Start()` поднимает бота, проксирует через `Bot.ProxyURL` если задан, и вешает все хендлеры через `Manager.RegisterBotHandlers()`.
- `pkg/bot/handlers_default.go` — `Manager` хранит ссылки на БД, Telegram-клиент, ChatGPT-клиент и in-memory мапы для FSM-состояний пользователя между сообщениями: `SheetMap`, `APIMap`, `ReviewMap`.

### Состояния пользователя (FSM)

Хранятся в `users.statusId`:

- `StatusEnabled` — норма.
- `StatusWaitingWbState` — ждём ID отгрузки WB для печати стикеров.
- `StatusWaitingYaState` — ждём ID отгрузки Yandex.
- `StatusWaitingAPI` — ждём новый API-ключ в settings.
- `StatusWaitingSheet` — ждём ссылку на Google Sheet в settings.
- `StatusWaitingReview` — ждём текст ответа на отзыв (редактирование).

`DefaultHandler` (`handlers_default.go:98`) маршрутизирует любой текст не-команду на нужный обработчик по текущему статусу.

### Главное меню

- `/start` (`MessageStartHandler` / `CallbackStartHandler` → `startHandler`):
  - Создаёт/находит пользователя по `chatID`.
  - Регистрирует команды `/start`, `/settings`.
  - Админ видит: `[ВБ] [ЯНДЕКС] [ОЗОН]`.
  - Обычный пользователь — текст «Для доступа пиши @molchanovz» с ссылками на магазины.

- `/settings` (`settingsHandler` в `handlers_settings.go`):
  - Меню маркетплейсов → список кабинетов → меню `[Изменить ключ API] [Изменить таблицу для заказов]`.

### Wildberries (`handlers_wb.go`)

| Callback | Что делает |
|----------|-----------|
| `WB` | Меню кабинета: `[Этикетки FBS] [Анализ заказов] [Возвраты в ПВЗ]` |
| `WB-FBS` | Просит ID отгрузки → `getWbStickers` → PDF этикеток |
| `WB-STOCKS_` | `wbStocksHandler`: заказы за 14 дней + остатки → Excel `WB_stock_analysis.xlsx` |
| `WB-RETURNS_` | `returnsHandler`: возвраты в ПВЗ → Excel |
| `WB-ANSWER-REVIEW_{id}` | `wbAnswerReview`: ChatGPT генерирует ответ → POST в WB API → удаляет сообщение из чата отзывов |
| `WB-EDIT-REVIEW_{id}` | `wbEditReview`: ставит `StatusWaitingReview`, ждёт новый текст → `updateReview` пишет в БД, переотправляет в чат |
| `WB-DELETE-REVIEW` | Просто удаляет сообщение |

Поток отзывов: cron `sendNewReviews` каждую минуту вызывает `bot.Service.Manager().SendNewReviews()`, которая в `sendReview` (`handlers_wb.go:269`) отправляет новые отзывы в `ReviewChatID` с кнопками `[Ответить] [Редактировать] [Удалить]`.

### Ozon (`handlers_ozon.go`)

| Callback | Что делает |
|----------|-----------|
| `OZON` | Список кабинетов |
| `CABINET-OZON_{id}` | Меню: `[Этикетки FBS] [Анализ заказов]` |
| `OZON-STICKERS_{id}` | Список складов |
| `OZON-WAREHOUSE_{id}_{wh}` | Выбор: `[Новые] [Все из сборки]` |
| `OZON-PRINT-STICKERS_{id}_{wh}_{flag}` | `ozonPrintStickers`: PDF этикеток (батчи по 200 заказов через `pdfcpu`); `flag=new` пишет новые ID в БД |
| `OZON-STOCKS_{id}` | `ozonStocksHandler`: заказы 14 дней + остатки FBS/FBO + прогноз спроса → Excel `OZON_stock_analysis.xlsx` |

Прогноз спроса (`calculateSmartDemandForecast`, `handlers_default.go:463`):

- Анализ продаж за 14 дней.
- Если за последние 4 дня темп вырос в 2x — «горячий тренд», вес тренда поднимается до 0.8.
- `forecast = (avg_recent * trendWeight + avg_full * (1 − trendWeight)) * 14`.
- Гарантирует, что прогноз ≥ 70% от последнего дня продаж.

### Yandex (`handlers_yandex.go`)

| Callback | Что делает |
|----------|-----------|
| `YANDEX` | Список кабинетов |
| `CABINET-YANDEX_{id}` | Меню: `[Этикетки FBS]` |
| `YANDEX-STICKERS_{id}` | Просит ID отгрузки → `getYandexFbs` → Excel с заказами отгрузки |

### Настройки (`handlers_settings.go`)

- `SETTINGS_{MP}` → выбор кабинета.
- `CHANGE-API_{id}` → `StatusWaitingAPI` → `changeAPI` → `UpdateCabinet`.
- `CHANGE-SHEET_{id}` → `StatusWaitingSheet` → `changeSheet` → `UpdateCabinet`.

### Прогресс и отправка файлов

- `SendTextMessage` (`handlers_default.go:407`) — текст.
- `SendMediaMessage` (`handlers_default.go:415`) — документ из ФС.
- `WaitReadyFile` (`handlers_default.go:254`) — слушает три канала: `progressChan` (обновляет сообщение «Обработано N из M»), `done` (отправляет файл), `errChan` (сообщение об ошибке).

---

## 7. Бизнес-логика (`pkg/tradeplus/`)

### Корень

- `model.go` — `Cabinet` (обёртка `db.Cabinet`), `Product`, `User`, `Authorization`, утилиты `Ptr/Deref`.
- `wb_model.go` — `Card`, `Return`, `Review` (с `Stars()`, `ToMessage()`, `ToPrompt()`), конвертеры `NewReviewFromWB`, `NewReturns`, `NewCardList`.
- `manager.go` — CRUD кабинетов/пользователей/заказов/отзывов: `UserByChatID`, `CreateUser`, `SetUserStatus`, `GetCabinetsByMp`, `GetCabinetByID`, `GetPrintedOrders`, `CreateOrders`, `DeleteOrders`, `UpdateCabinet`, `UpdateReview`, `GetReviewByID`.
- `orderwriter.go` — общий интерфейс выгрузки в Sheets (`OrderManager`, `NewOrdersManager`, `OrdersDaysAgo = 1`).
- `printer.go` — структура папок для PDF (`CodesPath`, `ReadyPath`, `BatchesPath`, `GeneratedPath`), `Progress{Current,Total}`, `CreateDirectories`, `CleanFiles`.
- `collection.go` (генерация colgen) — `MapP`, `Products.SetRecommendations`.

### Ozon (`pkg/tradeplus/ozon/`)

#### `ordersAndReturns.go` — `OrdersManager`

- `WriteToGoogleSheets(titleRange, fbsRange, fboRange, returnsRange)` — лист `Заказы OZON-{день}`, три колонки: FBS, FBO, возвраты.
- `getPostingsMapFBS()` — `PostingsListFbs`, статус ≠ "cancelled".
- `getPostingsMapFBO()` — `PostingsListFbo`.
- `GetReturnsMap()` — `ReturnsList`, фильтр `status="ReturnedToOzon"`.

#### `shipments.go` — `ShipmentsManager`

Константы:

- `shipmentsReportLookback = 30 дней` (на сколько назад тянем отчёт).
- `shipmentsReportTail = 2 дня` (запас вперёд).
- `shipmentsReportTimeout = 10 минут` (ожидание готовности отчёта).
- `shipmentsParallelism = 4` (одновременных складов).
- `aggregatedCutoffHour = 17`.

Колонки CSV-отчёта, которые мы используем:

- `Фактическая дата передачи в доставку` (`shipmentsDateColumn`).
- `Принят в обработку` (`shipmentsInProcessColumn`).
- `Статус` (`shipmentsStatusColumn`).
- + первая своя колонка `ID склада`.

Два сценария:

1. **`WriteForDate(day)`** — лист `Отправлено на Ozon FBS-{день}`:
   - окно `[day 00:00 MSK, day+1 00:00 MSK)`;
   - для каждого склада в кабинете параллельно создаёт `/v1/report/postings/create`, ждёт `info.status="success"`, скачивает CSV;
   - фильтр строк — по `Фактическая дата передачи в доставку` в окне дня;
   - все склады включены (даже из `ExcludedShipmentsWarehouseIDs`).

2. **`WriteAggregatedForDate(day)`** — общий лист `Все заказы Ozon`:
   - окно `[day + 17h, day + 41h)` MSK (т.е. «со вчера 17:00 до сегодня 17:00»; cron срабатывает в 18:00 — это часовая дельта, чтобы маркетплейс успел отдать актуальные данные);
   - фильтр по `Принят в обработку`;
   - `dropCancelledNotShipped`: выкидываем строки, у которых `Статус` содержит «отмен» и `Фактическая дата передачи в доставку` пуста;
   - склады из `ExcludedShipmentsWarehouseIDs` пропускаются;
   - строки красятся `AppendColored` чередующимся серым (`aggregatedDayColor(day)`).

Общий сборщик отчётов вынесен в `fetchWarehouseCSVs(ctx, reportFromUTC, reportToUTC)` — параллельно по складам, возвращает `[]warehouseCSV{warehouseID, warehouseName, header, rows, err}`.

#### `stickers.go` — `StickerManager`

- `GetAllLabels()` — все FBS-заказы за 7 дней со статусом `awaiting_deliver`.
- `GetNewLabels()` — только те, которых нет в `printedOrders` (после успешной печати ID кладутся в БД).
- Процесс на один заказ: `Labels(postingNumber)` → PDF → `pdfcpu` извлекает первую страницу → накладывается баркод (повернутый на 90°) → группировка в батчи по 200 → `pdfcpu merge`.

#### `service.go`

- `Service` (на `Cabinet`) — фасады `GetOrdersAndReturnsManager()`, `GetStocksManager()` (`AnalyzeManager`), `GetStickersFBSManager()`.

### Wildberries (`pkg/tradeplus/wb/`)

#### `ordersAndReturns.go` — `OrdersManager`

- `Write()` — лист `Заказы WB-{день}`. Колонки A:B = FBO, D:E = FBS, G:H = возвраты.
- `getPostingsMap()` — `GetAllOrders(daysAgo, flagFbs)` + разделение по складу.
- `getReturnsMap()` — `GetSalesAndReturns`, фильтр по префиксу `SaleID="R"`.

#### `reviews.go` — `ReviewManager`

- Подробный prompt в `Prompt`: типы жалоб (качество, брак, эффект, комплектация), ограничения (не более 4 предложений, без компенсаций и мед-утверждений, без указания артикулов).
- Использует `chatgptsrv` для генерации ответов и `AnswerReview` (`POST /api/v1/feedbacks/answer`) для отправки.

#### `shipments.go` — `ShipmentsManager`

Константы:

- `wbSupplierURL = https://marketplace-api.wildberries.ru`.
- `wbSuppliesLimit = 1000`, `wbOrdersLimit = 1000`, `wbStickerBatch = 100`, `wbStatusBatch = 1000`.
- `wbOrdersWindowDaysBack = 60`, `wbOrdersWindowFw = 1` — окно поиска заказов в основном листе.
- `aggregatedCutoffHour = 17`.
- `wbCancelledWbStatuses = { canceled, canceled_by_client, declined_by_client, defect, canceled_by_carrier }` — отменённые статусы со стороны системы WB.
- `wbSupplierStatusRu` — мапа supplier-статусов в русский для отображения.

API-обёртка `wbShipmentsAPI` (auth: `Authorization: token`):

- `listSuppliesOnDate(loc, day)` — supplies со `ScanDt` в `[day, day+1)`.
- `orderIDsBySupply(supplyID)` — список order ID.
- `listOrdersInWindow(from, to, wanted)` — заказы по набору ID с обходом 30-дневного лимита чанками по 29 дней.
- `listOrdersByDate(from, to)` — все заказы с `CreatedAt` в окне (для агрегированного листа). Дополнительно фильтрует ответ на клиенте по `CreatedAt`.
- `stickers(ids)` — `POST /api/v3/orders/stickers` пачками по 100 (тип PNG 58×40).
- `statuses(ids)` — `POST /api/v3/orders/status` пачками по 1000. Возвращает **две** мапы: `supplierRu` (русская строка для отображения) и `wb` (raw `wbStatus` для проверки отмен).

Колонки листа (`wbHeader`, 15 шт.):

```
№ задания, QR-код поставки, Стикер, Дата создания, Дата сканирования ШК ТТН,
Наименование, Размер, Цвет, Баркод, Стоимость, Валюта,
Артикул Wildberries, Артикул продавца, ID склада, Статус задания
```

Два сценария:

1. **`WriteForDate(day)`** — лист `Отправлено на WB FBS-{день}`:
   - supplies со `ScanDt` в дне → `order-ids` → заказы за `[day-60d, day+1d]` → стикеры/статусы;
   - 15 колонок заполнены полностью (supply, scanDt и т.д.).

2. **`WriteAggregatedForDate(day)`** — общий лист `Все отгрузки WB`:
   - окно `[day + 17h, day + 41h)` MSK;
   - `listOrdersByDate` тянет все заказы с `CreatedAt` в окне;
   - `isWBCancelledNotShipped(order, wbStatus)`: `SupplyID == ""` И `wbStatus ∈ wbCancelledWbStatuses` → выкидываем;
   - стикеры + статусы по оставшимся ID;
   - `excludedWHs` пропускаются;
   - те же 15 колонок (поля supply/scanDt пустые если заказ ещё не на сборке);
   - окраска `AppendColored` чередующимся серым.

#### `stickers.go` — `StickerManager`

- `GetReadyFile(supplyID)` — `orders(supplyID)` → `GetStickersFbs(orderID)` (base64 PDF) → `decodeToPDF` → батчи по 300 → `pdfcpu merge` в `{supplyID}_{batchNum}.pdf`.

### Yandex Market (`pkg/tradeplus/yandex/`)

#### `ordersAndReturns.go`

- `OrdersManager.Write()` — лист `Заказы YM-{день}`, FBO в A:B, FBS в D:E.
- `ordersMap(campaignID)` — `GetOrdersFbo`, суммирует по SKU.

#### `shipments.go` — `ShipmentsManager`

- `ymBaseURL = https://api.partner.market.yandex.ru`, `Api-Key: token`.
- `ymOrdersLimit = 50`, `ymStatsOrderCap = 200`.
- `ymCancelBeforeShipment = { CANCELLED_BEFORE_PROCESSING, CANCELLED_IN_PROCESSING }` — фазы отмены до отгрузки, такие заказы исключаются.

API-обёртка `ymShipmentsAPI`:

- `businessID(campaignID)` — нужен для дальнейших запросов.
- `listOrdersByShipmentDate(day)` — FBS-заказы по дате отгрузки.
- `cancelPhases(ids)` — для CANCELLED: `POST /v2/campaigns/{cid}/stats/orders`.
- `shipmentWarehouses(ids)` — названия складов по shipment ID.

`WriteForDate(day)`:

- лист `Отправлено на Яндекс FBS-{день}`;
- разворачивает `Items` в строки (по товару), пересчитывает цену за штуку;
- колонки: `Номер заказа, Ваш номер, Дата оформления, SKU, Название, Кол-во, Цена, Статус, Статус изменён, Способ оплаты, Склад отгрузки, Дата отгрузки, Грузоместа, Регион доставки`.

#### `service.go`

- `Service` поверх списка кабинетов; разделяет на fbo/fbs, отдаёт `OrdersManager`, `StickersManager`.

### Оркестрация (`pkg/tradeplus/schedule/schedule.go`)

`Manager` зависит от `tradeplus.Manager`, `bot.Service`, логгера. Все методы отгрузок берут `yesterday = now.In(MSK).AddDate(0, 0, -OrdersDaysAgo)` (с обнулением времени) и проходят по всем кабинетам нужного маркетплейса.

| Метод | Условие на кабинет | Что вызывает |
|-------|--------------------|--------------|
| `WriteWB` | у `cabinets[0]` есть `SheetLink` | `wb.NewOrdersManager(...).Write()` |
| `WriteOzon` | — | два кабинета пишут на одну таблицу со сдвигом строк |
| `WriteYandex` | — | `yandex.NewService(cabinets...).GetOrdersAndReturnsManager().Write()` |
| `WriteWBShipments` | `Settings.ShipmentsSheetID != ""` | `wb.NewShipmentsManager(cab).WriteForDate(yesterday)` |
| `WriteWBShipmentsAll` | `Settings.ShipmentsAllSheetID != ""` | `wb.NewShipmentsManager(cab).WriteAggregatedForDate(yesterday)` |
| `WriteOzonShipments` | `Settings.ShipmentsSheetID != ""` | `ozon.NewShipmentsManager(cab).WriteForDate(yesterday)` |
| `WriteOzonShipmentsAll` | `Settings.ShipmentsAllSheetID != ""` | `ozon.NewShipmentsManager(cab).WriteAggregatedForDate(yesterday)` |
| `WriteYandexShipments` | `cab.Type == "fbs"` И `ShipmentsSheetID != ""` | `yandex.NewShipmentsManager(...).WriteForDate(yesterday)` |
| `SendNewReviews` | — | `bot.Service.Manager().SendNewReviews()` |
| `ClearOrders` | — | `tradeplus.Manager.DeleteOrders()` |

---

## 8. Внешние клиенты (`pkg/client/`)

### Ozon Seller (`pkg/client/ozon/`)

Заголовки: `Client-Id`, `Api-Key`.

| HTTP | Endpoint | Метод клиента |
|------|----------|----------------|
| POST | `/v2/posting/fbo/get` | `PostingFbo` |
| POST | `/v3/posting/fbs/get` | `PostingFbs` |
| POST | `/v3/posting/fbs/list` | `PostingsListFbs` |
| POST | `/v2/posting/fbo/list` | `PostingsListFbo` |
| POST | `/v2/posting/fbs/package-label` | `Labels` |
| POST | `/v3/returns/company/fbo` | `v3ReturnsCompanyFbo` |
| POST | `/v3/returns/company/fbs` | `v3ReturnsCompanyFbs` |
| POST | `/v1/returns/list` | `ReturnsList` |
| POST | `/v2/analytics/stock_on_warehouses` | `Stocks` |
| POST | `/v1/analytics/stocks` | `StocksAnalytics` |
| POST | `/v2/warehouse/list` | `Warehouses` |
| POST | `/v1/cluster/list` | `Clusters` |
| POST | `/v3/product/list` | `Products` (статус `TO_SUPPLY`) |
| POST | `/v4/product/info/attributes` | `ProductsWithAttributes` (видимые) |

Плюс отдельная обёртка `ozonReportAPI` в `pkg/tradeplus/ozon/shipments.go` для `/v1/report/postings/{create,info}` — потому что нам нужно видеть полное тело ответа при ошибке и не привязываться к существующему `Client`.

### Wildberries (`pkg/client/wb/`)

Заголовок: `Authorization: token`.

| HTTP | Endpoint | Метод |
|------|----------|-------|
| GET | `/api/v3/orders` | `GetOrdersFBS` |
| GET | `/api/v3/orders/status` | `ordersFBSStatus` |
| POST | `/api/v3/orders/stickers` | `getCodesByOrderID` (PNG 58×40) |
| POST | `/content/v2/get/cards/list` | `getCards` (cursor: updatedAt, nmID, limit) |
| GET | `/api/v1/supplier/stocks` | `stocksFbo` (от 2019-06-20) |
| GET | `/api/v1/supplier/orders` | `apiOrdersALL` |
| GET | `/api/v1/supplier/sales` | `apiSalesAndReturns` |
| GET | `/api/marketplace/v3/supplies/{supplyID}/order-ids` | `getOrdersBySupplyID` |
| GET | `/api/v1/analytics/goods-return` | `getReturns` |
| GET | `/api/v1/feedbacks` | `Reviews` (unanswered=false, take=100) |
| POST | `/api/v1/feedbacks/answer` | `AnswerReview` |

Также отдельная обёртка `wbShipmentsAPI` в `pkg/tradeplus/wb/shipments.go` — используется только из shipments-логики, более узкий API.

### Yandex Market (`pkg/client/yandex/`)

Заголовок: `Api-Key: token`. campaignID захардкодирован как `90788543`.

| HTTP | Endpoint | Метод |
|------|----------|-------|
| GET | `/campaigns/{id}/first-mile/shipments/{supplyID}` | `ShipmentInfo` |
| GET | `/campaigns/{id}/orders/{orderID}` | `OrderInfo` |
| GET | `/campaigns/{id}/orders/{orderID}/delivery/labels` | `GetStickers` (A9_HORIZONTALLY) |
| POST | `/campaigns/{id}/stats/orders` | `getOrders` |

### Google Sheets (`pkg/client/google/sheets.go`)

OAuth2: `credentials.json` + `token.json`. При первом запуске — интерактивная авторизация через `getTokenFromWeb` (open auth URL, paste code).

Методы:

- `EnsureSheet(spreadsheetID, title) (bool, error)` — создаёт лист если нет. Толерантен к ошибке «лист уже существует» (русский и английский вариант сообщения от Google).
- `Append(spreadsheetID, writeRange, values)` — `INSERT_ROWS`, `USER_ENTERED`.
- `AppendColored(spreadsheetID, sheetTitle, values, r, g, b)` — добавляет строки и красит вставленный блок через `BatchUpdate.RepeatCell` (RGB в `[0..1]`). Используется для агрегированных листов отгрузок.
- `Write(spreadsheetID, writeRange, values)` — перезапись, `RAW`.
- `parseUpdatedRange(r)` — разбор `Sheet!A1:A5` в `(start, end)`.

### ChatGPT-сервис (`pkg/client/chatgptsrv/chatgptsrv.go`)

JSON-RPC 2.0 клиент (сгенерирован `rpcgen v2.5.0`):

- `NewClient(endpoint, httpClient)` — таймаут 30s по умолчанию.
- `Send(ctx, request string) (string, error)` — единственный RPC-метод `chatgpt.Send`. Запрос/ответ обёрнуты в JSON-RPC, `X-Request-ID` берётся из `appkit` контекста.

URL берётся из `cfg.Service.ChatGPTSrvURL`.

---

## 9. БД (`pkg/db/`)

`go-pg/v10`, модели генерируются `mfd-generator` из `docs/model/tradebot.mfd`. Не редактируй руками.

### Ключевые таблицы

| Таблица | Поля |
|---------|------|
| `cabinets` | id, name, clientId, key, marketplace, type, sheetLink, statusId, **settings** (JSON `CabinetSettings`) |
| `orders` | id, postingNumber, article, count, cabinetId, createdAt, statusId |
| `stocks` | id, article, updatedAt, countFbo, countFbs, cabinetId |
| `users` | id, tgId, isAdmin, cabinetIds[], statusId, login, password, authKey, createdAt, lastActivityAt |
| `reviews` | id, cabinetId, externalId, text, pros, cons, valuation, answer, article, createdAt, statusId, customerName |
| `products` | id, cabinetId, article, title, externalId, description, recommendationIds[], statusId |
| `vfsFiles`, `vfsFolders`, `vfsHashes` | модуль VFS (если включен `Server.EnableVFS`) |

### `CabinetSettings` (`pkg/db/model_params.go`)

```go
type CabinetSettings struct {
    ShipmentsSheetID              string   `json:"shipmentsSheetId"`
    ShipmentsAllSheetID           string   `json:"shipmentsAllSheetId,omitempty"`
    ExcludedShipmentsWarehouseIDs []string `json:"excludedShipmentsWarehouseIds,omitempty"`
}
```

- `ShipmentsSheetID` — таблица для основных дневных листов отгрузок (`Отправлено на ...`).
- `ShipmentsAllSheetID` — таблица для агрегированных листов `Все отгрузки ...`. Если пусто — соответствующий `*ShipmentsAll` cron пропускает кабинет.
- `ExcludedShipmentsWarehouseIDs` — ID складов, которые **не** попадают в агрегированный лист (основной лист их включает).

### Репозиторий

`pkg/db/tradebot.go`:

- `NewTradebotRepo(db orm.DB)` — фильтры по статусу включены по умолчанию.
- `WithTransaction(tx)`, `WithEnabledOnly()` — переопределение фильтров.
- `CabinetByID`, `OneCabinet`, `CabinetsByFilters` и т.п.

---

## 10. Сборка, запуск, кодген

```bash
make build          # бинарь, CGO_ENABLED=0
make run            # cfg/local.toml, dev=true
make mod            # go mod tidy + vendor

make test           # все тесты с coverage
make test-short     # без Database-тегов

make lint           # golangci-lint
make fmt            # auto-fmt

make db             # пересоздать БД tradebot + миграции

make generate       # RPC + VT codegen
make mfd-model      # модели из docs/model/tradebot.mfd
make mfd-repo       # репозитории
make mfd-vt-rpc     # VT RPC
```

---

## 11. Сводка фич (TL;DR)

**Для пользователя бота:**

- Печать FBS-этикеток WB / Ozon / Yandex.
- Анализ остатков WB и Ozon (Excel).
- Анализ возвратов в ПВЗ WB (Excel).
- Авто-ответы на отзывы WB через ChatGPT с возможностью редактировать/удалить.
- Управление API-ключами и Google Sheet-ссылками кабинетов прямо из бота.

**Автоматически по cron:**

- Заливка заказов и возвратов в Sheets раз в сутки (WB/Ozon/Yandex).
- Заливка отгрузок за вчера в Sheets (`Отправлено на ... FBS-N`).
- Заливка всех заказов за 24 часа с отсечкой 17:00 MSK (cron запускается в 18:00 MSK — час дельты на синхронизацию маркетплейсов) в общий лист `Все отгрузки ...` с автоматическим выкидыванием отменённых неотгруженных (Ozon — по колонке `Статус`, WB — по `wbStatus`).
- Чистка старых заказов из БД.
- Минутная отправка новых отзывов WB в Telegram-чат отзывов.

**Наблюдаемость:**

- `/metrics` (Prometheus) — HTTP + db pool + cron-job метрики.
- `/status` — health-check.
- `/debug/cron` — UI cron-менеджера.
- `/debug/pprof/*` — профили.
- Sentry для паник.
- `embedlog` + JSON-логи.
