package ozon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

type Client struct {
	hc               *http.Client
	clientID, apiKey string
}

func NewClient(clientID, apiKey string) Client {
	return Client{
		hc: &http.Client{
			Timeout: time.Second * 10,
		},
		clientID: clientID,
		apiKey:   apiKey,
	}
}

func (c Client) request(reqType, baseURL string, headers map[string]string, params map[string]string, body []byte) (string, error) {
	req, err := http.NewRequest(reqType, baseURL, bytes.NewBuffer(body))

	if err != nil {
		return "", fmt.Errorf("ошибка создания запроса: %w", err)
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
		return "", fmt.Errorf("ошибка выполнения запроса: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("get status: %v: %s", resp.Status, respBody)
	}

	jsonString, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(jsonString), nil
}

func (c Client) get(obj any, baseURL string, headers map[string]string, params map[string]string, body []byte) error {
	response, err := c.request(http.MethodGet, baseURL, headers, params, body)
	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(response), &obj)
}

func (c Client) post(baseURL string, headers map[string]string, params map[string]string, body []byte) (string, error) {
	response, err := c.request(http.MethodPost, baseURL, headers, params, body)
	if err != nil {
		return "", err
	}

	return response, nil
}

// v2PostingFboGet для получения ФБО заказа исходя из posting_number (нужен для извлечения товаров в возврате)
func (c Client) PostingFbo(postingNumber string) (PostingFBO, error) {
	baseURL := "https://api-seller.ozon.ru/v2/posting/fbo/get"

	body := []byte(`{
  		"posting_number": "` + postingNumber + `",
  		"translit": true,
  		"with": {
    		"analytics_data": false,
    		"financial_data": false
 		 }
	}`)

	headers := map[string]string{
		"Content-Type": "application/json",
		"Client-Id":    c.clientID,
		"Api-Key":      c.apiKey,
	}

	params := map[string]string{}

	var p PostingFBO
	response, err := c.post(baseURL, headers, params, body)
	if err != nil {
		return p, err
	}
	err = json.Unmarshal([]byte(response), &p)
	return p, err
}

// v3ReturnsCompanyFbo метод получения ФБО возвратов со статусом ReturnedToOzon. Получаем возвраты только тогда, когда возврат приедет на склад озон
func (c Client) v3ReturnsCompanyFbo(daysAgo, lastID int) (string, error) {
	baseURL := "https://api-seller.ozon.ru/v3/returns/company/fbo"

	body := []byte(`{
  		"filter": {
    		"status": [
      		"ReturnedToOzon"
    		]
  		},
  		"last_id":` + strconv.Itoa(lastID) + `,
  		"limit": 1000
	}`)

	headers := map[string]string{
		"Content-Type": "application/json",
		"Client-Id":    c.clientID,
		"Api-Key":      c.apiKey,
	}

	params := map[string]string{}

	response, err := c.post(baseURL, headers, params, body)
	if err != nil {
		return "", err
	}

	return response, err
}
func (c Client) ReturnsList(lastID int, since, to string) (Returns, error) {
	baseURL := "https://api-seller.ozon.ru/v1/returns/list"

	body := []byte(fmt.Sprintf(`{
		"filter": {
    		"visual_status_change_moment": {
      		"time_from": "%v",
      		"time_to": "%v"
		}
  		},
  			"limit": 500,
  			"last_id": %v
	}`, since, to, lastID))

	headers := map[string]string{
		"Content-Type": "application/json",
		"Client-Id":    c.clientID,
		"Api-Key":      c.apiKey,
	}

	params := map[string]string{}

	var r Returns
	response, err := c.post(baseURL, headers, params, body)
	if err != nil {
		return r, err
	}
	err = json.Unmarshal([]byte(response), &r)
	return r, err
}

