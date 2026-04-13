package cli

import (
	"encoding/json"
	"io"
)

const (
	CodeOK                  = 0
	CodeUsage               = 1000
	CodeCaptureFailed       = 1100
	CodePermissionDenied    = 1201
	CodeUnsupportedPlatform = 1203
	CodeInjectInvalid       = 1301
	CodeExportFailed        = 1400
)

type CLIResponse struct {
	Status string         `json:"status"`
	Code   int            `json:"code"`
	Data   map[string]any `json:"data,omitempty"`
	Error  *ErrorPayload  `json:"error,omitempty"`
}

type ErrorPayload struct {
	Message   string         `json:"message"`
	Retryable bool           `json:"retryable"`
	Details   map[string]any `json:"details,omitempty"`
}

func WriteOK(w io.Writer, data map[string]any) error {
	return json.NewEncoder(w).Encode(CLIResponse{
		Status: "ok",
		Code:   CodeOK,
		Data:   data,
	})
}

func WriteError(w io.Writer, code int, message string, retryable bool, details map[string]any) error {
	return json.NewEncoder(w).Encode(CLIResponse{
		Status: "error",
		Code:   code,
		Error: &ErrorPayload{
			Message:   message,
			Retryable: retryable,
			Details:   details,
		},
	})
}
