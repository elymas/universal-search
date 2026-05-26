// @MX:NOTE: [AUTO] SSE consumer with reconnection for streaming search results
// @MX:SPEC: SPEC-UI-001

import type { Citation } from "./api";

export interface SSEEvent {
  type: "sentence" | "citation" | "complete" | "error";
  data: string;
}

export interface SSECallbacks {
  onSentence: (text: string) => void;
  onCitation: (citation: Citation) => void;
  onComplete: (elapsedMs: number) => void;
  onError: (error: string) => void;
}

// @MX:WARN: [AUTO] EventSource reconnection with exponential backoff
// @MX:REASON: Network interruptions may cause infinite reconnect loops without backoff cap
const MAX_RECONNECT_ATTEMPTS = 5;
const BASE_RECONNECT_DELAY_MS = 1000;

export function createSSEConnection(
  eventSource: EventSource,
  callbacks: SSECallbacks,
  signal?: AbortSignal,
): () => void {
  let reconnectAttempts = 0;

  const handleMessage = (event: MessageEvent) => {
    reconnectAttempts = 0;

    const sseEvent: SSEEvent = {
      type: event.type as SSEEvent["type"],
      data: event.data,
    };

    try {
      const parsed = JSON.parse(sseEvent.data);

      switch (sseEvent.type) {
        case "sentence":
          callbacks.onSentence(parsed.text ?? parsed);
          break;
        case "citation":
          callbacks.onCitation(parsed as Citation);
          break;
        case "complete":
          callbacks.onComplete(parsed.elapsed_ms ?? 0);
          break;
        case "error":
          callbacks.onError(parsed.message ?? parsed);
          break;
      }
    } catch {
      // Fallback: treat as raw sentence text
      if (sseEvent.type === "sentence") {
        callbacks.onSentence(sseEvent.data);
      }
    }
  };

  const handleError = () => {
    if (eventSource.readyState === EventSource.CLOSED) {
      if (reconnectAttempts < MAX_RECONNECT_ATTEMPTS) {
        const delay = BASE_RECONNECT_DELAY_MS * Math.pow(2, reconnectAttempts);
        reconnectAttempts++;
        callbacks.onError(
          `Connection lost. Reconnecting in ${delay}ms... (attempt ${reconnectAttempts})`,
        );
        setTimeout(() => {
          if (!signal?.aborted) {
            eventSource.close();
            // Caller must create a new EventSource and call createSSEConnection again
          }
        }, delay);
      } else {
        callbacks.onError(
          "Max reconnection attempts reached. Please try again.",
        );
      }
    }
  };

  eventSource.addEventListener("sentence", handleMessage);
  eventSource.addEventListener("citation", handleMessage);
  eventSource.addEventListener("complete", handleMessage);
  eventSource.addEventListener("error", handleError as EventListener);
  eventSource.onerror = handleError;

  // Return cleanup function
  return () => {
    eventSource.removeEventListener("sentence", handleMessage);
    eventSource.removeEventListener("citation", handleMessage);
    eventSource.removeEventListener("complete", handleMessage);
    eventSource.close();
  };
}
