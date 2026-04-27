package v1_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/merlindorin/sshark-api/api/public"
	v1 "github.com/merlindorin/sshark-api/api/public/v1"
	"github.com/merlindorin/sshark-api/internal/domain/publickeys"
	"github.com/merlindorin/sshark-api/internal/domain/sources"
	"github.com/merlindorin/sshark-api/internal/middleware"
)

type fakeSourcesRepo struct {
	sources.Repository
	source *sources.Entity
	err    error
}

func (f *fakeSourcesRepo) GetByProviderAndUsername(_ context.Context, _, _ string) (*sources.Entity, error) {
	return f.source, f.err
}

type fakePublicKeysRepo struct {
	publickeys.Repository
	ssh    []publickeys.Entity
	gpg    []publickeys.Entity
	sshErr error
	gpgErr error
}

func (f *fakePublicKeysRepo) ListBySourceID(
	_ context.Context, _ uuid.UUID, keyType publickeys.KeyType,
) ([]publickeys.Entity, error) {
	if keyType == publickeys.KeyTypeSSH {
		return f.ssh, f.sshErr
	}
	return f.gpg, f.gpgErr
}

func newTestRouter(srcRepo sources.Repository, pkRepo publickeys.Repository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.ErrorHandler(zap.NewNop()))
	handler := v1.NewServer(zap.NewNop(), srcRepo, pkRepo)
	public.RegisterHandlers(router, handler)
	return router
}

func TestGetSourceByProviderAndUsername_OK(t *testing.T) {
	sourceID := uuid.New()
	now := time.Now().UTC()
	intptr := func(v int) *int { return &v }

	srcRepo := &fakeSourcesRepo{source: &sources.Entity{
		ID: sourceID, Provider: "github", UserID: "42", Username: "torvalds",
		URI: "https://github.com/torvalds", CreatedAt: now, UpdatedAt: now,
	}}
	pkRepo := &fakePublicKeysRepo{
		ssh: []publickeys.Entity{{
			ID: uuid.New(), SourceID: sourceID, KeyType: publickeys.KeyTypeSSH,
			KeyData: []byte("ssh-ed25519 AAAA"), Fingerprint: "SHA256:abc",
			CreatedAt: now, UpdatedAt: now,
			SSHMetadata: &publickeys.SSHMetadata{Algorithm: "ssh-ed25519", Comment: "x"},
		}},
		gpg: []publickeys.Entity{{
			ID: uuid.New(), SourceID: sourceID, KeyType: publickeys.KeyTypeGPG,
			KeyData: []byte("-----BEGIN PGP-----"), Fingerprint: "SHA256:def",
			CreatedAt: now, UpdatedAt: now,
			GPGMetadata: &publickeys.GPGMetadata{
				Algorithm: "RSA", KeyBits: intptr(4096),
				UserIDs: []string{"linus@example.com"}, Capabilities: []string{"sign"},
			},
		}},
	}

	router := newTestRouter(srcRepo, pkRepo)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sources/github/torvalds", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	if cc := w.Header().Get("Cache-Control"); cc == "" {
		t.Errorf("expected Cache-Control header to be set, got empty")
	}

	var got public.SourceDetail
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON body: %v", err)
	}
	if got.Username != "torvalds" || got.Provider != "github" {
		t.Errorf("unexpected source identity: %+v", got)
	}
	if len(got.SshKeys) != 1 || len(got.GpgKeys) != 1 {
		t.Errorf("expected 1 ssh + 1 gpg key, got ssh=%d gpg=%d", len(got.SshKeys), len(got.GpgKeys))
	}
}

func TestGetSourceByProviderAndUsername_NotFound(t *testing.T) {
	srcRepo := &fakeSourcesRepo{err: sources.ErrSourceNotFound}
	pkRepo := &fakePublicKeysRepo{}

	router := newTestRouter(srcRepo, pkRepo)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sources/github/ghost", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (body=%s)", w.Code, w.Body.String())
	}
}

func TestGetSourceByProviderAndUsername_InvalidProvider(t *testing.T) {
	router := newTestRouter(&fakeSourcesRepo{}, &fakePublicKeysRepo{})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sources/bitbucket/anyone", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unsupported provider, got %d (body=%s)", w.Code, w.Body.String())
	}
}
