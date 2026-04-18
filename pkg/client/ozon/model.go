package ozon

import "time"

type ReturnFBO struct {
	ReturnID                   int    `json:"return_id"`
	ID                         int    `json:"id"`
	SKU                        int    `json:"sku"`
	CompanyID                  int    `json:"company_id"`
	PostingNumber              string `json:"posting_number"`
	AcceptedFromCustomerMoment string `json:"accepted_from_customer_moment"`
	ReturnReasonName           string `json:"return_reason_name"`
	IsOpened                   bool   `json:"is_opened"`
	StatusName                 string `json:"status_name"`
	ReturnedToOzonMoment       string `json:"returned_to_ozon_moment"`
	CurrentPlaceName           string `json:"current_place_name"`
	DstPlaceName               string `json:"dst_place_name"`
}

type ReturnsFBO struct {
	Returns []ReturnFBO `json:"returns"`
	LastID  int         `json:"last_id"`
}

type PostingFBO struct {
	Result struct {
		OrderID        int64     `json:"order_id"`
		OrderNumber    string    `json:"order_number"`
		PostingNumber  string    `json:"posting_number"`
		Status         string    `json:"status"`
		CancelReasonID int       `json:"cancel_reason_id"`
		CreatedAt      time.Time `json:"created_at"`
		InProcessAt    time.Time `json:"in_process_at"`
		Products       []struct {
			Sku          int           `json:"sku"`
			Name         string        `json:"name"`
			Quantity     int           `json:"quantity"`
			OfferID      string        `json:"offer_id"`
			Price        string        `json:"price"`
			DigitalCodes []interface{} `json:"digital_codes"`
			CurrencyCode string        `json:"currency_code"`
		} `json:"products"`
		AnalyticsData  interface{}   `json:"analytics_data"`
		FinancialData  interface{}   `json:"financial_data"`
		AdditionalData []interface{} `json:"additional_data"`
	} `json:"result"`
}

type ReturnFBS struct {
	ClearingID               int     `json:"clearing_id"`
	Commission               float64 `json:"commission"`
	CommissionPercent        float64 `json:"commission_percent"`
	ExemplarID               int     `json:"exemplar_id"`
	ID                       int     `json:"id"`
	IsMoving                 bool    `json:"is_moving"`
	IsOpened                 bool    `json:"is_opened"`
	LastFreeWaitingDay       string  `json:"last_free_waiting_day"`
	PlaceID                  int     `json:"place_id"`
	MovingToPlaceName        string  `json:"moving_to_place_name"`
	PickingAmount            int     `json:"picking_amount"`
	PostingNumber            string  `json:"posting_number"`
	PickingTag               string  `json:"picking_tag"`
	Price                    float64 `json:"price"`
	PriceWithoutCommission   float64 `json:"price_without_commission"`
	ProductID                int     `json:"product_id"`
	ProductName              string  `json:"product_name"`
	Quantity                 int     `json:"quantity"`
	ReturnBarcode            string  `json:"return_barcode"`
	ReturnClearingID         int     `json:"return_clearing_id"`
	ReturnDate               string  `json:"return_date"`
	ReturnReasonName         string  `json:"return_reason_name"`
	WaitingForSellerDateTime string  `json:"waiting_for_seller_date_time"`
	ReturnedToSellerDateTime string  `json:"returned_to_seller_date_time"`
	WaitingForSellerDays     int     `json:"waiting_for_seller_days"`
	ReturnsKeepingCost       int     `json:"returns_keeping_cost"`
	Sku                      int     `json:"sku"`
	Status                   string  `json:"status"`
}

type ReturnsFBS struct {
	Returns []ReturnFBS `json:"returns"`
	LastID  int         `json:"last_id"`
}

