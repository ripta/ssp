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
	contextKeyRegion    = &contextKey{"region"}
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

func Region(r *http.Request) string {
	if rv := r.Context().Value(contextKeyRegion); rv != nil {
		return rv.(string)
	}
	return ""
}

func SetBucket(r *http.Request, name string) *http.Request {
	ctx := context.WithValue(r.Context(), contextKeyBucket, name)
	return r.WithContext(ctx)
}

func SetObjectKey(r *http.Request, key string) *http.Request {
	ctx := context.WithValue(r.Context(), contextKeyObjectKey, key)
	return r.WithContext(ctx)
}

func SetRegion(r *http.Request, region string) *http.Request {
	ctx := context.WithValue(r.Context(), contextKeyRegion, region)
	return r.WithContext(ctx)
}

func SetRegionBucket(r *http.Request, region, bucket string) *http.Request {
	ctx := r.Context()
	ctx = context.WithValue(ctx, contextKeyRegion, region)
	ctx = context.WithValue(ctx, contextKeyBucket, bucket)
	return r.WithContext(ctx)
}
