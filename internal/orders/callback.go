package orders

import "context"

type StatusCallbackSender interface {
	SendStatusCallback(ctx context.Context, order Order) error
}

type NoopStatusCallbackSender struct{}

func (NoopStatusCallbackSender) SendStatusCallback(context.Context, Order) error {
	return nil
}