type PostingFBS struct {
	Result struct {
		PostingNumber  string `json:"posting_number"`
		OrderID        int    `json:"order_id"`
		OrderNumber    string `json:"order_number"`
		Status         string `json:"status"`
		Substatus      string `json:"substatus"`
		DeliveryMethod struct {
			ID            int64  `json:"id"`
			Name          string `json:"name"`
			WarehouseID   int64  `json:"warehouse_id"`
			Warehouse     string `json:"warehouse"`
			TplProviderID int    `json:"tpl_provider_id"`
			TplProvider   string `json:"tpl_provider"`
		} `json:"delivery_method"`
		TrackingNumber     string      `json:"tracking_number"`
		TplIntegrationType string      `json:"tpl_integration_type"`
		InProcessAt        time.Time   `json:"in_process_at"`
		ShipmentDate       time.Time   `json:"shipment_date"`
		DeliveringDate     interface{} `json:"delivering_date"`
		ProviderStatus     string      `json:"provider_status"`
		DeliveryPrice      string      `json:"delivery_price"`
		Cancellation       struct {
			CancelReasonID           int    `json:"cancel_reason_id"`
			CancelReason             string `json:"cancel_reason"`
			CancellationType         string `json:"cancellation_type"`
			CancelledAfterShip       bool   `json:"cancelled_after_ship"`
			AffectCancellationRating bool   `json:"affect_cancellation_rating"`
			CancellationInitiator    string `json:"cancellation_initiator"`
		} `json:"cancellation"`
		Customer  interface{} `json:"customer"`
		Addressee interface{} `json:"addressee"`
		Products  []struct {
			CurrencyCode  string        `json:"currency_code"`
			Price         string        `json:"price"`
			OfferID       string        `json:"offer_id"`
			Name          string        `json:"name"`
			Sku           int           `json:"sku"`
			Quantity      int           `json:"quantity"`
			JwUin         []interface{} `json:"jw_uin"`
			MandatoryMark []interface{} `json:"mandatory_mark"`
			Dimensions    struct {
				Height string `json:"height"`
				Length string `json:"length"`
				Weight string `json:"weight"`
				Width  string `json:"width"`
			} `json:"dimensions"`
		} `json:"products"`
		Barcodes       interface{}   `json:"barcodes"`
		AnalyticsData  interface{}   `json:"analytics_data"`
		FinancialData  interface{}   `json:"financial_data"`
		AdditionalData []interface{} `json:"additional_data"`
		IsExpress      bool          `json:"is_express"`
		Requirements   struct {
			ProductsRequiringGtd     []interface{} `json:"products_requiring_gtd"`
			ProductsRequiringCountry []interface{} `json:"products_requiring_country"`
			ProductsRequiringJwn     []interface{} `json:"products_requiring_jwn"`
		} `json:"requirements"`
		ProductExemplars interface{} `json:"product_exemplars"`
	} `json:"result"`
}

type ReportResponse struct {
	Result struct {
		Code string `json:"code"`
	} `json:"result"`
}

