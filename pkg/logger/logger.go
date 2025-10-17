package logger

import (
	"os"

	waLog "go.mau.fi/whatsmeow/util/log"
)

type Logger struct {
	App  waLog.Logger
	HTTP waLog.Logger
}

func New(level string) *Logger {
	if level == "" {
		level = "INFO"
	}
	app := waLog.Stdout("App", level, true)
	return &Logger{
		App:  app,
		HTTP: app.Sub("HTTP"),
	}
}

func (l *Logger) WithRequestID(id string) waLog.Logger {
	return l.HTTP.Sub(id)
}

func InitForTests() *Logger {
	return &Logger{App: waLog.Stdout("Test", "DEBUG", true), HTTP: waLog.Noop}
}

func DisableColor() {
	os.Setenv("NO_COLOR", "1")
}
