package s3

type contextKey struct {
	name string
}

var (
	contextKeyBucket    = &contextKey{"bucket"}
	contextKeyObjectKey = &contextKey{"object-key"}
	contextKeyRegion    = &contextKey{"region"}
)
