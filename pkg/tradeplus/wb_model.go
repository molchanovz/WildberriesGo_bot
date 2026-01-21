package tradeplus

import (
	"strings"
	"text/template"
	"time"
	wbc "tradebot/pkg/client/wb"
	"tradebot/pkg/db"
)

type Review struct {
	db.Review
	ArticleDescription *string
}

func NewReview(in *db.Review) *Review {
	if in == nil {
		return nil
	}

	return &Review{
		Review: *in,
	}
}

type ArticleInfo struct {
	Description    string `json:"description"`
	Recommendation string `json:"recommendation"`
}

func (r Review) Stars() string {
	if r.Valuation <= 0 {
		return "0"
	}
	stars := strings.Repeat("★", r.Valuation)
	emptyStars := strings.Repeat("☆", 5-r.Valuation)
	return stars + emptyStars
}

func (r Review) ToMessage() string {
	reviewTemplate := `Отзыв на <b>{{.Article}}</b> на {{.Stars}}.` + "\n" +
		`{{if .CustomerName}}<b>Покупатель</b>: {{.CustomerName}}` + "\n" +
		`{{end}}{{if .Pros}}<b>Достоинства</b>: {{.Pros}}` + "\n" +
		`{{end}}{{if .Cons}}<b>Недостатки</b>: {{.Cons}}` + "\n" +
		`{{end}}{{if .Text}}<b>Отзыв</b>: {{.Text}}` + "\n" +
		`{{end}}{{if .Answer}}<b>Ответ</b>: <pre>{{.Answer}}</pre>{{end}}`

	tmpl := template.Must(template.New("review").Parse(reviewTemplate))

	var sb strings.Builder
	err := tmpl.Execute(&sb, r)
	if err != nil {
		return "Ошибка формирования отзыва"
	}

	result := sb.String()
	return strings.TrimSpace(result)
}

func (r Review) ToPrompt(description *string) string {
	// add description for LLM
	r.ArticleDescription = description

	promptTemplate := `Отзыв на {{.Article}} на {{.Valuation}} звезд.
	{{if .ArticleDescription}}Описание товара: {{.ArticleDescription}}
	{{end}}{{if .CustomerName}}Покупатель: {{.CustomerName}}
	{{end}}{{if .Pros}}Достоинства: {{.Pros}}
	{{end}}{{if .Cons}}Недостатки: {{.Cons}}
	{{end}}{{if .Text}}Отзыв: {{.Text}}
	{{end}}{{if .Answer}}Ответ: {{.Answer}}{{end}}`

	tmpl := template.Must(template.New("review").Parse(promptTemplate))

	var sb strings.Builder
	err := tmpl.Execute(&sb, r)
	if err != nil {
		return "Ошибка формирования отзыва"
	}

	result := sb.String()
	return strings.TrimSpace(result)
}

func (r Review) ToDB() *db.Review {
	return &db.Review{
		ID:           r.ID,
		CabinetID:    r.CabinetID,
		ExternalID:   r.ExternalID,
		Text:         r.Text,
		Pros:         r.Pros,
		Cons:         r.Cons,
		Valuation:    r.Valuation,
		Answer:       r.Answer,
		Article:      r.Article,
		CreatedAt:    r.CreatedAt,
		StatusID:     r.StatusID,
		CustomerName: r.CustomerName,
	}
}

func NewReviewFromWB(in wbc.Feedback) Review {
	r := db.Review{
		ExternalID:   in.Id,
		Article:      in.ProductDetails.SupplierArticle,
		CustomerName: in.UserName,
		Text:         in.Text,
		Pros:         in.Pros,
		Cons:         in.Cons,
		Valuation:    in.ProductValuation,
	}

	if in.Bables != nil {
		r.Text += "\nПокупатель отметил: " + strings.Join(in.Bables, ", ")
	}

	return Review{
		Review: r,
	}
}

func NewReviewsFromWB(in *wbc.Review) Reviews {
	if in == nil {
		return nil
	}

	var reviews = make(Reviews, 0, len(in.Data.Feedbacks))
	for i := range in.Data.Feedbacks {
		review := NewReviewFromWB(in.Data.Feedbacks[i])
		review.StatusID = db.ReviewStatusCompleted
		reviews = append(reviews, review)
	}
	return reviews
}

