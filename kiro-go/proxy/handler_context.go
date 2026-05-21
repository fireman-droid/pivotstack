package proxy

import (
	"context"
	"net/http"
)

func withUserContext(r *http.Request, uc *UserContext) *http.Request {
	if uc == nil {
		return r
	}
	return r.WithContext(context.WithValue(r.Context(), userCtxKey, uc))
}

func getUserContext(ctx context.Context) *UserContext {
	if v, ok := ctx.Value(userCtxKey).(*UserContext); ok {
		return v
	}
	return nil
}

func allowReasoningSource(source *thinkingStreamSource) bool {
	if *source == thinkingSourceTagBlock {
		return false
	}
	*source = thinkingSourceReasoningEvent
	return true
}

func allowTagSource(source *thinkingStreamSource) bool {
	if *source == thinkingSourceReasoningEvent {
		return false
	}
	if *source == thinkingSourceUnknown {
		*source = thinkingSourceTagBlock
	}
	return *source == thinkingSourceTagBlock
}
