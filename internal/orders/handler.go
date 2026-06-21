package orders

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Handler struct {
	store    Store
	notifier Notifier
}

func NewHandler(store Store) *Handler {
	return NewHandlerWithNotifier(store, NoopNotifier{})
}

func NewHandlerWithNotifier(store Store, notifier Notifier) *Handler {
	if notifier == nil {
		notifier = NoopNotifier{}
	}
	return &Handler{store: store, notifier: notifier}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/orders/daily_status_totals", h.dailyStatusTotals)
	mux.HandleFunc("/api/v1/orders/daily_totals", h.dailyTotals)
	mux.HandleFunc("/api/v1/orders/lookup", h.lookupOrder)
	mux.HandleFunc("/api/v1/orders/", h.orderByID)
	mux.HandleFunc("/api/v1/orders", h.orders)
	mux.HandleFunc("/healthz", h.healthz)
}

func (h *Handler) dailyStatusTotals(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	req, err := parseDailyStatusTotalsRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	items, err := h.store.DailyStatusTotals(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func parseDailyStatusTotalsRequest(r *http.Request) (DailyTotalsRequest, error) {
	values := r.URL.Query()
	req := DailyTotalsRequest{
		NotifyStatus:   strings.ToLower(strings.TrimSpace(values.Get("notify_status"))),
		IncludeDeleted: strings.EqualFold(strings.TrimSpace(values.Get("include_deleted")), "true"),
	}
	if req.NotifyStatus != "" && !isValidNotifyStatus(req.NotifyStatus) {
		return DailyTotalsRequest{}, errors.New("notify_status must be pending, sent, or failed")
	}

	if start := strings.TrimSpace(values.Get("start_time")); start != "" {
		parsed, err := time.Parse(time.RFC3339, start)
		if err != nil {
			return DailyTotalsRequest{}, errors.New("start_time must be RFC3339 format")
		}
		req.StartTime = &parsed
	}
	if end := strings.TrimSpace(values.Get("end_time")); end != "" {
		parsed, err := time.Parse(time.RFC3339, end)
		if err != nil {
			return DailyTotalsRequest{}, errors.New("end_time must be RFC3339 format")
		}
		req.EndTime = &parsed
	}
	if req.StartTime != nil && req.EndTime != nil && req.StartTime.After(*req.EndTime) {
		return DailyTotalsRequest{}, errors.New("start_time cannot be after end_time")
	}

	return req, nil
}

func (h *Handler) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) orders(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.createOrder(w, r)
	case http.MethodGet:
		h.queryOrders(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) createOrder(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var req CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	order, err := h.store.Create(req)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if err := h.notifier.SendAdminOrderCreatedNotification(r.Context(), order); err != nil {
		log.Printf("telegram admin notification failed order_id=%s platform_order_no=%s error=%v", order.ID, order.PlatformOrderNo, err)
	}

	writeJSON(w, http.StatusCreated, order)
}

func (h *Handler) orderByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/orders/")
	id = strings.TrimSpace(id)
	if id == "" || strings.Contains(id, "/") {
		writeError(w, http.StatusNotFound, "order not found")
		return
	}

	switch r.Method {
	case http.MethodPut, http.MethodPatch:
		h.updateOrder(w, r, id)
	case http.MethodDelete:
		h.deleteOrder(w, id)
	default:
		w.Header().Set("Allow", "PUT, PATCH, DELETE")
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) updateOrder(w http.ResponseWriter, r *http.Request, id string) {
	defer r.Body.Close()

	var req UpdateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	before, err := h.store.Get(id)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	order, err := h.store.Update(id, req)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if before.NotifyStatus != NotifyStatusSent && order.NotifyStatus == NotifyStatusSent && order.TelegramUserID > 0 {
		if err := h.notifier.SendOrderNotification(r.Context(), order); err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
	}

	writeJSON(w, http.StatusOK, order)
}

func (h *Handler) deleteOrder(w http.ResponseWriter, id string) {
	if err := h.store.Delete(id); err != nil {
		writeStoreError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) queryOrders(w http.ResponseWriter, r *http.Request) {
	req, err := parseQueryOrdersRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	resp, err := h.store.Query(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) lookupOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	req, err := parseLookupOrderRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	resp, err := h.store.Query(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(resp.Items) == 0 {
		writeError(w, http.StatusNotFound, "order not found")
		return
	}

	writeJSON(w, http.StatusOK, resp.Items[0])
}

func (h *Handler) dailyTotals(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	req, err := parseDailyTotalsRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	items, err := h.store.DailyTotals(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func parseDailyTotalsRequest(r *http.Request) (DailyTotalsRequest, error) {
	values := r.URL.Query()
	req := DailyTotalsRequest{
		Status:         OrderStatusPaid,
		NotifyStatus:   strings.ToLower(strings.TrimSpace(values.Get("notify_status"))),
		IncludeDeleted: strings.EqualFold(strings.TrimSpace(values.Get("include_deleted")), "true"),
	}
	if req.NotifyStatus != "" && !isValidNotifyStatus(req.NotifyStatus) {
		return DailyTotalsRequest{}, errors.New("notify_status must be pending, sent, or failed")
	}

	if start := strings.TrimSpace(values.Get("start_time")); start != "" {
		parsed, err := time.Parse(time.RFC3339, start)
		if err != nil {
			return DailyTotalsRequest{}, errors.New("start_time must be RFC3339 format")
		}
		req.StartTime = &parsed
	}
	if end := strings.TrimSpace(values.Get("end_time")); end != "" {
		parsed, err := time.Parse(time.RFC3339, end)
		if err != nil {
			return DailyTotalsRequest{}, errors.New("end_time must be RFC3339 format")
		}
		req.EndTime = &parsed
	}
	if req.StartTime != nil && req.EndTime != nil && req.StartTime.After(*req.EndTime) {
		return DailyTotalsRequest{}, errors.New("start_time cannot be after end_time")
	}

	return req, nil
}

func parseQueryOrdersRequest(r *http.Request) (QueryOrdersRequest, error) {
	values := r.URL.Query()

	req := QueryOrdersRequest{
		CustomerOrderNo: strings.TrimSpace(values.Get("customer_order_no")),
		PlatformOrderNo: strings.TrimSpace(values.Get("platform_order_no")),
		Phone:           strings.TrimSpace(values.Get("phone")),
		Status:          strings.ToLower(strings.TrimSpace(values.Get("status"))),
		NotifyStatus:    strings.ToLower(strings.TrimSpace(values.Get("notify_status"))),
		IncludeDeleted:  strings.EqualFold(strings.TrimSpace(values.Get("include_deleted")), "true"),
	}

	var err error
	req.TelegramUserID, err = parseInt64(values.Get("telegram_user_id"), 0)
	if err != nil || req.TelegramUserID < 0 {
		return QueryOrdersRequest{}, errors.New("telegram_user_id must be a non-negative integer")
	}
	req.Limit, err = parseInt(values.Get("limit"), 20)
	if err != nil || req.Limit < 0 {
		return QueryOrdersRequest{}, errors.New("limit must be a positive integer")
	}
	req.Offset, err = parseInt(values.Get("offset"), 0)
	if err != nil || req.Offset < 0 {
		return QueryOrdersRequest{}, errors.New("offset must be a non-negative integer")
	}
	if req.Status != "" && !isValidOrderStatus(req.Status) {
		return QueryOrdersRequest{}, errors.New("status must be pending, paid, failed, or closed")
	}
	if req.NotifyStatus != "" && !isValidNotifyStatus(req.NotifyStatus) {
		return QueryOrdersRequest{}, errors.New("notify_status must be pending, sent, or failed")
	}

	if start := strings.TrimSpace(values.Get("start_time")); start != "" {
		parsed, err := time.Parse(time.RFC3339, start)
		if err != nil {
			return QueryOrdersRequest{}, errors.New("start_time must be RFC3339 format")
		}
		req.StartTime = &parsed
	}
	if end := strings.TrimSpace(values.Get("end_time")); end != "" {
		parsed, err := time.Parse(time.RFC3339, end)
		if err != nil {
			return QueryOrdersRequest{}, errors.New("end_time must be RFC3339 format")
		}
		req.EndTime = &parsed
	}
	if req.StartTime != nil && req.EndTime != nil && req.StartTime.After(*req.EndTime) {
		return QueryOrdersRequest{}, errors.New("start_time cannot be after end_time")
	}

	return req, nil
}

func parseLookupOrderRequest(r *http.Request) (QueryOrdersRequest, error) {
	values := r.URL.Query()
	req := QueryOrdersRequest{
		CustomerOrderNo: strings.TrimSpace(values.Get("customer_order_no")),
		PlatformOrderNo: strings.TrimSpace(values.Get("platform_order_no")),
		Limit:           1,
	}
	if req.CustomerOrderNo == "" && req.PlatformOrderNo == "" {
		return QueryOrdersRequest{}, errors.New("customer_order_no or platform_order_no is required")
	}
	return req, nil
}

func parseInt(raw string, fallback int) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	return strconv.Atoi(raw)
}

func parseInt64(raw string, fallback int64) (int64, error) {
	if strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	return strconv.ParseInt(raw, 10, 64)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidOrder):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}
