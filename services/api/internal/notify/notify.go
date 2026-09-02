package notify

import (
	"bytes"
	"context"
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
	Title string
	Body  string
	URL   string
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
	from := s.cfg.SMTPFrom
	if from == "" {
		from = "thedobra@" + s.cfg.SMTPHost
	}
	auth := smtp.PlainAuth("", s.cfg.SMTPUser, s.cfg.SMTPPass, s.cfg.SMTPHost)
	addr := s.cfg.SMTPHost
	if !strings.Contains(addr, ":") {
		addr += ":587"
	}
	raw := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s\n%s\n",
		from, to, msg.Title, msg.Body, msg.URL))
	return smtp.SendMail(addr, auth, from, []string{to}, raw)
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
