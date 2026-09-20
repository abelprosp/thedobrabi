package authn

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/thedobra/thedobra/services/api/internal/cryptoenc"
	"golang.org/x/crypto/bcrypt"
)

func (s *Service) BeginMFA(ctx context.Context, userID uuid.UUID) (string, error) {
	raw, err := cryptoenc.RandomToken(24)
	if err != nil {
		return "", err
	}
	_, _ = s.pg.Exec(ctx, `DELETE FROM mfa_challenges WHERE user_id=$1 OR expires_at < now()`, userID)
	_, err = s.pg.Exec(ctx, `INSERT INTO mfa_challenges (user_id, token_hash, expires_at) VALUES ($1,$2,$3)`,
		userID, cryptoenc.HashToken(raw), time.Now().Add(5*time.Minute))
	return raw, err
}

func (s *Service) FinishMFA(ctx context.Context, challenge, code string) (Principal, TokenPair, error) {
	var userID uuid.UUID
	var exp time.Time
	err := s.pg.QueryRow(ctx, `SELECT user_id, expires_at FROM mfa_challenges WHERE token_hash=$1`, cryptoenc.HashToken(challenge)).Scan(&userID, &exp)
	if err != nil || time.Now().After(exp) {
		return Principal{}, TokenPair{}, fmt.Errorf("desafio MFA inválido ou expirado")
	}
	var enc string
	_ = s.pg.QueryRow(ctx, `SELECT COALESCE(mfa_secret_enc,'') FROM users WHERE id=$1`, userID).Scan(&enc)
	secret, err := cryptoenc.Decrypt(s.encKey, enc)
	if err != nil || !VerifyTOTP(secret, code) {
		return Principal{}, TokenPair{}, fmt.Errorf("código MFA inválido")
	}
	_, _ = s.pg.Exec(ctx, `DELETE FROM mfa_challenges WHERE token_hash=$1`, cryptoenc.HashToken(challenge))
	p, err := s.principalForUser(ctx, userID, uuid.Nil)
	if err != nil {
		return Principal{}, TokenPair{}, err
	}
	pair, err := s.issue(ctx, p)
	return p, pair, err
}

func (s *Service) BeginEnrollMFA(ctx context.Context, userID uuid.UUID, email string) (secret, url string, err error) {
	secret, err = NewTOTPSecret()
	if err != nil {
		return "", "", err
	}
	enc, err := cryptoenc.Encrypt(s.encKey, secret)
	if err != nil {
		return "", "", err
	}
	_, err = s.pg.Exec(ctx, `UPDATE users SET mfa_pending_enc=$2, updated_at=now() WHERE id=$1`, userID, enc)
	return secret, OTPAuthURL(email, secret), err
}

func (s *Service) ConfirmEnrollMFA(ctx context.Context, userID uuid.UUID, code string) error {
	var enc string
	err := s.pg.QueryRow(ctx, `SELECT COALESCE(mfa_pending_enc,'') FROM users WHERE id=$1`, userID).Scan(&enc)
	if err != nil || enc == "" {
		return fmt.Errorf("não há inscrição MFA pendente")
	}
	secret, err := cryptoenc.Decrypt(s.encKey, enc)
	if err != nil || !VerifyTOTP(secret, code) {
		return fmt.Errorf("código MFA inválido")
	}
	_, err = s.pg.Exec(ctx, `UPDATE users SET mfa_secret_enc=$2, mfa_pending_enc=NULL, mfa_enabled=TRUE, updated_at=now() WHERE id=$1`, userID, enc)
	return err
}

func (s *Service) DisableMFA(ctx context.Context, userID uuid.UUID, code string) error {
	var enc string
	_ = s.pg.QueryRow(ctx, `SELECT COALESCE(mfa_secret_enc,'') FROM users WHERE id=$1`, userID).Scan(&enc)
	secret, err := cryptoenc.Decrypt(s.encKey, enc)
	if err != nil || !VerifyTOTP(secret, code) {
		return fmt.Errorf("código MFA inválido")
	}
	_, err = s.pg.Exec(ctx, `UPDATE users SET mfa_enabled=FALSE, mfa_secret_enc=NULL, updated_at=now() WHERE id=$1`, userID)
	return err
}

func (s *Service) RequestPasswordReset(ctx context.Context, email string) (plain string, found bool, err error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var userID uuid.UUID
	err = s.pg.QueryRow(ctx, `SELECT id FROM users WHERE email=$1`, email).Scan(&userID)
	if err == pgx.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	plain, err = cryptoenc.RandomToken(32)
	if err != nil {
		return "", false, err
	}
	_, err = s.pg.Exec(ctx, `INSERT INTO password_resets (user_id, token_hash, expires_at) VALUES ($1,$2,$3)`,
		userID, cryptoenc.HashToken(plain), time.Now().Add(2*time.Hour))
	return plain, true, err
}

