package orders

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCreateAndQueryOrders(t *testing.T) {
	store := NewMemoryStore()
	notifier := &recordingNotifier{}
	handler := NewHandlerWithNotifier(store, notifier)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	createBody := []byte(`{
		"customer_order_no":"C202606210001",
		"telegram_user_id":987654321,
		"amount":3599,
		"phone":"+8613800138000"
	}`)
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewReader(createBody))
	createResp := httptest.NewRecorder()

	mux.ServeHTTP(createResp, createReq)

	if createResp.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, createResp.Code, createResp.Body.String())
	}

	var created Order
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		t.Fatalf("decode created order: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected generated order id")
	}
	if created.PlatformOrderNo == "" {
		t.Fatal("expected generated platform order no")
	}
	if created.Status != OrderStatusPending {
		t.Fatalf("expected default status %s, got %s", OrderStatusPending, created.Status)
	}
	if created.NotifyStatus != NotifyStatusPending {
		t.Fatalf("expected default notify status %s, got %s", NotifyStatusPending, created.NotifyStatus)
	}
	if notifier.adminCount != 1 {
		t.Fatalf("expected one admin notification, got %d", notifier.adminCount)
	}

	queryReq := httptest.NewRequest(http.MethodGet, "/api/v1/orders?telegram_user_id=987654321&status=pending&notify_status=pending&limit=10", nil)
	queryResp := httptest.NewRecorder()

	mux.ServeHTTP(queryResp, queryReq)

	if queryResp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, queryResp.Code, queryResp.Body.String())
	}

	var queried QueryOrdersResponse
	if err := json.NewDecoder(queryResp.Body).Decode(&queried); err != nil {
		t.Fatalf("decode query response: %v", err)
	}
	if queried.Total != 1 || len(queried.Items) != 1 {
		t.Fatalf("expected one order, got total=%d len=%d", queried.Total, len(queried.Items))
	}
}

func TestCreateOrderValidation(t *testing.T) {
	store := NewMemoryStore()
	handler := NewHandler(store)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewReader([]byte(`{"customer_order_no":"C1"}`)))
	resp := httptest.NewRecorder()

	mux.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, resp.Code)
	}
}

func TestCreateOrderRequiresPhoneCountryCode(t *testing.T) {
	store := NewMemoryStore()
	handler := NewHandler(store)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	body := []byte(`{
		"customer_order_no":"C202606210101",
		"telegram_user_id":987654321,
		"amount":3599,
		"phone":"13800138000"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewReader(body))
	resp := httptest.NewRecorder()

	mux.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, resp.Code, resp.Body.String())
	}
}

func TestCreateOrderRequiresAmountGreaterThanTen(t *testing.T) {
	store := NewMemoryStore()
	handler := NewHandler(store)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	body := []byte(`{
		"customer_order_no":"C202606210102",
		"telegram_user_id":987654321,
		"amount":10,
		"phone":"+8613800138000"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewReader(body))
	resp := httptest.NewRecorder()

	mux.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, resp.Code, resp.Body.String())
	}
}

func TestCreateOrderAllowsEmptyTelegramUserID(t *testing.T) {
	store := NewMemoryStore()
	notifier := &recordingNotifier{}
	handler := NewHandlerWithNotifier(store, notifier)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	createBody := []byte(`{
		"customer_order_no":"C202606210103",
		"amount":3599,
		"phone":"+8613800138000"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewReader(createBody))
	resp := httptest.NewRecorder()

	mux.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, resp.Code, resp.Body.String())
	}

	var created Order
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode created order: %v", err)
	}
	if created.TelegramUserID != 0 {
		t.Fatalf("expected empty telegram_user_id to be stored as 0, got %d", created.TelegramUserID)
	}
	if notifier.adminCount != 1 {
		t.Fatalf("expected one admin notification, got %d", notifier.adminCount)
	}
}

func TestCreateOrderDefaultsEmptyStatuses(t *testing.T) {
	store := NewMemoryStore()
	handler := NewHandler(store)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	createBody := []byte(`{
		"customer_order_no":"C202606210099",
		"telegram_user_id":987654321,
		"amount":3599,
		"phone":"+8613800138000",
		"status":"",
		"notify_status":""
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewReader(createBody))
	resp := httptest.NewRecorder()

	mux.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, resp.Code, resp.Body.String())
	}

	var created Order
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode created order: %v", err)
	}
	if created.Status != OrderStatusPending || created.NotifyStatus != NotifyStatusPending {
		t.Fatalf("expected pending defaults, got status=%s notify_status=%s", created.Status, created.NotifyStatus)
	}
}

