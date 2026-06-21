package orders

import (
	"crypto/rand"
	"encoding/hex"
	"sort"
	"strings"
	"sync"
	"time"
)

type Store interface {
	Create(req CreateOrderRequest) (Order, error)
	Get(id string) (Order, error)
	Update(id string, req UpdateOrderRequest) (Order, error)
	Delete(id string) error
	Query(req QueryOrdersRequest) (QueryOrdersResponse, error)
	DailyTotals(req DailyTotalsRequest) ([]DailyTotal, error)
	DailyStatusTotals(req DailyTotalsRequest) ([]DailyStatusTotal, error)
}

type MemoryStore struct {
	mu     sync.RWMutex
	orders map[string]Order
	now    func() time.Time
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		orders: make(map[string]Order),
		now:    time.Now,
	}
}

func (s *MemoryStore) Create(req CreateOrderRequest) (Order, error) {
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

	s.mu.Lock()
	defer s.mu.Unlock()
	s.orders[order.ID] = order

	return order, nil
}

func (s *MemoryStore) Get(id string) (Order, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	order, ok := s.orders[id]
	if !ok || order.DeletedAt != nil {
		return Order{}, ErrNotFound
	}

	return order, nil
}

func (s *MemoryStore) Update(id string, req UpdateOrderRequest) (Order, error) {
	req = normalizeUpdateOrderRequest(req)
	if err := req.validate(); err != nil {
		return Order{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	order, ok := s.orders[id]
	if !ok || order.DeletedAt != nil {
		return Order{}, ErrNotFound
	}

	now := s.now().UTC()
	order.CustomerOrderNo = req.CustomerOrderNo
	order.TelegramUserID = req.TelegramUserID
	order.Amount = req.Amount
	order.Phone = req.Phone
	order.Status = req.Status
	order.NotifyStatus = req.NotifyStatus
	order.CallbackURL = req.CallbackURL
	order.UpdatedAt = now
	s.orders[id] = order

	return order, nil
}

func (s *MemoryStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	order, ok := s.orders[id]
	if !ok || order.DeletedAt != nil {
		return ErrNotFound
	}

	now := s.now().UTC()
	order.UpdatedAt = now
	order.DeletedAt = &now
	s.orders[id] = order

	return nil
}

func (s *MemoryStore) Query(req QueryOrdersRequest) (QueryOrdersResponse, error) {
	normalizeQueryOrdersRequest(&req)

	s.mu.RLock()
	defer s.mu.RUnlock()

	matched := make([]Order, 0, len(s.orders))
	for _, order := range s.orders {
		if !req.IncludeDeleted && order.DeletedAt != nil {
			continue
		}
		if req.CustomerOrderNo != "" && order.CustomerOrderNo != req.CustomerOrderNo {
			continue
		}
		if req.PlatformOrderNo != "" && order.PlatformOrderNo != req.PlatformOrderNo {
			continue
		}
		if req.TelegramUserID > 0 && order.TelegramUserID != req.TelegramUserID {
			continue
		}
		if req.Phone != "" && !strings.Contains(order.Phone, req.Phone) {
			continue
		}
		if req.Status != "" && order.Status != req.Status {
			continue
		}
		if req.NotifyStatus != "" && order.NotifyStatus != req.NotifyStatus {
			continue
		}
		if req.StartTime != nil && order.CreatedAt.Before(req.StartTime.UTC()) {
			continue
		}
		if req.EndTime != nil && order.CreatedAt.After(req.EndTime.UTC()) {
			continue
		}
		matched = append(matched, order)
	}

	sort.Slice(matched, func(i, j int) bool {
		return matched[i].CreatedAt.After(matched[j].CreatedAt)
	})

	total := len(matched)
	if req.Offset >= total {
		return QueryOrdersResponse{
			Items:  []Order{},
			Total:  total,
			Limit:  req.Limit,
			Offset: req.Offset,
		}, nil
	}

	end := req.Offset + req.Limit
	if end > total {
		end = total
	}

	return QueryOrdersResponse{
		Items:  matched[req.Offset:end],
		Total:  total,
		Limit:  req.Limit,
		Offset: req.Offset,
	}, nil
}

func (s *MemoryStore) DailyTotals(req DailyTotalsRequest) ([]DailyTotal, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	totals := make(map[string]*DailyTotal)
	for _, order := range s.orders {
		if !req.IncludeDeleted && order.DeletedAt != nil {
			continue
		}
		if req.Status != "" && order.Status != req.Status {
			continue
		}
		if req.NotifyStatus != "" && order.NotifyStatus != req.NotifyStatus {
			continue
		}
		if req.StartTime != nil && order.CreatedAt.Before(req.StartTime.UTC()) {
			continue
		}
		if req.EndTime != nil && order.CreatedAt.After(req.EndTime.UTC()) {
			continue
		}

		date := order.CreatedAt.UTC().Format("2006-01-02")
		total, ok := totals[date]
		if !ok {
			total = &DailyTotal{Date: date}
			totals[date] = total
		}
		total.TotalAmount += order.Amount
		total.OrderCount++
	}

	items := make([]DailyTotal, 0, len(totals))
	for _, total := range totals {
		items = append(items, *total)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Date < items[j].Date
	})

	return items, nil
}

func (s *MemoryStore) DailyStatusTotals(req DailyTotalsRequest) ([]DailyStatusTotal, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	totals := make(map[string]*DailyStatusTotal)
	for _, order := range s.orders {
		if !req.IncludeDeleted && order.DeletedAt != nil {
			continue
		}
		if req.NotifyStatus != "" && order.NotifyStatus != req.NotifyStatus {
			continue
		}
		if req.StartTime != nil && order.CreatedAt.Before(req.StartTime.UTC()) {
			continue
		}
		if req.EndTime != nil && order.CreatedAt.After(req.EndTime.UTC()) {
			continue
		}

		date := order.CreatedAt.UTC().Format("2006-01-02")
		key := date + "|" + order.Status
		total, ok := totals[key]
		if !ok {
			total = &DailyStatusTotal{Date: date, Status: order.Status}
			totals[key] = total
		}
		total.TotalAmount += order.Amount
		total.OrderCount++
	}

	items := make([]DailyStatusTotal, 0, len(totals))
	for _, total := range totals {
		items = append(items, *total)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Date == items[j].Date {
			return items[i].Status < items[j].Status
		}
		return items[i].Date < items[j].Date
	})

	return items, nil
}

func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(b[:])
}

func newPlatformOrderNo(now time.Time) string {
	return "BK" + now.UTC().Format("20060102150405") + strings.ToUpper(newID()[:8])
}
