package authn

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/thedobra/thedobra/services/api/internal/cryptoenc"
	"golang.org/x/crypto/bcrypt"
)

func TestProveExistingInviteeRequiresPassword(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), 4)
	if err != nil {
		t.Fatal(err)
	}
	invitee := uuid.New()
	admin := uuid.New()

	// Admin session alone must not authenticate as the invitee.
	needMFA, err := proveExistingInvitee(admin, invitee, "password", "wrong-password", string(hash), "", "", false, nil)
	if err == nil || needMFA {
		t.Fatalf("admin with wrong password should fail, needMFA=%v err=%v", needMFA, err)
	}

	needMFA, err = proveExistingInvitee(uuid.Nil, invitee, "password", "", string(hash), "", "", false, nil)
	if err == nil || !strings.Contains(err.Error(), "credenciais") {
		t.Fatalf("empty password must fail, got needMFA=%v err=%v", needMFA, err)
	}

	needMFA, err = proveExistingInvitee(uuid.Nil, invitee, "password", "correct-password", string(hash), "", "", false, nil)
	if err != nil || needMFA {
		t.Fatalf("correct password should succeed, needMFA=%v err=%v", needMFA, err)
	}
}

func TestProveExistingInviteeRequiresMFA(t *testing.T) {
	encKey := make([]byte, 32)
	secret, err := NewTOTPSecret()
	if err != nil {
		t.Fatal(err)
	}
	enc, err := cryptoenc.Encrypt(encKey, secret)
	if err != nil {
		t.Fatal(err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), 4)
	if err != nil {
		t.Fatal(err)
	}
	invitee := uuid.New()

	needMFA, err := proveExistingInvitee(uuid.Nil, invitee, "password", "correct-password", string(hash), "", enc, true, encKey)
	if err != nil || !needMFA {
		t.Fatalf("expected MFA challenge, needMFA=%v err=%v", needMFA, err)
	}

	needMFA, err = proveExistingInvitee(uuid.Nil, invitee, "password", "correct-password", string(hash), "000000", enc, true, encKey)
	if err == nil || needMFA || !strings.Contains(err.Error(), "MFA") {
		t.Fatalf("invalid MFA should fail, needMFA=%v err=%v", needMFA, err)
	}

	code := TOTPNow(secret)
	needMFA, err = proveExistingInvitee(uuid.Nil, invitee, "password", "correct-password", string(hash), code, enc, true, encKey)
	if err != nil || needMFA {
		t.Fatalf("valid MFA should succeed, needMFA=%v err=%v", needMFA, err)
	}
}

func TestProveExistingInviteeSessionSkipsPassword(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), 4)
	if err != nil {
		t.Fatal(err)
	}
	invitee := uuid.New()

	needMFA, err := proveExistingInvitee(invitee, invitee, "password", "", string(hash), "", "", true, nil)
	if err != nil || needMFA {
		t.Fatalf("authenticated invitee should skip password/MFA, needMFA=%v err=%v", needMFA, err)
	}
}

func TestProveExistingInviteeSSORequiresSession(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("x"), 4)
	if err != nil {
		t.Fatal(err)
	}
	invitee := uuid.New()

	_, err = proveExistingInvitee(uuid.Nil, invitee, "google", "anything", string(hash), "", "", false, nil)
	if err == nil || !strings.Contains(err.Error(), "SSO") {
		t.Fatalf("expected SSO error, got %v", err)
	}

	needMFA, err := proveExistingInvitee(invitee, invitee, "google", "", string(hash), "", "", false, nil)
	if err != nil || needMFA {
		t.Fatalf("SSO user with matching session should succeed, needMFA=%v err=%v", needMFA, err)
	}
}

func TestPrincipalForOrgRejectsNilOrg(t *testing.T) {
	s := &Service{}
	_, err := s.principalForOrg(nil, uuid.New(), uuid.Nil)
	if err == nil {
		t.Fatal("expected error for nil org")
	}
}
