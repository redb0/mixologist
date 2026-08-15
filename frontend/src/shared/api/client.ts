export const API_BASE_URL = "/api/v1";
export const CSRF_COOKIE_NAME = "csrf_token";
export const CSRF_HEADER_NAME = "X-CSRF-Token";

const MUTATING_METHODS = new Set(["POST", "PUT", "PATCH", "DELETE"]);

export type ErrorDetail = {
  field: string;
  message: string;
};

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly requestId?: string;
  readonly details?: ErrorDetail[];

  constructor(
    status: number,
    code: string,
    message: string,
    requestId?: string,
    details?: ErrorDetail[],
  ) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.requestId = requestId;
    this.details = details;
  }
}

type StructuredErrorBody = {
  error?: {
    code?: string;
    message?: string;
    request_id?: string;
    details?: ErrorDetail[];
  };
};

type LegacyErrorBody = {
  error?: string;
};

export function buildQueryString(
  params: Record<string, string | number | undefined>,
): string {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === "") {
      continue;
    }
    search.set(key, String(value));
  }
  const query = search.toString();
  return query ? `?${query}` : "";
}

function readCookie(name: string): string | undefined {
  if (typeof document === "undefined") {
    return undefined;
  }
  const prefix = `${name}=`;
  for (const part of document.cookie.split(";")) {
    const cookie = part.trim();
    if (cookie.startsWith(prefix)) {
      return decodeURIComponent(cookie.slice(prefix.length));
    }
  }
  return undefined;
}

function withCsrfHeaders(init?: RequestInit): Headers {
  const headers = new Headers(init?.headers);
  const method = (init?.method ?? "GET").toUpperCase();
  if (MUTATING_METHODS.has(method) && !headers.has(CSRF_HEADER_NAME)) {
    const token = readCookie(CSRF_COOKIE_NAME);
    if (token) {
      headers.set(CSRF_HEADER_NAME, token);
    }
  }
  return headers;
}

export async function apiRequest<T>(
  path: string,
  init?: RequestInit,
): Promise<T> {
  const response = await fetch(`${API_BASE_URL}${path}`, {
    ...init,
    headers: withCsrfHeaders(init),
    credentials: "include",
  });

  if (!response.ok) {
    let message = "Не удалось выполнить запрос";
    let code = "UNKNOWN";
    let requestId: string | undefined;
    let details: ErrorDetail[] | undefined;
    try {
      const body = (await response.json()) as
        StructuredErrorBody | LegacyErrorBody;
      if (
        body.error &&
        typeof body.error === "object" &&
        "code" in body.error &&
        body.error.code
      ) {
        const structured = body.error as NonNullable<
          StructuredErrorBody["error"]
        >;
        code = structured.code ?? code;
        message = structured.message ?? message;
        requestId = structured.request_id;
        details = structured.details;
      } else if (typeof body.error === "string") {
        message = body.error;
      }
    } catch {
      // Backend может вернуть ответ без JSON при инфраструктурной ошибке.
    }
    throw new ApiError(response.status, code, message, requestId, details);
  }

  if (response.status === 204) {
    return undefined as T;
  }
  return (await response.json()) as T;
}
