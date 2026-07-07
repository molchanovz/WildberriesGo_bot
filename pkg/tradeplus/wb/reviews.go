package wb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"

	"tradebot/pkg/client/chatgptsrv"
	"tradebot/pkg/client/wb"
	"tradebot/pkg/db"
	"tradebot/pkg/tradeplus"
)

const Prompt = `
Ты — автоответчик на отзывы покупателей магазина Vommy (товары для бассейнов: химия, фильтр-насосы, пылесосы, сачки, аксессуары) на маркетплейсе.

Тебе дают: оценку в звёздах, текст отзыва (достоинства/недостатки) и КАРТОЧКУ ТОВАРА (назначение, применение, комплектация). Опирайся на карточку — не выдумывай фактов, которых в ней нет.

# КАК ОТВЕЧАТЬ (главное)
Отвечай как знающий продавец, а не отпиской. Если у покупателя проблема с использованием товара — дай ОДНУ конкретную подсказку, которую он может проверить сам, и только потом, если не поможет, предложи написать в чат с продавцом. НЕ строй ответ по шаблону «сожалеем + пересказ состава + напишите в чат» — это звучит как робот. «Напишите в чат» — не основной ответ, а завершение после конкретной пользы.
- Пиши от лица «мы», начинай с приветствия, живо и по-человечески, 2–4 предложения.
- НЕ обещай ничего бесплатного (выслать, заменить, отремонтировать, компенсация).
- НЕ указывай артикулы, номера и названия товаров для покупки — рекомендацию добавит система.
- Без канцелярита и служебных слов («разбирательство», «обращение по заказу» и т.п.) — пиши естественно.

# ПОДСКАЗКИ ПО ЧАСТЫМ ПРОБЛЕМАМ (используй подходящую, если она к месту; это факты, а не гарантии):
- Химия «не работает / нет эффекта / зелень / мутно»: средство работает только при pH 7,2–7,4 (проверить тестером) и при работающей фильтрации (за сутки через фильтр должно пройти ~3 объёма воды). Альгицид — только против водорослей, не дезинфицирует и не убирает муть; хлор — дезинфекция; коагулянт — от мутности.
- Таблетки быстро растворяются / скачет уровень хлора: чаще всего причина в pH — стоит проверить тестером.
- Коричневая/бурая вода, «к утру стало хуже» после хлора: это окислившееся железо; фильтр и хлор его не убирают, нужен отдельный препарат от металлов.
- Фильтр-насос «слабый напор / не фильтрует / плохо качает»: чаще всего перепутаны шланги на прозрачной крышке — на бассейн должен идти шланг из ЦЕНТРА крышки, от насоса на боковой носик; также стоит проверить, не пересыпан ли песок.
- Ржавая вода: фильтр не убирает растворённое железо — осадок убирают донным пылесосом на режиме сброса воды.
- Ручной пылесос / щётка «не собирает / мутит воду»: результат зависит от мощности насоса и фильтрации; для больших бассейнов с фильтрацией нужен донный пылесос от насоса.
- Битая упаковка химии (треснуло ведро/банка, раскрошились таблетки/шайбы): сам состав рабочий — раскрошенные таблетки растворяются через дозатор/скиммер так же, при необходимости пересыпьте в сухую закрытую ёмкость. Если предпочитаете — можно оформить возврат через личный кабинет.
Давай подсказку по существу, но БЕЗ рискованных указаний по разборке/ремонту и без обещаний результата.

# БРАК / ВОЗВРАТ / ДОСТАВКА
- Брак, повреждение, разбито, не работает встроенная/несъёмная часть товара, прислали не тот товар, товар не подошёл → предложи оформить возврат через личный кабинет маркетплейса (в разделе «Покупки» → у товара три точки → «Вернуть товар»), деньги вернутся. НЕ оправдывай брак как «норму» или «особенность».
- Доставка/упаковка: доставку до клиента везёт маркетплейс, мы на неё не влияем; если товар повреждён — возврат через личный кабинет.

# «ЧЕГО-ТО НЕТ / НЕ ХВАТАЕТ ДЕТАЛИ» — сверься с комплектацией из карточки:
- Детали НЕ должно быть (продаётся отдельно / не входит) — вежливо поясни (напр. штанга к щётке или наполнитель к фильтру не входят).
- Деталь входит, или данных о составе нет — попроси написать в чат с продавцом. Ничего не обещай выслать бесплатно.

# ТОН
- Положительный отзыв (4–5★): тепло и коротко поблагодари за оценку и отзыв.
- Низкая оценка (1–3★) без текста или общие эмоции («отстой», «ужас») — не считай похвалой; коротко посочувствуй и попроси написать в чат, что именно не так.
- Отзыв не про наш товар / агрессия или мат без сути: «Добрый день! Благодарим за отзыв.»

# ЗАПРЕЩЕНО
- писать от первого лица единственного числа — всегда «мы», «нам», «наш»
- обещать бесплатную отправку, замену, ремонт, компенсацию, подарки, гарантии
- выдумывать артикулы, названия, числа, вес, фасовку, комплектацию или причину поломки, которых нет в карточке или отзыве
- строить ответ отпиской «напишите в чат» без конкретной пользы; служебный жаргон и канцелярит
- советовать покупателю то, что он уже сделал (если он написал «вернул / оформил возврат / отказался» — не повторяй инструкцию по возврату, просто коротко извинись)
- отвечать «так и задумано / это норма» на жалобу о качестве — признай замечание, не спорь
- оправдывать брак как норму, обвинять клиента или маркетплейс; писать больше 4 предложений

# ФОРМАТ ВЫВОДА — СТРОГО JSON и ничего кроме JSON:
{"answer": "<готовый текст ответа: приветствие + суть>", "to_operator": true|false}
- to_operator = true, если в отзыве есть проблема, брак, претензия, нехватка детали или «не работает» — то есть всё, что стоит показать человеку.
- to_operator = false, если это благодарность, положительный или нейтральный отзыв без проблемы (такой ответ отправится покупателю автоматически).

--------------------------------
ВХОДНЫЕ ДАННЫЕ
--------------------------------

`

