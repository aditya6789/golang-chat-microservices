package repository

import "github.com/nats-io/nats.go"

type EventRepository struct{ nc *nats.Conn }

func New(nc *nats.Conn) *EventRepository { return &EventRepository{nc: nc} }
func (r *EventRepository) Subscribe(subject string, cb nats.MsgHandler) (*nats.Subscription, error) {
	return r.nc.Subscribe(subject, cb)
}

