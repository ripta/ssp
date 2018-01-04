package server

import (
	"context"
	"net/http"
)

type contextKey struct {
	name string
}

var (
	contextKeyBucket    = &contextKey{"bucket"}
	contextKeyObjectKey = &contextKey{"object-key"}
)

func Bucket(r *http.Request) string {
	if rv := r.Context().Value(contextKeyBucket); rv != nil {
		return rv.(string)
	}
	return ""
}

func ObjectKey(r *http.Request) string {
	if rv := r.Context().Value(contextKeyObjectKey); rv != nil {
		return rv.(string)
	}
	return ""
}

func SetBucket(r *http.Request, name string) *http.Request {
	ctx := context.WithValue(r.Context(), contextKeyBucket, name)
	return r.WithContext(ctx)
}

func SetBucketAndObjectKey(r *http.Request, bucket, key string) *http.Request {
	ctx := r.Context()
	ctx = context.WithValue(ctx, contextKeyBucket, bucket)
	ctx = context.WithValue(ctx, contextKeyObjectKey, key)
	return r.WithContext(ctx)
}

func SetObjectKey(r *http.Request, key string) *http.Request {
	ctx := context.WithValue(r.Context(), contextKeyObjectKey, key)
	return r.WithContext(ctx)
}