// v3ReturnsCompanyFbs метод получения ФБС возвратов со статусом moving_to_resale
func (c Client) v3ReturnsCompanyFbs(lastID int) (string, error) {
	baseURL := "https://api-seller.ozon.ru/v3/returns/company/fbs"

	body := []byte(`{
  		"filter": {
    		"status": "moving_to_resale"
  		},
  		"limit": 1000,
		"last_id": ` + strconv.Itoa(lastID) + `
	}`)

	headers := map[string]string{
		"Content-Type": "application/json",
		"Client-Id":    c.clientID,
		"Api-Key":      c.apiKey,
	}

	params := map[string]string{}

	response, err := c.post(baseURL, headers, params, body)
	if err != nil {
		return "", err
	}
	return response, err
}

// v3PostingFbsGet метод получения ФБС заказа исходя из posting_number (нужен для извлечения товаров в возврате)
func (c Client) PostingFbs(postingNumber string) (PostingFBS, error) {
	baseURL := "https://api-seller.ozon.ru/v3/posting/fbs/get"

	body := []byte(`{
  	"posting_number": "` + postingNumber + `",
  	"with": {
    	"analytics_data": false,
    	"barcodes": false,
    	"financial_data": false,
    	"product_exemplars": false,
    	"translit": false
 	 }
	}`)

	headers := map[string]string{
		"Content-Type": "application/json",
		"Client-Id":    c.clientID,
		"Api-Key":      c.apiKey,
	}

	params := map[string]string{}

	var p PostingFBS
	response, err := c.post(baseURL, headers, params, body)
	if err != nil {
		return p, err
	}
	err = json.Unmarshal([]byte(response), &p)
	return p, err
}

// v3PostingFbsList метод получения ФБС заказов
func (c Client) PostingsListFbs(since, to string, offset int, status string) (PostingslistFbs, error) {
	baseURL := "https://api-seller.ozon.ru/v3/posting/fbs/list"

	body := []byte(fmt.Sprintf(`{
  "dir": "ASC",
  "filter": {
    "since": "%v",
    "to": "%v",
	"status": "%v"
},
  "limit": 1000,
  "offset": %v,
  "with": {
    "analytics_data": false,
    "barcodes": false,
    "financial_data": true,
    "translit": false
  }
}`, since, to, status, offset))

	headers := map[string]string{
		"Content-Type": "application/json",
		"Client-Id":    c.clientID,
		"Api-Key":      c.apiKey,
	}

	params := map[string]string{}

	var p PostingslistFbs
	response, err := c.post(baseURL, headers, params, body)
	if err != nil {
		return p, err
	}
	err = json.Unmarshal([]byte(response), &p)
	return p, err
}

func (c Client) Labels(postingNumber string) (string, error) {
	baseURL := "https://api-seller.ozon.ru/v2/posting/fbs/package-label"

	body := []byte(fmt.Sprintf(`{
  "posting_number": [
    "%v"
  ]
}`, postingNumber))

	headers := map[string]string{
		"Content-Type": "application/json",
		"Client-Id":    c.clientID,
		"Api-Key":      c.apiKey,
	}

	params := map[string]string{}

	response, err := c.post(baseURL, headers, params, body)
	if err != nil {
		return "", err
	}
	return response, err

}

