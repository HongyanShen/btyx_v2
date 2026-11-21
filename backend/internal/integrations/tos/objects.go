package tos

import (
	"context"
	"io"
	"mime"

	"baituber.cn/yanxuan/v2/internal/platform/objectstore"
	sdk "github.com/volcengine/ve-tos-golang-sdk/v2/tos"
	sdkenum "github.com/volcengine/ve-tos-golang-sdk/v2/tos/enum"
)

func (gateway *Gateway) Inspect(ctx context.Context, reference objectstore.Reference) (objectstore.Metadata, error) {
	result, err := gateway.client.HeadObjectV2(ctx, &sdk.HeadObjectV2Input{
		Bucket:    reference.Bucket,
		Key:       reference.Key,
		VersionID: reference.VersionID,
	})
	if err != nil {
		return objectstore.Metadata{}, err
	}

	return objectstore.Metadata{
		Size:      result.ContentLength,
		MediaType: result.ContentType,
		ETag:      result.ETag,
		VersionID: result.VersionID,
	}, nil
}

func (gateway *Gateway) ReadPrefix(ctx context.Context, reference objectstore.Reference, etag string) ([]byte, error) {
	result, err := gateway.client.GetObjectV2(ctx, &sdk.GetObjectV2Input{
		Bucket:    reference.Bucket,
		Key:       reference.Key,
		VersionID: reference.VersionID,
		IfMatch:   etag,
		Range:     "bytes=0-511",
	})
	if err != nil {
		return nil, err
	}

	defer result.Content.Close()
	return io.ReadAll(io.LimitReader(result.Content, 512))
}

func (gateway *Gateway) Copy(ctx context.Context, request objectstore.CopyRequest) (objectstore.Metadata, error) {
	result, err := gateway.client.CopyObject(ctx, &sdk.CopyObjectInput{
		Bucket:             request.Destination.Bucket,
		Key:                request.Destination.Key,
		SrcBucket:          request.Source.Bucket,
		SrcKey:             request.Source.Key,
		SrcVersionID:       request.Source.VersionID,
		CopySourceIfMatch:  request.SourceETag,
		ACL:                sdkenum.ACLPrivate,
		MetadataDirective:  sdkenum.MetadataDirectiveReplace,
		ContentType:        string(request.MediaType),
		ContentDisposition: mime.FormatMediaType("attachment", map[string]string{"filename": request.FileName}),
		CacheControl:       "private, no-store",
		ForbidOverwrite:    true,
	})
	if err != nil {
		return objectstore.Metadata{}, err
	}

	return objectstore.Metadata{
		ETag:      result.ETag,
		VersionID: result.VersionID,
	}, nil
}

func (gateway *Gateway) Delete(ctx context.Context, reference objectstore.Reference) error {
	_, err := gateway.client.DeleteObjectV2(ctx, &sdk.DeleteObjectV2Input{
		Bucket:    reference.Bucket,
		Key:       reference.Key,
		VersionID: reference.VersionID,
	})
	return err
}
