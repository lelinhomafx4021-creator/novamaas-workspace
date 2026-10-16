package runtimewatch

import "context"

type categoryContextKey struct{}

func WithCategory(ctx context.Context, category string) context.Context {
	if !Enabled() {
		return ctx
	}
	return context.WithValue(ctx, categoryContextKey{}, categories[counterIndex(ScopeInbound, category)%categoryCount])
}

func Category(ctx context.Context) string {
	if category, ok := ctx.Value(categoryContextKey{}).(string); ok {
		return category
	}
	return CategoryOther
}
