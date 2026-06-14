// Errores tipados según response shape de Domain (rules/api.md).

export interface DomainErrorDetail {
  field?: string;
  code: string;
  message: string;
}

export class DomainError extends Error {
  readonly code: string;
  readonly status: number;
  readonly requestId: string;
  readonly details: DomainErrorDetail[];

  constructor(opts: {
    message: string;
    code?: string;
    status?: number;
    requestId?: string;
    details?: DomainErrorDetail[];
  }) {
    super(opts.message);
    this.name = "DomainError";
    this.code = opts.code ?? "";
    this.status = opts.status ?? 0;
    this.requestId = opts.requestId ?? "";
    this.details = opts.details ?? [];
  }
}

export class AuthError extends DomainError {
  constructor(opts: ConstructorParameters<typeof DomainError>[0]) {
    super(opts);
    this.name = "AuthError";
  }
}

export class NotFoundError extends DomainError {
  constructor(opts: ConstructorParameters<typeof DomainError>[0]) {
    super(opts);
    this.name = "NotFoundError";
  }
}

export class ValidationError extends DomainError {
  constructor(opts: ConstructorParameters<typeof DomainError>[0]) {
    super(opts);
    this.name = "ValidationError";
  }
}

export class ConflictError extends DomainError {
  constructor(opts: ConstructorParameters<typeof DomainError>[0]) {
    super(opts);
    this.name = "ConflictError";
  }
}

export class RateLimitError extends DomainError {
  readonly retryAfter: number;
  constructor(opts: ConstructorParameters<typeof DomainError>[0] & { retryAfter?: number }) {
    super(opts);
    this.name = "RateLimitError";
    this.retryAfter = opts.retryAfter ?? 0;
  }
}

export class QuotaExceededError extends DomainError {
  constructor(opts: ConstructorParameters<typeof DomainError>[0]) {
    super(opts);
    this.name = "QuotaExceededError";
  }
}
