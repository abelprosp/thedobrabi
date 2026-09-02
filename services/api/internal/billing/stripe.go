package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stripe/stripe-go/v82"
	portalsession "github.com/stripe/stripe-go/v82/billingportal/session"
	checkoutsession "github.com/stripe/stripe-go/v82/checkout/session"
	"github.com/stripe/stripe-go/v82/webhook"
	"github.com/thedobra/thedobra/services/api/internal/config"
)

type Service struct {
	cfg config.Config
	pg  *pgxpool.Pool
}

func New(cfg config.Config, pg *pgxpool.Pool) *Service {
	if cfg.StripeSecret != "" {
		stripe.Key = cfg.StripeSecret
	}
	return &Service{cfg: cfg, pg: pg}
}

func (s *Service) Enabled() bool { return s.cfg.StripeSecret != "" }

func (s *Service) PublicConfig() map[string]any {
	return map[string]any{
		"enabled": s.Enabled(),
		"plans": []map[string]any{
			{"id": "starter", "name": "Starter", "users": 5, "datasets": 10, "queries": 10000, "ai": 500},
			{"id": "growth", "name": "Growth", "users": 20, "datasets": 50, "queries": 100000, "ai": 5000},
			{"id": "business", "name": "Business", "users": 100, "datasets": 200, "queries": 1000000, "ai": 25000},
			{"id": "enterprise", "name": "Enterprise", "users": -1, "datasets": -1, "queries": -1, "ai": -1},
		},
	}
}

func (s *Service) Checkout(_ context.Context, orgID uuid.UUID, email, plan string) (string, error) {
	if !s.Enabled() {
		return "", fmt.Errorf("Stripe não está configurado (STRIPE_SECRET_KEY)")
	}
	price := s.priceFor(plan)
	if price == "" {
		return "", fmt.Errorf("este plano ainda não tem price ID no Stripe")
	}
	params := &stripe.CheckoutSessionParams{
		Mode:              stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		SuccessURL:        stripe.String(s.cfg.WebOrigin + "/billing?sucesso=1"),
		CancelURL:         stripe.String(s.cfg.WebOrigin + "/billing?cancelado=1"),
		CustomerEmail:     stripe.String(email),
		ClientReferenceID: stripe.String(orgID.String()),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{Price: stripe.String(price), Quantity: stripe.Int64(1)},
		},
		Metadata: map[string]string{"org_id": orgID.String(), "plan": plan},
	}
	sess, err := checkoutsession.New(params)
	if err != nil {
		return "", err
	}
	return sess.URL, nil
}

func (s *Service) Portal(_ context.Context, orgID uuid.UUID) (string, error) {
	if !s.Enabled() {
		return "", fmt.Errorf("Stripe não está configurado")
	}
	var cust string
	err := s.pg.QueryRow(context.Background(), `SELECT stripe_customer_id FROM stripe_customers WHERE org_id=$1`, orgID).Scan(&cust)
	if err != nil {
		return "", fmt.Errorf("nenhuma assinatura Stripe nesta organização")
	}
	sess, err := portalsession.New(&stripe.BillingPortalSessionParams{
		Customer:  stripe.String(cust),
		ReturnURL: stripe.String(s.cfg.WebOrigin + "/billing"),
	})
	if err != nil {
		return "", err
	}
	return sess.URL, nil
}

