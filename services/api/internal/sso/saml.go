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
	"io"
	"math/big"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

type samlResponse struct {
	XMLName   xml.Name      `xml:"Response"`
	Assertion samlAssertion `xml:"Assertion"`
}

type samlAssertion struct {
	Subject struct {
		NameID string `xml:"NameID"`
	} `xml:"Subject"`
	AttributeStatement struct {
		Attributes []struct {
			Name   string   `xml:"Name,attr"`
			Values []string `xml:"AttributeValue"`
		} `xml:"Attribute"`
	} `xml:"AttributeStatement"`
}

func ParseSAMLResponse(raw string) (email, name, subject string, err error) {
	raw = strings.TrimSpace(raw)
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		decoded, err = base64.URLEncoding.DecodeString(raw)
		if err != nil {
			return "", "", "", fmt.Errorf("SAMLResponse inválido")
		}
	}
	if !bytes.Contains(decoded, []byte("<")) {
		decoded = inflateRaw(decoded)
	}
	if !bytes.Contains(decoded, []byte("Signature")) && !bytes.Contains(decoded, []byte("ds:Signature")) {
		return "", "", "", fmt.Errorf("asserção SAML sem assinatura XML")
	}
	var resp samlResponse
	if err := xml.Unmarshal(decoded, &resp); err != nil {
		return "", "", "", fmt.Errorf("XML SAML inválido: %w", err)
	}
	subject = strings.TrimSpace(resp.Assertion.Subject.NameID)
	for _, a := range resp.Assertion.AttributeStatement.Attributes {
		n := strings.ToLower(a.Name)
		val := ""
		if len(a.Values) > 0 {
			val = a.Values[0]
		}
		if strings.Contains(n, "email") || n == "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress" {
			email = val
		}
		if strings.Contains(n, "displayname") || strings.Contains(n, "name") && email == "" {
			name = val
		}
	}
	if email == "" && strings.Contains(subject, "@") {
		email = subject
	}
	if email == "" {
		return "", "", "", fmt.Errorf("asserção SAML sem e-mail")
	}
	if name == "" {
		name = email
	}
	if subject == "" {
		subject = email
	}
	return email, name, subject, nil
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

func inflateRaw(b []byte) []byte {
	r := flate.NewReader(bytes.NewReader(b))
	defer r.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		return b
	}
	return out
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