func (s *Service) CreateEmailVerification(ctx context.Context, userID uuid.UUID) (string, error) {
	plain, err := cryptoenc.RandomToken(32)
	if err != nil {
		return "", err
	}
	_, _ = s.pg.Exec(ctx, `DELETE FROM email_verification_tokens WHERE user_id=$1 OR expires_at < now()`, userID)
	_, err = s.pg.Exec(ctx, `
		INSERT INTO email_verification_tokens (user_id, token_hash, expires_at)
		VALUES ($1,$2,$3)
	`, userID, cryptoenc.HashToken(plain), time.Now().Add(24*time.Hour))
	return plain, err
}

func (s *Service) VerifyEmail(ctx context.Context, token string) error {
	var userID uuid.UUID
	var expiresAt time.Time
	var usedAt *time.Time
	hash := cryptoenc.HashToken(token)
	err := s.pg.QueryRow(ctx, `
		SELECT user_id, expires_at, used_at
		FROM email_verification_tokens
		WHERE token_hash=$1
	`, hash).Scan(&userID, &expiresAt, &usedAt)
	if err != nil || usedAt != nil || time.Now().After(expiresAt) {
		return fmt.Errorf("ligação de confirmação inválida ou expirada")
	}
	if _, err := s.pg.Exec(ctx, `UPDATE users SET email_verified_at=now(), updated_at=now() WHERE id=$1`, userID); err != nil {
		return err
	}
	_, err = s.pg.Exec(ctx, `UPDATE email_verification_tokens SET used_at=now() WHERE token_hash=$1`, hash)
	return err
}

