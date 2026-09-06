package gateway

import (
	"context"
	"errors"
	"strings"
	"time"
)

const (
	maxOutboxThreadChars = 200
	maxOutboxBodyBytes   = 8 * 1024
	maxOutboxPoll        = 50
)

// OutboxMessage is one reply waiting for the owner's gateway daemon.
type OutboxMessage struct {
	ID        string    `json:"id"`
	Thread    string    `json:"thread"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
}

// Enqueue stores a reply for the gateway to deliver. The daemon polls and
// deletes; the app never delivers directly, so there is exactly one writer
// per thread and retries cannot double-send.
func (s *Store) Enqueue(ctx context.Context, userID string, thread string, body string) (OutboxMessage, error) {
	thread = strings.TrimSpace(thread)
	body = strings.TrimSpace(body)
	if thread == "" || len(thread) > maxOutboxThreadChars {
		return OutboxMessage{}, errors.Join(ErrInvalidConfig, errors.New("thread is required"))
	}
	if body == "" || len(body) > maxOutboxBodyBytes {
		return OutboxMessage{}, errors.Join(ErrInvalidConfig, errors.New("body is required"))
	}
	var message OutboxMessage
	err := s.db.QueryRow(ctx, `
		INSERT INTO gateway_outbox (user_id, thread, body)
		VALUES ($1, $2, $3)
		RETURNING id::text, thread, body, created_at
	`, userID, thread, body).Scan(&message.ID, &message.Thread, &message.Body, &message.CreatedAt)
	if err != nil {
		return OutboxMessage{}, err
	}
	return message, nil
}

// Poll claims up to maxOutboxPoll pending replies and deletes them in the
// same transaction, so a second poller never sees the same message. The
// daemon long-polls this endpoint; the settings UI never calls it.
func (s *Store) Poll(ctx context.Context, userID string) ([]OutboxMessage, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `
		SELECT id::text, thread, body, created_at
		FROM gateway_outbox
		WHERE user_id = $1
		ORDER BY created_at
		LIMIT $2
		FOR UPDATE SKIP LOCKED
	`, userID, maxOutboxPoll)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	messages := []OutboxMessage{}
	for rows.Next() {
		var message OutboxMessage
		if err := rows.Scan(&message.ID, &message.Thread, &message.Body, &message.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(messages) == 0 {
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return []OutboxMessage{}, nil
	}
	ids := make([]string, 0, len(messages))
	for _, message := range messages {
		ids = append(ids, message.ID)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM gateway_outbox WHERE id = ANY($1::uuid[])`, ids); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return messages, nil
}
