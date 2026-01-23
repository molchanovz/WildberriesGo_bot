package db

import (
	"context"
	"errors"

	"github.com/go-pg/pg/v10"
	"github.com/go-pg/pg/v10/orm"
)

type TradebotRepo struct {
	db      orm.DB
	filters map[string][]Filter
	sort    map[string][]SortField
	join    map[string][]string
}

// NewTradebotRepo returns new repository
func NewTradebotRepo(db orm.DB) TradebotRepo {
	return TradebotRepo{
		db: db,
		filters: map[string][]Filter{
			Tables.Cabinet.Name: {StatusFilter},
			Tables.Order.Name:   {StatusFilter},
			Tables.User.Name:    {StatusUserFilter},
			Tables.Review.Name:  {StatusFilter},
			Tables.Product.Name: {StatusFilter},
		},
		sort: map[string][]SortField{
			Tables.Cabinet.Name: {{Column: Columns.Cabinet.ID, Direction: SortDesc}},
			Tables.Order.Name:   {{Column: Columns.Order.CreatedAt, Direction: SortDesc}},
			Tables.Stock.Name:   {{Column: Columns.Stock.ID, Direction: SortDesc}},
			Tables.User.Name:    {{Column: Columns.User.CreatedAt, Direction: SortDesc}},
			Tables.Review.Name:  {{Column: Columns.Review.CreatedAt, Direction: SortDesc}},
			Tables.Product.Name: {{Column: Columns.Product.Title, Direction: SortAsc}},
		},
		join: map[string][]string{
			Tables.Cabinet.Name: {TableColumns},
			Tables.Order.Name:   {TableColumns, Columns.Order.Cabinet},
			Tables.Stock.Name:   {TableColumns, Columns.Stock.Cabinet},
			Tables.User.Name:    {TableColumns},
			Tables.Review.Name:  {TableColumns, Columns.Review.Cabinet},
			Tables.Product.Name: {TableColumns, Columns.Product.Cabinet},
		},
	}
}

// WithTransaction is a function that wraps TradebotRepo with pg.Tx transaction.
func (tr TradebotRepo) WithTransaction(tx *pg.Tx) TradebotRepo {
	tr.db = tx
	return tr
}

// WithEnabledOnly is a function that adds "statusId"=1 as base filter.
func (tr TradebotRepo) WithEnabledOnly() TradebotRepo {
	f := make(map[string][]Filter, len(tr.filters))
	for i := range tr.filters {
		f[i] = make([]Filter, len(tr.filters[i]))
		copy(f[i], tr.filters[i])
		f[i] = append(f[i], StatusEnabledFilter)
	}
	tr.filters = f

	return tr
}

/*** Cabinet ***/

// FullCabinet returns full joins with all columns
func (tr TradebotRepo) FullCabinet() OpFunc {
	return WithColumns(tr.join[Tables.Cabinet.Name]...)
}

// DefaultCabinetSort returns default sort.
func (tr TradebotRepo) DefaultCabinetSort() OpFunc {
	return WithSort(tr.sort[Tables.Cabinet.Name]...)
}

// CabinetByID is a function that returns Cabinet by ID(s) or nil.
func (tr TradebotRepo) CabinetByID(ctx context.Context, id int, ops ...OpFunc) (*Cabinet, error) {
	return tr.OneCabinet(ctx, &CabinetSearch{ID: &id}, ops...)
}

// OneCabinet is a function that returns one Cabinet by filters. It could return pg.ErrMultiRows.
func (tr TradebotRepo) OneCabinet(ctx context.Context, search *CabinetSearch, ops ...OpFunc) (*Cabinet, error) {
	obj := &Cabinet{}
	err := buildQuery(ctx, tr.db, obj, search, tr.filters[Tables.Cabinet.Name], PagerTwo, ops...).Select()

	if errors.Is(err, pg.ErrMultiRows) {
		return nil, err
	} else if errors.Is(err, pg.ErrNoRows) {
		return nil, nil
	}

	return obj, err
}

// CabinetsByFilters returns Cabinet list.
func (tr TradebotRepo) CabinetsByFilters(ctx context.Context, search *CabinetSearch, pager Pager, ops ...OpFunc) (cabinets []Cabinet, err error) {
	err = buildQuery(ctx, tr.db, &cabinets, search, tr.filters[Tables.Cabinet.Name], pager, ops...).Select()
	return
}

