package server

type contextKey struct {
	name string
}

var (
	contextKeyBucket    = &contextKey{"bucket"}
	contextKeyObjectKey = &contextKey{"object-key"}
	contextKeyRegion    = &contextKey{"region"}
)
