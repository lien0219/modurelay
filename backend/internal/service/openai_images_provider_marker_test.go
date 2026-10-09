package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMarkOpenAIImagesProviderStartedFailsBeforeBudgetBoundary(t *testing.T) {
	markerErr := errors.New("durable image marker unavailable")
	called := false
	ctx := WithMediaProviderStart(context.Background(), func(context.Context, int64) error {
		called = true
		return markerErr
	})

	err := markOpenAIImagesProviderStarted(ctx, 42)

	require.ErrorIs(t, err, markerErr)
	require.True(t, called)
}
