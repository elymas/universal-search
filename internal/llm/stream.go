// Package llm — channel-based streaming Delta iterator.
// REQ-LLM-008: Buffered channel (capacity 16), backpressure timeout (30s),
// context cancellation propagation, error surfaced on final Delta.
package llm

import (
	"context"
	"time"

	openai "github.com/openai/openai-go"
)

const (
	// streamChannelCap is the Delta channel buffer capacity.
	streamChannelCap = 16
	// streamBackpressureTimeout is the max time a consumer can stall before
	// the upstream stream is cancelled.
	streamBackpressureTimeout = 30 * time.Second
)

// runStream reads from the openai-go SSE stream and sends Delta values to ch.
// It closes ch when the stream ends or ctx is cancelled.
// If the consumer stalls for >30s, ErrStreamBackpressureTimeout is sent on the
// final Delta and the upstream context is cancelled.
//
// @MX:WARN: [AUTO] Goroutine launched per Stream call; cancellable via ctx
// @MX:REASON: goroutine lifetime is bounded by ctx + backpressure timeout
func runStream(
	ctx context.Context,
	cancel context.CancelFunc,
	stream interface {
		Next() bool
		Current() openai.ChatCompletionChunk
		Err() error
		Close() error
	},
	ch chan<- Delta,
) {
	defer close(ch)
	defer cancel()
	defer func() { _ = stream.Close() }()

	send := func(d Delta) bool {
		// Try to send within the backpressure timeout.
		select {
		case ch <- d:
			return true
		case <-time.After(streamBackpressureTimeout):
			// Consumer stalled; send final error delta (non-blocking).
			select {
			case ch <- Delta{Err: ErrStreamBackpressureTimeout}:
			default:
			}
			cancel()
			return false
		case <-ctx.Done():
			return false
		}
	}

	for stream.Next() {
		chunk := stream.Current()
		if len(chunk.Choices) == 0 {
			continue
		}
		choice := chunk.Choices[0]
		d := Delta{
			Content:      choice.Delta.Content,
			FinishReason: string(choice.FinishReason),
		}
		if !send(d) {
			return
		}
	}

	if err := stream.Err(); err != nil {
		// Surface stream error on final delta.
		select {
		case ch <- Delta{Err: err}:
		case <-ctx.Done():
		}
	}
}