// CountCabinets returns count
func (tr TradebotRepo) CountCabinets(ctx context.Context, search *CabinetSearch, ops ...OpFunc) (int, error) {
	return buildQuery(ctx, tr.db, &Cabinet{}, search, tr.filters[Tables.Cabinet.Name], PagerOne, ops...).Count()
}

// AddCabinet adds Cabinet to DB.
func (tr TradebotRepo) AddCabinet(ctx context.Context, cabinet *Cabinet, ops ...OpFunc) (*Cabinet, error) {
	q := tr.db.ModelContext(ctx, cabinet)
	applyOps(q, ops...)
	_, err := q.Insert()

	return cabinet, err
}

// UpdateCabinet updates Cabinet in DB.
func (tr TradebotRepo) UpdateCabinet(ctx context.Context, cabinet *Cabinet, ops ...OpFunc) (bool, error) {
	q := tr.db.ModelContext(ctx, cabinet).WherePK()
	if len(ops) == 0 {
		q = q.ExcludeColumn(Columns.Cabinet.ID)
	}
	applyOps(q, ops...)
	res, err := q.Update()
	if err != nil {
		return false, err
	}

	return res.RowsAffected() > 0, err
}

// DeleteCabinet set statusId to deleted in DB.
func (tr TradebotRepo) DeleteCabinet(ctx context.Context, id int) (deleted bool, err error) {
	cabinet := &Cabinet{ID: id, StatusID: StatusDeleted}

	return tr.UpdateCabinet(ctx, cabinet, WithColumns(Columns.Cabinet.StatusID))
}

/*** Order ***/

// FullOrder returns full joins with all columns
func (tr TradebotRepo) FullOrder() OpFunc {
	return WithColumns(tr.join[Tables.Order.Name]...)
}

// DefaultOrderSort returns default sort.
func (tr TradebotRepo) DefaultOrderSort() OpFunc {
	return WithSort(tr.sort[Tables.Order.Name]...)
}

// OrderByID is a function that returns Order by ID(s) or nil.
func (tr TradebotRepo) OrderByID(ctx context.Context, id int, ops ...OpFunc) (*Order, error) {
	return tr.OneOrder(ctx, &OrderSearch{ID: &id}, ops...)
}

// OneOrder is a function that returns one Order by filters. It could return pg.ErrMultiRows.
func (tr TradebotRepo) OneOrder(ctx context.Context, search *OrderSearch, ops ...OpFunc) (*Order, error) {
	obj := &Order{}
	err := buildQuery(ctx, tr.db, obj, search, tr.filters[Tables.Order.Name], PagerTwo, ops...).Select()

	if errors.Is(err, pg.ErrMultiRows) {
		return nil, err
	} else if errors.Is(err, pg.ErrNoRows) {
		return nil, nil
	}

	return obj, err
}

// OrdersByFilters returns Order list.
func (tr TradebotRepo) OrdersByFilters(ctx context.Context, search *OrderSearch, pager Pager, ops ...OpFunc) (orders []Order, err error) {
	err = buildQuery(ctx, tr.db, &orders, search, tr.filters[Tables.Order.Name], pager, ops...).Select()
	return
}

// CountOrders returns count
func (tr TradebotRepo) CountOrders(ctx context.Context, search *OrderSearch, ops ...OpFunc) (int, error) {
	return buildQuery(ctx, tr.db, &Order{}, search, tr.filters[Tables.Order.Name], PagerOne, ops...).Count()
}

// AddOrder adds Order to DB.
func (tr TradebotRepo) AddOrder(ctx context.Context, order *Order, ops ...OpFunc) (*Order, error) {
	q := tr.db.ModelContext(ctx, order)
	if len(ops) == 0 {
		q = q.ExcludeColumn(Columns.Order.CreatedAt)
	}
	applyOps(q, ops...)
	_, err := q.Insert()

	return order, err
}

// UpdateOrder updates Order in DB.
func (tr TradebotRepo) UpdateOrder(ctx context.Context, order *Order, ops ...OpFunc) (bool, error) {
	q := tr.db.ModelContext(ctx, order).WherePK()
	if len(ops) == 0 {
		q = q.ExcludeColumn(Columns.Order.ID, Columns.Order.CreatedAt)
	}
	applyOps(q, ops...)
	res, err := q.Update()
	if err != nil {
		return false, err
	}

	return res.RowsAffected() > 0, err
}

