package wb

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type Client struct {
	hc    *http.Client
	token string
}

func NewClient(token string) Client {
	return Client{
		hc: &http.Client{
			Timeout: time.Second * 10,
		},
		token: token,
	}
}

func (c Client) request(reqType, baseURL string, headers, params map[string]string, body []byte) (int, string, error) {
	req, err := http.NewRequest(reqType, baseURL, bytes.NewBuffer(body))

	if err != nil {
		return http.StatusInternalServerError, "", fmt.Errorf("ошибка создания запроса: %w", err)
	}

	for s := range headers {
		req.Header.Set(s, headers[s])
	}

	q := req.URL.Query()
	for s := range params {
		q.Add(s, params[s])
	}

	req.URL.RawQuery = q.Encode()

	resp, err := c.hc.Do(req)
	if err != nil {
		return http.StatusInternalServerError, "", fmt.Errorf("ошибка выполнения запроса: %w", err)
	}
	defer resp.Body.Close()

	jsonString, err := io.ReadAll(resp.Body)
	if err != nil {
		return http.StatusInternalServerError, "", err
	}
	return resp.StatusCode, string(jsonString), nil
}

func (c Client) get(baseURL string, headers map[string]string, params map[string]string, body []byte) (int, string, error) {
	return c.request(http.MethodGet, baseURL, headers, params, body)
}

func (c Client) post(baseURL string, headers map[string]string, params map[string]string, body []byte) (int, string, error) {
	return c.request(http.MethodPost, baseURL, headers, params, body)
}

func (c Client) stocksFbo() (string, error) {
	baseURL := "https://statistics-api.wildberries.ru/api/v1/supplier/stocks"

	body := []byte(``)

	headers := map[string]string{
		"Authorization": c.token,
	}

	params := map[string]string{
		"dateFrom": "2019-06-20",
	}

	_, response, err := c.get(baseURL, headers, params, body)
	if err != nil {
		return "", err
	}

	return response, nil
}

func (c Client) getOrdersBySupplyID(supplyID string) (string, error) {
	baseURL := "https://marketplace-api.wildberries.ru/api/marketplace/v3/supplies/" + supplyID + "/order-ids"
	body := []byte(``)

	headers := map[string]string{
		"Authorization": c.token,
	}

	_, response, err := c.get(baseURL, headers, make(map[string]string), body)
	if err != nil {
		return "", err
	}

	return response, nil
}

func (c Client) getReturns(dateFrom, dateTo string) (string, error) {

	params := url.Values{}
	params.Add("dateFrom", dateFrom)
	params.Add("dateTo", dateTo)

	// Основной URL
	baseURL := "https://seller-analytics-api.wildberries.ru/api/v1/analytics/goods-return"

	// Добавление параметров к URL
	fullURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	body := []byte(``)

	headers := map[string]string{
		"Authorization": c.token,
	}

	_, response, err := c.get(fullURL, headers, make(map[string]string), body)
	if err != nil {
		return "", err
	}

	return response, nil
}

func (c Client) getCodesByOrderID(orderID int) (string, error) {
	params := url.Values{}
	params.Add("type", "png")
	params.Add("width", "58")
	params.Add("height", "40")

	// Основной URL
	baseURL := "https://marketplace-api.wildberries.ru/api/v3/orders/stickers"

	// Добавление параметров к URL
	fullURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	type RequestBody struct {
		Orders []int `json:"orders"`
	}

	data := RequestBody{
		Orders: []int{orderID},
	}

	body, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": c.token,
	}

	_, response, err := c.post(fullURL, headers, make(map[string]string), body)
	if err != nil {
		return "", err
	}

	return response, nil
}

// GetOrdersFBS отдает фбс заказы
func (c Client) GetOrdersFBS(dateFrom, dateTo int) (*OrdersListFBS, error) {
	baseURL := "https://marketplace-api.wildberries.ru/api/v3/orders"
	body := []byte(``)

	headers := map[string]string{
		"Authorization": c.token,
	}

	params := map[string]string{
		"limit":    "1000",
		"next":     "0",
		"dateFrom": strconv.Itoa(dateFrom),
		"dateTo":   strconv.Itoa(dateTo),
	}

	_, response, err := c.get(baseURL, headers, params, body)
	if err != nil {
		return nil, err
	}

	var posting OrdersListFBS
	err = json.Unmarshal([]byte(response), &posting)
	return &posting, err
}

