package proxy_test

import (
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/s3iface"
)

type mockedS3Client struct {
	s3iface.S3API
	GetObjectOutput     s3.GetObjectOutput
	HeadObjectOutput    s3.HeadObjectOutput
	ListObjectsV2Output s3.ListObjectsV2Output
}

func (m *mockedS3Client) HeadObjectRequest(in *s3.HeadObjectInput) *s3.HeadObjectOutput {
	return &m.HeadObjectOutput
}
