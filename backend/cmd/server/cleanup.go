package main

import (
	"context"
	"errors"
	"fmt"
	"log"
)

type cleanupStep struct {
	name string
	fn   func() error
}

type cleanupStage struct {
	name     string
	parallel bool
	steps    []cleanupStep
}

// runCleanupStages bounds the wait even for legacy Stop methods without a
// context. On deadline it leaves dependencies open: a still-running worker must
// not race a database close. The process exits immediately after this returns.
func runCleanupStages(ctx context.Context, stages ...cleanupStage) error {
	var failures []error
	for _, stage := range stages {
		if err := ctx.Err(); err != nil {
			return errors.Join(append(failures, err)...)
		}
		log.Printf("[Cleanup] stage=%s started", stage.name)
		stageFailures := len(failures)
		run := func(step cleanupStep) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := step.fn(); err != nil {
				log.Printf("[Cleanup] %s failed: %v", step.name, err)
				return fmt.Errorf("%s: %w", step.name, err)
			}
			log.Printf("[Cleanup] %s succeeded", step.name)
			return nil
		}
		if stage.parallel {
			results := make(chan error, len(stage.steps))
			for _, step := range stage.steps {
				go func() { results <- run(step) }()
			}
			for range stage.steps {
				select {
				case err := <-results:
					if err != nil {
						failures = append(failures, err)
					}
				case <-ctx.Done():
					return errors.Join(append(failures, fmt.Errorf("cleanup stage %s: %w", stage.name, ctx.Err()))...)
				}
			}
		} else {
			for _, step := range stage.steps {
				if err := ctx.Err(); err != nil {
					return errors.Join(append(failures, err)...)
				}
				result := make(chan error, 1)
				go func() { result <- run(step) }()
				select {
				case err := <-result:
					if err != nil {
						return errors.Join(append(failures, err)...)
					}
				case <-ctx.Done():
					return errors.Join(append(failures, fmt.Errorf("cleanup step %s: %w", step.name, ctx.Err()))...)
				}
			}
		}
		if len(failures) > stageFailures {
			return errors.Join(failures...)
		}
	}
	return errors.Join(failures...)
}
