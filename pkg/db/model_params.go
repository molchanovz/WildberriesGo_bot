package db

type CabinetSettings struct {
	ShipmentsSheetID string `json:"shipmentsSheetId"`
	MainWarehouseIDs []int  `json:"mainWarehouseIds"`
}
