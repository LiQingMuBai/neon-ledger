package orders

import "context"

type Notifier interface {
	SendOrderNotification(ctx context.Context, order Order) error
	SendAdminOrderCreatedNotification(ctx context.Context, order Order) error
}

type NoopNotifier struct{}

func (NoopNotifier) SendOrderNotification(context.Context, Order) error {
	return nil
}

func (NoopNotifier) SendAdminOrderCreatedNotification(context.Context, Order) error {
	return nil
}