const (
	ArticlesPath       = "assets/articles.json"
	Recommendation     = "Рекомендуем к покупке %s нашего производства, артикул %s. Вставьте его в поисковую строку маркетплейса."
	NullRecommendation = "Рекомендуем к покупке другие товары нашего производства, их вы найдете в нашем магазине."
)

type ReviewManager struct {
	dbc     db.DB
	repo    db.TradebotRepo
	client  wb.Client
	chatgpt *chatgptsrv.Client
	cabinet *tradeplus.Cabinet
}

func NewReviewManager(dbc db.DB, cabinet *tradeplus.Cabinet, chatgpt *chatgptsrv.Client) ReviewManager {
	return ReviewManager{
		dbc:     dbc,
		repo:    db.NewTradebotRepo(dbc),
		client:  wb.NewClient(cabinet.Key),
		chatgpt: chatgpt,
		cabinet: cabinet,
	}
}

// Fetch pulls ALL unanswered feedbacks from WB (paginated) and stores the ones
// not yet in the DB as Created. No model calls, no Telegram — pure ingest.
func (m ReviewManager) Fetch(ctx context.Context) error {
	reviews, err := m.client.Reviews()
	if err != nil {
		return err
	}

	all := tradeplus.NewReviewsFromWB(reviews)
	existsReviews, err := m.repo.ReviewsByFilters(ctx, &db.ReviewSearch{ExternalIDs: all.UniqueExternalIDs()}, db.PagerNoLimit)
	if err != nil {
		return err
	}
	externalIDx := tradeplus.NewReviews(existsReviews).IndexByExternalID()

	for _, nr := range all {
		if _, ok := externalIDx[nr.ExternalID]; ok {
			continue // already ingested
		}

		nr.CabinetID = m.cabinet.ID
		nr.StatusID = db.ReviewStatusCreated
		if _, err := m.repo.AddReview(ctx, nr.ToDB()); err != nil {
			return err
		}
	}

	return nil
}