// DeleteOrder set statusId to deleted in DB.
func (tr TradebotRepo) DeleteOrder(ctx context.Context, id int) (deleted bool, err error) {
	order := &Order{ID: id, StatusID: StatusDeleted}

	return tr.UpdateOrder(ctx, order, WithColumns(Columns.Order.StatusID))
}

/*** Stock ***/

// FullStock returns full joins with all columns
func (tr TradebotRepo) FullStock() OpFunc {
	return WithColumns(tr.join[Tables.Stock.Name]...)
}

// DefaultStockSort returns default sort.
func (tr TradebotRepo) DefaultStockSort() OpFunc {
	return WithSort(tr.sort[Tables.Stock.Name]...)
}

// StockByID is a function that returns Stock by ID(s) or nil.
func (tr TradebotRepo) StockByID(ctx context.Context, id int, ops ...OpFunc) (*Stock, error) {
	return tr.OneStock(ctx, &StockSearch{ID: &id}, ops...)
}

// OneStock is a function that returns one Stock by filters. It could return pg.ErrMultiRows.
func (tr TradebotRepo) OneStock(ctx context.Context, search *StockSearch, ops ...OpFunc) (*Stock, error) {
	obj := &Stock{}
	err := buildQuery(ctx, tr.db, obj, search, tr.filters[Tables.Stock.Name], PagerTwo, ops...).Select()

	if errors.Is(err, pg.ErrMultiRows) {
		return nil, err
	} else if errors.Is(err, pg.ErrNoRows) {
		return nil, nil
	}

	return obj, err
}

// StocksByFilters returns Stock list.
func (tr TradebotRepo) StocksByFilters(ctx context.Context, search *StockSearch, pager Pager, ops ...OpFunc) (stocks []Stock, err error) {
	err = buildQuery(ctx, tr.db, &stocks, search, tr.filters[Tables.Stock.Name], pager, ops...).Select()
	return
}

// CountStocks returns count
func (tr TradebotRepo) CountStocks(ctx context.Context, search *StockSearch, ops ...OpFunc) (int, error) {
	return buildQuery(ctx, tr.db, &Stock{}, search, tr.filters[Tables.Stock.Name], PagerOne, ops...).Count()
}

// AddStock adds Stock to DB.
func (tr TradebotRepo) AddStock(ctx context.Context, stock *Stock, ops ...OpFunc) (*Stock, error) {
	q := tr.db.ModelContext(ctx, stock)
	applyOps(q, ops...)
	_, err := q.Insert()

	return stock, err
}

// UpdateStock updates Stock in DB.
func (tr TradebotRepo) UpdateStock(ctx context.Context, stock *Stock, ops ...OpFunc) (bool, error) {
	q := tr.db.ModelContext(ctx, stock).WherePK()
	if len(ops) == 0 {
		q = q.ExcludeColumn(Columns.Stock.ID)
	}
	applyOps(q, ops...)
	res, err := q.Update()
	if err != nil {
		return false, err
	}

	return res.RowsAffected() > 0, err
}

// DeleteStock deletes Stock from DB.
func (tr TradebotRepo) DeleteStock(ctx context.Context, id int) (deleted bool, err error) {
	stock := &Stock{ID: id}

	res, err := tr.db.ModelContext(ctx, stock).WherePK().Delete()
	if err != nil {
		return false, err
	}

	return res.RowsAffected() > 0, err
}

/*** User ***/

// FullUser returns full joins with all columns
func (tr TradebotRepo) FullUser() OpFunc {
	return WithColumns(tr.join[Tables.User.Name]...)
}

// DefaultUserSort returns default sort.
func (tr TradebotRepo) DefaultUserSort() OpFunc {
	return WithSort(tr.sort[Tables.User.Name]...)
}

// UserByID is a function that returns User by ID(s) or nil.
func (tr TradebotRepo) UserByID(ctx context.Context, id int, ops ...OpFunc) (*User, error) {
	return tr.OneUser(ctx, &UserSearch{ID: &id}, ops...)
}