func (s *Service) HandleWebhook(r *http.Request) error {
	body, err := io.ReadAll(io.LimitReader(r.Body, 65536))
	if err != nil {
		return err
	}
	var event stripe.Event
	if s.cfg.StripeWebhookSecret != "" {
		event, err = webhook.ConstructEventWithOptions(body, r.Header.Get("Stripe-Signature"), s.cfg.StripeWebhookSecret, webhook.ConstructEventOptions{IgnoreAPIVersionMismatch: true})
		if err != nil {
			return err
		}
	} else if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("evento inválido")
	}
	ctx := r.Context()
	_, _ = s.pg.Exec(ctx, `INSERT INTO billing_events (stripe_event_id, type, payload) VALUES ($1,$2,$3) ON CONFLICT (stripe_event_id) DO NOTHING`,
		event.ID, string(event.Type), event.Data.Raw)

	switch event.Type {
	case "checkout.session.completed":
		var sess stripe.CheckoutSession
		if json.Unmarshal(event.Data.Raw, &sess) != nil {
			return nil
		}
		orgID, _ := uuid.Parse(sess.ClientReferenceID)
		if orgID == uuid.Nil && sess.Metadata != nil {
			orgID, _ = uuid.Parse(sess.Metadata["org_id"])
		}
		plan := "growth"
		if sess.Metadata != nil && sess.Metadata["plan"] != "" {
			plan = sess.Metadata["plan"]
		}
		cust, sub := "", ""
		if sess.Customer != nil {
			cust = sess.Customer.ID
		}
		if sess.Subscription != nil {
			sub = sess.Subscription.ID
		}
		_, _ = s.pg.Exec(ctx, `
			INSERT INTO stripe_customers (org_id, stripe_customer_id, stripe_sub_id, status, price_id)
			VALUES ($1,$2,$3,'active',$4)
			ON CONFLICT (org_id) DO UPDATE SET stripe_customer_id=EXCLUDED.stripe_customer_id, stripe_sub_id=EXCLUDED.stripe_sub_id, status='active', updated_at=now()
		`, orgID, cust, sub, s.priceFor(plan))
		_, _ = s.pg.Exec(ctx, `UPDATE organizations SET plan=$2, updated_at=now() WHERE id=$1`, orgID, plan)
	case "customer.subscription.deleted":
		var sub stripe.Subscription
		if json.Unmarshal(event.Data.Raw, &sub) != nil {
			return nil
		}
		_, _ = s.pg.Exec(ctx, `UPDATE stripe_customers SET status='canceled', updated_at=now() WHERE stripe_sub_id=$1`, sub.ID)
		_, _ = s.pg.Exec(ctx, `UPDATE organizations SET plan='starter' WHERE id IN (SELECT org_id FROM stripe_customers WHERE stripe_sub_id=$1)`, sub.ID)
	case "customer.subscription.updated", "invoice.paid":
		var raw map[string]any
		_ = json.Unmarshal(event.Data.Raw, &raw)
		subID, _ := raw["id"].(string)
		if event.Type == "invoice.paid" {
			if sub, ok := raw["subscription"].(string); ok {
				subID = sub
			}
		}
		status, _ := raw["status"].(string)
		if status == "" {
			status = "active"
		}
		if subID != "" {
			_, _ = s.pg.Exec(ctx, `UPDATE stripe_customers SET status=$2, updated_at=now() WHERE stripe_sub_id=$1`, subID, status)
		}
		if meta, ok := raw["metadata"].(map[string]any); ok {
			if p, ok := meta["plan"].(string); ok && p != "" {
				_, _ = s.pg.Exec(ctx, `UPDATE organizations SET plan=$2, updated_at=now() WHERE id IN (SELECT org_id FROM stripe_customers WHERE stripe_sub_id=$1)`, subID, p)
			}
		}
	}
	return nil
}

func (s *Service) Status(ctx context.Context, orgID uuid.UUID) map[string]any {
	var plan string
	_ = s.pg.QueryRow(ctx, `SELECT plan FROM organizations WHERE id=$1`, orgID).Scan(&plan)
	var cust, status string
	_ = s.pg.QueryRow(ctx, `SELECT COALESCE(stripe_customer_id,''), COALESCE(status,'') FROM stripe_customers WHERE org_id=$1`, orgID).Scan(&cust, &status)
	return map[string]any{"plan": plan, "stripe_customer": cust != "", "subscription_status": status, "enabled": s.Enabled()}
}

func (s *Service) priceFor(plan string) string {
	switch plan {
	case "starter":
		return s.cfg.StripePriceStarter
	case "growth":
		return s.cfg.StripePriceGrowth
	case "business":
		return s.cfg.StripePriceBusiness
	case "enterprise":
		return s.cfg.StripePriceEnterprise
	default:
		return s.cfg.StripePriceGrowth
	}
}
