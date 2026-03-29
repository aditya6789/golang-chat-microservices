package attachment

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

// Store wraps S3-compatible storage (MinIO, AWS S3) for presigned uploads.
type Store struct {
	client     *s3.Client
	presign    *s3.PresignClient
	bucket     string
	publicBase string
	maxBytes   int64
	presignTTL time.Duration
}

type Config struct {
	Endpoint     string
	Region       string
	AccessKey    string
	SecretKey    string
	Bucket       string
	PublicBase   string
	UsePathStyle bool
	MaxBytes     int64
	PresignTTL   time.Duration
}

func newS3Client(ctx context.Context, endpointURL, region, accessKey, secretKey string, usePathStyle bool) (*s3.Client, error) {
	endpointURL = strings.TrimRight(endpointURL, "/")
	resolver := aws.EndpointResolverWithOptionsFunc(func(service, reg string, options ...interface{}) (aws.Endpoint, error) {
		if service == s3.ServiceID {
			return aws.Endpoint{URL: endpointURL, HostnameImmutable: true}, nil
		}
		return aws.Endpoint{}, &aws.EndpointNotFoundError{}
	})
	awscfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithEndpointResolverWithOptions(resolver),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		return nil, err
	}
	return s3.NewFromConfig(awscfg, func(o *s3.Options) {
		o.UsePathStyle = usePathStyle
	}), nil
}

// NewStore builds an S3 client for MinIO or AWS. Missing required fields returns nil, nil.
// Presigned URLs use PublicBase (browser-reachable host); HeadObject uses Endpoint (in-cluster host).
func NewStore(ctx context.Context, c Config) (*Store, error) {
	if c.Endpoint == "" || c.Bucket == "" || c.AccessKey == "" || c.SecretKey == "" {
		return nil, nil
	}
	region := c.Region
	if region == "" {
		region = "us-east-1"
	}
	maxB := c.MaxBytes
	if maxB <= 0 {
		maxB = 10 << 20
	}
	ttl := c.PresignTTL
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	internalURL := strings.TrimRight(c.Endpoint, "/")
	publicBase := strings.TrimRight(c.PublicBase, "/")
	if publicBase == "" {
		publicBase = internalURL
	}
	internalCli, err := newS3Client(ctx, internalURL, region, c.AccessKey, c.SecretKey, c.UsePathStyle)
	if err != nil {
		return nil, err
	}
	presignBase := publicBase
	if presignBase != internalURL {
		// Browser must call this host; signature is bound to it.
		presignCli, err := newS3Client(ctx, presignBase, region, c.AccessKey, c.SecretKey, c.UsePathStyle)
		if err != nil {
			return nil, err
		}
		return &Store{
			client:     internalCli,
			presign:    s3.NewPresignClient(presignCli),
			bucket:     c.Bucket,
			publicBase: publicBase,
			maxBytes:   maxB,
			presignTTL: ttl,
		}, nil
	}
	return &Store{
		client:     internalCli,
		presign:    s3.NewPresignClient(internalCli),
		bucket:     c.Bucket,
		publicBase: publicBase,
		maxBytes:   maxB,
		presignTTL: ttl,
	}, nil
}

var safeNameRe = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func sanitizeFilename(name string) string {
	name = path.Base(strings.TrimSpace(name))
	if name == "" || name == "." {
		return "file"
	}
	if utf8.RuneCountInString(name) > 120 {
		name = string([]rune(name)[:120])
	}
	return safeNameRe.ReplaceAllString(name, "_")
}

// AllowedContentType returns whether uploads of this type are allowed.
func AllowedContentType(ct string) bool {
	ct = strings.ToLower(strings.TrimSpace(strings.Split(ct, ";")[0]))
	if ct == "" {
		return false
	}
	if strings.HasPrefix(ct, "image/") {
		return true
	}
	if strings.HasPrefix(ct, "text/") {
		return true
	}
	switch ct {
	case "application/pdf",
		"application/msword",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/vnd.ms-excel",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"application/vnd.ms-powerpoint",
		"application/vnd.openxmlformats-officedocument.presentationml.presentation":
		return true
	default:
		return false
	}
}

// KeyPrefixForChat returns the required S3 key prefix for objects in a chat.
func KeyPrefixForChat(chatID string) string {
	return fmt.Sprintf("chats/%s/", chatID)
}

// PresignPut returns a PUT URL and object key under chats/{chatID}/...
func (s *Store) PresignPut(ctx context.Context, chatID, filename, contentType string, size int64) (uploadURL, objectKey string, headers map[string]string, err error) {
	if s == nil {
		return "", "", nil, fmt.Errorf("attachments not configured")
	}
	if size <= 0 || size > s.maxBytes {
		return "", "", nil, fmt.Errorf("size must be between 1 and %d bytes", s.maxBytes)
	}
	if !AllowedContentType(contentType) {
		return "", "", nil, fmt.Errorf("content type not allowed")
	}
	objectKey = fmt.Sprintf("%s%s-%s", KeyPrefixForChat(chatID), uuid.NewString(), sanitizeFilename(filename))
	out, err := s.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(objectKey),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(s.presignTTL))
	if err != nil {
		return "", "", nil, err
	}
	h := make(map[string]string)
	for k, v := range out.SignedHeader {
		if strings.EqualFold(k, "host") {
			continue // browser sets Host from the URL; do not forward internal Docker hostname
		}
		if len(v) > 0 {
			h[k] = v[0]
		}
	}
	return out.URL, objectKey, h, nil
}

// HeadObject verifies the object exists and returns size and content type from storage.
func (s *Store) HeadObject(ctx context.Context, key string) (size int64, contentType string, err error) {
	if s == nil {
		return 0, "", fmt.Errorf("attachments not configured")
	}
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return 0, "", err
	}
	if out.ContentLength != nil {
		size = *out.ContentLength
	}
	if out.ContentType != nil {
		contentType = strings.TrimSpace(strings.Split(*out.ContentType, ";")[0])
	}
	return size, contentType, nil
}

// DownloadURL is a path-style URL clients can use when the bucket allows anonymous read.
func (s *Store) DownloadURL(key string) string {
	if s == nil {
		return ""
	}
	return fmt.Sprintf("%s/%s/%s", s.publicBase, s.bucket, key)
}

// MaxBytes returns the configured upload limit.
func (s *Store) MaxBytes() int64 {
	if s == nil {
		return 0
	}
	return s.maxBytes
}
