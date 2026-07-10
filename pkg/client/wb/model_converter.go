package wb

import (
	"encoding/json"
	"time"
)

func (c Client) GetOrderIDsFbs(supplyID string) (OrderIDs, error) {
	var orders OrderIDs
	jsonString, err := c.getOrdersBySupplyID(supplyID)
	if err != nil || jsonString == "" {
		return orders, err
	}

	err = json.Unmarshal([]byte(jsonString), &orders)
	if err != nil {
		return orders, err
	}

	//sortOrdersByArticle(orders.Orders)
	return orders, nil
}
func (c Client) GetCards(nmID *int, updatedAt *time.Time, limit *int) (*CardList, error) {
	var cards CardList
	jsonString, err := c.getCards(nmID, updatedAt, limit)
	if err != nil || jsonString == "" {
		return nil, err
	}

	err = json.Unmarshal([]byte(jsonString), &cards)
	if err != nil {
		return nil, err
	}

	return &cards, nil
}

func (c Client) GetReturns(dateFrom, dateTo string) (*ReturnList, error) {
	var returns ReturnList
	jsonString, err := c.getReturns(dateFrom, dateTo)
	if err != nil || jsonString == "" {
		return nil, err
	}

	err = json.Unmarshal([]byte(jsonString), &returns)
	if err != nil {
		return nil, err
	}

	return &returns, nil
}

func (c Client) GetStickersFbs(orderID int) (StickerWB, error) {
	var stickers StickerWB
	jsonString, err := c.getCodesByOrderID(orderID)
	if err != nil || jsonString == "" {
		return stickers, err
	}

	err = json.Unmarshal([]byte(jsonString), &stickers)
	return stickers, err
}

func (c Client) GetAllOrders(daysAgo, flag int) (OrdersListALL, error) {
	var posting OrdersListALL
	jsonString, err := c.apiOrdersALL(daysAgo, flag)
	if err != nil || jsonString == "" {
		return nil, err
	}

	err = json.Unmarshal([]byte(jsonString), &posting)
	return posting, err
}

func (c Client) GetSalesAndReturns(daysAgo int) (SalesReturns, error) {
	var sales SalesReturns
	jsonString, err := c.apiSalesAndReturns(daysAgo)
	if err != nil || jsonString == "" {
		return nil, err
	}

	err = json.Unmarshal([]byte(jsonString), &sales)
	return sales, err
}

func (c Client) GetPostingStatus(postingID int) (string, error) {
	var postingStatuses OrdersWithStatuses
	jsonString, err := c.ordersFBSStatus(postingID)
	if err != nil || jsonString == "" {
		return "", err
	}

	err = json.Unmarshal([]byte(jsonString), &postingStatuses)
	if err != nil {
		return "", err
	}

	return postingStatuses.Orders[0].WbStatus, nil
}

func (c Client) GetStockFbo() ([]Stock, error) {
	var stocks []Stock
	jsonString, err := c.stocksFbo()
	if err != nil || jsonString == "" {
		return nil, err
	}

	err = json.Unmarshal([]byte(jsonString), &stocks)
	return stocks, nil
}
