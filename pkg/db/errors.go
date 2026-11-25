package db

import "errors"

var (
	ErrGlobalGallery  = errors.New("action not supported for the global gallery")
	ErrInvalidPercent = errors.New("percent must be greater than 0 and at most 100")
)
