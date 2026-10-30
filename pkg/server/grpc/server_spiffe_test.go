package grpc

import (
	"context"
	"crypto/tls"
	"errors"
	"testing"

	"google.golang.org/grpc"

	"open-cluster-management.io/sdk-go/pkg/server/grpc/authz"
)

type failingAuthenticator struct{}

func (failingAuthenticator) Authenticate(ctx context.Context) (context.Context, error) {
	return ctx, errors.New("no credentials")
}

func TestUnauthenticatedMethodsSkipAuthnAndAuthz(t *testing.T) {
	const open, closed = "/svc/Open", "/svc/Closed"
	authn := newAuthnUnaryInterceptor(map[string]bool{open: true}, failingAuthenticator{})
	authzI := newAuthzUnaryInterceptor(map[string]bool{open: true},
		&testUnaryAuthorizer{decision: authz.DecisionDeny})
	handler := func(ctx context.Context, req any) (any, error) { return "ok", nil }
	chain := func(method string) error {
		info := &grpc.UnaryServerInfo{FullMethod: method}
		_, err := authn(context.Background(), nil, info, func(ctx context.Context, req any) (any, error) {
			return authzI(ctx, req, info, handler)
		})
		return err
	}

	if err := chain(open); err != nil {
		t.Errorf("unauthenticated method was refused: %v", err)
	}
	if err := chain(closed); err == nil {
		t.Error("other methods must still be authenticated")
	}
}

func TestSelfAuthorizedMethodsStillAuthenticate(t *testing.T) {
	const method = "/svc/Self"
	b := NewGRPCServer(NewGRPCServerOptions()).WithSelfAuthorizedMethods(method)
	authn := newAuthnUnaryInterceptor(b.unauthenticated, failingAuthenticator{})
	if _, err := authn(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: method},
		func(ctx context.Context, req any) (any, error) { return nil, nil }); err == nil {
		t.Fatal("a self-authorized method must still be authenticated")
	}
	authzI := newAuthzUnaryInterceptor(b.skipAuthz(), &testUnaryAuthorizer{decision: authz.DecisionDeny})
	if _, err := authzI(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: method},
		func(ctx context.Context, req any) (any, error) { return nil, nil }); err != nil {
		t.Fatalf("a self-authorized method must skip the authorizers: %v", err)
	}
}

func TestWithTLSConfigAppliesVersionBounds(t *testing.T) {
	opts := NewGRPCServerOptions()
	opts.TLSMinVersion = "VersionTLS13"
	if err := opts.Validate(); err != nil {
		t.Fatal(err)
	}
	getCert := func(*tls.ClientHelloInfo) (*tls.Certificate, error) { return &tls.Certificate{}, nil }
	b := NewGRPCServer(opts).WithTLSConfig(&tls.Config{GetCertificate: getCert, MinVersion: tls.VersionTLS12})
	cfg, err := b.buildTLSConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MinVersion != tls.VersionTLS13 {
		t.Errorf("want the configured minimum TLS version enforced, got %x", cfg.MinVersion)
	}
	if cfg.GetCertificate == nil {
		t.Error("the in-memory certificate callback was dropped")
	}
}
