package nats

import (
	"context"
	"errors"
	"testing"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
	natslib "github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeConn struct {
	publishFn       func(string, []byte) error
	flushFn         func() error
	subscribeSyncFn func(string) (natsSubscription, error)
	closeFn         func()
}

func (f *fakeConn) Close() {
	if f.closeFn != nil {
		f.closeFn()
	}
}

func (f *fakeConn) Publish(subject string, data []byte) error {
	return f.publishFn(subject, data)
}

func (f *fakeConn) Flush() error {
	return f.flushFn()
}

func (f *fakeConn) SubscribeSync(subj string) (natsSubscription, error) {
	return f.subscribeSyncFn(subj)
}

type fakeSubscription struct {
	unsubscribeFn func() error
	nextMsgFn     func(time.Duration) (*natslib.Msg, error)
}

func (f *fakeSubscription) Unsubscribe() error {
	if f.unsubscribeFn != nil {
		return f.unsubscribeFn()
	}
	return nil
}

func (f *fakeSubscription) NextMsg(timeout time.Duration) (*natslib.Msg, error) {
	return f.nextMsgFn(timeout)
}

func TestPublishAndSubscribe(t *testing.T) {
	t.Parallel()

	server := runTestNATSServer(t)
	client, err := NewClient(server.ClientURL())
	require.NoError(t, err)
	defer client.Close()

	received := make(chan string, 1)
	subscribeCtx, subscribeCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer subscribeCancel()

	done := make(chan error, 1)
	go func() {
		done <- client.Subscribe(subscribeCtx, "deploy.status.dev", 1, func(msg *natslib.Msg) error {
			received <- string(msg.Data)
			return nil
		})
	}()

	time.Sleep(150 * time.Millisecond)

	publishCtx, publishCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer publishCancel()
	require.NoError(t, client.Publish(publishCtx, "deploy.status.dev", []byte("ready")))

	select {
	case payload := <-received:
		assert.Equal(t, "ready", payload)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for message")
	}

	require.NoError(t, <-done)
}

func TestSubscribeHonorsContextCancellation(t *testing.T) {
	t.Parallel()

	server := runTestNATSServer(t)
	client, err := NewClient(server.ClientURL())
	require.NoError(t, err)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = client.Subscribe(ctx, "deploy.status.dev", 1, func(*natslib.Msg) error { return nil })
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestPublishErrors(t *testing.T) {
	t.Parallel()

	t.Run("publish", func(t *testing.T) {
		t.Parallel()
		client := &Client{
			conn: &fakeConn{
				publishFn: func(string, []byte) error { return errors.New("boom") },
				flushFn:   func() error { t.Fatal("unexpected flush"); return nil },
			},
		}
		err := client.Publish(context.Background(), "subject", []byte("payload"))
		require.Error(t, err)
		assert.ErrorContains(t, err, "publish nats message")
	})

	t.Run("context", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		client := &Client{
			conn: &fakeConn{
				publishFn: func(string, []byte) error { return nil },
				flushFn:   func() error { t.Fatal("unexpected flush"); return nil },
			},
		}
		err := client.Publish(ctx, "subject", []byte("payload"))
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("flush", func(t *testing.T) {
		t.Parallel()
		client := &Client{
			conn: &fakeConn{
				publishFn: func(string, []byte) error { return nil },
				flushFn:   func() error { return errors.New("boom") },
			},
		}
		err := client.Publish(context.Background(), "subject", []byte("payload"))
		require.Error(t, err)
		assert.ErrorContains(t, err, "boom")
	})
}

func TestSubscribeErrors(t *testing.T) {
	t.Parallel()

	t.Run("subscribe", func(t *testing.T) {
		t.Parallel()
		client := &Client{
			conn: &fakeConn{
				subscribeSyncFn: func(string) (natsSubscription, error) { return nil, errors.New("boom") },
			},
		}
		err := client.Subscribe(context.Background(), "subject", 1, func(*natslib.Msg) error { return nil })
		require.Error(t, err)
		assert.ErrorContains(t, err, `subscribe to "subject"`)
	})

	t.Run("receive", func(t *testing.T) {
		t.Parallel()
		client := &Client{
			conn: &fakeConn{
				subscribeSyncFn: func(string) (natsSubscription, error) {
					return &fakeSubscription{
						nextMsgFn: func(time.Duration) (*natslib.Msg, error) { return nil, errors.New("boom") },
					}, nil
				},
			},
		}
		err := client.Subscribe(context.Background(), "subject", 1, func(*natslib.Msg) error { return nil })
		require.Error(t, err)
		assert.ErrorContains(t, err, "receive nats message")
	})

	t.Run("handler", func(t *testing.T) {
		t.Parallel()
		client := &Client{
			conn: &fakeConn{
				subscribeSyncFn: func(string) (natsSubscription, error) {
					return &fakeSubscription{
						nextMsgFn: func(time.Duration) (*natslib.Msg, error) {
							return &natslib.Msg{Data: []byte("payload")}, nil
						},
					}, nil
				},
			},
		}
		err := client.Subscribe(context.Background(), "subject", 1, func(*natslib.Msg) error { return errors.New("handler") })
		require.Error(t, err)
		assert.ErrorContains(t, err, "handler")
	})
}

func runTestNATSServer(t *testing.T) *natsserver.Server {
	t.Helper()

	server, err := natsserver.NewServer(&natsserver.Options{
		Host: "127.0.0.1",
		Port: -1,
	})
	require.NoError(t, err)

	go server.Start()
	if !server.ReadyForConnections(5 * time.Second) {
		t.Fatal("nats server did not become ready")
	}

	t.Cleanup(func() {
		server.Shutdown()
	})

	return server
}