func (s *Service) ResetPassword(ctx context.Context, token, password string) error {
	if len(password) < 8 {
		return fmt.Errorf("a senha deve ter pelo menos 8 caracteres")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return err
	}
	tokenHash := cryptoenc.HashToken(token)

	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var userID uuid.UUID
	err = tx.QueryRow(ctx, `
		UPDATE password_resets
		SET used_at=now()
		WHERE token_hash=$1 AND used_at IS NULL AND expires_at > now()
		RETURNING user_id
	`, tokenHash).Scan(&userID)
	if err != nil {
		return fmt.Errorf("ligação de recuperação inválida ou expirada")
	}
	if _, err := tx.Exec(ctx, `UPDATE users SET password_hash=$2, updated_at=now() WHERE id=$1`, userID, string(hash)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at=now()
		WHERE user_id=$1 AND revoked_at IS NULL
	`, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) CreateInvite(ctx context.Context, orgID, by uuid.UUID, email, role string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return "", fmt.Errorf("e-mail obrigatório")
	}
	if role == "" {
		role = "analyst"
	}
	if role != "viewer" && role != "analyst" && role != "admin" {
		return "", fmt.Errorf("função de convite inválida")
	}
	plain, err := cryptoenc.RandomToken(24)
	if err != nil {
		return "", err
	}
	_, err = s.pg.Exec(ctx, `
		INSERT INTO organization_invites (org_id, email, role, token_hash, invited_by, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6)
	`, orgID, email, role, cryptoenc.HashToken(plain), by, time.Now().Add(7*24*time.Hour))
	return plain, err
}

// AcceptInvite consumes an organization invite.
// New users: create account, join inviting org, issue tokens scoped to that org.
// Existing users: require proof of identity (password + MFA if enabled, or an
// already-authenticated session as that user) before joining; tokens are only
// issued after successful auth and are scoped to the invite's organization.
func (s *Service) AcceptInvite(ctx context.Context, token, name, password, mfaCode string, sessionUserID uuid.UUID) (Principal, TokenPair, error) {
	tokenHash := cryptoenc.HashToken(token)
	var orgID uuid.UUID
	var email, role string
	var exp time.Time
	var accepted *time.Time
	err := s.pg.QueryRow(ctx, `SELECT org_id, email, role, expires_at, accepted_at FROM organization_invites WHERE token_hash=$1`,
		tokenHash).Scan(&orgID, &email, &role, &exp, &accepted)
	if err != nil || accepted != nil || time.Now().After(exp) {
		return Principal{}, TokenPair{}, fmt.Errorf("convite inválido ou expirado")
	}
	if name == "" {
		name = strings.Split(email, "@")[0]
	}

	var userID uuid.UUID
	var passwordHash, authProvider, mfaSecretEnc string
	var mfaEnabled, active bool
	err = s.pg.QueryRow(ctx, `
		SELECT id, password_hash, COALESCE(auth_provider,'password'), COALESCE(mfa_enabled, FALSE),
		       COALESCE(mfa_secret_enc,''), COALESCE(active, TRUE)
		FROM users WHERE email=$1
	`, email).Scan(&userID, &passwordHash, &authProvider, &mfaEnabled, &mfaSecretEnc, &active)

	switch {
	case err == pgx.ErrNoRows:
		if len(password) < 8 {
			return Principal{}, TokenPair{}, fmt.Errorf("a senha deve ter pelo menos 8 caracteres")
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
		if err != nil {
			return Principal{}, TokenPair{}, err
		}
		tx, err := s.pg.Begin(ctx)
		if err != nil {
			return Principal{}, TokenPair{}, err
		}
		defer tx.Rollback(ctx)

		err = tx.QueryRow(ctx, `INSERT INTO users (email, password_hash, name) VALUES ($1,$2,$3) RETURNING id`,
			email, string(hash), name).Scan(&userID)
		if err != nil {
			return Principal{}, TokenPair{}, err
		}
		if err := consumeInviteTx(ctx, tx, tokenHash, orgID, userID, role); err != nil {
			return Principal{}, TokenPair{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return Principal{}, TokenPair{}, err
		}

	case err != nil:
		return Principal{}, TokenPair{}, err

	default:
		// Existing account: never issue tokens from the invite alone.
		if !active {
			return Principal{}, TokenPair{}, fmt.Errorf("conta desactivada")
		}
		needMFA, proofErr := proveExistingInvitee(sessionUserID, userID, authProvider, password, passwordHash, mfaCode, mfaSecretEnc, mfaEnabled, s.encKey)
		if proofErr != nil {
			return Principal{}, TokenPair{}, proofErr
		}
		if needMFA {
			return Principal{UserID: userID, Email: email, MFAEnabled: true}, TokenPair{TokenType: "mfa_required", ExpiresIn: 300}, nil
		}

		tx, err := s.pg.Begin(ctx)
		if err != nil {
			return Principal{}, TokenPair{}, err
		}
		defer tx.Rollback(ctx)

		if err := consumeInviteTx(ctx, tx, tokenHash, orgID, userID, role); err != nil {
			return Principal{}, TokenPair{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return Principal{}, TokenPair{}, err
		}
	}

	p, err := s.principalForOrg(ctx, userID, orgID)
	if err != nil {
		return Principal{}, TokenPair{}, err
	}
	pair, err := s.issue(ctx, p)
	return p, pair, err
}

// proveExistingInvitee verifies identity before joining an existing user to an org via invite.
// Returns needMFA=true when password is valid but MFA code is still required (invite not consumed).
func proveExistingInvitee(
	sessionUserID, inviteeID uuid.UUID,
	authProvider, password, passwordHash, mfaCode, mfaSecretEnc string,
	mfaEnabled bool,
	encKey []byte,
) (needMFA bool, err error) {
	if sessionUserID != uuid.Nil && sessionUserID == inviteeID {
		return false, nil
	}
	if authProvider != "" && authProvider != "password" {
		return false, fmt.Errorf("esta conta entra via SSO (%s); inicie sessão e aceite o convite autenticado", authProvider)
	}
	if password == "" || bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) != nil {
		return false, fmt.Errorf("credenciais inválidas")
	}
	if !mfaEnabled {
		return false, nil
	}
	if strings.TrimSpace(mfaCode) == "" {
		return true, nil
	}
	secret, decErr := cryptoenc.Decrypt(encKey, mfaSecretEnc)
	if decErr != nil || !VerifyTOTP(secret, mfaCode) {
		return false, fmt.Errorf("código MFA inválido")
	}
	return false, nil
}

func consumeInviteTx(ctx context.Context, tx pgx.Tx, tokenHash string, orgID, userID uuid.UUID, role string) error {
	tag, err := tx.Exec(ctx, `
		UPDATE organization_invites
		SET accepted_at=now()
		WHERE token_hash=$1 AND accepted_at IS NULL AND expires_at > now()
	`, tokenHash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("convite inválido ou expirado")
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO organization_members (org_id, user_id, role)
		VALUES ($1,$2,$3)
		ON CONFLICT (org_id, user_id) DO UPDATE SET role=EXCLUDED.role
	`, orgID, userID, role)
	return err
}
