package tradeplus

import (
	"encoding/json"
	"strings"
	"text/template"
	"time"

	wbc "tradebot/pkg/client/wb"
	"tradebot/pkg/db"
)

// ReviewPayload is the jsonb shape stored in reviews.payload. It holds the
// review content plus WB-derived context that we feed the model, replacing the
// former text/pros/cons/customerName/photos columns. Add a field here (no
// migration needed) to feed more WB data to the answer generator.
type ReviewPayload struct {
	Text         string     `json:"text,omitempty"`
	Pros         string     `json:"pros,omitempty"`
	Cons         string     `json:"cons,omitempty"`
	CustomerName string     `json:"customerName,omitempty"`
	Photos       []string   `json:"photos,omitempty"`
	Bables       []string   `json:"bables,omitempty"`
	MatchingSize string     `json:"matchingSize,omitempty"` // normalized RU: "маломерит"/"большемерит"/""
	ReturnStatus string     `json:"returnStatus,omitempty"` // "available"/"returned"/"unavailable"/""
	OrderedAt    *time.Time `json:"orderedAt,omitempty"`
	ProductName  string     `json:"productName,omitempty"`
	SubjectName  string     `json:"subjectName,omitempty"`
	Color        string     `json:"color,omitempty"`
}

type Review struct {
	db.Review
	ArticleDescription *string

	// Decoded view of db.Review.Payload — the source of truth for display and
	// prompt building. Populated by NewReview / NewReviewFromWB.
	Text         string
	Pros         string
	Cons         string
	CustomerName string
	Photos       []string
	Bables       []string
	MatchingSize string
	ReturnStatus string
	OrderedAt    time.Time
	ProductName  string
	SubjectName  string
	Color        string
}

func NewReview(in *db.Review) *Review {
	if in == nil {
		return nil
	}

	r := &Review{Review: *in}
	r.decodePayload()
	return r
}

// decodePayload unpacks db.Review.Payload into the flat fields used by the
// templates. A missing/invalid payload leaves the fields zero-valued.
func (r *Review) decodePayload() {
	if len(r.Review.Payload) == 0 {
		return
	}
	var p ReviewPayload
	if err := json.Unmarshal(r.Review.Payload, &p); err != nil {
		return
	}
	r.Text = p.Text
	r.Pros = p.Pros
	r.Cons = p.Cons
	r.CustomerName = p.CustomerName
	r.Photos = p.Photos
	r.Bables = p.Bables
	r.MatchingSize = p.MatchingSize
	r.ReturnStatus = p.ReturnStatus
	r.ProductName = p.ProductName
	r.SubjectName = p.SubjectName
	r.Color = p.Color
	if p.OrderedAt != nil {
		r.OrderedAt = *p.OrderedAt
	}
}

// ReturnNote renders the WB return status as a human phrase (empty when a
// return is still available / unknown). Used by both the operator message and
// the model prompt so the bot never offers a return that already happened.
func (r Review) ReturnNote() string {
	switch r.ReturnStatus {
	case "returned":
		return "покупатель уже оформил возврат по этому заказу"
	case "unavailable":
		return "возврат по этому заказу недоступен"
	default:
		return ""
	}
}

// SizeNote surfaces WB's structured size-match signal only when it indicates a
// mismatch.
func (r Review) SizeNote() string {
	switch r.MatchingSize {
	case "маломерит", "большемерит":
		return "по данным WB товар " + r.MatchingSize
	default:
		return ""
	}
}

// OrderedNote renders the order date (empty when unknown).
func (r Review) OrderedNote() string {
	if r.OrderedAt.IsZero() {
		return ""
	}
	return r.OrderedAt.Format("02.01.2006")
}

type ArticleInfo struct {
	Description    string           `json:"description"`
	Recommendation []Recommendation `json:"recommendation"`
}

type Recommendation struct {
	Title   string `json:"title"`
	Article string `json:"article"`
}

