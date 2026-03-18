package cairn

import "errors"

// Sentinel errors returned by Store operations.
var (
	ErrPayloadTooLarge       = errors.New("cairn: payload too large")
	ErrStoreNotOpen          = errors.New("cairn: store not open")
	ErrImmutabilityViolation = errors.New("cairn: immutability violation")
	ErrEmptyTopic            = errors.New("cairn: empty topic")
	ErrEmptyPayload          = errors.New("cairn: empty payload")
)

// MaxPayloadSize is the maximum allowed payload size in bytes (1 MiB).
const MaxPayloadSize = 1_048_576