type PostingslistFbs struct {
	Result struct {
		PostingsFBS []struct {
			PostingNumber  string `json:"posting_number"`
			OrderID        int    `json:"order_id"`
			OrderNumber    string `json:"order_number"`
			Status         string `json:"status"`
			Substatus      string `json:"substatus"`
			DeliveryMethod struct {
				ID            int64  `json:"id"`
				Name          string `json:"name"`
				WarehouseID   int64  `json:"warehouse_id"`
				Warehouse     string `json:"warehouse"`
				TplProviderID int    `json:"tpl_provider_id"`
				TplProvider   string `json:"tpl_provider"`
			} `json:"delivery_method"`
			TrackingNumber     string      `json:"tracking_number"`
			TplIntegrationType string      `json:"tpl_integration_type"`
			InProcessAt        time.Time   `json:"in_process_at"`
			ShipmentDate       time.Time   `json:"shipment_date"`
			DeliveringDate     interface{} `json:"delivering_date"`
			Cancellation       struct {
				CancelReasonID           int    `json:"cancel_reason_id"`
				CancelReason             string `json:"cancel_reason"`
				CancellationType         string `json:"cancellation_type"`
				CancelledAfterShip       bool   `json:"cancelled_after_ship"`
				AffectCancellationRating bool   `json:"affect_cancellation_rating"`
				CancellationInitiator    string `json:"cancellation_initiator"`
			} `json:"cancellation"`
			Customer interface{} `json:"customer"`
			Products []struct {
				Price         string        `json:"price"`
				CurrencyCode  string        `json:"currency_code"`
				OfferID       string        `json:"offer_id"`
				Name          string        `json:"name"`
				Sku           int           `json:"sku"`
				Quantity      int           `json:"quantity"`
				MandatoryMark []interface{} `json:"mandatory_mark"`
			} `json:"products"`
			Addressee     interface{} `json:"addressee"`
			Barcodes      interface{} `json:"barcodes"`
			AnalyticsData interface{} `json:"analytics_data"`

			FinancialData struct {
				Products []struct {
					CommissionAmount     float64     `json:"commission_amount"`
					CommissionPercent    int         `json:"commission_percent"`
					Payout               float64     `json:"payout"`
					ProductID            int         `json:"product_id"`
					OldPrice             float64     `json:"old_price"`
					Price                float64     `json:"price"`
					TotalDiscountValue   float64     `json:"total_discount_value"`
					TotalDiscountPercent float64     `json:"total_discount_percent"`
					Actions              []string    `json:"actions"`
					Picking              interface{} `json:"picking"`
					Quantity             int         `json:"quantity"`
					ClientPrice          string      `json:"client_price"`
					ItemServices         struct {
						MarketplaceServiceItemFulfillment                int     `json:"marketplace_service_item_fulfillment"`
						MarketplaceServiceItemPickup                     int     `json:"marketplace_service_item_pickup"`
						MarketplaceServiceItemDropoffPvz                 int     `json:"marketplace_service_item_dropoff_pvz"`
						MarketplaceServiceItemDropoffSc                  int     `json:"marketplace_service_item_dropoff_sc"`
						MarketplaceServiceItemDropoffFf                  int     `json:"marketplace_service_item_dropoff_ff"`
						MarketplaceServiceItemDirectFlowTrans            int     `json:"marketplace_service_item_direct_flow_trans"`
						MarketplaceServiceItemReturnFlowTrans            int     `json:"marketplace_service_item_return_flow_trans"`
						MarketplaceServiceItemDelivToCustomer            float64 `json:"marketplace_service_item_deliv_to_customer"`
						MarketplaceServiceItemReturnNotDelivToCustomer   int     `json:"marketplace_service_item_return_not_deliv_to_customer"`
						MarketplaceServiceItemReturnPartGoodsCustomer    int     `json:"marketplace_service_item_return_part_goods_customer"`
						MarketplaceServiceItemReturnAfterDelivToCustomer int     `json:"marketplace_service_item_return_after_deliv_to_customer"`
					} `json:"item_services"`
					CurrencyCode string `json:"currency_code"`
				} `json:"products"`
				PostingServices struct {
					MarketplaceServiceItemFulfillment                int `json:"marketplace_service_item_fulfillment"`
					MarketplaceServiceItemPickup                     int `json:"marketplace_service_item_pickup"`
					MarketplaceServiceItemDropoffPvz                 int `json:"marketplace_service_item_dropoff_pvz"`
					MarketplaceServiceItemDropoffSc                  int `json:"marketplace_service_item_dropoff_sc"`
					MarketplaceServiceItemDropoffFf                  int `json:"marketplace_service_item_dropoff_ff"`
					MarketplaceServiceItemDirectFlowTrans            int `json:"marketplace_service_item_direct_flow_trans"`
					MarketplaceServiceItemReturnFlowTrans            int `json:"marketplace_service_item_return_flow_trans"`
					MarketplaceServiceItemDelivToCustomer            int `json:"marketplace_service_item_deliv_to_customer"`
					MarketplaceServiceItemReturnNotDelivToCustomer   int `json:"marketplace_service_item_return_not_deliv_to_customer"`
					MarketplaceServiceItemReturnPartGoodsCustomer    int `json:"marketplace_service_item_return_part_goods_customer"`
					MarketplaceServiceItemReturnAfterDelivToCustomer int `json:"marketplace_service_item_return_after_deliv_to_customer"`
				} `json:"posting_services"`
				ClusterFrom string `json:"cluster_from"`
				ClusterTo   string `json:"cluster_to"`
			} `json:"financial_data"`
			IsExpress    bool `json:"is_express"`
			Requirements struct {
				ProductsRequiringGtd           []interface{} `json:"products_requiring_gtd"`
				ProductsRequiringCountry       []interface{} `json:"products_requiring_country"`
				ProductsRequiringMandatoryMark []interface{} `json:"products_requiring_mandatory_mark"`
				ProductsRequiringJwn           []interface{} `json:"products_requiring_jwn"`
			} `json:"requirements"`
		} `json:"postings"`
		HasNext bool `json:"has_next"`
	} `json:"result"`
}