// IsEmpty reports that the customer left no content at all — only a star
// rating, with no text/pros/cons, no selected tags and no photos. WB does
// return such "silent" reviews. Only WB-attached metadata (returnStatus,
// productName, …) may be present; it is not customer content. Such reviews
// need no generated answer.
func (r Review) IsEmpty() bool {
	return strings.TrimSpace(r.Text) == "" &&
		strings.TrimSpace(r.Pros) == "" &&
		strings.TrimSpace(r.Cons) == "" &&
		len(r.Bables) == 0 &&
		len(r.Photos) == 0
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
		`{{end}}{{if .Bables}}<b>Отметил</b>: {{join .Bables}}` + "\n" +
		`{{end}}{{if .SizeNote}}<b>Размер</b>: {{.SizeNote}}` + "\n" +
		`{{end}}{{if .ReturnNote}}<b>⚠️ Возврат</b>: {{.ReturnNote}}` + "\n" +
		`{{end}}{{if .Photos}}<b>Фото</b>:{{range $i, $p := .Photos}} <a href="{{$p}}">📷{{addOne $i}}</a>{{end}}` + "\n" +
		`{{end}}{{if .Answer}}<b>Ответ</b>: <pre>{{.Answer}}</pre>{{end}}`

	tmpl := template.Must(template.New("review").
		Funcs(template.FuncMap{
			"addOne": func(i int) int { return i + 1 },
			"join":   func(s []string) string { return strings.Join(s, ", ") },
		}).
		Parse(reviewTemplate))

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
	{{end}}{{if .Bables}}Покупатель отметил: {{join .Bables}}
	{{end}}{{if .SizeNote}}Соответствие размера: {{.SizeNote}}
	{{end}}{{if .ReturnNote}}Статус возврата: {{.ReturnNote}}
	{{end}}{{if .OrderedNote}}Дата заказа: {{.OrderedNote}}
	{{end}}{{if .Answer}}Ответ: {{.Answer}}{{end}}`

	tmpl := template.Must(template.New("review").
		Funcs(template.FuncMap{"join": func(s []string) string { return strings.Join(s, ", ") }}).
		Parse(promptTemplate))

	var sb strings.Builder
	err := tmpl.Execute(&sb, r)
	if err != nil {
		return "Ошибка формирования отзыва"
	}

	result := sb.String()
	return strings.TrimSpace(result)
}

// ToDB returns the persistence model. The content lives in Payload (jsonb);
// only control-plane fields are separate columns.
func (r Review) ToDB() *db.Review {
	out := r.Review
	return &out
}

func NewReviewFromWB(in wbc.Feedback) Review {
	p := ReviewPayload{
		Text:         in.Text,
		Pros:         in.Pros,
		Cons:         in.Cons,
		CustomerName: in.UserName,
		Bables:       in.Bables,
		MatchingSize: normalizeMatchingSize(in.MatchingSize),
		ReturnStatus: returnStatusFromWB(in),
		ProductName:  in.ProductDetails.ProductName,
		SubjectName:  in.SubjectName,
		Color:        in.Color,
	}

	for _, ph := range in.PhotoLinks {
		if ph.FullSize != "" {
			p.Photos = append(p.Photos, ph.FullSize)
		}
	}
	if !in.LastOrderCreatedAt.IsZero() {
		t := in.LastOrderCreatedAt
		p.OrderedAt = &t
	}

	payload, _ := json.Marshal(p)

	r := Review{
		Review: db.Review{
			ExternalID: in.Id,
			Article:    in.ProductDetails.SupplierArticle,
			Valuation:  in.ProductValuation,
			Payload:    payload,
		},
	}
	r.decodePayload()
	return r
}

// returnStatusFromWB derives a conservative return status from the WB feedback:
// a processed return date wins ("returned"), otherwise an unavailable flag
// means "unavailable", otherwise the return is still available.
func returnStatusFromWB(in wbc.Feedback) string {
	switch {
	case !in.ReturnProductOrdersDate.IsZero():
		return "returned"
	case !in.IsAbleReturnProductOrders:
		return "unavailable"
	default:
		return "available"
	}
}

// normalizeMatchingSize maps WB's size-match token to a RU phrase; unknown
// non-empty values pass through, "ok"/empty become "".
func normalizeMatchingSize(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "ok", "true", "matched", "нормально", "соответствует":
		return ""
	case "smaller", "small", "malomer", "less", "маломерит":
		return "маломерит"
	case "bigger", "big", "bolshemer", "larger", "more", "большемерит":
		return "большемерит"
	default:
		return s
	}
}

func NewReviewsFromWB(in *wbc.Review) Reviews {
	if in == nil {
		return nil
	}

	var reviews = make(Reviews, 0, len(in.Data.Feedbacks))
	for i := range in.Data.Feedbacks {
		review := NewReviewFromWB(in.Data.Feedbacks[i])
		review.StatusID = db.ReviewStatusCreated
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
