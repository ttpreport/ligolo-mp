package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"time"
)

type LogHandler struct {
	slog.Handler
	l *log.Logger
}

func (h *LogHandler) Handle(ctx context.Context, r slog.Record) error {
	level := r.Level.String() + ":"

	switch r.Level {
	case slog.LevelDebug:
		level = fmt.Sprintf("[#FF00FF]%s[-]", level)
	case slog.LevelInfo:
		level = fmt.Sprintf("[#87CEEB]%s[-]", level)
	case slog.LevelWarn:
		level = fmt.Sprintf("[#FFFF00]%s[-]", level)
	case slog.LevelError:
		level = fmt.Sprintf("[#880808]%s[-]", level)
	}

	fields := make(map[string]interface{}, r.NumAttrs())
	r.Attrs(func(a slog.Attr) bool {
		fields[a.Key] = a.Value.Any()
		return true
	})

	metadata := bytes.Buffer{}
	var metadataStr string
	if len(fields) > 0 {
		src, err := json.MarshalIndent(fields, "", "  ")
		if err != nil {
			return err
		}

		json.Compact(&metadata, src)
		metadataStr = fmt.Sprintf("| [#F9F6EE]%s[-]", metadata.String())
	}

	timeStr := r.Time.Format(time.RFC3339)
	msg := fmt.Sprintf("[#008B8B]%s[-]", r.Message)

	h.l.Println(timeStr, level, msg, metadataStr)

	return nil
}

func NewLogHandler(out io.Writer, opts *slog.HandlerOptions) *LogHandler {
	h := &LogHandler{
		Handler: slog.NewTextHandler(out, opts),
		l:       log.New(out, "", 0),
	}

	return h
}
