package db

type CabinetSettings struct {
	ShipmentsSheetID              string   `json:"shipmentsSheetId"`
	ShipmentsAllSheetID           string   `json:"shipmentsAllSheetId,omitempty"`
	ExcludedShipmentsWarehouseIDs []string `json:"excludedShipmentsWarehouseIds,omitempty"`
}