type ReviewWB struct {
	wbc.Review
}

type Card struct {
	NmID        int
	ImtID       int
	NmUUID      string
	SubjectID   int
	SubjectName string
	VendorCode  string
	Brand       string
	Title       string
	Description string
	NeedKiz     bool
	Dimensions  struct {
		Width        int
		Height       int
		Length       int
		WeightBrutto float64
		IsValid      bool
	}
	Characteristics []struct {
		Id    int
		Name  string
		Value interface{}
	}
	Sizes []struct {
		ChrtID   int
		TechSize string
		WbSize   string
		Skus     []string
	}
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewCardList(in *wbc.CardList) Cards {
	if in == nil {
		return nil
	}

	cards := make([]Card, 0, len(in.Cards))
	for i := range in.Cards {
		c := Card{
			NmID:            in.Cards[i].NmID,
			ImtID:           in.Cards[i].ImtID,
			NmUUID:          in.Cards[i].NmUUID,
			SubjectID:       in.Cards[i].SubjectID,
			SubjectName:     in.Cards[i].SubjectName,
			VendorCode:      in.Cards[i].VendorCode,
			Brand:           in.Cards[i].Brand,
			Title:           in.Cards[i].Title,
			Description:     in.Cards[i].Description,
			NeedKiz:         in.Cards[i].NeedKiz,
			Characteristics: nil,
			Sizes:           nil,
			CreatedAt:       time.Time{},
			UpdatedAt:       time.Time{},
		}
		c.Dimensions.Height = in.Cards[i].Dimensions.Height
		c.Dimensions.Length = in.Cards[i].Dimensions.Length
		c.Dimensions.Width = in.Cards[i].Dimensions.Width
		c.Dimensions.IsValid = in.Cards[i].Dimensions.IsValid
		c.Dimensions.WeightBrutto = in.Cards[i].Dimensions.WeightBrutto

		cards = append(cards, c)
	}

	return cards
}

type Return struct {
	Barcode          string `json:"barcode"`
	Brand            string `json:"brand"`
	CompletedDt      string `json:"completedDt"`
	DstOfficeAddress string `json:"dstOfficeAddress"`
	DstOfficeId      int    `json:"dstOfficeId"`
	ExpiredDt        string `json:"expiredDt"`
	IsStatusActive   int    `json:"isStatusActive"`
	NmId             int    `json:"nmId"`
	OrderDt          string `json:"orderDt"`
	OrderId          int    `json:"orderId"`
	ReadyToReturnDt  string `json:"readyToReturnDt"`
	Reason           string `json:"reason"`
	ReturnType       string `json:"returnType"`
	ShkId            int64  `json:"shkId"`
	Srid             string `json:"srid"`
	Status           string `json:"status"`
	StickerId        string `json:"stickerId"`
	SubjectName      string `json:"subjectName"`
	TechSize         string `json:"techSize"`
}

func NewReturns(in *wbc.ReturnList) []Return {
	if in == nil {
		return nil
	}

	returns := make([]Return, 0, len(in.Report))
	for i := range in.Report {
		returns = append(returns, Return{
			Barcode:          in.Report[i].Barcode,
			Brand:            in.Report[i].Brand,
			CompletedDt:      in.Report[i].CompletedDt,
			DstOfficeAddress: in.Report[i].DstOfficeAddress,
			DstOfficeId:      in.Report[i].DstOfficeId,
			ExpiredDt:        in.Report[i].ExpiredDt,
			IsStatusActive:   in.Report[i].IsStatusActive,
			NmId:             in.Report[i].NmId,
			OrderDt:          in.Report[i].OrderDt,
			OrderId:          in.Report[i].OrderId,
			ReadyToReturnDt:  in.Report[i].ReadyToReturnDt,
			Reason:           in.Report[i].Reason,
			ReturnType:       in.Report[i].ReturnType,
			ShkId:            in.Report[i].ShkId,
			Srid:             in.Report[i].Srid,
			Status:           in.Report[i].Status,
			StickerId:        in.Report[i].StickerId,
			SubjectName:      in.Report[i].SubjectName,
			TechSize:         in.Report[i].TechSize,
		})
	}

	return returns
}
