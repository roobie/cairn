/**
 * Maximum payload size in bytes (1 MiB).
 * Payloads exceeding this size trigger PayloadTooLargeError.
 */
export const MAX_PAYLOAD_SIZE = 1_048_576

/**
 * Thrown when a payload exceeds the 1 MiB limit.
 */
export class PayloadTooLargeError extends Error {
  readonly kind = 'payload_too_large' as const

  constructor() {
    super('cairn: payload too large')
    this.name = 'PayloadTooLargeError'
  }
}

/**
 * Thrown when an operation is attempted on a closed store.
 */
export class StoreNotOpenError extends Error {
  readonly kind = 'store_not_open' as const

  constructor() {
    super('cairn: store not open')
    this.name = 'StoreNotOpenError'
  }
}

/**
 * Thrown when an immutability trigger rejects an UPDATE or DELETE.
 */
export class ImmutabilityViolationError extends Error {
  readonly kind = 'immutability_violation' as const

  constructor(message?: string) {
    super(message ?? 'cairn: immutability violation')
    this.name = 'ImmutabilityViolationError'
  }
}

/**
 * Thrown when topic is an empty string.
 */
export class EmptyTopicError extends Error {
  readonly kind = 'empty_topic' as const

  constructor() {
    super('cairn: empty topic')
    this.name = 'EmptyTopicError'
  }
}

/**
 * Thrown when payload is zero length.
 */
export class EmptyPayloadError extends Error {
  readonly kind = 'empty_payload' as const

  constructor() {
    super('cairn: empty payload')
    this.name = 'EmptyPayloadError'
  }
}
