package event

import "errors"

var (
	ErrInvalidEvent = errors.New("invalid runtime event")

	ErrRateLimited = errors.New("runtime event rate limited")

	ErrDuplicated = errors.New("duplicated runtime event")
)