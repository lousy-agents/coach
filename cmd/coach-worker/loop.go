package main

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/worker"
)

func runLoop(ctx context.Context, w *worker.Worker, idlePoll time.Duration) error {
	if idlePoll <= 0 {
		idlePoll = time.Second
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		ok, err := w.ProcessNext(ctx)
		if stop, ret := handleProcessResult(err); stop {
			return ret
		}
		if ok {
			continue
		}
		if err := waitIdle(ctx, idlePoll); err != nil {
			return err
		}
	}
}

// handleProcessResult logs non-fatal process errors. stop=true means the loop
// should exit with ret (context cancel).
func handleProcessResult(err error) (stop bool, ret error) {
	if err == nil {
		return false, nil
	}
	if errors.Is(err, coachapi.ErrClaimLost) {
		log.Printf("coach-worker: claim lost; abandoning without ack")
		return false, nil
	}
	if errors.Is(err, context.Canceled) {
		return true, err
	}
	log.Printf("coach-worker: process error: %v", err)
	return false, nil
}

func waitIdle(ctx context.Context, idlePoll time.Duration) error {
	timer := time.NewTimer(idlePoll)
	select {
	case <-ctx.Done():
		timer.Stop()
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