// OneUser is a function that returns one User by filters. It could return pg.ErrMultiRows.
func (tr TradebotRepo) OneUser(ctx context.Context, search *UserSearch, ops ...OpFunc) (*User, error) {
	obj := &User{}
	err := buildQuery(ctx, tr.db, obj, search, tr.filters[Tables.User.Name], PagerTwo, ops...).Select()

	if errors.Is(err, pg.ErrMultiRows) {
		return nil, err
	} else if errors.Is(err, pg.ErrNoRows) {
		return nil, nil
	}

	return obj, err
}

// UsersByFilters returns User list.
func (tr TradebotRepo) UsersByFilters(ctx context.Context, search *UserSearch, pager Pager, ops ...OpFunc) (users []User, err error) {
	err = buildQuery(ctx, tr.db, &users, search, tr.filters[Tables.User.Name], pager, ops...).Select()
	return
}

// CountUsers returns count
func (tr TradebotRepo) CountUsers(ctx context.Context, search *UserSearch, ops ...OpFunc) (int, error) {
	return buildQuery(ctx, tr.db, &User{}, search, tr.filters[Tables.User.Name], PagerOne, ops...).Count()
}

// AddUser adds User to DB.
func (tr TradebotRepo) AddUser(ctx context.Context, user *User, ops ...OpFunc) (*User, error) {
	q := tr.db.ModelContext(ctx, user)
	if len(ops) == 0 {
		q = q.ExcludeColumn(Columns.User.CreatedAt)
	}
	applyOps(q, ops...)
	_, err := q.Insert()

	return user, err
}

// UpdateUser updates User in DB.
func (tr TradebotRepo) UpdateUser(ctx context.Context, user *User, ops ...OpFunc) (bool, error) {
	q := tr.db.ModelContext(ctx, user).WherePK()
	if len(ops) == 0 {
		q = q.ExcludeColumn(Columns.User.ID, Columns.User.CreatedAt)
	}
	applyOps(q, ops...)
	res, err := q.Update()
	if err != nil {
		return false, err
	}

	return res.RowsAffected() > 0, err
}

// DeleteUser set statusId to deleted in DB.
func (tr TradebotRepo) DeleteUser(ctx context.Context, id int) (deleted bool, err error) {
	user := &User{ID: id, StatusID: StatusDeleted}

	return tr.UpdateUser(ctx, user, WithColumns(Columns.User.StatusID))
}

/*** Review ***/

// FullReview returns full joins with all columns
func (tr TradebotRepo) FullReview() OpFunc {
	return WithColumns(tr.join[Tables.Review.Name]...)
}

// DefaultReviewSort returns default sort.
func (tr TradebotRepo) DefaultReviewSort() OpFunc {
	return WithSort(tr.sort[Tables.Review.Name]...)
}

// ReviewByID is a function that returns Review by ID(s) or nil.
func (tr TradebotRepo) ReviewByID(ctx context.Context, id int, ops ...OpFunc) (*Review, error) {
	return tr.OneReview(ctx, &ReviewSearch{ID: &id}, ops...)
}

// OneReview is a function that returns one Review by filters. It could return pg.ErrMultiRows.
func (tr TradebotRepo) OneReview(ctx context.Context, search *ReviewSearch, ops ...OpFunc) (*Review, error) {
	obj := &Review{}
	err := buildQuery(ctx, tr.db, obj, search, tr.filters[Tables.Review.Name], PagerTwo, ops...).Select()

	if errors.Is(err, pg.ErrMultiRows) {
		return nil, err
	} else if errors.Is(err, pg.ErrNoRows) {
		return nil, nil
	}

	return obj, err
}

// ReviewsByFilters returns Review list.
func (tr TradebotRepo) ReviewsByFilters(ctx context.Context, search *ReviewSearch, pager Pager, ops ...OpFunc) (reviews []Review, err error) {
	err = buildQuery(ctx, tr.db, &reviews, search, tr.filters[Tables.Review.Name], pager, ops...).Select()
	return
}

// CountReviews returns count
func (tr TradebotRepo) CountReviews(ctx context.Context, search *ReviewSearch, ops ...OpFunc) (int, error) {
	return buildQuery(ctx, tr.db, &Review{}, search, tr.filters[Tables.Review.Name], PagerOne, ops...).Count()
}

