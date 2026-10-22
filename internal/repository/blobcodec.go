package repository

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"time"
)

const (
	EncodingJSONB       = "jsonb"
	EncodingGzipJSONV1  = "gzip-json-v1"
	jsonbStub           = `{}`
)

func EncodeGzipJSON(raw []byte) (blob []byte, encoding string, err error) {
	start := time.Now()
	defer func() {
		slog.Debug("EncodeGzipJSON done",
			"raw_bytes", len(raw),
			"blob_bytes", len(blob),
			"encoding", encoding,
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	}()
	slog.Debug("EncodeGzipJSON start", "raw_bytes", len(raw))
	if len(raw) == 0 {
		raw = []byte(jsonbStub)
	}
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, writeErr := zw.Write(raw); writeErr != nil {
		err = fmt.Errorf("gzip write: %w", writeErr)
		return nil, "", err
	}
	if closeErr := zw.Close(); closeErr != nil {
		err = fmt.Errorf("gzip close: %w", closeErr)
		return nil, "", err
	}
	blob = buf.Bytes()
	encoding = EncodingGzipJSONV1
	return blob, encoding, nil
}

func DecodeStoredJSON(encoding string, jsonb json.RawMessage, blob []byte) (raw json.RawMessage, err error) {
	start := time.Now()
	defer func() {
		slog.Debug("DecodeStoredJSON done",
			"encoding", encoding,
			"jsonb_bytes", len(jsonb),
			"blob_bytes", len(blob),
			"raw_bytes", len(raw),
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	}()
	slog.Debug("DecodeStoredJSON start", "encoding", encoding, "blob_bytes", len(blob), "jsonb_bytes", len(jsonb))
	switch encoding {
	case "", EncodingJSONB:
		if len(jsonb) == 0 || string(jsonb) == "null" {
			return json.RawMessage(jsonbStub), nil
		}
		return jsonb, nil
	case EncodingGzipJSONV1:
		if len(blob) == 0 {
			err = fmt.Errorf("gzip-json-v1 blob is empty")
			return nil, err
		}
		zr, openErr := gzip.NewReader(bytes.NewReader(blob))
		if openErr != nil {
			err = fmt.Errorf("gzip open: %w", openErr)
			return nil, err
		}
		defer zr.Close()
		decoded, readErr := io.ReadAll(zr)
		if readErr != nil {
			err = fmt.Errorf("gzip read: %w", readErr)
			return nil, err
		}
		if len(decoded) == 0 {
			return json.RawMessage(jsonbStub), nil
		}
		raw = decoded
		return raw, nil
	default:
		err = fmt.Errorf("unsupported storage encoding %q", encoding)
		return nil, err
	}
}
