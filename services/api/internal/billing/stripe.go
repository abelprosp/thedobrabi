package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stripe/stripe-go/v82"
	portalsession "github.com/stripe/stripe-go/v82/billingportal/session"
	checkoutsession "github.com/stripe/stripe-go/v82/checkout/session"
	"github.com/stripe/stripe-go/v82/webhook"
	"github.com/thedobra/thedobra/services/api/internal/config"
	"github.com/thedobra/thedobra/services/api/internal/entitlements"
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
	catalog := entitlements.Catalog()
	plans := make([]map[string]any, 0, len(catalog))
	for _, p := range catalog {
		plans = append(plans, map[string]any{
			"id": p.ID, "name": p.Name, "price_brl": p.PriceBRL,
			"users": p.Users, "datasets": p.Datasets, "queries": p.Queries,
			"ai": p.AI, "dashboards": p.Dashboards, "connectors": p.Connectors,
		})
	}
	return map[string]any{
		"enabled": s.Enabled(),
		"plans":   plans,
	}
}

func (s *Service) Checkout(_ context.Context, orgID uuid.UUID, email, plan string) (string, error) {
	if !s.Enabled() {
		return "", fmt.Errorf("Stripe não está configurado (STRIPE_SECRET_KEY)")
	}
	plan = entitlements.NormalizePlan(plan)
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
	if s.cfg.StripeWebhookSecret == "" {
		return fmt.Errorf("webhook do Stripe não configurado")
	}
	var event stripe.Event
	event, err = webhook.ConstructEventWithOptions(body, r.Header.Get("Stripe-Signature"), s.cfg.StripeWebhookSecret, webhook.ConstructEventOptions{IgnoreAPIVersionMismatch: true})
	if err != nil {
		return err
	}

	ctx := r.Context()
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Claim the event: only the first successful insert applies side effects.
	tag, err := tx.Exec(ctx, `
		INSERT INTO billing_events (stripe_event_id, type, payload)
		VALUES ($1,$2,$3)
		ON CONFLICT (stripe_event_id) DO NOTHING
	`, event.ID, string(event.Type), event.Data.Raw)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		// Already processed — acknowledge without reapplying effects.
		return tx.Commit(ctx)
	}

	if err := s.applyWebhookEvent(ctx, tx, event); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) applyWebhookEvent(ctx context.Context, tx pgx.Tx, event stripe.Event) error {
	switch event.Type {
	case "checkout.session.completed":
		var sess stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
			return fmt.Errorf("checkout.session.completed payload: %w", err)
		}
		orgID, _ := uuid.Parse(sess.ClientReferenceID)
		if orgID == uuid.Nil && sess.Metadata != nil {
			orgID, _ = uuid.Parse(sess.Metadata["org_id"])
		}
		if orgID == uuid.Nil {
			return fmt.Errorf("checkout.session.completed sem org_id")
		}
		plan := entitlements.PlanPro
		if sess.Metadata != nil && sess.Metadata["plan"] != "" {
			plan = entitlements.NormalizePlan(sess.Metadata["plan"])
		}
		cust, sub := "", ""
		if sess.Customer != nil {
			cust = sess.Customer.ID
		}
		if sess.Subscription != nil {
			sub = sess.Subscription.ID
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO stripe_customers (org_id, stripe_customer_id, stripe_sub_id, status, price_id)
			VALUES ($1,$2,$3,'active',$4)
			ON CONFLICT (org_id) DO UPDATE SET
				stripe_customer_id=EXCLUDED.stripe_customer_id,
				stripe_sub_id=EXCLUDED.stripe_sub_id,
				status='active',
				price_id=EXCLUDED.price_id,
				updated_at=now()
		`, orgID, cust, sub, s.priceFor(plan)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE organizations SET plan=$2, updated_at=now() WHERE id=$1`, orgID, plan); err != nil {
			return err
		}
	case "customer.subscription.deleted":
		var sub stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			return fmt.Errorf("customer.subscription.deleted payload: %w", err)
		}
		if _, err := tx.Exec(ctx, `UPDATE stripe_customers SET status='canceled', updated_at=now() WHERE stripe_sub_id=$1`, sub.ID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE organizations SET plan=$2, updated_at=now()
			WHERE id IN (SELECT org_id FROM stripe_customers WHERE stripe_sub_id=$1)
		`, sub.ID, entitlements.PlanEssencial); err != nil {
			return err
		}
	case "customer.subscription.updated", "invoice.paid":
		var raw map[string]any
		if err := json.Unmarshal(event.Data.Raw, &raw); err != nil {
			return fmt.Errorf("%s payload: %w", event.Type, err)
		}
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
			if _, err := tx.Exec(ctx, `UPDATE stripe_customers SET status=$2, updated_at=now() WHERE stripe_sub_id=$1`, subID, status); err != nil {
				return err
			}
		}
		if meta, ok := raw["metadata"].(map[string]any); ok {
			if p, ok := meta["plan"].(string); ok && p != "" {
				plan := entitlements.NormalizePlan(p)
				if _, err := tx.Exec(ctx, `
					UPDATE organizations SET plan=$2, updated_at=now()
					WHERE id IN (SELECT org_id FROM stripe_customers WHERE stripe_sub_id=$1)
				`, subID, plan); err != nil {
					return err
				}
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
	return map[string]any{"plan": entitlements.NormalizePlan(plan), "stripe_customer": cust != "", "subscription_status": status, "enabled": s.Enabled()}
}

func (s *Service) priceFor(plan string) string {
	switch entitlements.NormalizePlan(plan) {
	case entitlements.PlanEssencial:
		return s.cfg.StripePriceStarter
	case entitlements.PlanPro:
		return s.cfg.StripePriceGrowth
	case entitlements.PlanCompleto:
		if s.cfg.StripePriceBusiness != "" {
			return s.cfg.StripePriceBusiness
		}
		return s.cfg.StripePriceEnterprise
	default:
		return s.cfg.StripePriceGrowth
	}
}
