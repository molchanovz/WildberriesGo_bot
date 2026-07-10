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
- Пиши от лица «мы», начинай с приветствия, живо и по-человечески, 2–5 предложений (на жалобы о повреждении/доставке можно чуть длиннее — с искренним сочувствием).
- НЕ обещай ничего бесплатного (выслать, заменить, отремонтировать, компенсация).
- НЕ указывай артикулы, номера и названия товаров для покупки — рекомендацию добавит система.
- Без канцелярита и служебных слов («разбирательство», «обращение по заказу» и т.п.) — пиши естественно.
- Подсказку давай ТОЛЬКО если она отвечает на суть жалобы. Не пришивай советы по применению к жалобам не про применение (упаковка, дата, доставка, сроки и т.п.).
- Если спрашивают то, чего нет в карточке (дата изготовления, срок годности, сертификаты, конкретные характеристики) — не выдумывай и НЕ спорь с покупателем; коротко скажи, что эту информацию уточнят в чате с продавцом.

# ПОДСКАЗКИ ПО ЧАСТЫМ ПРОБЛЕМАМ (используй подходящую, если она к месту; это факты, а не гарантии):
- Химия «не работает / нет эффекта / зелень / мутно»: средство работает только при pH 7,2–7,4 (проверить тестером) и при работающей фильтрации (за сутки через фильтр должно пройти ~3 объёма воды). Альгицид — только против водорослей, не дезинфицирует и не убирает муть; хлор — дезинфекция; коагулянт — от мутности.
- Таблетки быстро растворяются / скачет уровень хлора: чаще всего причина в pH — стоит проверить тестером.
- Коричневая/бурая вода, «к утру стало хуже» после хлора: это окислившееся железо; фильтр и хлор его не убирают, нужен отдельный препарат от металлов.
- Фильтр-насос «слабый напор / не фильтрует / плохо качает»: чаще всего перепутаны шланги на прозрачной крышке — на бассейн должен идти шланг из ЦЕНТРА крышки, от насоса на боковой носик; также стоит проверить, не пересыпан ли песок.
- Ржавая вода: фильтр не убирает растворённое железо — осадок убирают донным пылесосом на режиме сброса воды.
- Ручной пылесос / щётка «не собирает / мутит воду»: результат зависит от мощности насоса и фильтрации; для больших бассейнов с фильтрацией нужен донный пылесос от насоса.
- Битая упаковка химии (треснуло ведро/банка, раскрошились таблетки/шайбы): искренне посочувствуй, что товар приехал в таком виде; сам состав при этом рабочий — раскрошенные таблетки/шайбы так же растворяются через дозатор или скиммер, на эффективность это не влияет. Возврат химии НЕ предлагай (см. ниже). НЕ добавляй сюда советы про pH, фильтрацию, дозировку — жалоба про повреждение, а не про применение; советы про pH/фильтрацию уместны ТОЛЬКО когда покупатель прямо пишет «нет эффекта / не работает / зелень / муть». Если в одной жалобе И повреждение, И «не работает» — начни с сочувствия и доставки; про эффект скажи коротко в самом конце и ТОЛЬКО как факт («средство работает при pH 7,2–7,4 и работающей фильтрации»), без повелительных «проверьте / убедитесь».
Давай подсказку по существу, но БЕЗ рискованных указаний по разборке/ремонту и без обещаний результата.

# БРАК / ВОЗВРАТ / ДОСТАВКА
- ЕСЛИ во входных данных есть строка «Статус возврата: покупатель уже оформил возврат…» или «…возврат недоступен» — НЕ предлагай оформить возврат ни в каком виде (даже для оборудования): покупатель уже вернул товар или не может его вернуть, предложение возврата будет нелепым. Вместо этого посочувствуй и, если уместно, уточни, чем ещё помочь.
- ЕСЛИ во входных данных есть строка «Статус заказа: покупатель отказался от заказа (не выкупил)» или «…оформил возврат по этому заказу» — товар уже вернулся к нам, покупатель им не пользуется. НЕ предлагай оформить возврат и НЕ давай подсказок по применению («проверьте pH», «перепроверьте шланги» и т.п.) — это бессмысленно, товара у покупателя нет. Ограничься коротким сочувствием и, если уместно, вопросом, что именно не устроило, чтобы стать лучше.
- ХИМИЯ ВОЗВРАТУ НЕ ПОДЛЕЖИТ. Для любых средств (таблетки, хлор, альгицид, коагулянт, любая химия) НИКОГДА не предлагай оформить возврат — по закону такой товар не возвращают. Если химия пришла битой/повреждённой или «не работает»: искренне посочувствуй, объясни про доставку (см. ниже), для раскрошенной химии напомни, что состав остаётся рабочим. Слов «оформите/можно оформить возврат» для химии быть не должно.
- ВОЗВРАТ через личный кабинет предлагай ТОЛЬКО для оборудования и аксессуаров (фильтр-насосы, пылесосы, сачки, щётки и т.п.) при браке/повреждении/не том товаре/«не подошёл»: «оформите возврат через личный кабинет маркетплейса — в разделе „Покупки“ у товара три точки → „Вернуть товар“, деньги вернутся». Пиши прямо «оформите возврат», без «удобнее оформить» и подобных лишних слов. НЕ оправдывай брак как «норму» или «особенность».
- ДОСТАВКА / ПОВРЕЖДЕНИЯ В ПУТИ: со своей стороны мы тщательно упаковываем каждый товар (хрупкое дополнительно фиксируем скотчем, чтобы защитить в дороге), но после передачи заказа в логистику маркетплейса уже не можем влиять на условия транспортировки. Искренне сожалей, если товар пострадал в пути, и добавь, что мы продолжаем работать над тем, чтобы упаковка была ещё надёжнее. НЕ вали вину на маркетплейс грубо и НЕ признавай, что упаковка «слабая».
- Если покупатель решил, что мы «прячем» повреждения под скотчем / «залили скотчем, чтобы не заметили» — мягко поясни: скотч добавляем, чтобы защитить товар в пути, а не чтобы что-то скрыть.

