package serverutils

import (
	"context"
	"net/http"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	userIDHeader    = "user-id"
	forceQueryParam = "force"
)

func GetUserIdentifier(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Errorf(codes.DataLoss, "failed to get metadata")
	}

	xrid := md[userIDHeader]
	if len(xrid) == 0 {
		return "", status.Errorf(codes.InvalidArgument, "missing '%s' header", userIDHeader)
	}

	userID := strings.TrimSpace(xrid[0])
	if userID == "" {
		return "", status.Errorf(codes.InvalidArgument, "empty '%s' header", userIDHeader)
	}

	return userID, nil
}

func GetForceDelete(ctx context.Context) bool {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return false
	}

	vals := md[forceQueryParam]
	if len(vals) == 0 {
		return false
	}

	return strings.EqualFold(strings.TrimSpace(vals[0]), "true")
}

func ExtractRequestMetadata(_ context.Context, request *http.Request) metadata.MD {
	header := request.Header.Get(userIDHeader)
	pairs := []string{userIDHeader, header}
	if force := request.URL.Query().Get(forceQueryParam); force != "" {
		pairs = append(pairs, forceQueryParam, force)
	}
	return metadata.Pairs(pairs...)
}
