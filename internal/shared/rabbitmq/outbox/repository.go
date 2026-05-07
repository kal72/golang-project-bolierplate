package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// EnqueueParams = data minimum untuk satu event outbox.
type EnqueueParams struct {
	Exchange    string
	RoutingKey  string
	ContentType string // default "application/json"
	Payload     []byte // raw bytes; gunakan EnqueueJSON untuk auto-marshal
	TraceID     string
	RequestID   string
}

// Enqueue menyisipkan satu row outbox. Caller WAJIB memanggil ini di dalam
// transaksi yang sama dengan write business data — itulah inti outbox pattern.
//
// Contoh:
//
//	db.Transaction(func(tx *gorm.DB) error {
//	    if err := userRepo.Create(tx, &user); err != nil { return err }
//	    return outboxRepo.Enqueue(tx, outbox.EnqueueParams{...})
//	})
func (r *Repository) Enqueue(tx *gorm.DB, p EnqueueParams) error {
	if p.RoutingKey == "" {
		return fmt.Errorf("outbox: routing_key required")
	}
	if len(p.Payload) == 0 {
		return fmt.Errorf("outbox: payload required")
	}

	contentType := p.ContentType
	if contentType == "" {
		contentType = "application/json"
	}

	now := time.Now()
	row := &Outbox{
		ID:            uuid.NewString(),
		Exchange:      p.Exchange,
		RoutingKey:    p.RoutingKey,
		ContentType:   contentType,
		Payload:       p.Payload,
		TraceID:       p.TraceID,
		RequestID:     p.RequestID,
		Status:        StatusPending,
		NextAttemptAt: now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	return tx.Create(row).Error
}

// EnqueueJSON adalah convenience wrapper yang marshal payload sebagai JSON.
func (r *Repository) EnqueueJSON(tx *gorm.DB, exchange, routingKey string, payload any, traceID, requestID string) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("outbox: marshal payload: %w", err)
	}
	return r.Enqueue(tx, EnqueueParams{
		Exchange:    exchange,
		RoutingKey:  routingKey,
		ContentType: "application/json",
		Payload:     body,
		TraceID:     traceID,
		RequestID:   requestID,
	})
}

// FetchPending mengambil hingga `limit` row yang siap dipublish, dengan
// row-level lock agar relayer instance lain (horizontal scale) tidak ambil
// row yang sama. Pakai SKIP LOCKED (MySQL 8+).
func (r *Repository) FetchPending(ctx context.Context, limit int) ([]Outbox, error) {
	var rows []Outbox
	err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
		Where("status = ? AND next_attempt_at <= ?", StatusPending, time.Now()).
		Order("next_attempt_at ASC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

// MarkSent menandai row sukses publish.
func (r *Repository) MarkSent(ctx context.Context, id string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&Outbox{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":     StatusSent,
			"sent_at":    now,
			"updated_at": now,
		}).Error
}

// MarkRetry menambah attempts, set next_attempt_at = now + backoff, simpan error.
func (r *Repository) MarkRetry(ctx context.Context, id string, errMsg string, nextAttemptAt time.Time) error {
	return r.db.WithContext(ctx).
		Model(&Outbox{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"attempts":        gorm.Expr("attempts + 1"),
			"last_error":      errMsg,
			"next_attempt_at": nextAttemptAt,
			"updated_at":      time.Now(),
		}).Error
}

// MarkFailed menandai row gagal permanen (sudah lewat max attempts).
func (r *Repository) MarkFailed(ctx context.Context, id string, errMsg string) error {
	return r.db.WithContext(ctx).
		Model(&Outbox{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":     StatusFailed,
			"last_error": errMsg,
			"updated_at": time.Now(),
		}).Error
}
