// Package bus wraps NATS for the two ways services talk to each other:
//
//   - request/reply RPC with JSON bodies and typed apperr errors, served
//     through queue groups so every service scales horizontally;
//   - JetStream domain events (at-least-once, de-duplicated by event id) for
//     asynchronous integration, e.g. combat -> game.monster.killed ->
//     character/item/history services.
package bus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/mrjvadi/ommrpg/backend/pkg/apperr"
)

// Bus holds a NATS connection and its JetStream context.
type Bus struct {
	NC  *nats.Conn
	JS  jetstream.JetStream
	log *slog.Logger
}

// Connect dials NATS with sane reconnect settings.
func Connect(url, name string, log *slog.Logger) (*Bus, error) {
	nc, err := nats.Connect(url,
		nats.Name(name),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Second),
		nats.RetryOnFailedConnect(true),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			if err != nil {
				log.Warn("nats disconnected", "err", err)
			}
		}),
		nats.ReconnectHandler(func(*nats.Conn) { log.Info("nats reconnected") }),
	)
	if err != nil {
		return nil, err
	}
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, err
	}
	return &Bus{NC: nc, JS: js, log: log}, nil
}

func (b *Bus) Close() { _ = b.NC.Drain() }

type envelope struct {
	Data  json.RawMessage `json:"data,omitempty"`
	Error *apperr.Error   `json:"error,omitempty"`
}

// Handle serves a request/reply subject in a queue group.
func Handle[Req, Resp any](b *Bus, subject string, fn func(ctx context.Context, req Req) (Resp, error)) error {
	_, err := b.NC.QueueSubscribe(subject, "svc", func(m *nats.Msg) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		var env envelope
		var req Req
		if len(m.Data) > 0 {
			if err := json.Unmarshal(m.Data, &req); err != nil {
				env.Error = apperr.New(apperr.Invalid, "bad request body: %v", err)
			}
		}
		if env.Error == nil {
			resp, err := safeCall(fn, ctx, req)
			if err != nil {
				e := apperr.From(err)
				if e.Code == apperr.Internal {
					b.log.Error("handler failed", "subject", subject, "err", err)
				}
				env.Error = e
			} else if env.Data, err = json.Marshal(resp); err != nil {
				env.Error = apperr.New(apperr.Internal, "encode response")
			}
		}
		out, _ := json.Marshal(env)
		if m.Reply != "" {
			_ = m.Respond(out)
		}
	})
	return err
}

func safeCall[Req, Resp any](fn func(context.Context, Req) (Resp, error), ctx context.Context, req Req) (resp Resp, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return fn(ctx, req)
}

// Request performs a typed RPC call.
func Request[Resp any](ctx context.Context, b *Bus, subject string, req any) (Resp, error) {
	var zero Resp
	body, err := json.Marshal(req)
	if err != nil {
		return zero, err
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}
	msg, err := b.NC.RequestWithContext(ctx, subject, body)
	if err != nil {
		if errors.Is(err, nats.ErrNoResponders) {
			return zero, apperr.New(apperr.Unavailable, "%s is unavailable", subject)
		}
		return zero, apperr.New(apperr.Unavailable, "%s: %v", subject, err)
	}
	var env envelope
	if err := json.Unmarshal(msg.Data, &env); err != nil {
		return zero, fmt.Errorf("decode %s reply: %w", subject, err)
	}
	if env.Error != nil {
		return zero, env.Error
	}
	if len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, &zero); err != nil {
			return zero, fmt.Errorf("decode %s data: %w", subject, err)
		}
	}
	return zero, nil
}

// ----- JetStream events -----

// Event is the envelope of every domain event.
type Event struct {
	ID   string          `json:"id"`
	Type string          `json:"type"`
	Time time.Time       `json:"time"`
	Data json.RawMessage `json:"data"`
}

const GameStream = "GAME"

// EnsureStreams creates the event streams (idempotent).
func (b *Bus) EnsureStreams(ctx context.Context) error {
	_, err := b.JS.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:       GameStream,
		Subjects:   []string{"game.>"},
		Retention:  jetstream.LimitsPolicy,
		MaxAge:     7 * 24 * time.Hour,
		Storage:    jetstream.FileStorage,
		Duplicates: 10 * time.Minute,
	})
	return err
}

// Publish emits a domain event. id must be deterministic for the fact it
// describes (e.g. "kill:<monster>:<epoch>") so retries are de-duplicated.
func (b *Bus) Publish(ctx context.Context, subject, id string, data any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	body, _ := json.Marshal(Event{ID: id, Type: subject, Time: time.Now().UTC(), Data: raw})
	_, err = b.JS.Publish(ctx, subject, body, jetstream.WithMsgID(id))
	return err
}

// Consume attaches a durable consumer. Handlers must be idempotent: a
// returned error triggers redelivery with backoff.
func (b *Bus) Consume(ctx context.Context, durable string, subjects []string, fn func(ctx context.Context, ev Event) error) error {
	cons, err := b.JS.CreateOrUpdateConsumer(ctx, GameStream, jetstream.ConsumerConfig{
		Durable:        durable,
		FilterSubjects: subjects,
		AckPolicy:      jetstream.AckExplicitPolicy,
		MaxDeliver:     8,
		BackOff:        []time.Duration{time.Second, 2 * time.Second, 5 * time.Second, 10 * time.Second, 30 * time.Second, time.Minute, 2 * time.Minute},
		AckWait:        30 * time.Second,
		DeliverPolicy:  jetstream.DeliverAllPolicy,
	})
	if err != nil {
		return err
	}
	cc, err := cons.Consume(func(m jetstream.Msg) {
		var ev Event
		if err := json.Unmarshal(m.Data(), &ev); err != nil {
			b.log.Error("drop malformed event", "subject", m.Subject(), "err", err)
			_ = m.Term()
			return
		}
		hctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := fn(hctx, ev); err != nil {
			b.log.Warn("event handler failed, will retry", "type", ev.Type, "id", ev.ID, "err", err)
			_ = m.Nak()
			return
		}
		_ = m.Ack()
	})
	if err != nil {
		return err
	}
	go func() { <-ctx.Done(); cc.Stop() }()
	return nil
}

// Decode unmarshals event data.
func Decode[T any](ev Event) (T, error) {
	var v T
	err := json.Unmarshal(ev.Data, &v)
	return v, err
}
