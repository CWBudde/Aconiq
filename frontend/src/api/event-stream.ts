/**
 * A `text/event-stream` client built on `fetch`, for the API's
 * `GET /api/v1/events`.
 *
 * Not `EventSource`: a browser `EventSource` cannot send a request header, and
 * `aconiq serve --api-token` refuses every request that does not carry the
 * token in `Authorization`. The only way to reach the stream with
 * `EventSource` would be to put the token in the URL, where it lands in the
 * server's access log, the browser history and every proxy in between — so
 * this reads the stream with `fetch`, which sends the same headers as every
 * other request.
 *
 * What it takes over from `EventSource` is the part a caller cannot do without:
 * parsing the event stream format, reconnecting when the stream drops, and
 * noticing a stream that is open but silent. The server heartbeats every two
 * seconds, so a stream that has said nothing for `EVENT_STREAM_STALL_MS` is
 * treated as dropped — a sleeping laptop or a proxy holding the socket open
 * would otherwise leave the caller believing it is still being told things.
 */

/** One dispatched event: its `event:` name (`message` when unnamed) and its data. */
export interface ServerSentEvent {
  event: string;
  data: string;
}

export interface EventStreamHandlers {
  onEvent: (event: ServerSentEvent) => void;
  /**
   * Called with `true` when a stream has been opened, and with `false` when
   * it drops or is closed. Only changes are reported, so a reconnect that
   * fails reports nothing — the caller is already disconnected.
   */
  onConnectionChange: (connected: boolean) => void;
}

export interface EventStreamOptions {
  url: string;
  headers: Readonly<Record<string, string>>;
  /** Injected by tests; the global `fetch` otherwise. */
  fetch?: typeof fetch;
}

/** First reconnect delay, unless the server names its own with `retry:`. */
export const EVENT_STREAM_RETRY_MS = 1_000;
/** The reconnect delay doubles per failed attempt up to this. */
export const EVENT_STREAM_MAX_RETRY_MS = 30_000;
/**
 * How long an open stream may go without a byte before it counts as dropped.
 * Five of the server's two-second heartbeats.
 */
export const EVENT_STREAM_STALL_MS = 10_000;

/**
 * An incremental parser for the event stream format (WHATWG HTML, "Server-sent
 * events"): feed it text in chunks of any size, and it dispatches each event on
 * the blank line that ends it. `retry:` is reported rather than dispatched.
 * `id:` is ignored — the API sends none, and nothing here could resume from
 * one.
 */
export function createSSEParser(
  onEvent: (event: ServerSentEvent) => void,
  onRetry: (ms: number) => void,
): { push: (chunk: string) => void } {
  let buffer = "";
  let eventName = "";
  let data: string[] = [];

  function dispatch() {
    if (data.length > 0) {
      onEvent({ event: eventName || "message", data: data.join("\n") });
    }
    eventName = "";
    data = [];
  }

  function line(text: string) {
    if (text === "") {
      dispatch();
      return;
    }
    if (text.startsWith(":")) return;

    const colon = text.indexOf(":");
    const field = colon === -1 ? text : text.slice(0, colon);
    let value = colon === -1 ? "" : text.slice(colon + 1);
    if (value.startsWith(" ")) value = value.slice(1);

    switch (field) {
      case "event":
        eventName = value;
        break;
      case "data":
        data.push(value);
        break;
      case "retry":
        if (/^\d+$/.test(value)) onRetry(Number(value));
        break;
      default:
        break;
    }
  }

  return {
    push(chunk: string) {
      buffer += chunk;
      // A line ends at CRLF, LF or CR. A CR at the very end of the buffer may
      // be the first half of a CRLF split across chunks, so it waits.
      for (;;) {
        const match = /\r\n|\n|\r(?!$)/.exec(buffer);
        if (match === null) break;
        line(buffer.slice(0, match.index));
        buffer = buffer.slice(match.index + match[0].length);
      }
    },
  };
}

/**
 * Open the stream and keep it open — reconnecting with a doubling delay while
 * the server cannot be reached — until the returned function is called.
 */
export function openEventStream(
  options: EventStreamOptions,
  handlers: EventStreamHandlers,
): () => void {
  const fetchFn = options.fetch ?? fetch;

  let closed = false;
  let connected = false;
  let baseRetryMs = EVENT_STREAM_RETRY_MS;
  let failedAttempts = 0;
  let controller: AbortController | null = null;
  let reconnectTimer: ReturnType<typeof setTimeout> | undefined;
  let stallTimer: ReturnType<typeof setTimeout> | undefined;

  function setConnected(next: boolean) {
    if (connected === next) return;
    connected = next;
    handlers.onConnectionChange(next);
  }

  function armStallTimer() {
    clearTimeout(stallTimer);
    stallTimer = setTimeout(() => controller?.abort(), EVENT_STREAM_STALL_MS);
  }

  function scheduleReconnect() {
    if (closed) return;
    const delay = Math.min(
      baseRetryMs * 2 ** failedAttempts,
      EVENT_STREAM_MAX_RETRY_MS,
    );
    failedAttempts++;
    reconnectTimer = setTimeout(() => void connect(), delay);
  }

  async function read(response: Response, signal: AbortSignal) {
    const body = response.body;
    if (body === null) return;

    const reader = body.getReader();
    // Aborting the request ends a pending read in the browser, but that is the
    // fetch implementation's doing; cancelling the reader ends it whatever
    // produced the body.
    const cancel = () => void reader.cancel().catch(() => undefined);
    signal.addEventListener("abort", cancel, { once: true });
    const decoder = new TextDecoder();
    const parser = createSSEParser(
      (event) => {
        // A stream that has delivered an event is a stream that works: the
        // next drop starts the backoff over.
        failedAttempts = 0;
        handlers.onEvent(event);
      },
      (ms) => {
        baseRetryMs = ms;
      },
    );

    try {
      for (;;) {
        const { done, value } = await reader.read();
        if (done) return;
        armStallTimer();
        parser.push(decoder.decode(value, { stream: true }));
      }
    } finally {
      signal.removeEventListener("abort", cancel);
      reader.releaseLock();
    }
  }

  async function connect() {
    if (closed) return;
    controller = new AbortController();
    const { signal } = controller;
    armStallTimer();

    try {
      const response = await fetchFn(options.url, {
        headers: { ...options.headers },
        cache: "no-store",
        signal,
      });
      if (!response.ok) {
        // Drained so the connection is released; the refusal's text is the
        // REST endpoints' business to show, not the stream's.
        await response.body?.cancel();
        return;
      }
      setConnected(true);
      await read(response, signal);
    } catch {
      // A network failure, an abort from the stall timer or from close():
      // all the same here. The stream is down; whether to retry is below.
    } finally {
      clearTimeout(stallTimer);
      controller = null;
      setConnected(false);
      scheduleReconnect();
    }
  }

  void connect();

  return () => {
    closed = true;
    clearTimeout(reconnectTimer);
    clearTimeout(stallTimer);
    controller?.abort();
  };
}
