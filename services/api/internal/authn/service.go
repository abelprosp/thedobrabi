package authn

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thedobra/thedobra/services/api/internal/cryptoenc"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	pg     *pgxpool.Pool
	secret []byte
	encKey []byte
}

func New(pg *pgxpool.Pool, secret, encKey []byte) *Service {
	return &Service{pg: pg, secret: secret, encKey: encKey}
}

type Principal struct {
	UserID              uuid.UUID `json:"user_id"`
	OrgID               uuid.UUID `json:"org_id"`
	WorkspaceID         uuid.UUID `json:"workspace_id"`
	Role                string    `json:"role"`
	Email               string    `json:"email"`
	Name                string    `json:"name"`
	Plan                string    `json:"plan"`
	OrgName             string    `json:"org_name"`
	MFAEnabled          bool      `json:"mfa_enabled"`
	OnboardingStep      string    `json:"onboarding_step"`
	OnboardingCompleted bool      `json:"onboarding_completed"`
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

func (s *Service) Register(ctx context.Context, name, email, password, orgName string) (Principal, TokenPair, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || password == "" || name == "" {
		return Principal{}, TokenPair{}, fmt.Errorf("nome, e-mail e senha são obrigatórios")
	}
	if len(password) < 8 {
		return Principal{}, TokenPair{}, fmt.Errorf("a senha deve ter pelo menos 8 caracteres")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return Principal{}, TokenPair{}, err
	}
	org := strings.TrimSpace(orgName)
	if org == "" {
		org = name + " Org"
	}
	slug := slugify(org)

	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return Principal{}, TokenPair{}, err
	}
	defer tx.Rollback(ctx)

	var userID uuid.UUID
	err = tx.QueryRow(ctx, `INSERT INTO users (email, password_hash, name) VALUES ($1,$2,$3) RETURNING id`,
		email, string(hash), name).Scan(&userID)
	if err != nil {
		if strings.Contains(err.Error(), "unique") {
			return Principal{}, TokenPair{}, fmt.Errorf("este e-mail já está cadastrado")
		}
		return Principal{}, TokenPair{}, err
	}

	var orgID uuid.UUID
	baseSlug := slug
	for i := 0; i < 8; i++ {
		try := baseSlug
		if i > 0 {
			try = fmt.Sprintf("%s-%d", baseSlug, i+1)
		}
		err = tx.QueryRow(ctx, `INSERT INTO organizations (name, slug) VALUES ($1,$2) RETURNING id`, org, try).Scan(&orgID)
		if err == nil {
			slug = try
			break
		}
		if !strings.Contains(err.Error(), "unique") {
			return Principal{}, TokenPair{}, err
		}
	}

	if _, err := tx.Exec(ctx, `INSERT INTO organization_members (org_id, user_id, role) VALUES ($1,$2,'owner')`, orgID, userID); err != nil {
		return Principal{}, TokenPair{}, err
	}

	var wsID uuid.UUID
	err = tx.QueryRow(ctx, `INSERT INTO workspaces (org_id, name, slug) VALUES ($1,'Padrão','default') RETURNING id`, orgID).Scan(&wsID)
	if err != nil {
		return Principal{}, TokenPair{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Principal{}, TokenPair{}, err
	}

	p := Principal{UserID: userID, OrgID: orgID, WorkspaceID: wsID, Role: "owner", Email: email, Name: name, Plan: "starter", OrgName: org, OnboardingStep: "welcome", OnboardingCompleted: false}
	pair, err := s.issue(ctx, p)
	return p, pair, err
}

func (s *Service) Login(ctx context.Context, email, password string) (Principal, TokenPair, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var p Principal
	var hash string
	err := s.pg.QueryRow(ctx, `
		SELECT u.id, u.email, u.name, u.password_hash, o.id, o.name, o.plan, om.role, w.id,
		       o.onboarding_step, o.onboarding_completed_at IS NOT NULL
		FROM users u
		JOIN organization_members om ON om.user_id = u.id
		JOIN organizations o ON o.id = om.org_id
		JOIN workspaces w ON w.org_id = o.id
		WHERE u.email = $1
		ORDER BY om.created_at ASC, w.created_at ASC
		LIMIT 1
	`, email).Scan(&p.UserID, &p.Email, &p.Name, &hash, &p.OrgID, &p.OrgName, &p.Plan, &p.Role, &p.WorkspaceID, &p.OnboardingStep, &p.OnboardingCompleted)
	if err != nil {
		if err == pgx.ErrNoRows {
			return Principal{}, TokenPair{}, fmt.Errorf("credenciais inválidas")
		}
		return Principal{}, TokenPair{}, err
	}
	var provider string
	_ = s.pg.QueryRow(ctx, `SELECT COALESCE(auth_provider,'password') FROM users WHERE id=$1`, p.UserID).Scan(&provider)
	if provider != "" && provider != "password" {
		return Principal{}, TokenPair{}, fmt.Errorf("esta conta entra via SSO (%s)", provider)
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return Principal{}, TokenPair{}, fmt.Errorf("credenciais inválidas")
	}
	var active, mfa bool
	_ = s.pg.QueryRow(ctx, `SELECT COALESCE(active, TRUE), mfa_enabled FROM users WHERE id=$1`, p.UserID).Scan(&active, &mfa)
	if !active {
		return Principal{}, TokenPair{}, fmt.Errorf("conta desactivada")
	}
	if mfa {
		ch, err := s.BeginMFA(ctx, p.UserID)
		if err != nil {
			return Principal{}, TokenPair{}, err
		}
		return p, TokenPair{AccessToken: ch, TokenType: "mfa_required", ExpiresIn: 300}, nil
	}
	pair, err := s.issue(ctx, p)
	return p, pair, err
}

func (s *Service) Refresh(ctx context.Context, refresh string) (Principal, TokenPair, error) {
	hash := cryptoenc.HashToken(refresh)
	var userID uuid.UUID
	var expires time.Time
	var revoked *time.Time
	err := s.pg.QueryRow(ctx, `SELECT user_id, expires_at, revoked_at FROM refresh_tokens WHERE token_hash=$1`, hash).
		Scan(&userID, &expires, &revoked)
	if err != nil {
		return Principal{}, TokenPair{}, fmt.Errorf("refresh token inválido")
	}
	if revoked != nil || time.Now().After(expires) {
		return Principal{}, TokenPair{}, fmt.Errorf("refresh token expirado")
	}
	p, err := s.principalForUser(ctx, userID, uuid.Nil)
	if err != nil {
		return Principal{}, TokenPair{}, err
	}
	_, _ = s.pg.Exec(ctx, `UPDATE refresh_tokens SET revoked_at=now() WHERE token_hash=$1`, hash)
	pair, err := s.issue(ctx, p)
	return p, pair, err
}

func (s *Service) Logout(ctx context.Context, refresh string) {
	if refresh == "" {
		return
	}
	_, _ = s.pg.Exec(ctx, `UPDATE refresh_tokens SET revoked_at=now() WHERE token_hash=$1`, cryptoenc.HashToken(refresh))
}

func (s *Service) ParseAccess(token string) (Principal, error) {
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected alg")
		}
		return s.secret, nil
	})
	if err != nil || !parsed.Valid {
		return Principal{}, fmt.Errorf("unauthorized")
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return Principal{}, fmt.Errorf("unauthorized")
	}
	p := Principal{
		Role:                str(claims["role"]),
		Email:               str(claims["email"]),
		Name:                str(claims["name"]),
		Plan:                str(claims["plan"]),
		OnboardingStep:      str(claims["onboarding_step"]),
		OnboardingCompleted: boolClaim(claims["onboarding_completed"]),
	}
	p.UserID, _ = uuid.Parse(str(claims["sub"]))
	p.OrgID, _ = uuid.Parse(str(claims["org_id"]))
	p.WorkspaceID, _ = uuid.Parse(str(claims["workspace_id"]))
	if p.UserID == uuid.Nil {
		return Principal{}, fmt.Errorf("unauthorized")
	}
	return p, nil
}

