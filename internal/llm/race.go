package llm

import (
	"context"
	"sync"
)

type raceResult struct {
	text string
	err  error
}

// RaceComplete fires two providers concurrently and returns the first response
// whose length is >= minLen. The slower provider is cancelled immediately.
//
// Use case: "fast" = cheap/quick model, "accurate" = stronger model.
// If the fast model returns a long-enough answer first, we save latency + cost.
// If the accurate model wins, we get higher quality. Either way the user wins.
//
// Streaming is not used here — we need the full response to check minLen.
func RaceComplete(
	ctx context.Context,
	fast, accurate Provider,
	model, system string,
	messages []Message,
	maxTokens, minLen int,
) (string, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ch := make(chan raceResult, 2)
	var wg sync.WaitGroup

	fire := func(p Provider) {
		defer wg.Done()
		text, err := p.Complete(ctx, model, system, messages, maxTokens)
		ch <- raceResult{text, err}
	}

	wg.Add(2)
	go fire(fast)
	go fire(accurate)

	// Close ch after both goroutines finish so the reader loop below can exit.
	go func() {
		wg.Wait()
		close(ch)
	}()

	var lastErr error
	for r := range ch {
		if r.err != nil {
			lastErr = r.err
			continue
		}
		if len(r.text) >= minLen {
			cancel() // signal the loser to stop
			return r.text, nil
		}
		// Response too short — wait for the other provider.
	}
	// Both providers finished; return whatever we have or the last error.
	return "", lastErr
}
