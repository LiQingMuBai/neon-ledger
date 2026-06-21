package orders

import (
	"errors"
	"strings"
	"time"
)

const (
	OrderStatusPending = "pending"
	OrderStatusPaid    = "paid"
	OrderStatusFailed  = "failed"
	OrderStatusClosed  = "closed"

	NotifyStatusPending = "pending"
	NotifyStatusSent    = "sent"
	NotifyStatusFailed  = "failed"
)

var (
	ErrInvalidOrder = errors.New("invalid order")
	ErrNotFound     = errors.New("order not found")
)

type Order struct {
	ID              string     `json:"id"`
	CustomerOrderNo string     `json:"customer_order_no"`
	PlatformOrderNo string     `json:"platform_order_no"`
	TelegramUserID  int64      `json:"telegram_user_id"`
	Amount          int64      `json:"amount"`
	Phone           string     `json:"phone"`
	Status          string     `json:"status"`
	NotifyStatus    string     `json:"notify_status"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	DeletedAt       *time.Time `json:"deleted_at,omitempty"`
}

type CreateOrderRequest struct {
	CustomerOrderNo string `json:"customer_order_no"`
	TelegramUserID  int64  `json:"telegram_user_id"`
	Amount          int64  `json:"amount"`
	Phone           string `json:"phone"`
	Status          string `json:"status"`
	NotifyStatus    string `json:"notify_status"`
}

type UpdateOrderRequest struct {
	CustomerOrderNo string `json:"customer_order_no"`
	TelegramUserID  int64  `json:"telegram_user_id"`
	Amount          int64  `json:"amount"`
	Phone           string `json:"phone"`
	Status          string `json:"status"`
	NotifyStatus    string `json:"notify_status"`
}

type QueryOrdersRequest struct {
	CustomerOrderNo string
	PlatformOrderNo string
	TelegramUserID  int64
	Phone           string
	Status          string
	NotifyStatus    string
	StartTime       *time.Time
	EndTime         *time.Time
	IncludeDeleted  bool
	Limit           int
	Offset          int
}

type QueryOrdersResponse struct {
	Items  []Order `json:"items"`
	Total  int     `json:"total"`
	Limit  int     `json:"limit"`
	Offset int     `json:"offset"`
}

type DailyTotal struct {
	Date        string `json:"date"`
	TotalAmount int64  `json:"total_amount"`
	OrderCount  int    `json:"order_count"`
}

type DailyStatusTotal struct {
	Date        string `json:"date"`
	Status      string `json:"status"`
	TotalAmount int64  `json:"total_amount"`
	OrderCount  int    `json:"order_count"`
}

type DailyTotalsRequest struct {
	StartTime      *time.Time
	EndTime        *time.Time
	Status         string
	NotifyStatus   string
	IncludeDeleted bool
}

func (r CreateOrderRequest) validate(now time.Time) error {
	if strings.TrimSpace(r.CustomerOrderNo) == "" {
		return errors.Join(ErrInvalidOrder, errors.New("customer_order_no is required"))
	}
	if r.TelegramUserID < 0 {
		return errors.Join(ErrInvalidOrder, errors.New("telegram_user_id must be greater than or equal to 0"))
	}
	if r.Amount < 10 {
		return errors.Join(ErrInvalidOrder, errors.New("amount must be greater than or equal to 10"))
	}
	if strings.TrimSpace(r.Phone) == "" {
		return errors.Join(ErrInvalidOrder, errors.New("phone is required"))
	}
	if !isValidPhoneWithCountryCode(r.Phone) {
		return errors.Join(ErrInvalidOrder, errors.New("phone must include country code, for example +8613800138000"))
	}
	if r.Status != "" && !isValidOrderStatus(r.Status) {
		return errors.Join(ErrInvalidOrder, errors.New("status must be pending, paid, failed, or closed"))
	}
	if r.NotifyStatus != "" && !isValidNotifyStatus(r.NotifyStatus) {
		return errors.Join(ErrInvalidOrder, errors.New("notify_status must be pending, sent, or failed"))
	}
	_ = now
	return nil
}

func (r UpdateOrderRequest) validate() error {
	if strings.TrimSpace(r.CustomerOrderNo) == "" {
		return errors.Join(ErrInvalidOrder, errors.New("customer_order_no is required"))
	}
	if r.TelegramUserID < 0 {
		return errors.Join(ErrInvalidOrder, errors.New("telegram_user_id must be greater than or equal to 0"))
	}
	if r.Amount < 10 {
		return errors.Join(ErrInvalidOrder, errors.New("amount must be greater than or equal to 10"))
	}
	if strings.TrimSpace(r.Phone) == "" {
		return errors.Join(ErrInvalidOrder, errors.New("phone is required"))
	}
	if !isValidPhoneWithCountryCode(r.Phone) {
		return errors.Join(ErrInvalidOrder, errors.New("phone must include country code, for example +8613800138000"))
	}
	if !isValidOrderStatus(r.Status) {
		return errors.Join(ErrInvalidOrder, errors.New("status must be pending, paid, failed, or closed"))
	}
	if !isValidNotifyStatus(r.NotifyStatus) {
		return errors.Join(ErrInvalidOrder, errors.New("notify_status must be pending, sent, or failed"))
	}
	return nil
}

func normalizeOrderRequest(r CreateOrderRequest) CreateOrderRequest {
	r.CustomerOrderNo = strings.TrimSpace(r.CustomerOrderNo)
	r.Phone = strings.TrimSpace(r.Phone)
	r.Status = strings.ToLower(strings.TrimSpace(r.Status))
	r.NotifyStatus = strings.ToLower(strings.TrimSpace(r.NotifyStatus))
	if r.Status == "" {
		r.Status = OrderStatusPending
	}
	if r.NotifyStatus == "" {
		r.NotifyStatus = NotifyStatusPending
	}
	return r
}

func normalizeUpdateOrderRequest(r UpdateOrderRequest) UpdateOrderRequest {
	r.CustomerOrderNo = strings.TrimSpace(r.CustomerOrderNo)
	r.Phone = strings.TrimSpace(r.Phone)
	r.Status = strings.ToLower(strings.TrimSpace(r.Status))
	r.NotifyStatus = strings.ToLower(strings.TrimSpace(r.NotifyStatus))
	return r
}

func isValidOrderStatus(status string) bool {
	switch status {
	case OrderStatusPending, OrderStatusPaid, OrderStatusFailed, OrderStatusClosed:
		return true
	default:
		return false
	}
}

func isValidNotifyStatus(status string) bool {
	switch status {
	case NotifyStatusPending, NotifyStatusSent, NotifyStatusFailed:
		return true
	default:
		return false
	}
}

func isValidPhoneWithCountryCode(phone string) bool {
	if len(phone) < 8 || len(phone) > 16 || phone[0] != '+' || phone[1] == '0' {
		return false
	}
	for _, r := range phone[1:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
