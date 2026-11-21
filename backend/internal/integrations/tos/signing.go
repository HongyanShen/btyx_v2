package tos

import (
	"context"
	"net/url"
	"time"

	"baituber.cn/yanxuan/v2/internal/platform/objectstore"
	sdk "github.com/volcengine/ve-tos-golang-sdk/v2/tos"
	sdkenum "github.com/volcengine/ve-tos-golang-sdk/v2/tos/enum"
)

func (gateway *Gateway) SignUpload(ctx context.Context, request objectstore.UploadRequest) (objectstore.UploadForm, error) {
	result, err := gateway.client.PreSignedPostSignature(ctx, &sdk.PreSingedPostSignatureInput{
		Bucket:  request.Object.Bucket,
		Key:     request.Object.Key,
		Expires: int64(request.Expires / time.Second),
		ContentLengthRange: &sdk.ContentLengthRange{
			RangeStart: request.Size,
			RangeEnd:   request.Size,
		},
		Conditions: []sdk.PostSignatureCondition{
			{
				Key:   "acl",
				Value: "private",
			},
			{
				Key:   "Content-Type",
				Value: string(request.MediaType),
			},
			{
				Key:   "success_action_status",
				Value: "204",
			},
			{
				Key:   "x-tos-forbid-overwrite",
				Value: "true",
			},
		},
	})
	if err != nil {
		return objectstore.UploadForm{}, err
	}

	endpoint, err := url.Parse(gateway.endpoint)
	if err != nil {
		return objectstore.UploadForm{}, err
	}

	endpoint.Host = request.Object.Bucket + "." + endpoint.Host
	endpoint.Path = "/"
	return objectstore.UploadForm{
		URL: endpoint.String(),
		Fields: objectstore.UploadFields{
			Key:             request.Object.Key,
			Policy:          result.Policy,
			Algorithm:       result.Algorithm,
			Credential:      result.Credential,
			Date:            result.Date,
			Signature:       result.Signature,
			ACL:             "private",
			ContentType:     string(request.MediaType),
			SuccessStatus:   "204",
			ForbidOverwrite: "true",
		},
	}, nil
}

func (gateway *Gateway) SignDownload(ctx context.Context, reference objectstore.Reference, lifetime time.Duration) (string, error) {
	query := map[string]string{
		"response-cache-control": "private, no-store",
	}
	if reference.VersionID != "" {
		query["versionId"] = reference.VersionID
	}

	result, err := gateway.client.PreSignedURL(&sdk.PreSignedURLInput{
		HTTPMethod: sdkenum.HttpMethodGet,
		Bucket:     reference.Bucket,
		Key:        reference.Key,
		Expires:    int64(lifetime / time.Second),
		Query:      query,
	})
	if err != nil {
		return "", err
	}

	return result.SignedUrl, nil
}