func TestLookupOrderByCustomerOrPlatformOrderNo(t *testing.T) {
	store := NewMemoryStore()
	handler := NewHandler(store)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	created := createOrder(t, mux, `{"customer_order_no":"C202606210201","telegram_user_id":1001,"amount":1200,"phone":"+8613800138000"}`)

	byCustomerReq := httptest.NewRequest(http.MethodGet, "/api/v1/orders/lookup?customer_order_no=C202606210201", nil)
	byCustomerResp := httptest.NewRecorder()
	mux.ServeHTTP(byCustomerResp, byCustomerReq)

	if byCustomerResp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, byCustomerResp.Code, byCustomerResp.Body.String())
	}
	var byCustomer Order
	if err := json.NewDecoder(byCustomerResp.Body).Decode(&byCustomer); err != nil {
		t.Fatalf("decode lookup by customer order no: %v", err)
	}
	if byCustomer.ID != created.ID {
		t.Fatalf("expected order id %s, got %s", created.ID, byCustomer.ID)
	}

	byPlatformReq := httptest.NewRequest(http.MethodGet, "/api/v1/orders/lookup?platform_order_no="+created.PlatformOrderNo, nil)
	byPlatformResp := httptest.NewRecorder()
	mux.ServeHTTP(byPlatformResp, byPlatformReq)

	if byPlatformResp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, byPlatformResp.Code, byPlatformResp.Body.String())
	}
	var byPlatform Order
	if err := json.NewDecoder(byPlatformResp.Body).Decode(&byPlatform); err != nil {
		t.Fatalf("decode lookup by platform order no: %v", err)
	}
	if byPlatform.ID != created.ID {
		t.Fatalf("expected order id %s, got %s", created.ID, byPlatform.ID)
	}
}

func TestLookupOrderValidationAndNotFound(t *testing.T) {
	store := NewMemoryStore()
	handler := NewHandler(store)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	missingParamReq := httptest.NewRequest(http.MethodGet, "/api/v1/orders/lookup", nil)
	missingParamResp := httptest.NewRecorder()
	mux.ServeHTTP(missingParamResp, missingParamReq)

	if missingParamResp.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, missingParamResp.Code, missingParamResp.Body.String())
	}

	notFoundReq := httptest.NewRequest(http.MethodGet, "/api/v1/orders/lookup?customer_order_no=NOT-FOUND", nil)
	notFoundResp := httptest.NewRecorder()
	mux.ServeHTTP(notFoundResp, notFoundReq)

	if notFoundResp.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, notFoundResp.Code, notFoundResp.Body.String())
	}
}

func TestUpdateAndDeleteOrder(t *testing.T) {
	store := NewMemoryStore()
	handler := NewHandler(store)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	created := createOrder(t, mux, `{"customer_order_no":"C202606210003","telegram_user_id":1001,"amount":1200,"phone":"+8613800138000"}`)

	updateBody := []byte(`{
		"customer_order_no":"C202606210003-A",
		"telegram_user_id":1002,
		"amount":1500,
		"phone":"+8613900139000",
		"status":"paid",
		"notify_status":"sent"
	}`)
	updateReq := httptest.NewRequest(http.MethodPut, "/api/v1/orders/"+created.ID, bytes.NewReader(updateBody))
	updateResp := httptest.NewRecorder()
	mux.ServeHTTP(updateResp, updateReq)

	if updateResp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, updateResp.Code, updateResp.Body.String())
	}

	var updated Order
	if err := json.NewDecoder(updateResp.Body).Decode(&updated); err != nil {
		t.Fatalf("decode updated order: %v", err)
	}
	if updated.Amount != 1500 || updated.Status != OrderStatusPaid || updated.NotifyStatus != NotifyStatusSent {
		t.Fatalf("unexpected updated order: %+v", updated)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/orders/"+created.ID, nil)
	deleteResp := httptest.NewRecorder()
	mux.ServeHTTP(deleteResp, deleteReq)

	if deleteResp.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, deleteResp.Code, deleteResp.Body.String())
	}

	queryReq := httptest.NewRequest(http.MethodGet, "/api/v1/orders?customer_order_no=C202606210003-A", nil)
	queryResp := httptest.NewRecorder()
	mux.ServeHTTP(queryResp, queryReq)

	var queried QueryOrdersResponse
	if err := json.NewDecoder(queryResp.Body).Decode(&queried); err != nil {
		t.Fatalf("decode query response: %v", err)
	}
	if queried.Total != 0 {
		t.Fatalf("expected deleted order to be hidden, got total=%d", queried.Total)
	}
}