// AddReview adds Review to DB.
func (tr TradebotRepo) AddReview(ctx context.Context, review *Review, ops ...OpFunc) (*Review, error) {
	q := tr.db.ModelContext(ctx, review)
	if len(ops) == 0 {
		q = q.ExcludeColumn(Columns.Review.CreatedAt)
	}
	applyOps(q, ops...)
	_, err := q.Insert()

	return review, err
}

// UpdateReview updates Review in DB.
func (tr TradebotRepo) UpdateReview(ctx context.Context, review *Review, ops ...OpFunc) (bool, error) {
	q := tr.db.ModelContext(ctx, review).WherePK()
	if len(ops) == 0 {
		q = q.ExcludeColumn(Columns.Review.ID, Columns.Review.CreatedAt)
	}
	applyOps(q, ops...)
	res, err := q.Update()
	if err != nil {
		return false, err
	}

	return res.RowsAffected() > 0, err
}

// DeleteReview set statusId to deleted in DB.
func (tr TradebotRepo) DeleteReview(ctx context.Context, id int) (deleted bool, err error) {
	review := &Review{ID: id, StatusID: StatusDeleted}

	return tr.UpdateReview(ctx, review, WithColumns(Columns.Review.StatusID))
}

/*** Product ***/

// FullProduct returns full joins with all columns
func (tr TradebotRepo) FullProduct() OpFunc {
	return WithColumns(tr.join[Tables.Product.Name]...)
}

// DefaultProductSort returns default sort.
func (tr TradebotRepo) DefaultProductSort() OpFunc {
	return WithSort(tr.sort[Tables.Product.Name]...)
}

// ProductByID is a function that returns Product by ID(s) or nil.
func (tr TradebotRepo) ProductByID(ctx context.Context, id int, ops ...OpFunc) (*Product, error) {
	return tr.OneProduct(ctx, &ProductSearch{ID: &id}, ops...)
}

// OneProduct is a function that returns one Product by filters. It could return pg.ErrMultiRows.
func (tr TradebotRepo) OneProduct(ctx context.Context, search *ProductSearch, ops ...OpFunc) (*Product, error) {
	obj := &Product{}
	err := buildQuery(ctx, tr.db, obj, search, tr.filters[Tables.Product.Name], PagerTwo, ops...).Select()

	if errors.Is(err, pg.ErrMultiRows) {
		return nil, err
	} else if errors.Is(err, pg.ErrNoRows) {
		return nil, nil
	}

	return obj, err
}

// ProductsByFilters returns Product list.
func (tr TradebotRepo) ProductsByFilters(ctx context.Context, search *ProductSearch, pager Pager, ops ...OpFunc) (products []Product, err error) {
	err = buildQuery(ctx, tr.db, &products, search, tr.filters[Tables.Product.Name], pager, ops...).Select()
	return
}

// CountProducts returns count
func (tr TradebotRepo) CountProducts(ctx context.Context, search *ProductSearch, ops ...OpFunc) (int, error) {
	return buildQuery(ctx, tr.db, &Product{}, search, tr.filters[Tables.Product.Name], PagerOne, ops...).Count()
}

// AddProduct adds Product to DB.
func (tr TradebotRepo) AddProduct(ctx context.Context, product *Product, ops ...OpFunc) (*Product, error) {
	q := tr.db.ModelContext(ctx, product)
	applyOps(q, ops...)
	_, err := q.Insert()

	return product, err
}

// UpdateProduct updates Product in DB.
func (tr TradebotRepo) UpdateProduct(ctx context.Context, product *Product, ops ...OpFunc) (bool, error) {
	q := tr.db.ModelContext(ctx, product).WherePK()
	if len(ops) == 0 {
		q = q.ExcludeColumn(Columns.Product.ID)
	}
	applyOps(q, ops...)
	res, err := q.Update()
	if err != nil {
		return false, err
	}

	return res.RowsAffected() > 0, err
}

// DeleteProduct set statusId to deleted in DB.
func (tr TradebotRepo) DeleteProduct(ctx context.Context, id int) (deleted bool, err error) {
	product := &Product{ID: id, StatusID: StatusDeleted}

	return tr.UpdateProduct(ctx, product, WithColumns(Columns.Product.StatusID))
}
