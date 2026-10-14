import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  EVENT_STREAM_MAX_RETRY_MS,
  EVENT_STREAM_STALL_MS,
  createSSEParser,
  openEventStream,
} from "./event-stream";
import type { ServerSentEvent } from "./event-stream";

describe("createSSEParser", () => {
  function parseAll(...chunks: string[]) {
    const events: ServerSentEvent[] = [];
    const retries: number[] = [];
    const parser = createSSEParser(
      (event) => events.push(event),
      (ms) => retries.push(ms),
    );
    for (const chunk of chunks) parser.push(chunk);
    return { events, retries };
  }

  it("dispatches an event on the blank line that ends it", () => {
    const { events } = parseAll(
      'event: project_status\ndata: {"a":1}\n\nevent: heartbeat\ndata: {}\n\n',
    );
    expect(events).toEqual([
      { event: "project_status", data: '{"a":1}' },
      { event: "heartbeat", data: "{}" },
    ]);
  });

  it("reassembles an event split across chunks at any byte", () => {
    const text = 'event: project_status\ndata: {"a":1}\n\n';
    for (let cut = 1; cut < text.length; cut++) {
      const { events } = parseAll(text.slice(0, cut), text.slice(cut));
      expect(events).toEqual([{ event: "project_status", data: '{"a":1}' }]);
    }
  });

  it("joins multi-line data, accepts CRLF and skips comments", () => {
    const { events } = parseAll(
      ": keep-alive\r\ndata: one\r\ndata: two\r\n\r\n",
    );
    expect(events).toEqual([{ event: "message", data: "one\ntwo" }]);
  });

  it("reports the server's reconnect interval and dispatches nothing for it", () => {
    const { events, retries } = parseAll("retry: 3000\n\n");
    expect(events).toEqual([]);
    expect(retries).toEqual([3000]);
  });

  it("drops a block that carried no data", () => {
    const { events } = parseAll("event: heartbeat\n\n");
    expect(events).toEqual([]);
  });
});

/**
 * A response whose body the test writes into, chunk by chunk, and can end or
 * fail — the shapes a long-lived stream takes on the wire.
 */
function controllableResponse(init: ResponseInit = {}) {
  let controller!: ReadableStreamDefaultController<Uint8Array>;
  const body = new ReadableStream<Uint8Array>({
    start(c) {
      controller = c;
    },
  });
  const encoder = new TextEncoder();
  return {
    response: new Response(body, {
      status: 200,
      headers: { "Content-Type": "text/event-stream" },
      ...init,
    }),
    write(text: string) {
      controller.enqueue(encoder.encode(text));
    },
    end() {
      controller.close();
    },
  };
}

async function flush() {
  await vi.advanceTimersByTimeAsync(0);
}

