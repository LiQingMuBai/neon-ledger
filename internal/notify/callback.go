package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"bookkeeping-api/internal/orders"
)

type HTTPStatusCallbackSender struct {
	client *http.Client
}

func NewHTTPStatusCallbackSender() *HTTPStatusCallbackSender {
	return &HTTPStatusCallbackSender{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *HTTPStatusCallbackSender) SendStatusCallback(ctx context.Context, order orders.Order) error {
	payload := map[string]string{
		"customer_order_no": order.CustomerOrderNo,
		"platform_order_no": order.PlatformOrderNo,
		"status":            order.Status,
		"phone":             order.Phone,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, order.CallbackURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("status callback failed with status %d", resp.StatusCode)
	}
	return nil
}
