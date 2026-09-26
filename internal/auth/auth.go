// Package auth verifies Keycloak access tokens (realm `neno`) against the realm's JWKS.
package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Config struct {
	Issuer   string // e.g. https://sso.mala.co.tz/realms/neno
	Audience string // e.g. neno-api
	// JWKSURL defaults to Issuer + /protocol/openid-connect/certs.
	JWKSURL string
}

// Principal is the authenticated caller.
type Principal struct {
	UserID uuid.UUID
	Email  string
	Name   string
	Role   string // member | editor | reviewer | admin
}

type Verifier struct {
	cfg    Config
	kf     keyfunc.Keyfunc
	parser *jwt.Parser
}

var ErrUnauthorized = errors.New("unauthorized")

// New starts a background JWKS refresher. An unreachable JWKS does not fail startup: tokens are simply
// rejected until keys load (unknown key IDs trigger a rate-limited refetch).
func New(ctx context.Context, cfg Config) (*Verifier, error) {
	if cfg.Issuer == "" || cfg.Audience == "" {
		return nil, errors.New("auth: issuer and audience are required")
	}
	cfg.Issuer = strings.TrimRight(cfg.Issuer, "/")
	if cfg.JWKSURL == "" {
		cfg.JWKSURL = cfg.Issuer + "/protocol/openid-connect/certs"
	}
	kf, err := keyfunc.NewDefaultCtx(ctx, []string{cfg.JWKSURL})
	if err != nil {
		return nil, fmt.Errorf("auth: jwks: %w", err)
	}
	return &Verifier{
		cfg: cfg,
		kf:  kf,
		parser: jwt.NewParser(
			jwt.WithValidMethods([]string{"RS256"}),
			jwt.WithIssuer(cfg.Issuer),
			jwt.WithAudience(cfg.Audience),
			jwt.WithExpirationRequired(),
			jwt.WithLeeway(30*time.Second),
		),
	}, nil
}

type claims struct {
	jwt.RegisteredClaims
	Email       string `json:"email"`
	Name        string `json:"name"`
	RealmAccess struct {
		Roles []string `json:"roles"`
	} `json:"realm_access"`
}

// Verify parses a raw bearer token (without the "Bearer " prefix).
func (v *Verifier) Verify(raw string) (Principal, error) {
	var c claims
	if _, err := v.parser.ParseWithClaims(raw, &c, v.kf.Keyfunc); err != nil {
		return Principal{}, fmt.Errorf("%w: %v", ErrUnauthorized, err)
	}
	id, err := uuid.Parse(c.Subject)
	if err != nil {
		return Principal{}, fmt.Errorf("%w: sub is not a uuid", ErrUnauthorized)
	}
	return Principal{UserID: id, Email: c.Email, Name: c.Name, Role: roleFrom(c.RealmAccess.Roles)}, nil
}

// roleFrom picks the highest Neno role among the realm roles.
func roleFrom(roles []string) string {
	rank := map[string]int{"editor": 1, "reviewer": 2, "admin": 3}
	best, bestRank := "member", 0
	for _, r := range roles {
		if n := rank[r]; n > bestRank {
			best, bestRank = r, n
		}
	}
	return best
}

type ctxKey struct{}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

func FromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(ctxKey{}).(Principal)
	return p, ok
}