// v2PostingFboList метод получения ФБО заказов
func (c Client) PostingsListFbo(since, to string, offset int) (PostingslistFbo, error) {
	baseURL := "https://api-seller.ozon.ru/v2/posting/fbo/list"

	body := []byte(fmt.Sprintf(`{
  		"dir": "ASC",
  		"filter": {
    		"since": "%v",
    		"to": "%v"
  		},
  		"limit": 1000,
  		"offset": %v,
  		"translit": false,
  		"with": {
			"analytics_data": false,
			"financial_data": true
  		}
	}`, since, to, offset))

	headers := map[string]string{
		"Content-Type": "application/json",
		"Client-Id":    c.clientID,
		"Api-Key":      c.apiKey,
	}

	params := map[string]string{}

	var p PostingslistFbo
	response, err := c.post(baseURL, headers, params, body)
	if err != nil {
		return p, err
	}
	err = json.Unmarshal([]byte(response), &p)
	return p, err

}
func (c Client) Stocks() (StocksList, error) {
	baseURL := "https://api-seller.ozon.ru/v2/analytics/stock_on_warehouses"

	body := []byte(`{
  		"limit": 1000,
  		"offset": 0,
  		"warehouse_type": "ALL"
	}`)

	headers := map[string]string{
		"Content-Type": "application/json",
		"Client-Id":    c.clientID,
		"Api-Key":      c.apiKey,
	}

	params := map[string]string{}

	var s StocksList
	response, err := c.post(baseURL, headers, params, body)
	if err != nil {
		return s, err
	}

	err = json.Unmarshal([]byte(response), &s)
	return s, err

}
func (c Client) Clusters() (ClustersList, error) {
	baseURL := "https://api-seller.ozon.ru/v1/cluster/list"

	body := []byte(`{
  		"cluster_type": "CLUSTER_TYPE_OZON"
	}`)

	headers := map[string]string{
		"Content-Type": "application/json",
		"Client-Id":    c.clientID,
		"Api-Key":      c.apiKey,
	}

	params := map[string]string{}

	var cl ClustersList
	response, err := c.post(baseURL, headers, params, body)
	if err != nil {
		return cl, err
	}
	err = json.Unmarshal([]byte(response), &cl)
	return cl, err
}

func (c Client) StocksAnalytics(skus []string) (StocksNew, error) {
	skusJSON, err := json.Marshal(skus)
	if err != nil {
		return StocksNew{}, err
	}

	baseURL := "https://api-seller.ozon.ru/v1/analytics/stocks"
	body := []byte(fmt.Sprintf(`{
  		"skus": %v
	}`, string(skusJSON)))

	headers := map[string]string{
		"Content-Type": "application/json",
		"Client-Id":    c.clientID,
		"Api-Key":      c.apiKey,
	}

	params := map[string]string{}

	var s StocksNew
	response, err := c.post(baseURL, headers, params, body)
	if err != nil {
		return s, err
	}
	err = json.Unmarshal([]byte(response), &s)
	return s, err

}

func (c Client) Warehouses(cursor string) (WarehouseList, error) {
	baseURL := "https://api-seller.ozon.ru/v2/warehouse/list"

	body := []byte(fmt.Sprintf(`{"cursor": "%v"}`, cursor))

	headers := map[string]string{
		"Content-Type": "application/json",
		"Client-Id":    c.clientID,
		"Api-Key":      c.apiKey,
	}

	params := map[string]string{}

	var w WarehouseList
	response, err := c.post(baseURL, headers, params, body)
	if err != nil {
		return w, err
	}
	err = json.Unmarshal([]byte(response), &w)
	return w, err
}

func (c Client) Products() (ProductList, error) {
	baseURL := "https://api-seller.ozon.ru/v3/product/list"
	body := []byte(`{
  "filter": {
    "visibility": "TO_SUPPLY"
  },
  "last_id": "",
  "limit": 1000
}`)

	headers := map[string]string{
		"Content-Type": "application/json",
		"Client-Id":    c.clientID,
		"Api-Key":      c.apiKey,
	}

	params := map[string]string{}

	var p ProductList
	response, err := c.post(baseURL, headers, params, body)
	if err != nil {
		return p, err
	}
	err = json.Unmarshal([]byte(response), &p)
	return p, err

}

func (c Client) ProductsWithAttributes() (ProductListWithAttributes, error) {
	baseURL := "https://api-seller.ozon.ru/v4/product/info/attributes"
	body := []byte(`{
  "filter": {
    "visibility": "VISIBLE"
  },
  "limit": 1000,
  "sort_dir": "ASC"
}`)

	headers := map[string]string{
		"Content-Type": "application/json",
		"Client-Id":    c.clientID,
		"Api-Key":      c.apiKey,
	}

	params := map[string]string{}

	var p ProductListWithAttributes
	response, err := c.post(baseURL, headers, params, body)
	if err != nil {
		return p, err
	}
	err = json.Unmarshal([]byte(response), &p)
	return p, err
}
