package tradeplus

import "tradebot/pkg/db"

type Authorization struct {
	ClientID, Token, Type string
}

//go:generate colgen

type Product struct {
	db.Product
	Cabinet         *Cabinet
	Recommendations Products
}

func NewProduct(in *db.Product) *Product {
	if in == nil {
		return nil
	}

	return &Product{
		Product: *in,
		Cabinet: NewCabinet(in.Cabinet),
	}
}

type Cabinet struct {
	db.Cabinet
}

func NewCabinet(in *db.Cabinet) *Cabinet {
	if in == nil {
		return nil
	}

	return &Cabinet{
		Cabinet: *in,
	}
}

type User struct {
	db.User
}

func NewUser(in *db.User) *User {
	if in == nil {
		return nil
	}

	return &User{
		User: *in,
	}
}

type Cabinets []Cabinet

func NewUserFromChatID(chatID int64) *User {
	return &User{
		db.User{
			TgID:       chatID,
			StatusID:   db.StatusEnabled,
			CabinetIDs: make([]int, 0),
		},
	}
}

func (u *User) ToDB() *db.User {
	return &u.User
}

func NewCabinets(in []db.Cabinet) Cabinets {
	newCabinets := Cabinets{}
	for _, c := range in {
		newCabinets = append(newCabinets, *NewCabinet(&c))
	}
	return newCabinets
}

// Ptr is a generic to create pointer from value
func Ptr[T any](v T) *T {
	return &v
}

// Deref is a generic to dereference pointer with fallback to "def" value
func Deref[T any](v *T, def T) T {
	if v == nil {
		return def
	}

	return *v
}