func TestUpdateOrderRequiresStatuses(t *testing.T) {
	store := NewMemoryStore()
	handler := NewHandler(store)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	created := createOrder(t, mux, `{"customer_order_no":"C202606210005","telegram_user_id":1001,"amount":1200,"phone":"+8613800138000"}`)

	updateBody := []byte(`{
		"customer_order_no":"C202606210005",
		"telegram_user_id":1001,
		"amount":1200,
		"phone":"+8613800138000",
		"status":"",
		"notify_status":""
	}`)
	updateReq := httptest.NewRequest(http.MethodPut, "/api/v1/orders/"+created.ID, bytes.NewReader(updateBody))
	updateResp := httptest.NewRecorder()
	mux.ServeHTTP(updateResp, updateReq)

	if updateResp.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, updateResp.Code, updateResp.Body.String())
	}
}

func TestUpdateOrderSendsNotificationOnlyWhenNotifyStatusBecomesSent(t *testing.T) {
	store := NewMemoryStore()
	notifier := &recordingNotifier{}
	handler := NewHandlerWithNotifier(store, notifier)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	created := createOrder(t, mux, `{"customer_order_no":"C202606210004","telegram_user_id":1001,"amount":1200,"phone":"+8613800138000"}`)

	updateBody := []byte(`{
		"customer_order_no":"C202606210004",
		"telegram_user_id":1001,
		"amount":1200,
		"phone":"+8613800138000",
		"status":"paid",
		"notify_status":"sent"
	}`)
	updateReq := httptest.NewRequest(http.MethodPut, "/api/v1/orders/"+created.ID, bytes.NewReader(updateBody))
	updateResp := httptest.NewRecorder()
	mux.ServeHTTP(updateResp, updateReq)

	if updateResp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, updateResp.Code, updateResp.Body.String())
	}
	if notifier.count != 1 {
		t.Fatalf("expected one notification, got %d", notifier.count)
	}
	if notifier.lastOrder.TelegramUserID != 1001 {
		t.Fatalf("expected notification to telegram user 1001, got %d", notifier.lastOrder.TelegramUserID)
	}

	retryReq := httptest.NewRequest(http.MethodPut, "/api/v1/orders/"+created.ID, bytes.NewReader(updateBody))
	retryResp := httptest.NewRecorder()
	mux.ServeHTTP(retryResp, retryReq)

	if retryResp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, retryResp.Code, retryResp.Body.String())
	}
	if notifier.count != 1 {
		t.Fatalf("expected no duplicate notification, got %d", notifier.count)
	}
}

func TestUpdateOrderSkipsNotificationWithoutTelegramUserID(t *testing.T) {
	store := NewMemoryStore()
	notifier := &recordingNotifier{}
	handler := NewHandlerWithNotifier(store, notifier)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	created := createOrder(t, mux, `{"customer_order_no":"C202606210006","amount":1200,"phone":"+8613800138000"}`)

	updateBody := []byte(`{
		"customer_order_no":"C202606210006",
		"amount":1200,
		"phone":"+8613800138000",
		"status":"paid",
		"notify_status":"sent"
	}`)
	updateReq := httptest.NewRequest(http.MethodPut, "/api/v1/orders/"+created.ID, bytes.NewReader(updateBody))
	updateResp := httptest.NewRecorder()
	mux.ServeHTTP(updateResp, updateReq)

	if updateResp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, updateResp.Code, updateResp.Body.String())
	}
	if notifier.count != 0 {
		t.Fatalf("expected no user notification without telegram_user_id, got %d", notifier.count)
	}
}