func (c Client) ordersFBSStatus(orderID int) (string, error) {
	baseURL := "https://marketplace-api.wildberries.ru/api/v3/orders/status"

	type RequestBody struct {
		Orders []int `json:"orders"`
	}

	data := RequestBody{
		Orders: []int{orderID},
	}

	// Преобразование данных в JSON
	body, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("ошибка при преобразовании данных в JSON: %w", err)
	}

	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": c.token,
	}

	_, response, err := c.post(baseURL, headers, make(map[string]string), body)
	if err != nil {
		return "", err
	}

	return response, nil
}
func (c Client) getCards(nmID *int, updatedAt *time.Time, limit *int) (string, error) {
	baseURL := "https://content-api.wildberries.ru/content/v2/get/cards/list"

	type RequestBody struct {
		Settings struct {
			Sort struct {
				Ascending bool `json:"ascending"`
			} `json:"sort"`
			Cursor struct {
				UpdatedAt *time.Time `json:"updatedAt"`
				NmId      *int       `json:"nmID"`
				Limit     *int       `json:"limit"`
			} `json:"cursor"`
			Filter struct {
				WithPhoto int `json:"withPhoto"`
			} `json:"filter"`
		} `json:"settings"`
	}

	var data RequestBody
	data.Settings.Cursor.Limit = limit
	data.Settings.Cursor.NmId = nmID
	data.Settings.Cursor.UpdatedAt = updatedAt

	data.Settings.Sort.Ascending = true
	data.Settings.Filter.WithPhoto = -1

	if limit != nil {
		data.Settings.Cursor.Limit = limit
	}

	// Преобразование данных в JSON
	body, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("ошибка при преобразовании данных в JSON: %w", err)
	}

	headers := map[string]string{
		"Authorization": c.token,
	}

	_, response, err := c.post(baseURL, headers, make(map[string]string), body)
	if err != nil {
		return "", err
	}

	return response, nil
}

/*
API метод для получения всех заказов за дату, равную (now() - daysAgo). Максимум 1 запрос в минуту
*/
func (c Client) apiOrdersALL(daysAgo, flag int) (string, error) {
	date := time.Now().AddDate(0, 0, -daysAgo)

	baseURL := "https://statistics-api.wildberries.ru/api/v1/supplier/orders"

	body := []byte(``)

	headers := map[string]string{
		"Authorization": c.token,
	}

	params := map[string]string{
		"dateFrom": date.Format("2006-01-02"),
		"flag":     strconv.Itoa(flag),
	}

	_, response, err := c.get(baseURL, headers, params, body)
	if err != nil {
		return "", err
	}

	return response, nil
}

func (c Client) apiSalesAndReturns(daysAgo int) (string, error) {
	date := time.Now().AddDate(0, 0, -daysAgo)

	baseURL := "https://statistics-api.wildberries.ru/api/v1/supplier/sales"

	body := []byte(``)

	headers := map[string]string{
		"Authorization": c.token,
	}

	params := map[string]string{
		"dateFrom": date.Format("2006-01-02"),
		"flag":     "1",
	}

	_, response, err := c.get(baseURL, headers, params, body)
	if err != nil {
		return "", err
	}

	return response, nil
}
func (c Client) Reviews() (*Review, error) {
	baseURL := "https://feedbacks-api.wildberries.ru/api/v1/feedbacks"

	body := []byte(``)

	headers := map[string]string{
		"Authorization": c.token,
	}

	params := map[string]string{
		"isAnswered": "false",
		"take":       "100",
		"skip":       "0",
	}

	_, response, err := c.get(baseURL, headers, params, body)
	if err != nil {
		return nil, err
	}

	var review *Review

	err = json.Unmarshal([]byte(response), &review)
	if err != nil {
		return nil, err
	} else if review.Error {
		return nil, errors.New(response)
	}

	return review, nil
}

func (c Client) AnswerReview(id, answer string) error {
	baseURL := "https://feedbacks-api.wildberries.ru/api/v1/feedbacks/answer"

	type RequestBody struct {
		Id   string `json:"id"`
		Text string `json:"text"`
	}

	var data = RequestBody{
		Id:   id,
		Text: answer,
	}

	body, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("data marshal failed: %w", err)
	}

	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": c.token,
	}

	params := map[string]string{}

	status, _, err := c.post(baseURL, headers, params, body)
	if err != nil {
		return fmt.Errorf("response get failed: %w", err)
	}

	if status != http.StatusNoContent {
		return fmt.Errorf("status not OK: %d", status)
	}

	return nil
}

func GetUnix(date time.Time) int64 {
	nowStr := fmt.Sprint(date.Format("2006-01-02"), "T21:00:00")
	t, _ := time.Parse("2006-01-02T15:04:05", nowStr)
	return t.Unix()
}
