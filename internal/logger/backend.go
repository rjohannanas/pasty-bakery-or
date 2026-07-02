package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
)

// L es el logger global del backend (stdout + archivo).
var L zerolog.Logger

// Init configura el logger dual. Debe llamarse una vez en main.
func Init(logFile string) error {
	// Crear directorio si no existe
	if err := os.MkdirAll(filepath.Dir(logFile), 0755); err != nil {
		return fmt.Errorf("no se pudo crear directorio de logs: %w", err)
	}

	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("no se pudo abrir archivo de log %s: %w", logFile, err)
	}

	// Output dual: consola con colores + archivo plano
	consoleWriter := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: "2006-01-02 15:04:05"}
	multi := io.MultiWriter(consoleWriter, f)

	L = zerolog.New(multi).With().Timestamp().Logger()
	return nil
}
