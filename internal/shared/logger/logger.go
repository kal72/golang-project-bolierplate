package logger

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

const SessionLogKey = "session_log_key"

type LogMessage struct {
	Level   logrus.Level
	Message string
	Fields  logrus.Fields
	Time    time.Time
}

type Logger struct {
	Log     *logrus.Logger
	AppName string
	ch      chan LogMessage
	quit    chan struct{}
	wg      sync.WaitGroup
}

func New(log *logrus.Logger, appName string, bufferSize int) *Logger {
	l := &Logger{
		Log:     log,
		AppName: appName,
		ch:      make(chan LogMessage, bufferSize),
		quit:    make(chan struct{}),
	}

	// worker goroutine
	l.wg.Add(1)
	go func() {
		defer l.wg.Done()
		for {
			select {
			case msg := <-l.ch:
				entry := l.Log.WithFields(msg.Fields)
				switch msg.Level {
				case logrus.InfoLevel:
					entry.Info(msg.Message)
				case logrus.WarnLevel:
					entry.Warn(msg.Message)
				case logrus.ErrorLevel:
					entry.Error(msg.Message)
				case logrus.DebugLevel:
					entry.Debug(msg.Message)
				default:
					entry.Print(msg.Message)
				}
			case <-l.quit:
				// flush semua log tersisa di channel sebelum exit
				for msg := range l.ch {
					l.Log.WithFields(msg.Fields).Log(msg.Level, msg.Message)
				}
				return
			}
		}
	}()

	return l
}

func (l *Logger) logMsg(level logrus.Level, msg string, fields logrus.Fields) {
	l.ch <- LogMessage{Level: level, Message: msg, Fields: fields, Time: time.Now()}
}

// func (l *Logger) StartRequest(ctx context.Context, request any) {
// 	logFieldMap := mappingLog(ctx)
// 	l.Log.
// 		WithFields(logFieldMap).
// 		WithField("request", request).
// 		Info("Incoming request")
// }

// func (l *Logger) FinishRequest(ctx context.Context, request any, response any) {
// 	logFieldMap := mappingLog(ctx)
// 	l.Log.
// 		WithFields(logFieldMap).
// 		WithField("request", request).
// 		WithField("response", response).
// 		Info("Finish request")
// }

func (l *Logger) LogEvent(ctx context.Context, err error, request any, response any) {
	fields, ok := ctx.Value(SessionLogKey).(logrus.Fields)
	if ok {
		req, _ := json.Marshal(request)
		resp, _ := json.Marshal(response)
		fields["request"] = string(req)
		fields["response"] = string(resp)
		fields["error"] = err
		fields["status"] = "success"
		if err != nil {
			fields["status"] = "error"
		}
		l.logMsg(0, "HTTP Event", fields)
	}
}

func (l *Logger) Info(msg string, fields logrus.Fields)  { l.logMsg(logrus.InfoLevel, msg, fields) }
func (l *Logger) Warn(msg string, fields logrus.Fields)  { l.logMsg(logrus.WarnLevel, msg, fields) }
func (l *Logger) Error(msg string, fields logrus.Fields) { l.logMsg(logrus.ErrorLevel, msg, fields) }
func (l *Logger) Debug(msg string, fields logrus.Fields) { l.logMsg(logrus.DebugLevel, msg, fields) }

// Close → auto flush sebelum shutdown
func (l *Logger) Close() {
	close(l.quit)
	l.wg.Wait() // tunggu worker selesai
	close(l.ch) // close channel setelah flush
}
