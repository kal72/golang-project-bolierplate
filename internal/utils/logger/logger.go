package logger

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
)

const SessionLogKey = "session_log_key"

// standar log  field
type LogField struct {
	Status       string      `json:"status"`
	CorelationID string      `json:"corelation_id"`
	RequestID    string      `json:"request_id"`
	ClientIP     string      `json:"client_ip"`
	Service      string      `json:"service"`
	Source       string      `json:"Source"`
	UserAgent    string      `json:"user_agent"`
	HTTPMethod   string      `json:"http_method"`
	HTTPStatus   int         `json:"http_status"`
	Endpoint     string      `json:"endpoint"`
	Request      interface{} `json:"request"`
	Response     interface{} `json:"response,"`
	StartTime    time.Time
}

type Logger struct {
	Log     *logrus.Logger
	AppName string
}

func New(log *logrus.Logger, appName string) *Logger {
	return &Logger{Log: log, AppName: appName}
}

func mappingLog(ctx context.Context) logrus.Fields {
	logField, ok := ctx.Value(SessionLogKey).(*LogField)
	if ok {
		return logrus.Fields{
			"corelation_id": logField.CorelationID,
			"request_id":    logField.RequestID,
			"client_ip":     logField.ClientIP,
			"service":       logField.Service,
			"source":        logField.Source,
			"user_agent":    logField.UserAgent,
			"http_method":   logField.HTTPMethod,
			"http_status":   logField.HTTPStatus,
			"endpoint":      logField.Endpoint,
		}
	}

	return nil
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

func (l *Logger) Error(ctx context.Context, err error, msg ...string) {
	logFieldMap := mappingLog(ctx)
	l.Log.
		WithFields(logFieldMap).
		WithError(err).
		Error(msg)
}

func (l *Logger) Response(logField LogField, err error, msg ...string) {
	logFieldMap := logrus.Fields{
		"Status":        logField.Status,
		"corelation_id": logField.CorelationID,
		"request_id":    logField.RequestID,
		"client_ip":     logField.ClientIP,
		"service":       logField.Service,
		"source":        logField.Source,
		"user_agent":    logField.UserAgent,
		"http_method":   logField.HTTPMethod,
		"http_status":   logField.HTTPStatus,
		"endpoint":      logField.Endpoint,
		"request":       logField.Request,
		"response":      logField.Response,
	}
	l.Log.
		WithFields(logFieldMap).
		WithError(err).
		Print()
}
