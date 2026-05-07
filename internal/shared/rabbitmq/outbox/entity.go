package outbox

import "time"

// Status outbox row.
const (
	StatusPending = "pending"
	StatusSent    = "sent"
	StatusFailed  = "failed" // gagal permanen setelah max attempts
)

// Outbox merepresentasikan satu pesan yang menunggu di-publish ke RabbitMQ.
//
// Skema MySQL (jalankan saat migrasi awal):
//
//	CREATE TABLE outbox (
//	  id VARCHAR(36) PRIMARY KEY,
//	  exchange VARCHAR(255) NOT NULL DEFAULT '',
//	  routing_key VARCHAR(255) NOT NULL,
//	  content_type VARCHAR(64) NOT NULL DEFAULT 'application/json',
//	  payload LONGBLOB NOT NULL,
//	  trace_id VARCHAR(64) NOT NULL DEFAULT '',
//	  request_id VARCHAR(64) NOT NULL DEFAULT '',
//	  status VARCHAR(16) NOT NULL DEFAULT 'pending',
//	  attempts INT NOT NULL DEFAULT 0,
//	  last_error TEXT NULL,
//	  next_attempt_at DATETIME NOT NULL,
//	  sent_at DATETIME NULL,
//	  created_at DATETIME NOT NULL,
//	  updated_at DATETIME NOT NULL,
//	  INDEX idx_outbox_status_next (status, next_attempt_at)
//	);
type Outbox struct {
	ID            string    `gorm:"column:id;primaryKey;size:36"`
	Exchange      string    `gorm:"column:exchange;size:255;not null;default:''"`
	RoutingKey    string    `gorm:"column:routing_key;size:255;not null"`
	ContentType   string    `gorm:"column:content_type;size:64;not null;default:'application/json'"`
	Payload       []byte    `gorm:"column:payload;type:longblob;not null"`
	TraceID       string    `gorm:"column:trace_id;size:64;not null;default:''"`
	RequestID     string    `gorm:"column:request_id;size:64;not null;default:''"`
	Status        string    `gorm:"column:status;size:16;not null;default:'pending';index:idx_outbox_status_next,priority:1"`
	Attempts      int       `gorm:"column:attempts;not null;default:0"`
	LastError     *string   `gorm:"column:last_error;type:text"`
	NextAttemptAt time.Time `gorm:"column:next_attempt_at;not null;index:idx_outbox_status_next,priority:2"`
	SentAt        *time.Time `gorm:"column:sent_at"`
	CreatedAt     time.Time `gorm:"column:created_at;not null"`
	UpdatedAt     time.Time `gorm:"column:updated_at;not null"`
}

func (Outbox) TableName() string { return "outbox" }
