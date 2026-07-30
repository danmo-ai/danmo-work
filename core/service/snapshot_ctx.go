package service

import "context"

type snapshotPathsKey struct{}

// WithSnapshotPaths attaches extra pre-turn snapshot paths to the StartTurn context.
func WithSnapshotPaths(ctx context.Context, paths []string) context.Context {
	if len(paths) == 0 {
		return ctx
	}
	cp := append([]string(nil), paths...)
	return context.WithValue(ctx, snapshotPathsKey{}, cp)
}

// SnapshotPathsFromCtx returns paths queued for pre-turn snapshot, if any.
func SnapshotPathsFromCtx(ctx context.Context) []string {
	v, _ := ctx.Value(snapshotPathsKey{}).([]string)
	return v
}