func (s *Service) Principal(ctx context.Context, userID, workspaceID uuid.UUID) (Principal, error) {
	return s.principalForUser(ctx, userID, workspaceID)
}

func (s *Service) principalForUser(ctx context.Context, userID, workspaceID uuid.UUID) (Principal, error) {
	var p Principal
	q := `
		SELECT u.id, u.email, u.name, o.id, o.name, o.plan, om.role, w.id, COALESCE(u.mfa_enabled, FALSE),
		       o.onboarding_step, o.onboarding_completed_at IS NOT NULL
		FROM users u
		JOIN organization_members om ON om.user_id = u.id
		JOIN organizations o ON o.id = om.org_id
		JOIN workspaces w ON w.org_id = o.id
		WHERE u.id = $1
	`
	args := []any{userID}
	if workspaceID != uuid.Nil {
		q += ` AND w.id = $2`
		args = append(args, workspaceID)
	}
	q += ` ORDER BY om.created_at ASC, w.created_at ASC LIMIT 1`
	err := s.pg.QueryRow(ctx, q, args...).Scan(&p.UserID, &p.Email, &p.Name, &p.OrgID, &p.OrgName, &p.Plan, &p.Role, &p.WorkspaceID, &p.MFAEnabled, &p.OnboardingStep, &p.OnboardingCompleted)
	return p, err
}

