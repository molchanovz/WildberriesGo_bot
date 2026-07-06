//nolint:gocritic
package db

import (
	"context"

	"github.com/go-pg/pg/v10/orm"
)

//
//import (
//	"context"
//	"errors"
//	"time"
//)
//
//var (
//	ErrNoFound = errors.New("not found")
//)

func (tr TradebotRepo) DeleteOrdersLastWeek(ctx context.Context) (orm.Result, error) {
	return tr.db.ModelContext(ctx, &Order{}).Where(`"createdAt" < NOW() - INTERVAL '7 days'`).Delete()
}

func (tr TradebotRepo) SelectOrdersByFilter(ctx context.Context) ([]Order, error) {
	var orders []Order
	err := tr.db.ModelContext(ctx, &orders).Select()
	return orders, err
}

//
// AuthenticateUser update authKey and last activity while user login/logout
//func (tr TradebotRepo) AuthenticateUser(ctx context.Context, dbu *User, authKey string) (bool, error) {
//	dbu.AuthKey = authKey
//	now := time.Now()
//	dbu.LastActivityAt = &now
//	return tr.UpdateUser(ctx, dbu, WithColumns(Columns.User.AuthKey, Columns.User.LastActivityAt))
//}
//
//func (cr CommonRepo) UpdateUserActivity(ctx context.Context, dbu *User) (bool, error) {
//	now := time.Now()
//	dbu.LastActivityAt = &now
//	return cr.UpdateUser(ctx, dbu, WithColumns(Columns.User.LastActivityAt))
//}
//
//func (cr CommonRepo) EnabledUserByAuthKey(ctx context.Context, authKey string) (*User, error) {
//	s := StatusEnabled
//	return cr.OneUser(ctx, &UserSearch{AuthKey: &authKey, StatusID: &s})
//}
//
//func (cr CommonRepo) EnabledUserByLogin(ctx context.Context, login string) (*User, error) {
//	s := StatusEnabled
//	return cr.OneUser(ctx, &UserSearch{Login: &login, StatusID: &s})
//}
//
//func (cr CommonRepo) UpdateUserPassword(ctx context.Context, dbu *User) (bool, error) {
//	return cr.UpdateUser(ctx, dbu, WithColumns(Columns.User.Password, Columns.User.AuthKey))
//}
