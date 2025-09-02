package breaker

import (
	"context"
	"errors"
)

func ExecuteWithFallbackChain(
	ctx context.Context,
	primary func(context.Context) (interface{}, error),
	fallbacks ...func(context.Context) (interface{}, error),
) (interface{}, error) {

	// coba primary
	result, err := primary(ctx)
	if err == nil {
		return result, nil
	}

	// coba fallback satu per satu
	for _, fb := range fallbacks {
		result, err := fb(ctx)
		if err == nil {
			return result, nil
		}
	}

	return nil, errors.New("all services failed")
}
