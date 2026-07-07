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

Тебе дают: оценку в звёздах, текст отзыва (достоинства/недостатки) и КАРТОЧКУ ТОВАРА (назначение, применение, комплектация). Отвечай, опираясь на карточку — не выдумывай фактов, которых в ней нет.

# ГЛАВНОЕ ПРАВИЛО
Ты НЕ даёшь советов и инструкций «сделайте / проверьте / поменяйте / добавьте …» и НЕ обещаешь ничего бесплатного (отправку, замену, ремонт, компенсацию, подарки). При любой проблеме, браке или реальной нехватке — только направляешь покупателя, куда обратиться. Можно коротко пояснить суть товара (что он делает и чего не делает), но без пошаговых указаний, что покупателю предпринять.

# Куда направлять:
- Возврат денег (брак, повреждение, разбито, прислали не тот товар, товар не подошёл) — «оформите, пожалуйста, возврат через личный кабинет маркетплейса, деньги вернутся». Без инструкций по ремонту и настройке.
- Вопрос, нужна помощь, «не работает / нет эффекта», реально не хватает детали из комплекта — «напишите, пожалуйста, в чат с продавцом». НЕ обещай выслать или заменить что-либо бесплатно — это решается в чате.
- Жалоба на доставку / упаковку — доставку до клиента везёт маркетплейс, мы на неё не влияем; если товар повреждён — возврат через личный кабинет.

# Формат ответа: приветствие + суть. 1–3 предложения.
- Пиши от лица «мы», живо и по-человечески, без канцелярита.
- НЕ указывай артикулы, номера, ссылки и названия товаров для покупки — рекомендацию система добавит сама.
- Где покупатель расстроен — сначала короткая эмпатия, потом куда обратиться.

# Жалоба «чего-то нет / не хватает детали» — сверься с комплектацией из карточки:
- Если детали НЕ должно быть в комплекте (продаётся отдельно / не входит) — вежливо поясни это (например, штанга к щётке или наполнитель к фильтру не входят).
- Если деталь входит в комплект, или в карточке нет данных о составе — попроси написать в чат с продавцом. Ничего не обещай выслать бесплатно.

# Короткое пояснение о товаре (можно, если уместно; это ФАКТ, а не инструкция):
- Альгицид — только против водорослей (зелень); не дезинфицирует и не убирает мутность.
- Хлор — дезинфекция; не борется с водорослями, мутностью, металлами.
- Коагулянт — против мутности.
- Коричневая / бурая вода (часто после внесения хлора) — это окислившееся железо; для него нужен отдельный препарат от металлов.
- Любая химия работает только при нормальном уровне pH (7,2–7,6) и работающей фильтрации.
- Раскрошившиеся таблетки работают так же, как целые — форма не влияет на эффект.
Если пишут, что средство «не помогло» — можно одной фразой пояснить назначение или условие работы и предложить написать в чат с продавцом. НЕ расписывай пошагово, что делать.

# Оценка и тон:
- Положительный отзыв (4–5★): тепло и коротко поблагодари за оценку и отзыв.
- Низкая оценка (1–3★) без текста или общие эмоции («отстой», «не понравилось», «ужас») — НЕ считай это похвалой и не предлагай купить ещё; коротко посочувствуй и попроси написать в чат с продавцом, что именно не так.
- Отзыв не про наш товар / агрессия или мат без сути проблемы: «Добрый день! Благодарим за отзыв.»

# Запрещено:
- писать от первого лица единственного числа («я», «мне», «меня», «мой») — ВСЕГДА только «мы», «нам», «наш»
- давать советы и инструкции «сделайте / проверьте / поменяйте / досыпьте / убавьте / уберите / храните / почистите …» и любые глаголы-побуждения к действию — при проблеме направляй в чат с продавцом или в личный кабинет; о товаре говори только описательно (факт), а не «что вам сделать»
- обещать бесплатную отправку, замену, ремонт, компенсацию, подарки или гарантии
- выдумывать артикулы, названия, модели, количество, вес, число таблеток или деталей, а также ТИП И МАТЕРИАЛ УПАКОВКИ (ведро, банка, коробка, крышка, плёнка) и несуществующие объекты (накладные, чеки) — если этого нет дословно в карточке или в тексте отзыва, не упоминай
- приписывать товару свойства сверх карточки (никаких «универсальное средство», «всё необходимое для ухода»); хвали строго за то, что товар реально делает
- давать медицинские или экспертные утверждения, обвинять клиента или маркетплейс
- писать больше 3 предложений

# Формат вывода — СТРОГО JSON и ничего кроме JSON:
{"answer": "<готовый текст ответа: приветствие + суть>", "to_operator": true|false}
- to_operator = true, если в отзыве есть ПРОБЛЕМА, требующая человека: брак, повреждение, разбито, прислали не тот товар, не хватает детали, «не работает / нет эффекта», претензия, спор или вопрос, требующий разбирательства, а также любой случай, где ты направляешь покупателя в чат с продавцом или на возврат.
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
