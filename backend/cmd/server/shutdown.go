package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/lifecycle"
)

type shutdownOptions struct {
	drainDelay        time.Duration
	requestTimeout    time.Duration
	forceCloseTimeout time.Duration
	cleanupTimeout    time.Duration
}

func shutdownOptionsFromConfig(cfg config.ServerConfig) shutdownOptions {
	return shutdownOptions{
		drainDelay:        time.Duration(cfg.DrainDelaySeconds) * time.Second,
		requestTimeout:    time.Duration(cfg.ShutdownTimeoutSeconds) * time.Second,
		forceCloseTimeout: time.Duration(cfg.ForceCloseTimeoutSeconds) * time.Second,
		cleanupTimeout:    time.Duration(cfg.CleanupTimeoutSeconds) * time.Second,
	}
}

// shutdownServer waits for both HTTP handlers and upgraded connections before
// draining their dependent queues. All waits share a deadline within each phase.
func shutdownServer(srv *http.Server, cleanup func(context.Context) error, opts shutdownOptions) error {
	started := time.Now()
	manager := lifecycle.FromHandler(srv.Handler)
	log.Printf("shutdown_started drain_delay=%s request_timeout=%s force_close_timeout=%s cleanup_timeout=%s", opts.drainDelay, opts.requestTimeout, opts.forceCloseTimeout, opts.cleanupTimeout)
	if manager != nil {
		manager.BeginDrain()
		log.Printf("shutdown_draining in_flight=%+v", manager.Snapshot())
	}
	// Keep serving residual routed traffic while readiness changes propagate.
	if opts.drainDelay > 0 {
		time.Sleep(opts.drainDelay)
	}
	if manager != nil {
		manager.StopAdmission()
	}
	drainCtx, cancelDrain := context.WithTimeout(context.Background(), opts.requestTimeout)
	httpDone := make(chan error, 1)
	go func() { httpDone <- srv.Shutdown(drainCtx) }()
	var waitErr error
	if manager != nil {
		waitErr = manager.Wait(drainCtx)
	}
	httpErr := <-httpDone
	cancelDrain()
	var shutdownErr error
	if waitErr != nil || httpErr != nil {
		shutdownErr = fmt.Errorf("request drain: %w", errors.Join(waitErr, httpErr))
		if manager != nil {
			log.Printf("shutdown_drain_timeout in_flight=%+v error=%v", manager.Snapshot(), shutdownErr)
			manager.ForceClose()
		}
		if err := srv.Close(); err != nil {
			log.Printf("shutdown_force_close_failed error=%v", err)
		}
		if manager != nil {
			forceCtx, cancelForce := context.WithTimeout(context.Background(), opts.forceCloseTimeout)
			err := manager.Wait(forceCtx)
			cancelForce()
			if err != nil {
				// Closing DB/Redis now would race a still-running handler's accounting.
				log.Printf("shutdown_incomplete phase=requests in_flight=%+v elapsed=%s; dependencies left open for process exit", manager.Snapshot(), time.Since(started))
				return errors.Join(shutdownErr, fmt.Errorf("forced request finalization: %w", err))
			}
		}
	} else {
		log.Printf("shutdown_drain_completed elapsed=%s", time.Since(started))
	}
	if cleanup != nil {
		cleanupCtx, cancelCleanup := context.WithTimeout(context.Background(), opts.cleanupTimeout)
		done := make(chan error, 1)
		go func() { done <- cleanup(cleanupCtx) }()
		var err error
		select {
		case err = <-done:
		case <-cleanupCtx.Done():
			err = cleanupCtx.Err()
		}
		cancelCleanup()
		if err != nil {
			shutdownErr = errors.Join(shutdownErr, fmt.Errorf("cleanup: %w", err))
			log.Printf("shutdown_cleanup_failed error=%v", err)
		}
	}
	log.Printf("shutdown_completed elapsed=%s clean=%t", time.Since(started), shutdownErr == nil)
	return shutdownErr
}
