package tradeplus

import (
	"context"
	"errors"
	"fmt"
	"time"

	"tradebot/pkg/client/ozon"
	"tradebot/pkg/db"

	"github.com/go-pg/pg/v10"
)

type Manager struct {
	repo db.TradebotRepo
	db   db.DB
}

func NewManager(dbc db.DB) *Manager {
	return &Manager{repo: db.NewTradebotRepo(dbc), db: dbc}
}

func (m Manager) UserByChatID(ctx context.Context, chatID int64) (*User, error) {
	user, err := m.repo.OneUser(ctx, &db.UserSearch{TgID: Pointer(chatID)})

	return NewUser(user), err
}

func (m Manager) SetUserStatus(ctx context.Context, user *User, status int) (bool, error) {
	user.StatusID = status

	return m.repo.UpdateUser(ctx, &user.User, db.WithColumns(db.Columns.User.StatusID))
}

func (m Manager) CreateUser(ctx context.Context, chatID int64) (*User, error) {
	var dbUser *db.User
	err := m.db.RunInLock(ctx, fmt.Sprintf("user-%d", chatID), func(tx *pg.Tx) error {
		repo := m.repo.WithTransaction(tx)

		// check user from db if exists
		var err error
		dbUser, err = repo.OneUser(ctx, &db.UserSearch{TgID: Pointer(chatID)})
		if err != nil {
			return fmt.Errorf("get user failed: %w", err)
		}

		if dbUser == nil {
			// create user
			dbUser, err = repo.AddUser(ctx, NewUserFromChatID(chatID).ToDB())
			if err != nil {
				return fmt.Errorf("add user failed: %w", err)
			}
		} else {
			// set enabled status
			dbUser.StatusID = db.StatusEnabled
			if _, err = repo.UpdateUser(ctx, dbUser); err != nil {
				return fmt.Errorf("update user failed: %w", err)
			}
		}

		return nil
	})

	return NewUser(dbUser), err
}

func (m Manager) GetCabinetsByMp(ctx context.Context, mp string) ([]Cabinet, error) {
	dbCabinets, err := m.repo.CabinetsByFilters(ctx, &db.CabinetSearch{Marketplace: Pointer(mp)}, db.PagerNoLimit)
	if err != nil {
		return nil, err
	}
	return NewCabinets(dbCabinets), nil
}

func (m Manager) GetCabinetByID(ctx context.Context, id int) (Cabinet, error) {
	cabinet, err := m.repo.CabinetByID(ctx, id)
	if err != nil {
		return Cabinet{}, err
	} else if cabinet == nil {
		return Cabinet{}, errors.New("кабинет не найден")
	}

	return *NewCabinet(cabinet), err
}

func (m Manager) GetPrintedOrders(ctx context.Context, id int) (map[string]struct{}, error) {
	printedOrdersMap := make(map[string]struct{})

	printedOrders, err := m.repo.OrdersByFilters(ctx, &db.OrderSearch{CabinetID: Pointer(id)}, db.PagerNoLimit)
	if err != nil {
		return nil, err
	}

	for _, order := range printedOrders {
		printedOrdersMap[order.PostingNumber] = struct{}{}
	}

	return printedOrdersMap, nil
}

func (m Manager) CreateOrders(ctx context.Context, cabinetID int, newOrders ozon.PostingslistFbs) error {
	return m.db.RunInTransaction(ctx, func(tx *pg.Tx) error {
		repo := m.repo.WithTransaction(tx)
		for _, order := range newOrders.Result.PostingsFBS {
			dbOrder := db.Order{
				PostingNumber: order.PostingNumber,
				CabinetID:     cabinetID,
				Article:       order.Products[0].OfferID,
				CreatedAt:     time.Now(),
				StatusID:      db.StatusEnabled,
			}
			if _, err := repo.AddOrder(ctx, &dbOrder); err != nil {
				return err
			}
		}
		return nil
	})
}

func (m Manager) DeleteOrders(ctx context.Context) error {
	_, err := m.repo.DeleteOrdersLastWeek(ctx)
	return err
}

func (m Manager) UpdateCabinet(ctx context.Context, cabinet Cabinet) error {
	_, err := m.repo.UpdateCabinet(ctx, &cabinet.Cabinet)
	return err
}
func (m Manager) GetReviewByID(ctx context.Context, reviewID string) (*Review, error) {
	review, err := m.repo.OneReview(ctx, &db.ReviewSearch{ExternalID: &reviewID})
	return NewReview(review), err
}

func (m Manager) UpdateReview(ctx context.Context, review *Review) (*Review, error) {
	_, err := m.repo.UpdateReview(ctx, review.ToDB(), db.WithColumns(db.Columns.Review.Answer))
	if err != nil {
		return nil, err
	}
	return review, nil
}

func Pointer[T any](in T) *T {
	return &in
}