describe("openEventStream", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("sends the headers it was given and no token in the URL", async () => {
    const stream = controllableResponse();
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(stream.response);

    const close = openEventStream(
      {
        url: "http://127.0.0.1:8080/api/v1/events",
        headers: {
          Authorization: "Bearer secret",
          Accept: "text/event-stream",
        },
        fetch: fetchMock,
      },
      { onEvent: () => undefined, onConnectionChange: () => undefined },
    );
    await flush();

    const [url, init] = fetchMock.mock.calls[0] ?? [];
    // Exactly the URL: no query string for a token to hide in.
    expect(url).toBe("http://127.0.0.1:8080/api/v1/events");
    expect(init?.headers).toEqual({
      Authorization: "Bearer secret",
      Accept: "text/event-stream",
    });
    close();
  });

  it("delivers events as they arrive and says when it is connected", async () => {
    const stream = controllableResponse();
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(stream.response);
    const events: ServerSentEvent[] = [];
    const connection: boolean[] = [];

    const close = openEventStream(
      { url: "/api/v1/events", headers: {}, fetch: fetchMock },
      {
        onEvent: (event) => events.push(event),
        onConnectionChange: (connected) => connection.push(connected),
      },
    );
    await flush();
    expect(connection).toEqual([true]);

    stream.write("event: project_status\ndata: {}\n\n");
    await flush();
    expect(events).toEqual([{ event: "project_status", data: "{}" }]);

    close();
    await flush();
    expect(connection).toEqual([true, false]);
  });

  it("reconnects with a growing delay while the server is down, and resets it once a stream is up", async () => {
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockRejectedValue(new TypeError("Failed to fetch"));
    const close = openEventStream(
      { url: "/api/v1/events", headers: {}, fetch: fetchMock },
      { onEvent: () => undefined, onConnectionChange: () => undefined },
    );
    await flush();
    expect(fetchMock).toHaveBeenCalledTimes(1);

    // 1 s, then 2 s, then 4 s: no hammering of a server that is not there.
    await vi.advanceTimersByTimeAsync(999);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(1);
    expect(fetchMock).toHaveBeenCalledTimes(2);
    await vi.advanceTimersByTimeAsync(2_000);
    expect(fetchMock).toHaveBeenCalledTimes(3);
    await vi.advanceTimersByTimeAsync(4_000);
    expect(fetchMock).toHaveBeenCalledTimes(4);

    // Capped, however long it stays down.
    await vi.advanceTimersByTimeAsync(10 * EVENT_STREAM_MAX_RETRY_MS);
    const calls = fetchMock.mock.calls.length;
    await vi.advanceTimersByTimeAsync(EVENT_STREAM_MAX_RETRY_MS);
    expect(fetchMock.mock.calls.length).toBe(calls + 1);

    // The server comes back, sends an event, and then drops: the next
    // attempt waits the first step again, not the cap.
    const stream = controllableResponse();
    fetchMock.mockResolvedValueOnce(stream.response);
    await vi.advanceTimersByTimeAsync(EVENT_STREAM_MAX_RETRY_MS);
    stream.write("event: heartbeat\ndata: {}\n\n");
    await flush();
    stream.end();
    await flush();
    const before = fetchMock.mock.calls.length;
    await vi.advanceTimersByTimeAsync(1_000);
    expect(fetchMock.mock.calls.length).toBe(before + 1);

    close();
  });

  it("treats a refusal as a dropped stream, not as events", async () => {
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(
        new Response('{"error":{"code":"unauthorized"}}', { status: 401 }),
      )
      .mockRejectedValue(new TypeError("Failed to fetch"));
    const events: ServerSentEvent[] = [];
    const connection: boolean[] = [];

    const close = openEventStream(
      { url: "/api/v1/events", headers: {}, fetch: fetchMock },
      {
        onEvent: (event) => events.push(event),
        onConnectionChange: (connected) => connection.push(connected),
      },
    );
    await flush();
    expect(events).toEqual([]);
    expect(connection).toEqual([]);

    await vi.advanceTimersByTimeAsync(1_000);
    expect(fetchMock).toHaveBeenCalledTimes(2);
    close();
  });

  it("gives up on a stream that has gone silent and opens a new one", async () => {
    const silent = controllableResponse();
    const fresh = controllableResponse();
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(silent.response)
      .mockResolvedValueOnce(fresh.response);
    const connection: boolean[] = [];

    const close = openEventStream(
      { url: "/api/v1/events", headers: {}, fetch: fetchMock },
      {
        onEvent: () => undefined,
        onConnectionChange: (connected) => connection.push(connected),
      },
    );
    await flush();

    // Heartbeats keep it alive...
    for (let i = 0; i < 5; i++) {
      await vi.advanceTimersByTimeAsync(EVENT_STREAM_STALL_MS / 2);
      silent.write("event: heartbeat\ndata: {}\n\n");
    }
    expect(fetchMock).toHaveBeenCalledTimes(1);

    // ...and their absence does not: a proxy that holds the socket open
    // without forwarding anything is a dropped stream too.
    await vi.advanceTimersByTimeAsync(EVENT_STREAM_STALL_MS);
    expect(connection).toEqual([true, false]);
    await vi.advanceTimersByTimeAsync(1_000);
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(connection).toEqual([true, false, true]);

    close();
  });

  it("stops for good once closed", async () => {
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockRejectedValue(new TypeError("Failed to fetch"));
    const close = openEventStream(
      { url: "/api/v1/events", headers: {}, fetch: fetchMock },
      { onEvent: () => undefined, onConnectionChange: () => undefined },
    );
    await flush();
    close();
    await vi.advanceTimersByTimeAsync(10 * EVENT_STREAM_MAX_RETRY_MS);
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });
});
