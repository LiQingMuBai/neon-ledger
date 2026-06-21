package orders

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

type MySQLStore struct {
	db  *sql.DB
	now func() time.Time
}

func NewMySQLStore(db *sql.DB) *MySQLStore {
	return &MySQLStore{
		db:  db,
		now: time.Now,
	}
}

func EnsureMySQLSchema(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS bookkeeping_orders (
	id CHAR(32) NOT NULL PRIMARY KEY,
	customer_order_no VARCHAR(64) NOT NULL,
	platform_order_no VARCHAR(64) NOT NULL,
	telegram_user_id BIGINT NOT NULL,
	amount BIGINT NOT NULL,
	phone VARCHAR(32) NOT NULL,
	status VARCHAR(32) NOT NULL DEFAULT 'pending',
	notify_status VARCHAR(32) NOT NULL DEFAULT 'pending',
	callback_url VARCHAR(512) NOT NULL DEFAULT '',
	created_at DATETIME(6) NOT NULL,
	updated_at DATETIME(6) NOT NULL,
	deleted_at DATETIME(6) NULL,
	UNIQUE KEY uk_customer_order_no (customer_order_no),
	UNIQUE KEY uk_platform_order_no (platform_order_no),
	INDEX idx_telegram_user_created_at (telegram_user_id, created_at DESC),
	INDEX idx_phone_created_at (phone, created_at DESC),
	INDEX idx_status_created_at (status, created_at DESC),
	INDEX idx_notify_status_created_at (notify_status, created_at DESC),
	INDEX idx_created_at (created_at DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
`)
	if err != nil {
		return err
	}
	return ensureCallbackURLColumn(ctx, db)
}

func ensureCallbackURLColumn(ctx context.Context, db *sql.DB) error {
	var count int
	if err := db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = DATABASE()
	AND TABLE_NAME = 'bookkeeping_orders'
	AND COLUMN_NAME = 'callback_url'
`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err := db.ExecContext(ctx, `
ALTER TABLE bookkeeping_orders
ADD COLUMN callback_url VARCHAR(512) NOT NULL DEFAULT '' AFTER notify_status
`)
	return err
}

func (s *MySQLStore) Create(req CreateOrderRequest) (Order, error) {
	req = normalizeOrderRequest(req)
	now := s.now().UTC()
	if err := req.validate(now); err != nil {
		return Order{}, err
	}

	order := Order{
		ID:              newID(),
		CustomerOrderNo: req.CustomerOrderNo,
		PlatformOrderNo: newPlatformOrderNo(now),
		TelegramUserID:  req.TelegramUserID,
		Amount:          req.Amount,
		Phone:           req.Phone,
		Status:          req.Status,
		NotifyStatus:    req.NotifyStatus,
		CallbackURL:     req.CallbackURL,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	_, err := s.db.Exec(`
INSERT INTO bookkeeping_orders (
	id, customer_order_no, platform_order_no, telegram_user_id, amount, phone, status, notify_status, callback_url, created_at, updated_at, deleted_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		order.ID,
		order.CustomerOrderNo,
		order.PlatformOrderNo,
		order.TelegramUserID,
		order.Amount,
		order.Phone,
		order.Status,
		order.NotifyStatus,
		order.CallbackURL,
		order.CreatedAt,
		order.UpdatedAt,
		order.DeletedAt,
	)
	if err != nil {
		return Order{}, err
	}

	return order, nil
}

func (s *MySQLStore) Get(id string) (Order, error) {
	return s.getByID(id, false)
}

func (s *MySQLStore) Update(id string, req UpdateOrderRequest) (Order, error) {
	req = normalizeUpdateOrderRequest(req)
	if err := req.validate(); err != nil {
		return Order{}, err
	}

	now := s.now().UTC()
	result, err := s.db.Exec(`
UPDATE bookkeeping_orders
SET customer_order_no = ?, telegram_user_id = ?, amount = ?, phone = ?, status = ?, notify_status = ?, callback_url = ?, updated_at = ?
WHERE id = ? AND deleted_at IS NULL`,
		req.CustomerOrderNo,
		req.TelegramUserID,
		req.Amount,
		req.Phone,
		req.Status,
		req.NotifyStatus,
		req.CallbackURL,
		now,
		id,
	)
	if err != nil {
		return Order{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Order{}, err
	}
	if affected == 0 {
		return Order{}, ErrNotFound
	}

	return s.getByID(id, false)
}

func (s *MySQLStore) Delete(id string) error {
	now := s.now().UTC()
	result, err := s.db.Exec(`
UPDATE bookkeeping_orders
SET updated_at = ?, deleted_at = ?
WHERE id = ? AND deleted_at IS NULL`, now, now, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *MySQLStore) getByID(id string, includeDeleted bool) (Order, error) {
	where := "WHERE id = ? AND deleted_at IS NULL"
	if includeDeleted {
		where = "WHERE id = ?"
	}

	var order Order
	err := s.db.QueryRow(`
SELECT id, customer_order_no, platform_order_no, telegram_user_id, amount, phone, status, notify_status, callback_url, created_at, updated_at, deleted_at
FROM bookkeeping_orders
`+where, id).Scan(
		&order.ID,
		&order.CustomerOrderNo,
		&order.PlatformOrderNo,
		&order.TelegramUserID,
		&order.Amount,
		&order.Phone,
		&order.Status,
		&order.NotifyStatus,
		&order.CallbackURL,
		&order.CreatedAt,
		&order.UpdatedAt,
		&order.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Order{}, ErrNotFound
		}
		return Order{}, err
	}
	return order, nil
}

func (s *MySQLStore) Query(req QueryOrdersRequest) (QueryOrdersResponse, error) {
	normalizeQueryOrdersRequest(&req)

	where, args := buildOrderWhereClause(req)

	var total int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM bookkeeping_orders "+where, args...).Scan(&total); err != nil {
		return QueryOrdersResponse{}, err
	}
	if req.Offset >= total {
		return QueryOrdersResponse{
			Items:  []Order{},
			Total:  total,
			Limit:  req.Limit,
			Offset: req.Offset,
		}, nil
	}

	queryArgs := append(append([]any{}, args...), req.Limit, req.Offset)
	rows, err := s.db.Query(`
SELECT id, customer_order_no, platform_order_no, telegram_user_id, amount, phone, status, notify_status, callback_url, created_at, updated_at, deleted_at
FROM bookkeeping_orders `+where+`
ORDER BY created_at DESC
LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return QueryOrdersResponse{}, err
	}
	defer rows.Close()

	items := make([]Order, 0, req.Limit)
	for rows.Next() {
		var order Order
		if err := rows.Scan(
			&order.ID,
			&order.CustomerOrderNo,
			&order.PlatformOrderNo,
			&order.TelegramUserID,
			&order.Amount,
			&order.Phone,
			&order.Status,
			&order.NotifyStatus,
			&order.CallbackURL,
			&order.CreatedAt,
			&order.UpdatedAt,
			&order.DeletedAt,
		); err != nil {
			return QueryOrdersResponse{}, err
		}
		items = append(items, order)
	}
	if err := rows.Err(); err != nil {
		return QueryOrdersResponse{}, err
	}

	return QueryOrdersResponse{
		Items:  items,
		Total:  total,
		Limit:  req.Limit,
		Offset: req.Offset,
	}, nil
}

func (s *MySQLStore) DailyTotals(req DailyTotalsRequest) ([]DailyTotal, error) {
	where, args := buildDailyTotalsWhereClause(req)

	rows, err := s.db.Query(`
SELECT DATE_FORMAT(created_at, '%Y-%m-%d') AS order_date, COALESCE(SUM(amount), 0) AS total_amount, COUNT(*) AS order_count
FROM bookkeeping_orders `+where+`
GROUP BY order_date
ORDER BY order_date ASC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]DailyTotal, 0)
	for rows.Next() {
		var item DailyTotal
		if err := rows.Scan(&item.Date, &item.TotalAmount, &item.OrderCount); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func (s *MySQLStore) DailyStatusTotals(req DailyTotalsRequest) ([]DailyStatusTotal, error) {
	where, args := buildDailyStatusTotalsWhereClause(req)

	rows, err := s.db.Query(`
SELECT DATE_FORMAT(created_at, '%Y-%m-%d') AS order_date, status, COALESCE(SUM(amount), 0) AS total_amount, COUNT(*) AS order_count
FROM bookkeeping_orders `+where+`
GROUP BY order_date, status
ORDER BY order_date ASC, status ASC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]DailyStatusTotal, 0)
	for rows.Next() {
		var item DailyStatusTotal
		if err := rows.Scan(&item.Date, &item.Status, &item.TotalAmount, &item.OrderCount); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func normalizeQueryOrdersRequest(req *QueryOrdersRequest) {
	req.CustomerOrderNo = strings.TrimSpace(req.CustomerOrderNo)
	req.PlatformOrderNo = strings.TrimSpace(req.PlatformOrderNo)
	req.Phone = strings.TrimSpace(req.Phone)
	req.Status = strings.ToLower(strings.TrimSpace(req.Status))
	req.NotifyStatus = strings.ToLower(strings.TrimSpace(req.NotifyStatus))
	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Limit > 100 {
		req.Limit = 100
	}
	if req.Offset < 0 {
		req.Offset = 0
	}
}

func buildOrderWhereClause(req QueryOrdersRequest) (string, []any) {
	clauses := make([]string, 0, 5)
	args := make([]any, 0, 5)

	if !req.IncludeDeleted {
		clauses = append(clauses, "deleted_at IS NULL")
	}
	if req.CustomerOrderNo != "" {
		clauses = append(clauses, "customer_order_no = ?")
		args = append(args, req.CustomerOrderNo)
	}
	if req.PlatformOrderNo != "" {
		clauses = append(clauses, "platform_order_no = ?")
		args = append(args, req.PlatformOrderNo)
	}
	if req.TelegramUserID > 0 {
		clauses = append(clauses, "telegram_user_id = ?")
		args = append(args, req.TelegramUserID)
	}
	if req.Phone != "" {
		clauses = append(clauses, "phone LIKE ?")
		args = append(args, "%"+escapeLikePattern(req.Phone)+"%")
	}
	if req.Status != "" {
		clauses = append(clauses, "status = ?")
		args = append(args, req.Status)
	}
	if req.NotifyStatus != "" {
		clauses = append(clauses, "notify_status = ?")
		args = append(args, req.NotifyStatus)
	}
	if req.StartTime != nil {
		clauses = append(clauses, "created_at >= ?")
		args = append(args, req.StartTime.UTC())
	}
	if req.EndTime != nil {
		clauses = append(clauses, "created_at <= ?")
		args = append(args, req.EndTime.UTC())
	}
	if len(clauses) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

func escapeLikePattern(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	value = strings.ReplaceAll(value, `_`, `\_`)
	return value
}

func buildDailyTotalsWhereClause(req DailyTotalsRequest) (string, []any) {
	clauses := make([]string, 0, 5)
	args := make([]any, 0, 5)

	if !req.IncludeDeleted {
		clauses = append(clauses, "deleted_at IS NULL")
	}
	if req.Status != "" {
		clauses = append(clauses, "status = ?")
		args = append(args, req.Status)
	}
	if req.NotifyStatus != "" {
		clauses = append(clauses, "notify_status = ?")
		args = append(args, req.NotifyStatus)
	}
	if req.StartTime != nil {
		clauses = append(clauses, "created_at >= ?")
		args = append(args, req.StartTime.UTC())
	}
	if req.EndTime != nil {
		clauses = append(clauses, "created_at <= ?")
		args = append(args, req.EndTime.UTC())
	}
	if len(clauses) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

func buildDailyStatusTotalsWhereClause(req DailyTotalsRequest) (string, []any) {
	clauses := make([]string, 0, 4)
	args := make([]any, 0, 4)

	if !req.IncludeDeleted {
		clauses = append(clauses, "deleted_at IS NULL")
	}
	if req.NotifyStatus != "" {
		clauses = append(clauses, "notify_status = ?")
		args = append(args, req.NotifyStatus)
	}
	if req.StartTime != nil {
		clauses = append(clauses, "created_at >= ?")
		args = append(args, req.StartTime.UTC())
	}
	if req.EndTime != nil {
		clauses = append(clauses, "created_at <= ?")
		args = append(args, req.EndTime.UTC())
	}
	if len(clauses) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}
