package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
)

// L es el logger global del backend. Escribe a stdout (lo captura journald
// bajo systemd); opcionalmente también a un archivo si se pasa logFile.
var L zerolog.Logger

// Init configura el logger. Debe llamarse una vez en main.
// logFile vacío = solo stdout (fuente única: journald). Con logFile = dual.
func Init(logFile string) error {
	consoleWriter := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: "2006-01-02 15:04:05"}

	// Sin archivo: solo stdout. Es el modo bajo systemd (journald es la fuente).
	if logFile == "" {
		L = zerolog.New(consoleWriter).With().Timestamp().Logger()
		return nil
	}

	// Con archivo: salida dual consola + archivo plano.
	if err := os.MkdirAll(filepath.Dir(logFile), 0755); err != nil {
		return fmt.Errorf("no se pudo crear directorio de logs: %w", err)
	}
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("no se pudo abrir archivo de log %s: %w", logFile, err)
	}
	multi := io.MultiWriter(consoleWriter, f)
	L = zerolog.New(multi).With().Timestamp().Logger()
	return nil
}