type PostingslistFbo struct {
	Result []struct {
		OrderID        int       `json:"order_id"`
		OrderNumber    string    `json:"order_number"`
		PostingNumber  string    `json:"posting_number"`
		Status         string    `json:"status"`
		CancelReasonID int       `json:"cancel_reason_id"`
		CreatedAt      time.Time `json:"created_at"`
		InProcessAt    time.Time `json:"in_process_at"`
		Products       []struct {
			Sku          int           `json:"sku"`
			Name         string        `json:"name"`
			Quantity     int           `json:"quantity"`
			OfferID      string        `json:"offer_id"`
			Price        string        `json:"price"`
			DigitalCodes []interface{} `json:"digital_codes"`
			CurrencyCode string        `json:"currency_code"`
		} `json:"products"`
		AnalyticsData struct {
			Region               string `json:"region"`
			City                 string `json:"city"`
			DeliveryType         string `json:"delivery_type"`
			IsPremium            bool   `json:"is_premium"`
			PaymentTypeGroupName string `json:"payment_type_group_name"`
			WarehouseID          int64  `json:"warehouse_id"`
			WarehouseName        string `json:"warehouse_name"`
			IsLegal              bool   `json:"is_legal"`
		} `json:"analytics_data"`

		FinancialData struct {
			Products []struct {
				CommissionAmount     float64     `json:"commission_amount"`
				CommissionPercent    int         `json:"commission_percent"`
				Payout               float64     `json:"payout"`
				ProductID            int         `json:"product_id"`
				OldPrice             float64     `json:"old_price"`
				Price                float64     `json:"price"`
				TotalDiscountValue   float64     `json:"total_discount_value"`
				TotalDiscountPercent float64     `json:"total_discount_percent"`
				Actions              []string    `json:"actions"`
				Picking              interface{} `json:"picking"`
				ClientPrice          string      `json:"client_price"`
				ItemServices         struct {
					MarketplaceServiceItemFulfillment                int     `json:"marketplace_service_item_fulfillment"`
					MarketplaceServiceItemPickup                     int     `json:"marketplace_service_item_pickup"`
					MarketplaceServiceItemDropoffPvz                 int     `json:"marketplace_service_item_dropoff_pvz"`
					MarketplaceServiceItemDropoffSc                  int     `json:"marketplace_service_item_dropoff_sc"`
					MarketplaceServiceItemDropoffFf                  int     `json:"marketplace_service_item_dropoff_ff"`
					MarketplaceServiceItemDirectFlowTrans            int     `json:"marketplace_service_item_direct_flow_trans"`
					MarketplaceServiceItemReturnFlowTrans            int     `json:"marketplace_service_item_return_flow_trans"`
					MarketplaceServiceItemDelivToCustomer            float64 `json:"marketplace_service_item_deliv_to_customer"`
					MarketplaceServiceItemReturnNotDelivToCustomer   int     `json:"marketplace_service_item_return_not_deliv_to_customer"`
					MarketplaceServiceItemReturnPartGoodsCustomer    int     `json:"marketplace_service_item_return_part_goods_customer"`
					MarketplaceServiceItemReturnAfterDelivToCustomer int     `json:"marketplace_service_item_return_after_deliv_to_customer"`
				} `json:"item_services"`
				CurrencyCode string `json:"currency_code"`
			} `json:"products"`
			PostingServices struct {
				MarketplaceServiceItemFulfillment                int `json:"marketplace_service_item_fulfillment"`
				MarketplaceServiceItemPickup                     int `json:"marketplace_service_item_pickup"`
				MarketplaceServiceItemDropoffPvz                 int `json:"marketplace_service_item_dropoff_pvz"`
				MarketplaceServiceItemDropoffSc                  int `json:"marketplace_service_item_dropoff_sc"`
				MarketplaceServiceItemDropoffFf                  int `json:"marketplace_service_item_dropoff_ff"`
				MarketplaceServiceItemDirectFlowTrans            int `json:"marketplace_service_item_direct_flow_trans"`
				MarketplaceServiceItemReturnFlowTrans            int `json:"marketplace_service_item_return_flow_trans"`
				MarketplaceServiceItemDelivToCustomer            int `json:"marketplace_service_item_deliv_to_customer"`
				MarketplaceServiceItemReturnNotDelivToCustomer   int `json:"marketplace_service_item_return_not_deliv_to_customer"`
				MarketplaceServiceItemReturnPartGoodsCustomer    int `json:"marketplace_service_item_return_part_goods_customer"`
				MarketplaceServiceItemReturnAfterDelivToCustomer int `json:"marketplace_service_item_return_after_deliv_to_customer"`
			} `json:"posting_services"`
			ClusterFrom string `json:"cluster_from"`
			ClusterTo   string `json:"cluster_to"`
		} `json:"financial_data"`
		AdditionalData []interface{} `json:"additional_data"`
	} `json:"result"`
}

