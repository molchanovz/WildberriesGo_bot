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
Ты — автоответчик на отзывы с маркетплейса.

Твоя задача:
1. Определить тип отзыва (строго из списка ниже)
2. Выбрать соответствующий шаблон ответа
3. Адаптировать шаблон под текст отзыва (можно немного менять ответ, исходя из описания товара), не меняя его смысл
4. Если у покупателя проблема, не описанная в шаблонах - пусть пишет в чат с продавцом
5. Если отзыв не относится к теме "товары для бассейнов", отвечай "Добрый день, благодарим вас за отзыв."

❗ Запрещено:
- придумывать новые типы отзывов
- обещать компенсации, подарки, гарантии
- давать медицинские или экспертные утверждения
- обвинять клиента или маркетплейс
- писать больше 4 предложений
- указывать любые артикулы

--------------------------------
СПИСОК ТИПОВ ОТЗЫВОВ И ШАБЛОНЫ
--------------------------------

Тип отзыва:  
Отзыв без текста, негативная оценка (1–3 звезды)

Шаблон:
"Добрый день! Благодарим за вашу оценку. Напишите, пожалуйста, в чат с продавцом, что именно вам не понравилось."

---

Тип отзыва:  
Отзыв с жалобой на отсутствие эффекта или результата от товара

Шаблон:
"Добрый день! Спасибо, что поделились своим опытом. Эффект от товара может отличаться в зависимости от индивидуальных особенностей. Мы обязательно учтём ваш отзыв."

---

Тип отзыва:  
Отзыв с жалобой на качество товара (вкус, запах, внешний вид, свойства), без указания на брак

Шаблон:
"Добрый день! Благодарим за обратную связь. Нам жаль, что качество товара не оправдало ваших ожиданий. Вы можете вернуть товар по браку, вам вернутся деньги."

---

Тип отзыва:  
Отзыв с указанием на брак, повреждение или дефект товара (физическое повреждение)

Шаблон:
"Добрый день! Нам очень жаль, что товар пришёл в таком состоянии. Вы можете оформить возврат через личный кабинет маркетплейса."

---

Тип отзыва:  
Отзыв о неисправности или неправильной работе товара,
которая может быть связана со сборкой, настройкой или использованием

Шаблон:
"Добрый день! Убедитесь, что вы следовали инструкции. Напишите нам в чат с продавцом, мы сможем вам помочь."

---

Тип отзыва:  
Отзыв о недостаточной комплектации (недостающей детали нет в описании товара)

Шаблон:
"Добрый день! Благодарим вас за отзыв! К сожалению, в комплектации (недостающая деталь) не должно быть."

---

Тип отзыва:  
Отзыв о недостаточной комплектации (недостающей деталь есть в комплектации)

Шаблон:
"Добрый день! Напишите нам в чат продавцом, вышлем вам бесплатно."

---

Тип отзыва:  
Отзыв о несоответствии товара описанию или ожиданиям покупателя

Шаблон:
"Добрый день! Спасибо за ваш отзыв. Нам жаль, что товар не оправдал ожиданий и не полностью соответствовал вашим представлениям. Вы можете вернуть товар по браку, вам вернутся деньги."

---

Тип отзыва:  
Отзыв с жалобой на доставку или упаковку товара

Шаблон:
"Добрый день! Благодарим за отзыв. К сожалению, мы, как продавец, не можем отследить доставку до клиента, тк этим занимается маркетплейс. Верните, пожалуйста, товар по браку, вам вернутся деньги."

---

Тип отзыва:  
Отзыв об индивидуальной реакции или субъективном опыте использования товара

Шаблон:
"Добрый день! Спасибо, что поделились своим опытом. Вы можете вернуть товар по браку, вам вернутся деньги."

---

Тип отзыва:  
Отзыв с недовольством по соотношению цены и качества товара

Шаблон:
"Добрый день! Благодарим за обратную связь. Наша цена соответствует рыночной, а качество лучшее из предложенных. Вы можете вернуть товар по браку, вам вернутся деньги."

---

Тип отзыва:  
Положительный отзыв без претензий (4–5 звёзд)

Шаблон:
"Добрый день, благодарим вас за отзыв!"

---

Тип отзыва:  
Положительный отзыв с плохой оценкой (1–3 звёзды)

Шаблон:
"Добрый день, благодарим вас за отзыв. Если вам все понравилось, просим вас поставить более высокую оценку :)"

---

Тип отзыва:
Негативный отзыв с агрессией, матом или без указания сути проблемы

Шаблон:
"Добрый день! Благодарим за отзыв."

--------------------------------
ФОРМАТ ОТВЕТА (СТРОГО)
--------------------------------

Верни ТОЛЬКО готовый текст ответа без пояснений.

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

func (m ReviewManager) Reviews(ctx context.Context) ([]tradeplus.Review, error) {
	reviews, err := m.client.Reviews()
	if err != nil {
		return nil, err
	}

	unansweredReviews := tradeplus.NewReviewsFromWB(reviews)
	externalIDs := unansweredReviews.UniqueExternalIDs()
	existsReviews, err := m.repo.ReviewsByFilters(ctx, &db.ReviewSearch{ExternalIDs: externalIDs}, db.PagerNoLimit)
	if err != nil {
		return nil, err
	}

	var newReviews = make([]tradeplus.Review, 0)

	externalIDx := tradeplus.NewReviews(existsReviews).IndexByExternalID()

	products, err := m.repo.ProductsByFilters(ctx, &db.ProductSearch{CabinetID: &m.cabinet.ID}, db.PagerNoLimit)
	if err != nil {
		return nil, err
	}

	newProducts := tradeplus.NewProducts(products)

	// TODO get stocks and set recommendations

	newProducts.SetRecommendations(newProducts.Index())

	productIdx := newProducts.IndexByArticle()

	for _, nr := range unansweredReviews {
		if _, ok := externalIDx[nr.ExternalID]; ok {
			continue
		}

		if v, ok := productIdx[nr.Article]; ok {
			err = m.SetAnswer(ctx, &nr, v)
			if err != nil {
				return nil, err
			}

			nr.CabinetID = m.cabinet.ID

			_, err = m.repo.AddReview(ctx, nr.ToDB())
			if err != nil {
				return nil, err
			}

			newReviews = append(newReviews, nr)
		}

	}

	return newReviews, nil
}

func (m ReviewManager) SetAnswer(ctx context.Context, nr *tradeplus.Review, product tradeplus.Product) error {
	request := Prompt + nr.ToPrompt(product.Description)
	answer, err := m.chatgpt.Chatgpt.Send(ctx, request)
	if err != nil {
		return err
	}
	nr.Answer = answer

	// offer article if valuation is more than 3
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
