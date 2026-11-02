package model

import "errors"

// ErrIdempotencyConflict means a key was reused for a different increment.
var ErrIdempotencyConflict = errors.New("idempotency key was already used for a different request")
