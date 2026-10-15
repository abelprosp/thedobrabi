package manage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/thedobra/thedobra/services/api/internal/cryptoenc"
)

type Priority struct {
	Title     string `json:"title"`
	OwnerID   string `json:"owner_id,omitempty"`
	OwnerName string `json:"owner_name,omitempty"`
	ActionID  string `json:"action_id,omitempty"`
}

type SaveReviewInput struct {
	Results      []map[string]any `json:"results"`
	PriorActions []map[string]any `json:"prior_actions"`
	Priorities   []Priority       `json:"priorities"`
	Notes        string           `json:"notes"`
	NextReviewAt string           `json:"next_review_at"`
	Period       string           `json:"period"`
}

func (s *Service) weekReviewReady(ctx context.Context, orgID, wsID uuid.UUID, cadence string) bool {
	if cadence == "daily" {
		var n int
		_ = s.pg.QueryRow(ctx, `SELECT COUNT(*) FROM manage_reviews WHERE org_id=$1 AND workspace_id=$2 AND created_at::date = CURRENT_DATE`, orgID, wsID).Scan(&n)
		return n == 0
	}
	period := isoWeekPeriod(nowBR())
	if cadence == "monthly" {
		t := nowBR()
		period = fmt.Sprintf("%d-%02d", t.Year(), t.Month())
	}
	var n int
	_ = s.pg.QueryRow(ctx, `SELECT COUNT(*) FROM manage_reviews WHERE org_id=$1 AND workspace_id=$2 AND period=$3`, orgID, wsID, period).Scan(&n)
	return n == 0
}