type StocksList struct {
	Result struct {
		Rows []struct {
			Sku              int      `json:"sku"`
			WarehouseName    string   `json:"warehouse_name"`
			ItemCode         string   `json:"item_code"`
			ItemName         string   `json:"item_name"`
			PromisedAmount   int      `json:"promised_amount"`
			FreeToSellAmount int      `json:"free_to_sell_amount"`
			ReservedAmount   int      `json:"reserved_amount"`
			Idc              *float64 `json:"idc"`
		} `json:"rows"`
	} `json:"result"`
}

type ClustersList struct {
	Clusters []struct {
		LogisticClusters []struct {
			Warehouses []struct {
				WarehouseID int64  `json:"warehouse_id"`
				Type        string `json:"type"`
				Name        string `json:"name"`
			} `json:"warehouses"`
		} `json:"logistic_clusters"`
		ID   int    `json:"id"`
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"clusters"`
}

type Returns struct {
	Returns []struct {
		ID               int    `json:"id"`
		CompanyID        int    `json:"company_id"`
		ReturnReasonName string `json:"return_reason_name"`
		Type             string `json:"type"`
		Schema           string `json:"schema"`
		OrderID          int64  `json:"order_id"`
		OrderNumber      string `json:"order_number"`
		Place            struct {
			ID      int64  `json:"id"`
			Name    string `json:"name"`
			Address string `json:"address"`
		} `json:"place"`
		TargetPlace struct {
			ID      int64  `json:"id"`
			Name    string `json:"name"`
			Address string `json:"address"`
		} `json:"target_place"`
		Storage struct {
			Sum struct {
				CurrencyCode string  `json:"currency_code"`
				Price        float64 `json:"price"`
			} `json:"sum"`
			TarifficationFirstDate time.Time `json:"tariffication_first_date"`
			TarifficationStartDate time.Time `json:"tariffication_start_date"`
			ArrivedMoment          time.Time `json:"arrived_moment"`
			Days                   int       `json:"days"`
			UtilizationSum         struct {
				CurrencyCode string  `json:"currency_code"`
				Price        float64 `json:"price"`
			} `json:"utilization_sum"`
			UtilizationForecastDate time.Time `json:"utilization_forecast_date"`
		} `json:"storage"`
		Product struct {
			Sku     int    `json:"sku"`
			OfferID string `json:"offer_id"`
			Name    string `json:"name"`
			Price   struct {
				CurrencyCode string  `json:"currency_code"`
				Price        float64 `json:"price"`
			} `json:"price"`
			PriceWithoutCommission struct {
				CurrencyCode string  `json:"currency_code"`
				Price        float64 `json:"price"`
			} `json:"price_without_commission"`
			CommissionPercent float64 `json:"commission_percent"`
			Commission        struct {
				CurrencyCode string  `json:"currency_code"`
				Price        float64 `json:"price"`
			} `json:"commission"`
			Quantity int `json:"quantity"`
		} `json:"product"`
		Logistic struct {
			TechnicalReturnMoment           time.Time `json:"technical_return_moment"`
			FinalMoment                     time.Time `json:"final_moment"`
			CancelledWithCompensationMoment time.Time `json:"cancelled_with_compensation_moment"`
			ReturnDate                      time.Time `json:"return_date"`
			Barcode                         string    `json:"barcode"`
		} `json:"logistic"`
		Visual struct {
			Status struct {
				ID          int    `json:"id"`
				DisplayName string `json:"display_name"`
				SysName     string `json:"sys_name"`
			} `json:"status"`
			ChangeMoment time.Time `json:"change_moment"`
		} `json:"visual"`
		Exemplars []struct {
			ID int64 `json:"id"`
		} `json:"exemplars"`
		AdditionalInfo struct {
			IsOpened      bool `json:"is_opened"`
			IsSuperEconom bool `json:"is_super_econom"`
		} `json:"additional_info"`
		ClearingID       int64  `json:"clearing_id"`
		PostingNumber    string `json:"posting_number"`
		ReturnClearingID int64  `json:"return_clearing_id"`
		SourceID         int64  `json:"source_id"`
	} `json:"returns"`
	HasNext bool `json:"has_next"`
}

type PackageLabel struct {
	ContentType string `json:"content_type"`
	FileName    string `json:"file_name"`
	FileContent string `json:"file_content"`
}

type StocksNew struct {
	Items []struct {
		Sku                          int      `json:"sku"`
		Name                         string   `json:"name"`
		OfferID                      string   `json:"offer_id"`
		WarehouseID                  int64    `json:"warehouse_id"`
		WarehouseName                string   `json:"warehouse_name"`
		ClusterID                    int      `json:"cluster_id"`
		ClusterName                  string   `json:"cluster_name"`
		ItemTags                     []string `json:"item_tags"`
		Ads                          float64  `json:"ads"`
		DaysWithoutSales             int      `json:"days_without_sales"`
		TurnoverGrade                string   `json:"turnover_grade"`
		Idc                          int      `json:"idc"`
		AvailableStockCount          int      `json:"available_stock_count"`
		ValidStockCount              int      `json:"valid_stock_count"`
		WaitingDocsStockCount        int      `json:"waiting_docs_stock_count"`
		ExpiringStockCount           int      `json:"expiring_stock_count"`
		TransitDefectStockCount      int      `json:"transit_defect_stock_count"`
		StockDefectStockCount        int      `json:"stock_defect_stock_count"`
		ExcessStockCount             int      `json:"excess_stock_count"`
		OtherStockCount              int      `json:"other_stock_count"`
		RequestedStockCount          int      `json:"requested_stock_count"`
		TransitStockCount            int      `json:"transit_stock_count"`
		ReturnFromCustomerStockCount int      `json:"return_from_customer_stock_count"`
		ReturnToSellerStockCount     int      `json:"return_to_seller_stock_count"`
	} `json:"items"`
}

type WarehouseList struct {
	Cursor     string `json:"cursor"`
	HasNext    bool   `json:"has_next"`
	Warehouses []struct {
		AddressInfo struct {
			Address   string  `json:"address"`
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
			Utc       string  `json:"utc"`
		} `json:"address_info"`
		CarriageLabelType string   `json:"carriage_label_type"`
		CourierComment    string   `json:"courier_comment"`
		CourierPhones     []string `json:"courier_phones"`
		CreatedAt         string   `json:"created_at"`
		FirstMile         struct {
			Type                string `json:"type"`
			DropoffPointID      string `json:"dropoff_point_id"`
			TimeslotFrom        string `json:"timeslot_from"`
			TimeslotID          int64  `json:"timeslot_id"`
			TimeslotTo          string `json:"timeslot_to"`
			FirstMileIsChanging bool   `json:"first_mile_is_changing"`
		} `json:"first_mile"`
		HasEntrustedAcceptance bool   `json:"has_entrusted_acceptance"`
		HasPostingsLimit       bool   `json:"has_postings_limit"`
		IsAutoAssembly         bool   `json:"is_auto_assembly"`
		IsKgt                  bool   `json:"is_kgt"`
		IsRfbs                 bool   `json:"is_rfbs"`
		IsWaybillEnabled       bool   `json:"is_waybill_enabled"`
		MinPostingsLimit       int    `json:"min_postings_limit"`
		IsComfort              bool   `json:"is_comfort"`
		IsExpress              bool   `json:"is_express"`
		WarehouseType          string `json:"warehouse_type"`
		CutInTime              int    `json:"cut_in_time"`
		Name                   string `json:"name"`
		Phone                  string `json:"phone"`
		PostingsLimit          int    `json:"postings_limit"`
		SlaCutIn               int    `json:"sla_cut_in"`
		Status                 string `json:"status"`
		Timetable              struct {
			TimetableFrom string `json:"timetable_from"`
			TimetableTo   string `json:"timetable_to"`
			WorkingHours  []struct {
				TimeFrom string `json:"time_from"`
				TimeTo   string `json:"time_to"`
			} `json:"working_hours"`
		} `json:"timetable"`
		UpdatedAt    string   `json:"updated_at"`
		WarehouseID  int      `json:"warehouse_id"`
		WithItemList bool     `json:"with_item_list"`
		WorkingDays  []string `json:"working_days"`
	} `json:"warehouses"`
}

type ProductList struct {
	Result struct {
		Items []struct {
			ProductID    int           `json:"product_id"`
			OfferID      string        `json:"offer_id"`
			HasFboStocks bool          `json:"has_fbo_stocks"`
			HasFbsStocks bool          `json:"has_fbs_stocks"`
			Archived     bool          `json:"archived"`
			IsDiscounted bool          `json:"is_discounted"`
			Quants       []interface{} `json:"quants"`
		} `json:"items"`
		Total  int    `json:"total"`
		LastID string `json:"last_id"`
	} `json:"result"`
}

type ProductListWithAttributes struct {
	Result []struct {
		ID                    int    `json:"id"`
		Barcode               string `json:"barcode"`
		Name                  string `json:"name"`
		OfferID               string `json:"offer_id"`
		Height                int    `json:"height"`
		Depth                 int    `json:"depth"`
		Width                 int    `json:"width"`
		DimensionUnit         string `json:"dimension_unit"`
		Weight                int    `json:"weight"`
		WeightUnit            string `json:"weight_unit"`
		DescriptionCategoryID int    `json:"description_category_id"`
		TypeID                int    `json:"type_id"`
		PrimaryImage          string `json:"primary_image"`
		ModelInfo             struct {
			ModelID int `json:"model_id"`
			Count   int `json:"count"`
		} `json:"model_info"`
		Images     []string      `json:"images"`
		PdfList    []interface{} `json:"pdf_list"`
		Attributes []struct {
			ID        int `json:"id"`
			ComplexID int `json:"complex_id"`
			Values    []struct {
				DictionaryValueID int    `json:"dictionary_value_id"`
				Value             string `json:"value"`
			} `json:"values"`
		} `json:"attributes"`
		ComplexAttributes []struct {
			ID        int `json:"id"`
			ComplexID int `json:"complex_id"`
			Values    []struct {
				DictionaryValueID int    `json:"dictionary_value_id"`
				Value             string `json:"value"`
			} `json:"values"`
		} `json:"complex_attributes"`
		ColorImage string   `json:"color_image"`
		Sku        int      `json:"sku"`
		Barcodes   []string `json:"barcodes"`
	} `json:"result"`
	Total  int    `json:"total"`
	LastID string `json:"last_id"`
}