func (s *Service) Issue(ctx context.Context, p Principal) (TokenPair, error) {
	return s.issue(ctx, p)
}

func (s *Service) issue(ctx context.Context, p Principal) (TokenPair, error) {
	now := time.Now()
	accessTTL := 8 * time.Hour
	claims := jwt.MapClaims{
		"sub":                  p.UserID.String(),
		"org_id":               p.OrgID.String(),
		"workspace_id":         p.WorkspaceID.String(),
		"role":                 p.Role,
		"email":                p.Email,
		"name":                 p.Name,
		"plan":                 p.Plan,
		"onboarding_step":      p.OnboardingStep,
		"onboarding_completed": p.OnboardingCompleted,
		"iat":                  now.Unix(),
		"exp":                  now.Add(accessTTL).Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	access, err := tok.SignedString(s.secret)
	if err != nil {
		return TokenPair{}, err
	}
	refresh, err := cryptoenc.RandomToken(32)
	if err != nil {
		return TokenPair{}, err
	}
	_, err = s.pg.Exec(ctx, `INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1,$2,$3)`,
		p.UserID, cryptoenc.HashToken(refresh), now.Add(30*24*time.Hour))
	if err != nil {
		return TokenPair{}, err
	}
	return TokenPair{AccessToken: access, RefreshToken: refresh, ExpiresIn: int(accessTTL.Seconds()), TokenType: "Bearer"}, nil
}

func (s *Service) UpsertSSO(ctx context.Context, email, name, provider, subject string) (Principal, TokenPair, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return Principal{}, TokenPair{}, fmt.Errorf("o provedor SSO não enviou um e-mail")
	}
	if name == "" {
		name = strings.Split(email, "@")[0]
	}
	var userID uuid.UUID
	err := s.pg.QueryRow(ctx, `SELECT user_id FROM oauth_identities WHERE provider=$1 AND subject=$2`, provider, subject).Scan(&userID)
	if err == pgx.ErrNoRows {
		err = s.pg.QueryRow(ctx, `SELECT id FROM users WHERE email=$1`, email).Scan(&userID)
		if err == pgx.ErrNoRows {
			hash, _ := bcrypt.GenerateFromPassword([]byte(uuid.NewString()), 10)
			err = s.pg.QueryRow(ctx, `INSERT INTO users (email, password_hash, name, auth_provider, external_id) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
				email, string(hash), name, provider, subject).Scan(&userID)
			if err != nil {
				return Principal{}, TokenPair{}, err
			}
			org := name
			slug := slugify(org)
			var orgID uuid.UUID
			err = s.pg.QueryRow(ctx, `INSERT INTO organizations (name, slug) VALUES ($1,$2) RETURNING id`, org, slug+"-"+userID.String()[:6]).Scan(&orgID)
			if err != nil {
				return Principal{}, TokenPair{}, err
			}
			_, _ = s.pg.Exec(ctx, `INSERT INTO organization_members (org_id, user_id, role) VALUES ($1,$2,'owner')`, orgID, userID)
			_, _ = s.pg.Exec(ctx, `INSERT INTO workspaces (org_id, name, slug) VALUES ($1,'Padrão','default')`, orgID)
		} else if err != nil {
			return Principal{}, TokenPair{}, err
		}
		_, _ = s.pg.Exec(ctx, `INSERT INTO oauth_identities (user_id, provider, subject, email) VALUES ($1,$2,$3,$4) ON CONFLICT (provider, subject) DO NOTHING`,
			userID, provider, subject, email)
		_, _ = s.pg.Exec(ctx, `UPDATE users SET auth_provider=$2, external_id=$3 WHERE id=$1`, userID, provider, subject)
	} else if err != nil {
		return Principal{}, TokenPair{}, err
	}
	p, err := s.principalForUser(ctx, userID, uuid.Nil)
	if err != nil {
		return Principal{}, TokenPair{}, err
	}
	if p.OnboardingStep == "" {
		p.OnboardingStep = "welcome"
	}
	pair, err := s.issue(ctx, p)
	return p, pair, err
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "org"
	}
	return out
}

func str(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func boolClaim(v any) bool {
	if v == nil {
		return false
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return str(v) == "true"
}
