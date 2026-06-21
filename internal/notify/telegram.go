package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"bookkeeping-api/internal/orders"
)

type TelegramNotifier struct {
	token       string
	apiBase     string
	adminChatID int64
	client      *http.Client
}

func NewTelegramNotifier(token, apiBase string, adminChatID int64) *TelegramNotifier {
	if strings.TrimSpace(apiBase) == "" {
		apiBase = "https://api.telegram.org"
	}
	return &TelegramNotifier{
		token:       strings.TrimSpace(token),
		apiBase:     strings.TrimRight(apiBase, "/"),
		adminChatID: adminChatID,
		client:      &http.Client{Timeout: 10 * time.Second},
	}
}

func (n *TelegramNotifier) SendOrderNotification(ctx context.Context, order orders.Order) error {
	return n.sendMessage(ctx, order.TelegramUserID, telegramOrderText(order))
}

func (n *TelegramNotifier) SendAdminOrderCreatedNotification(ctx context.Context, order orders.Order) error {
	if n.adminChatID == 0 {
		return nil
	}
	return n.sendMessage(ctx, n.adminChatID, telegramAdminOrderCreatedText(order))
}

func (n *TelegramNotifier) sendMessage(ctx context.Context, chatID int64, text string) error {
	if n.token == "" {
		return errors.New("TELEGRAM_BOT_TOKEN is required to send notification")
	}

	payload := map[string]any{
		"chat_id": chatID,
		"text":    text,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		fmt.Sprintf("%s/bot%s/sendMessage", n.apiBase, n.token),
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("telegram notification failed with status %d", resp.StatusCode)
	}
	return nil
}

func telegramOrderText(order orders.Order) string {
	return fmt.Sprintf(
		"订单通知\n平台订单号：%s\n客户订单号：%s\n金额：%d\n手机号：%s\n订单状态：%s",
		order.PlatformOrderNo,
		order.CustomerOrderNo,
		order.Amount,
		order.Phone,
		order.Status,
	)
}

func telegramAdminOrderCreatedText(order orders.Order) string {
	return fmt.Sprintf(
		"新订单创建\n平台订单号：%s\n客户订单号：%s\nTelegram用户ID：%d\n金额：%d\n手机号：%s\n订单状态：%s\n通知状态：%s",
		order.PlatformOrderNo,
		order.CustomerOrderNo,
		order.TelegramUserID,
		order.Amount,
		order.Phone,
		order.Status,
		order.NotifyStatus,
	)
}