func (s *Service) ListReviews(ctx context.Context, orgID, wsID uuid.UUID) ([]WeeklyReview, error) {
	rows, err := s.pg.Query(ctx, `
		SELECT id, period, agenda_json, share_token, next_review_at::text, created_at
		FROM manage_reviews
		WHERE org_id=$1 AND workspace_id=$2
		ORDER BY created_at DESC LIMIT 24
	`, orgID, wsID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WeeklyReview{}
	for rows.Next() {
		r, err := scanWeekly(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func scanWeekly(row rowScanner) (WeeklyReview, error) {
	var r WeeklyReview
	var agenda []byte
	var next *string
	if err := row.Scan(&r.ID, &r.Period, &agenda, &r.ShareToken, &next, &r.CreatedAt); err != nil {
		return r, err
	}
	_ = json.Unmarshal(agenda, &r.Agenda)
	if next != nil {
		r.NextReviewAt = strings.TrimSpace(*next)
	}
	r.ShareURL = "/share/review/" + r.ShareToken
	return r, nil
}

func (s *Service) GetReview(ctx context.Context, orgID, wsID, id uuid.UUID) (WeeklyReview, error) {
	row := s.pg.QueryRow(ctx, `
		SELECT id, period, agenda_json, share_token, next_review_at::text, created_at
		FROM manage_reviews WHERE id=$1 AND org_id=$2 AND workspace_id=$3
	`, id, orgID, wsID)
	return scanWeekly(row)
}

func (s *Service) PublicReview(ctx context.Context, token string) (map[string]any, error) {
	var orgID, wsID uuid.UUID
	var period string
	var agenda []byte
	var created time.Time
	var next *string
	err := s.pg.QueryRow(ctx, `
		SELECT org_id, workspace_id, period, agenda_json, created_at, next_review_at::text
		FROM manage_reviews WHERE share_token=$1
	`, token).Scan(&orgID, &wsID, &period, &agenda, &created, &next)
	if err != nil {
		return nil, fmt.Errorf("pauta não encontrada")
	}
	var parsed map[string]any
	_ = json.Unmarshal(agenda, &parsed)
	var brand, logo string
	_ = s.pg.QueryRow(ctx, `SELECT COALESCE(brand_name,''), COALESCE(brand_logo_url,'') FROM organizations WHERE id=$1`, orgID).Scan(&brand, &logo)
	out := map[string]any{
		"period": period, "agenda": parsed, "created_at": created,
		"brand_name": brand, "brand_logo_url": logo,
	}
	if next != nil {
		out["next_review_at"] = *next
	}
	return out, nil
}

func (s *Service) SaveReview(ctx context.Context, orgID, wsID, userID uuid.UUID, in SaveReviewInput) (WeeklyReview, error) {
	if len(in.Priorities) > 3 {
		return WeeklyReview{}, fmt.Errorf("escolha no máximo três prioridades")
	}
	period := strings.TrimSpace(in.Period)
	if period == "" {
		period = isoWeekPeriod(nowBR())
	}
	tok, err := cryptoenc.RandomToken(18)
	if err != nil {
		return WeeklyReview{}, err
	}
	var next any
	if t, ok := parseDate(in.NextReviewAt); ok {
		next = dateOnly(t)
	} else {
		next = dateOnly(nowBR().AddDate(0, 0, 7))
	}
	agenda := map[string]any{
		"results":       in.Results,
		"prior_actions": in.PriorActions,
		"priorities":    in.Priorities,
		"notes":         in.Notes,
	}
	raw, _ := json.Marshal(agenda)
	id := uuid.New()
	_, err = s.pg.Exec(ctx, `
		INSERT INTO manage_reviews (id, org_id, workspace_id, created_by, period, agenda_json, share_token, next_review_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (workspace_id, period) DO UPDATE SET
			agenda_json = EXCLUDED.agenda_json,
			next_review_at = EXCLUDED.next_review_at,
			updated_at = now()
		RETURNING id, share_token
	`, id, orgID, wsID, userID, period, raw, tok, next)
	if err != nil {
		return WeeklyReview{}, err
	}
	var storedID uuid.UUID
	var storedTok string
	_ = s.pg.QueryRow(ctx, `SELECT id, share_token FROM manage_reviews WHERE workspace_id=$1 AND period=$2`, wsID, period).Scan(&storedID, &storedTok)

	for _, p := range in.Priorities {
		if strings.TrimSpace(p.Title) == "" || p.ActionID != "" {
			continue
		}
		due := formatDate(nowBR().AddDate(0, 0, 7))
		_, _ = s.CreateAction(ctx, orgID, wsID, userID, "analyst", CreateActionInput{
			Title: p.Title, OwnerID: p.OwnerID, OwnerName: p.OwnerName,
			Body: "Prioridade da revisão " + period, DueAt: due, ReviewAt: due,
		})
	}
	_ = s.notifyUsers(ctx, orgID, wsID, "week_review", "A pauta da revisão está pronta",
		"Resultados, acções e até três prioridades ficaram registados para a equipa.", "/share/review/"+storedTok, 6)
	return s.GetReview(ctx, orgID, wsID, storedID)
}

func (s *Service) PastDecisions(ctx context.Context, orgID, wsID uuid.UUID, query string) ([]PastDecision, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []PastDecision{}, nil
	}
	words := strings.Fields(query)
	if len(words) > 4 {
		words = words[:4]
	}
	like := "%" + strings.Join(words, "%") + "%"
	rows, err := s.pg.Query(ctx, `
		SELECT period, agenda_json, created_at
		FROM manage_reviews
		WHERE org_id=$1 AND workspace_id=$2 AND agenda_json::text ILIKE $3
		ORDER BY created_at DESC LIMIT 5
	`, orgID, wsID, like)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PastDecision{}
	for rows.Next() {
		var period string
		var raw []byte
		var created time.Time
		if err := rows.Scan(&period, &raw, &created); err != nil {
			return nil, err
		}
		var agenda map[string]any
		_ = json.Unmarshal(raw, &agenda)
		title := "Revisão " + period
		owner := ""
		if arr, ok := agenda["priorities"].([]any); ok && len(arr) > 0 {
			if m, ok := arr[0].(map[string]any); ok {
				if t, _ := m["title"].(string); t != "" {
					title = t
				}
				if o, _ := m["owner_name"].(string); o != "" {
					owner = o
				}
			}
		}
		out = append(out, PastDecision{Period: period, Title: title, OwnerName: owner, When: created.Format("02/01/2006")})
	}
	return out, rows.Err()
}

func (s *Service) notifyUsers(ctx context.Context, orgID, wsID uuid.UUID, kind, title, body, url string, cooldownDays int) error {
	if cooldownDays < 1 {
		cooldownDays = 1
	}
	_, err := s.pg.Exec(ctx, `
		INSERT INTO notifications (org_id, workspace_id, user_id, kind, title, body, url)
		SELECT $1, $2, om.user_id, $3, $4, $5, $6
		FROM organization_members om
		WHERE om.org_id=$1
		  AND NOT EXISTS (
			SELECT 1 FROM notifications n
			WHERE n.user_id = om.user_id AND n.org_id=$1 AND n.workspace_id=$2
			  AND n.kind=$3 AND n.created_at > now() - ($7::int * interval '1 day')
		  )
	`, orgID, wsID, kind, title, body, url, cooldownDays)
	return err
}

func (s *Service) maybeNotify(ctx context.Context, orgID, wsID uuid.UUID, home Home) error {
	if home.WeekReviewReady {
		_ = s.notifyUsers(ctx, orgID, wsID, "week_review_ready", "A sua revisão semanal está pronta",
			"Abra o resumo com os pontos relevantes e decida o que fazer.", "/review", 6)
	}
	if len(home.DueReviews) > 0 {
		_ = s.notifyUsers(ctx, orgID, wsID, "action_review_due", "Chegou o momento de conferir o resultado da sua acção",
			home.DueReviews[0].Title, "/actions", 2)
	}
	return nil
}