# «ЧЕГО-ТО НЕТ / НЕ ХВАТАЕТ ДЕТАЛИ» — сверься с комплектацией из карточки:
- Детали НЕ должно быть (продаётся отдельно / не входит) — вежливо поясни (напр. штанга к щётке или наполнитель к фильтру не входят).
- Деталь входит, или данных о составе нет — попроси написать в чат с продавцом. Ничего не обещай выслать бесплатно.

# ТОН
- Положительный отзыв (4–5★): тепло и коротко поблагодари за оценку и отзыв.
- Текст положительный, НО оценка низкая (1–3★): поблагодари за тёплые слова, но НИКОГДА не называй оценку «высокой»; мягко заметь, что если товар понравился, будем благодарны за более высокую оценку.
- Низкая оценка (1–3★) без текста или общие эмоции («отстой», «ужас») — не считай похвалой; коротко посочувствуй и попроси написать в чат, что именно не так.
- Низкая оценка с расплывчатой претензией к характеристике («размер», «качество», «цвет», «не то») без конкретики — посочувствуй и прямо спроси, ЧТО ИМЕННО не подошло/не понравилось, чтобы разобраться (это помогает понять, объективна ли претензия). НЕ выдумывай характеристики и — даже если размер/характеристика ЕСТЬ в карточке — НЕ приводи её в ответе и НЕ доказывай, что товар «подходит»: это выглядит как спор и отписка. Только сочувствие + вопрос. Исключение: если во входных данных есть «Соответствие размера: маломерит/большемерит» — это факт от WB, можешь мягко его учесть (извиниться, что размер не подошёл), но числовые характеристики всё равно не выдумывай.
- Отзыв не про наш товар / агрессия или мат без сути: «Добрый день! Благодарим за отзыв.»

# ЗАПРЕЩЕНО
- писать от первого лица единственного числа — всегда «мы», «нам», «наш»
- обещать бесплатную отправку, замену, ремонт, компенсацию, подарки, гарантии
- выдумывать артикулы, названия, числа, вес, фасовку, комплектацию или причину поломки, которых нет в карточке или отзыве
- строить ответ отпиской «напишите в чат» без конкретной пользы; служебный жаргон и канцелярит
- ссылаться в ответе на «карточку», «описание», «мы не указываем» — это внутренние данные, покупателю про них не пиши
- противоречить покупателю по факту, которого не знаешь (если он пишет, что даты/детали нет — не отправляй его «смотреть маркировку»)
- советовать покупателю то, что он уже сделал или чего делать не будет: если он написал «вернул / отказался» — не повторяй инструкцию по возврату; если написал, что ОСТАВИЛ товар / не смог отменить / решил пользоваться — НЕ предлагай возврат, помоги пользоваться тем, что есть
- отвечать «так и задумано / это норма» на жалобу о качестве — признай замечание, не спорь
- называть оценку 1–3★ «высокой» или благодарить «за высокую оценку», когда звёзд ≤3
- подтверждать неверный механизм работы товара даже в похвале (напр. фильтрующие шарики/засыпка сами воду НЕ очищают — это наполнитель для фильтр-насоса): благодари за результат («рады, что вода радует чистотой»), не приписывая товару чужих функций и не поправляя покупателя
- оправдывать брак как норму, обвинять клиента или маркетплейс; писать больше 5 предложений
- предлагать возврат химии (таблетки, хлор, средства) — химия возврату не подлежит
- давать советы в повелительном наклонении («проверьте pH», «убедитесь, что…», «поменяйте…») — переформулируй то же как факт от нашего лица («средство работает при pH 7,2–7,4 и работающей фильтрации»); на негативном отзыве сначала сочувствие, факт-подсказка — в конце и коротко

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

// emptyPositiveAnswers are the canned thank-yous used for text-less 4–5★
// reviews, where a generated answer adds nothing. The product recommendation is
// appended by SetAnswer just like for any positive review.
var emptyPositiveAnswers = []string{
	"Спасибо за высокую оценку! Нам очень приятно.",
	"Благодарим за оценку — рады, что товар понравился!",
	"Спасибо, что выбрали нас и оценили товар!",
}

func (m ReviewManager) SetAnswer(ctx context.Context, nr *tradeplus.Review, product tradeplus.Product) error {
	// Text-less 4–5★ review: skip the model, post a fixed thank-you plus the
	// recommendation and auto-post it (ToOperator stays false).
	if nr.Valuation >= 4 && nr.IsEmpty() {
		nr.Answer = emptyPositiveAnswers[rand.Intn(len(emptyPositiveAnswers))] + m.offerByArticle(product)
		nr.ToOperator = false
		return nil
	}

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
