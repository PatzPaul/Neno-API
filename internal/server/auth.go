package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/PatzPaul/Neno-API/internal/api"
	"github.com/PatzPaul/Neno-API/internal/auth"
	"github.com/PatzPaul/Neno-API/internal/store"
)

// protectedOps mirrors the operations with `security: bearer` in api/openapi.yaml.
var protectedOps = map[string]bool{
	"LikeFeedItem": true, "UnlikeFeedItem": true, "AnswerQuiz": true,
	"GetMe": true, "UpdateMe": true, "Sync": true,
}

// AuthMiddleware verifies the bearer token. Protected operations get 401 without a valid one; on public
// operations a valid token is still attached so handlers can personalise.
func (s *Server) AuthMiddleware() api.StrictMiddlewareFunc {
	return func(next api.StrictHandlerFunc, op string) api.StrictHandlerFunc {
		return func(ctx context.Context, w http.ResponseWriter, r *http.Request, req any) (any, error) {
			p, err := s.authenticate(ctx, r)
			switch {
			case err == nil:
				ctx = auth.WithPrincipal(ctx, p)
			case protectedOps[op]:
				writeJSONError(w, http.StatusUnauthorized, "authentication required")
				return nil, nil
			}
			return next(ctx, w, r, req)
		}
	}
}

func (s *Server) authenticate(ctx context.Context, r *http.Request) (auth.Principal, error) {
	if s.verifier == nil {
		return auth.Principal{}, auth.ErrUnauthorized
	}
	h := r.Header.Get("Authorization")
	raw, ok := strings.CutPrefix(h, "Bearer ")
	if !ok || raw == "" {
		return auth.Principal{}, auth.ErrUnauthorized
	}
	p, err := s.verifier.Verify(raw)
	if err != nil {
		return auth.Principal{}, err
	}
	if err := s.ensureUser(ctx, p); err != nil {
		slog.ErrorContext(ctx, "auth: user upsert failed", "err", err)
		return auth.Principal{}, err
	}
	return p, nil
}

// userCache avoids a DB write on every request: users are upserted when first seen, when their email or
// role changes in Keycloak, and at most every 10 minutes otherwise.
type userCache struct {
	mu   sync.Mutex
	seen map[uuid.UUID]cachedUser
}

type cachedUser struct {
	email, role string
	at          time.Time
}

func (s *Server) ensureUser(ctx context.Context, p auth.Principal) error {
	s.users.mu.Lock()
	c, ok := s.users.seen[p.UserID]
	s.users.mu.Unlock()
	if ok && c.email == p.Email && c.role == p.Role && time.Since(c.at) < 10*time.Minute {
		return nil
	}
	err := s.q.UpsertUserFromToken(ctx, store.UpsertUserFromTokenParams{
		ID:          p.UserID,
		Email:       pgtype.Text{String: p.Email, Valid: p.Email != ""},
		DisplayName: pgtype.Text{String: p.Name, Valid: p.Name != ""},
		Role:        store.UserRole(p.Role),
	})
	if err != nil {
		return err
	}
	s.users.mu.Lock()
	s.users.seen[p.UserID] = cachedUser{email: p.Email, role: p.Role, at: time.Now()}
	s.users.mu.Unlock()
	return nil
}

// principal returns the caller for a protected operation (the middleware guarantees it is set).
func principal(ctx context.Context) (auth.Principal, error) {
	p, ok := auth.FromContext(ctx)
	if !ok {
		return p, errors.New("no principal in context for protected operation")
	}
	return p, nil
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(api.Error{Error: msg})
}
