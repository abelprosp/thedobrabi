package sso

import (
	"bytes"
	"compress/flate"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"encoding/xml"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ParseSAMLResponse refuses all assertions until real cryptographic validation
// (IdP certificate, issuer, audience, NotOnOrAfter, and replay protection) exists.
// Checking for the literal word "Signature" is not authentication and must not be used.
func ParseSAMLResponse(raw string) (email, name, subject string, err error) {
	_ = raw
	return "", "", "", fmt.Errorf("validação criptográfica SAML não implementada: respostas não são aceitas até verificação real de assinatura/certificado IdP, issuer, audience, tempo e replay")
}

func SPMetadata(entityID, acs string) string {
	return fmt.Sprintf(`<?xml version="1.0"?>
<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="%s">
  <SPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol" AuthnRequestsSigned="false" WantAssertionsSigned="true">
    <NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress</NameIDFormat>
    <AssertionConsumerService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="%s" index="0" isDefault="true"/>
  </SPSSODescriptor>
</EntityDescriptor>
`, xmlEscape(entityID), xmlEscape(acs))
}

func AuthnRequestRedirect(idpSSO, acs, entityID string) (string, error) {
	id := "_" + uuid.NewString()
	req := fmt.Sprintf(`<?xml version="1.0"?>
<samlp:AuthnRequest xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion" ID="%s" Version="2.0" IssueInstant="%s" AssertionConsumerServiceURL="%s" ProtocolBinding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST">
  <saml:Issuer>%s</saml:Issuer>
  <samlp:NameIDPolicy Format="urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress" AllowCreate="true"/>
</samlp:AuthnRequest>`, id, time.Now().UTC().Format(time.RFC3339), xmlEscape(acs), xmlEscape(entityID))
	deflated, err := deflateB64([]byte(req))
	if err != nil {
		return "", err
	}
	u, err := url.Parse(idpSSO)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("SAMLRequest", deflated)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

func deflateB64(b []byte) (string, error) {
	var buf bytes.Buffer
	w, err := flate.NewWriter(&buf, flate.DefaultCompression)
	if err != nil {
		return "", err
	}
	if _, err := w.Write(b); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func GenerateDevCert() (certPEM, keyPEM []byte, err error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "TheDobra SP"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour * 5),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, err
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	return certPEM, keyPEM, nil
}

func ExtractIDPSSO(metadataXML string) string {
	var md struct {
		IDPSSO struct {
			Services []struct {
				Binding  string `xml:"Binding,attr"`
				Location string `xml:"Location,attr"`
			} `xml:"SingleSignOnService"`
		} `xml:"IDPSSODescriptor"`
	}
	_ = xml.Unmarshal([]byte(metadataXML), &md)
	for _, s := range md.IDPSSO.Services {
		if strings.Contains(s.Binding, "HTTP-Redirect") || s.Location != "" {
			return s.Location
		}
	}
	return ""
}
