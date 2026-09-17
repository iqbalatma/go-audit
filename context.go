package audit

import "context"

// Actor is who performed the action, attached to context by your auth middleware.
type Actor struct {
	Table string
	ID    any
	Name  string
	Email string
	Phone string
}

// RequestInfo is network/request metadata, attached to context by your HTTP middleware.
type RequestInfo struct {
	IPAddress string
	Method    string
	Endpoint  string
	UserAgent string
}

type ctxKey int

const (
	ctxKeyActor ctxKey = iota
	ctxKeyRequestInfo
)

func WithActor(ctx context.Context, actor Actor) context.Context {
	return context.WithValue(ctx, ctxKeyActor, actor)
}

func WithRequestInfo(ctx context.Context, info RequestInfo) context.Context {
	return context.WithValue(ctx, ctxKeyRequestInfo, info)
}

func actorFromContext(ctx context.Context) Actor {
	actor, _ := ctx.Value(ctxKeyActor).(Actor)
	return actor
}

func requestInfoFromContext(ctx context.Context) RequestInfo {
	info, _ := ctx.Value(ctxKeyRequestInfo).(RequestInfo)
	return info
}