// ProcessPending takes Created reviews from the DB, generates an answer only if
// one is not stored yet, then routes: positives are auto-posted to WB, problems
// (and every 1–3★) go to sendToOperator (Telegram). A review's status advances
// to Completed ONLY after it was successfully sent; otherwise it stays Created
// and is retried on the next run.
func (m ReviewManager) ProcessPending(ctx context.Context, sendToOperator func(context.Context, tradeplus.Review) error) error {
	created := db.ReviewStatusCreated
	pending, err := m.repo.ReviewsByFilters(ctx, &db.ReviewSearch{CabinetID: &m.cabinet.ID, StatusID: &created}, db.PagerNoLimit)
	if err != nil {
		return err
	}
	if len(pending) == 0 {
		return nil
	}

	products, err := m.repo.ProductsByFilters(ctx, &db.ProductSearch{CabinetID: &m.cabinet.ID}, db.PagerNoLimit)
	if err != nil {
		return err
	}
	newProducts := tradeplus.NewProducts(products)
	newProducts.SetRecommendations(newProducts.Index())
	productIdx := newProducts.IndexByArticle()

	for i := range pending {
		nr := tradeplus.NewReview(&pending[i])

		product, ok := productIdx[nr.Article]
		if !ok {
			continue // no product card yet — cannot answer, stays Created
		}

		// Generate only when there is no answer yet (idempotent, no wasted tokens).
		if nr.Answer == "" {
			if err := m.SetAnswer(ctx, nr, product); err != nil {
				return err
			}
			if _, err := m.repo.UpdateReview(ctx, &db.Review{ID: nr.ID, Answer: nr.Answer, ToOperator: nr.ToOperator},
				db.WithColumns(db.Columns.Review.Answer, db.Columns.Review.ToOperator)); err != nil {
				return err
			}
		}

		// Safety floor: any 1–3★ always goes to a human, regardless of the stored flag.
		toOperator := nr.ToOperator || nr.Valuation <= 3

		var sendErr error
		if toOperator {
			sendErr = sendToOperator(ctx, *nr)
		} else {
			sendErr = m.client.AnswerReview(nr.ExternalID, nr.Answer)
		}
		if sendErr != nil {
			// Keep Created so the next run retries; the answer is already saved.
			continue
		}

		if _, err := m.repo.UpdateReview(ctx, &db.Review{ID: nr.ID, StatusID: db.ReviewStatusCompleted},
			db.WithColumns(db.Columns.Review.StatusID)); err != nil {
			return err
		}
	}

	return nil
}

// gptReviewResponse is the JSON contract returned by the review prompt.
type gptReviewResponse struct {
	Answer     string `json:"answer"`
	ToOperator bool   `json:"to_operator"`
}

func (m ReviewManager) SetAnswer(ctx context.Context, nr *tradeplus.Review, product tradeplus.Product) error {
	request := Prompt + nr.ToPrompt(product.Description)
	raw, err := m.chatgpt.Chatgpt.Send(ctx, request)
	if err != nil {
		return err
	}

	var resp gptReviewResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil || resp.Answer == "" {
		// Malformed output — keep the raw text and route to a human, never auto-post on doubt.
		nr.Answer = raw
		nr.ToOperator = true
		return nil
	}

	nr.Answer = resp.Answer
	nr.ToOperator = resp.ToOperator

	// Safety floor: any low rating (1–3★) is always handled by a human,
	// regardless of the model's judgement. Only 4–5★ can be auto-posted.
	if nr.Valuation <= 3 {
		nr.ToOperator = true
	}

	if nr.Valuation > 3 {
		nr.Answer += m.offerByArticle(product)
	}
	return nil
}

func LoadArticlesInfo() (map[string]tradeplus.ArticleInfo, error) {
	var articlesInfo map[string]tradeplus.ArticleInfo

	data, err := os.ReadFile(ArticlesPath)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(data, &articlesInfo)
	if err != nil {
		return nil, err
	}

	return articlesInfo, nil
}

func (m ReviewManager) offerByArticle(product tradeplus.Product) string {
	var rc = len(product.Recommendations)
	if rc != 0 {
		r := product.Recommendations[rand.Intn(rc)]
		return " " + createRecommendation(r.Title, r.ExternalID)
	}

	return NullRecommendation
}

func createRecommendation(title, article string) string {
	return fmt.Sprintf(Recommendation, title, article)
}

func (m ReviewManager) AnswerReview(ctx context.Context, reviewId string) error {
	review, err := m.repo.OneReview(ctx, &db.ReviewSearch{ExternalID: &reviewId}, db.WithColumns(db.Columns.Review.Answer))
	if err != nil {
		return err
	} else if review == nil {
		return errors.New("review not found")
	}

	err = m.client.AnswerReview(reviewId, review.Answer)
	if err != nil {
		return err
	}

	return nil
}
