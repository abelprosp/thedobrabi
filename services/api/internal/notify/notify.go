package notify

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/smtp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thedobra/thedobra/services/api/internal/config"
)

type Service struct {
	cfg config.Config
	pg  *pgxpool.Pool
	log *slog.Logger
}

func New(cfg config.Config, pg *pgxpool.Pool, log *slog.Logger) *Service {
	return &Service{cfg: cfg, pg: pg, log: log}
}

type Message struct {
	Title   string
	Body    string
	URL     string
	HTML    string
	PDF     []byte
	PDFName string
}

func (s *Service) Deliver(ctx context.Context, alertID uuid.UUID, channels []string, msg Message) {
	for _, ch := range channels {
		ch = strings.TrimSpace(strings.ToLower(ch))
		var err error
		switch {
		case ch == "email" || strings.HasPrefix(ch, "email:"):
			to := strings.TrimPrefix(ch, "email:")
			if to == "email" {
				to = s.cfg.AlertEmail
			}
			err = s.email(to, msg)
		case ch == "slack" || strings.HasPrefix(ch, "slack:") || strings.Contains(ch, "hooks.slack.com"):
			url := strings.TrimPrefix(ch, "slack:")
			if url == "slack" || url == "" {
				url = s.cfg.SlackWebhook
			}
			err = s.httpJSON(url, map[string]any{"text": msg.Title + "\n" + msg.Body})
		case ch == "whatsapp" || strings.HasPrefix(ch, "whatsapp:"):
			to := strings.TrimPrefix(ch, "whatsapp:")
			if to == "whatsapp" {
				to = ""
			}
			err = s.whatsapp(to, msg)
		case strings.HasPrefix(ch, "http://") || strings.HasPrefix(ch, "https://") || ch == "webhook":
			url := ch
			if ch == "webhook" {
				url = s.cfg.AlertWebhook
			}
			err = s.httpJSON(url, map[string]any{"title": msg.Title, "body": msg.Body, "url": msg.URL})
		default:
			err = nil // realtime / in-app
		}
		status := "ok"
		detail := ""
		if err != nil {
			status = "error"
			detail = err.Error()
			if s.log != nil {
				s.log.Warn("alert delivery", "channel", ch, "err", err)
			}
		}
		_, _ = s.pg.Exec(ctx, `INSERT INTO alert_deliveries (alert_id, channel, status, detail) VALUES ($1,$2,$3,$4)`,
			alertID, ch, status, detail)
	}
}

func (s *Service) SendMail(to, subject, body string) error {
	return s.email(to, Message{Title: subject, Body: body})
}

func (s *Service) SendMailFrom(from, to, subject, body string) error {
	return s.emailFrom(from, to, Message{Title: subject, Body: body})
}

func (s *Service) SendMailMessage(from, to string, msg Message) error {
	return s.emailFrom(from, to, msg)
}

func (s *Service) SendWhatsApp(to string, msg Message) error {
	return s.whatsapp(to, msg)
}

func (s *Service) email(to string, msg Message) error {
	if to == "" {
		if s.log != nil {
			s.log.Info("email (sem destinatário / SMTP)", "title", msg.Title, "body", msg.Body)
		}
		return nil
	}
	if s.cfg.SMTPHost == "" {
		if s.log != nil {
			s.log.Info("email (SMTP não configurado)", "to", to, "title", msg.Title, "body", msg.Body)
		}
		return nil
	}
	return s.emailFrom("", to, msg)
}

func (s *Service) emailFrom(from, to string, msg Message) error {
	if to == "" {
		if s.log != nil {
			s.log.Info("email (sem destinatário / SMTP)", "title", msg.Title, "body", msg.Body)
		}
		return nil
	}
	if s.cfg.SMTPHost == "" {
		if s.log != nil {
			s.log.Info("email (SMTP não configurado)", "to", to, "title", msg.Title, "body", msg.Body)
		}
		return nil
	}
	if from == "" {
		from = s.cfg.SMTPFrom
	}
	if from == "" {
		from = "thedobra@" + s.cfg.SMTPHost
	}
	auth := smtp.PlainAuth("", s.cfg.SMTPUser, s.cfg.SMTPPass, s.cfg.SMTPHost)
	addr := s.cfg.SMTPHost
	if !strings.Contains(addr, ":") {
		addr += ":587"
	}
	raw := buildMail(from, to, msg)
	return smtp.SendMail(addr, auth, from, []string{to}, raw)
}

func buildMail(from, to string, msg Message) []byte {
	subject := strings.ReplaceAll(msg.Title, "\n", " ")
	if msg.HTML == "" && len(msg.PDF) == 0 {
		return []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s\n%s\n",
			from, to, subject, msg.Body, msg.URL))
	}
	boundary := "dobra" + fmt.Sprint(time.Now().UnixNano())
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=%s\r\n\r\n", from, to, subject, boundary)
	b.WriteString("--" + boundary + "\r\n")
	if msg.HTML != "" {
		b.WriteString("Content-Type: text/html; charset=utf-8\r\n\r\n")
		b.WriteString(msg.HTML)
		if msg.URL != "" {
			b.WriteString(fmt.Sprintf(`<p><a href="%s">Abrir no TheDobra</a></p>`, msg.URL))
		}
		b.WriteString("\r\n")
	} else {
		b.WriteString("Content-Type: text/plain; charset=utf-8\r\n\r\n")
		b.WriteString(msg.Body + "\n" + msg.URL + "\r\n")
	}
	if len(msg.PDF) > 0 {
		name := msg.PDFName
		if name == "" {
			name = "relatorio.pdf"
		}
		b.WriteString("--" + boundary + "\r\n")
		fmt.Fprintf(&b, "Content-Type: application/pdf\r\nContent-Disposition: attachment; filename=\"%s\"\r\nContent-Transfer-Encoding: base64\r\n\r\n", name)
		b.WriteString(base64.StdEncoding.EncodeToString(msg.PDF))
		b.WriteString("\r\n")
	}
	b.WriteString("--" + boundary + "--\r\n")
	return []byte(b.String())
}

func (s *Service) whatsapp(to string, msg Message) error {
	url := s.cfg.WhatsAppWebhook
	if url == "" {
		if s.log != nil {
			s.log.Info("whatsapp (webhook não configurado)", "to", to, "title", msg.Title)
		}
		return nil
	}
	return s.httpJSON(url, map[string]any{"to": to, "title": msg.Title, "body": msg.Body, "url": msg.URL, "channel": "whatsapp"})
}

func (s *Service) httpJSON(url string, payload any) error {
	if url == "" {
		return fmt.Errorf("URL de webhook vazia")
	}
	b, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook HTTP %d", resp.StatusCode)
	}
	return nil
}
