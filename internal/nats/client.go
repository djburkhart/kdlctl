package nats

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

type Client struct {
	conn natsConn
}

type natsConn interface {
	Close()
	Publish(subject string, data []byte) error
	Flush() error
	SubscribeSync(subj string) (natsSubscription, error)
}

type natsSubscription interface {
	Unsubscribe() error
	NextMsg(timeout time.Duration) (*nats.Msg, error)
}

type natsConnWrapper struct {
	conn *nats.Conn
}

func NewClient(url string) (*Client, error) {
	conn, err := nats.Connect(url)
	if err != nil {
		return nil, fmt.Errorf("connect to nats: %w", err)
	}

	return &Client{conn: &natsConnWrapper{conn: conn}}, nil
}

func (w *natsConnWrapper) Close() {
	w.conn.Close()
}

func (w *natsConnWrapper) Publish(subject string, data []byte) error {
	return w.conn.Publish(subject, data)
}

func (w *natsConnWrapper) Flush() error {
	return w.conn.Flush()
}

func (w *natsConnWrapper) SubscribeSync(subj string) (natsSubscription, error) {
	return w.conn.SubscribeSync(subj)
}

func (c *Client) Close() {
	c.conn.Close()
}

func (c *Client) Publish(ctx context.Context, subject string, payload []byte) error {
	if err := c.conn.Publish(subject, payload); err != nil {
		return fmt.Errorf("publish nats message: %w", err)
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}

	return c.conn.Flush()
}

func (c *Client) Subscribe(ctx context.Context, subject string, count int, handler func(*nats.Msg) error) error {
	received := 0
	sub, err := c.conn.SubscribeSync(subject)
	if err != nil {
		return fmt.Errorf("subscribe to %q: %w", subject, err)
	}
	defer sub.Unsubscribe()

	for {
		if count > 0 && received >= count {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		msg, err := sub.NextMsg(2 * time.Second)
		if err == nats.ErrTimeout {
			continue
		}
		if err != nil {
			return fmt.Errorf("receive nats message: %w", err)
		}

		if err := handler(msg); err != nil {
			return err
		}
		received++
	}
}