func TestDailyTotals(t *testing.T) {
	store := NewMemoryStore()
	handler := NewHandler(store)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	store.now = func() time.Time {
		return time.Date(2026, 6, 20, 8, 0, 0, 0, time.UTC)
	}
	createOrder(t, mux, `{"customer_order_no":"C202606200001","telegram_user_id":1001,"amount":1200,"phone":"+8613800138000","status":"paid"}`)

	store.now = func() time.Time {
		return time.Date(2026, 6, 21, 8, 0, 0, 0, time.UTC)
	}
	createOrder(t, mux, `{"customer_order_no":"C202606210001","telegram_user_id":1001,"amount":2300,"phone":"+8613800138000","status":"paid"}`)
	createOrder(t, mux, `{"customer_order_no":"C202606210002","telegram_user_id":1002,"amount":700,"phone":"+8613900139000","status":"paid"}`)
	createOrder(t, mux, `{"customer_order_no":"C202606210003-PENDING","telegram_user_id":1003,"amount":9999,"phone":"+8613700137000","status":"pending"}`)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders/daily_totals", nil)
	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, resp.Code, resp.Body.String())
	}

	var body struct {
		Items []DailyTotal `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode daily totals: %v", err)
	}
	if len(body.Items) != 2 {
		t.Fatalf("expected two daily totals, got %d", len(body.Items))
	}
	if body.Items[0].Date != "2026-06-20" || body.Items[0].TotalAmount != 1200 || body.Items[0].OrderCount != 1 {
		t.Fatalf("unexpected first total: %+v", body.Items[0])
	}
	if body.Items[1].Date != "2026-06-21" || body.Items[1].TotalAmount != 3000 || body.Items[1].OrderCount != 2 {
		t.Fatalf("unexpected second total: %+v", body.Items[1])
	}
}

func TestDailyStatusTotals(t *testing.T) {
	store := NewMemoryStore()
	handler := NewHandler(store)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	store.now = func() time.Time {
		return time.Date(2026, 6, 21, 8, 0, 0, 0, time.UTC)
	}
	createOrder(t, mux, `{"customer_order_no":"C202606210010","telegram_user_id":1001,"amount":1200,"phone":"+8613800138000","status":"paid"}`)
	createOrder(t, mux, `{"customer_order_no":"C202606210011","telegram_user_id":1002,"amount":800,"phone":"+8613900139000","status":"paid"}`)
	createOrder(t, mux, `{"customer_order_no":"C202606210012","telegram_user_id":1003,"amount":500,"phone":"+8613700137000","status":"pending"}`)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders/daily_status_totals", nil)
	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, resp.Code, resp.Body.String())
	}

	var body struct {
		Items []DailyStatusTotal `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode daily status totals: %v", err)
	}

	got := make(map[string]DailyStatusTotal)
	for _, item := range body.Items {
		got[item.Status] = item
	}
	if got[OrderStatusPaid].TotalAmount != 2000 || got[OrderStatusPaid].OrderCount != 2 {
		t.Fatalf("unexpected paid total: %+v", got[OrderStatusPaid])
	}
	if got[OrderStatusPending].TotalAmount != 500 || got[OrderStatusPending].OrderCount != 1 {
		t.Fatalf("unexpected pending total: %+v", got[OrderStatusPending])
	}
}

func createOrder(t *testing.T, mux http.Handler, body string) Order {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewReader([]byte(body)))
	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, resp.Code, resp.Body.String())
	}

	var order Order
	if err := json.NewDecoder(resp.Body).Decode(&order); err != nil {
		t.Fatalf("decode created order: %v", err)
	}
	return order
}

type recordingNotifier struct {
	count          int
	adminCount     int
	lastOrder      Order
	lastAdminOrder Order
}

func (n *recordingNotifier) SendOrderNotification(_ context.Context, order Order) error {
	n.count++
	n.lastOrder = order
	return nil
}

func (n *recordingNotifier) SendAdminOrderCreatedNotification(_ context.Context, order Order) error {
	n.adminCount++
	n.lastAdminOrder = order
	return nil
}
