package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"bookkeeping-api/internal/orders"
)

func TestHTTPStatusCallbackSenderSendsExpectedPayload(t *testing.T) {
	var payload map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST callback, got %s", r.Method)
		}
		if contentType := r.Header.Get("Content-Type"); contentType != "application/json" {
			t.Fatalf("expected application/json content type, got %q", contentType)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode callback payload: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	sender := NewHTTPStatusCallbackSender()
	err := sender.SendStatusCallback(context.Background(), orders.Order{
		CustomerOrderNo: "C202606210007",
		PlatformOrderNo: "BK20260621123045A1B2C3D4",
		Status:          orders.OrderStatusPaid,
		Phone:           "+8613800138000",
		CallbackURL:     server.URL,
	})
	if err != nil {
		t.Fatalf("send status callback: %v", err)
	}

	expected := map[string]string{
		"customer_order_no": "C202606210007",
		"platform_order_no": "BK20260621123045A1B2C3D4",
		"status":            orders.OrderStatusPaid,
		"phone":             "+8613800138000",
	}
	for key, value := range expected {
		if payload[key] != value {
			t.Fatalf("expected payload[%s]=%q, got %q", key, value, payload[key])
		}
	}
	if len(payload) != len(expected) {
		t.Fatalf("expected payload to contain only %d fields, got %+v", len(expected), payload)
	}
}
