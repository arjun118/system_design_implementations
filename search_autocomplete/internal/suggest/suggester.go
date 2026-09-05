package suggest

import "context"

type Suggester interface {
	Suggest(prefix string) ([]string, string)
	Init(ctx context.Context, memUnits int, redisUnits int) error
}
