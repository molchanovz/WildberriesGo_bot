package wb

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"tradebot/pkg/client/chatgptsrv"
	"tradebot/pkg/client/wb"
	"tradebot/pkg/db"
	"tradebot/pkg/tradeplus"
)

const Prompt = `
Ты — автоответчик компании. Твоя задача — кратко и вежливо отвечать на отзывы покупателей.

Требования:
1. Обязательно поблагодари за отзыв в начале.
2. Пиши от лица компании (мы/нас).
3. Ответ <= 150 символов. Жёстко соблюдай.
4. Никакой "воды" — только по делу.
5. Запрещено рекомендовать другие товары.
6. Запрещено указывать или описывать товар, если покупатель сам его не назвал.
7. Всегда пиши с заглавной буквы
`

const ArticlesPath = "assets/articles.json"

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

	var articleTextMap map[string]string

	err = loadArticles(&articleTextMap)
	if err != nil {
		return nil, err
	}

	for _, nr := range unansweredReviews {
		if _, ok := externalIDx[nr.ExternalID]; ok {
			continue
		}
		var answer string
		if !nr.IsEmpty() {
			request := Prompt + nr.ToPrompt()
			answer, err = m.chatgpt.Chatgpt.Send(ctx, request)
			if err != nil {
				return nil, err
			}
		}

		nr.Answer = answer + offerByArticle(articleTextMap, nr.Article)
		nr.CabinetID = m.cabinet.ID

		_, err = m.repo.AddReview(ctx, nr.ToDB())
		if err != nil {
			return nil, err
		}

		newReviews = append(newReviews, nr)
	}

	return newReviews, nil
}

func offerByArticle(m map[string]string, article string) string {
	if v, ok := m[article]; ok {
		return " " + v
	}
	return ""
}

func loadArticles(m *map[string]string) error {
	data, err := os.ReadFile(ArticlesPath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &m)
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
