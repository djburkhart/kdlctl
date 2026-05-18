package nats

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

type Client struct {
	conn *nats.Conn
}

func NewClient(url string) (*Client, error) {
	conn, err := nats.Connect(url)
	if err != nil {
		return nil, fmt.Errorf("connect to nats: %w", err)
	}

	return &Client{conn: conn}, nil
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
