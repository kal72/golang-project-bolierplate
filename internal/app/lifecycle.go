package app

import (
	"context"
	"time"

	"golang-project-boilerplate/internal/shared/logger"

	"github.com/sirupsen/logrus"
)

// Lifecycle mengelola resource yang harus ditutup saat shutdown.
// Resource ditutup dalam urutan terbalik dari penambahan (LIFO) — analog defer.
//
// Penambahan ke Lifecycle harus mengikuti urutan dependency:
// resource yang bergantung pada resource lain harus di-Add SETELAH dependency-nya.
// Misal: tambahkan logger → connection → consumer/relayer.
// Saat shutdown, consumer/relayer berhenti dulu, baru connection ditutup,
// terakhir logger di-flush.
type Lifecycle struct {
	closers []namedCloser
	log     *logger.Logger
}

type namedCloser struct {
	name  string
	close func(ctx context.Context) error
}

func NewLifecycle(log *logger.Logger) *Lifecycle {
	return &Lifecycle{log: log}
}

// Add mendaftarkan closer dengan nama untuk logging. Closer dipanggil dengan
// context shutdown sehingga bisa respect deadline.
func (l *Lifecycle) Add(name string, fn func(ctx context.Context) error) {
	l.closers = append(l.closers, namedCloser{name: name, close: fn})
}

// AddFunc helper untuk closer tanpa context.
func (l *Lifecycle) AddFunc(name string, fn func() error) {
	l.Add(name, func(_ context.Context) error { return fn() })
}

// Shutdown menutup semua resource dalam urutan LIFO. Setiap closer dibatasi
// oleh deadline ctx; jika satu closer error, lanjut ke yang berikutnya.
func (l *Lifecycle) Shutdown(ctx context.Context) {
	for i := len(l.closers) - 1; i >= 0 ; i-- {
		c := l.closers[i]
		start := time.Now()
		err := c.close(ctx)
		fields := logrus.Fields{
			"component":   "lifecycle",
			"resource":    c.name,
			"duration_ms": time.Since(start).Milliseconds(),
		}
		if err != nil {
			fields["error"] = err.Error()
			if l.log != nil {
				l.log.Error("shutdown resource failed", fields)
			}
			continue
		}
		if l.log != nil {
			l.log.Info("shutdown resource ok", fields)
		}
	}
}
