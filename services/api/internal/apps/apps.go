package apps

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotInWorkspace is returned when a dashboard/report UUID does not belong
// to the caller's organization and workspace (IDOR guard).
var ErrNotInWorkspace = errors.New("resource does not belong to organization/workspace")

// RejectMissingIDs fails if any requested ID is absent from found.
// Used by SetDashboards/SetReports after a tenant-scoped SELECT.
func RejectMissingIDs(requested []uuid.UUID, found map[uuid.UUID]struct{}) error {
	for _, id := range requested {
		if _, ok := found[id]; !ok {
			return fmt.Errorf("%w: %s", ErrNotInWorkspace, id)
		}
	}
	return nil
}

// App is a packaged workspace app that groups dashboards and reports for read-only viewers.
type App struct {
	ID          uuid.UUID `json:"id"`
	OrgID       uuid.UUID `json:"org_id"`
	WorkspaceID uuid.UUID `json:"workspace_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Icon        string    `json:"icon,omitempty"`
	Status      string    `json:"status"`
	Theme       string    `json:"theme,omitempty"`
	CoverURL    string    `json:"cover_url,omitempty"`
	PublicToken *string   `json:"public_token,omitempty"`
	Permissions *Permissions `json:"permissions,omitempty"`
	CreatedBy   *uuid.UUID `json:"created_by,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Permissions controls which roles can access the published app.
type Permissions struct {
	Viewer  bool `json:"viewer"`
	Analyst bool `json:"analyst"`
}

type DashboardRef struct {
	ID      uuid.UUID `json:"id"`
	Name    string    `json:"name"`
	Order   int       `json:"order"`
	Section string    `json:"section"`
}

type ReportRef struct {
	ID      uuid.UUID `json:"id"`
	Name    string    `json:"name"`
	Order   int       `json:"order"`
	Section string    `json:"section"`
}

// Store manages workspace packaged apps.
type Store struct{ pg *pgxpool.Pool }

func NewStore(pg *pgxpool.Pool) *Store { return &Store{pg: pg} }

func (s *Store) List(ctx context.Context, orgID, wsID uuid.UUID) ([]App, error) {
	rows, err := s.pg.Query(ctx, `
		SELECT id, org_id, workspace_id, name, description, icon, status, public_token, theme, cover_url, permissions_json, created_by, created_at, updated_at
		FROM apps WHERE org_id=$1 AND workspace_id=$2 ORDER BY updated_at DESC
	`, orgID, wsID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []App
	for rows.Next() {
		var a App
		var perms []byte
		if err := rows.Scan(&a.ID, &a.OrgID, &a.WorkspaceID, &a.Name, &a.Description, &a.Icon, &a.Status, &a.PublicToken, &a.Theme, &a.CoverURL, &perms, &a.CreatedBy, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		a.Permissions = parsePermissions(perms)
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) Get(ctx context.Context, orgID, wsID, id uuid.UUID) (App, error) {
	var a App
	var perms []byte
	err := s.pg.QueryRow(ctx, `
		SELECT id, org_id, workspace_id, name, description, icon, status, public_token, theme, cover_url, permissions_json, created_by, created_at, updated_at
		FROM apps WHERE id=$1 AND org_id=$2 AND workspace_id=$3
	`, id, orgID, wsID).Scan(&a.ID, &a.OrgID, &a.WorkspaceID, &a.Name, &a.Description, &a.Icon, &a.Status, &a.PublicToken, &a.Theme, &a.CoverURL, &perms, &a.CreatedBy, &a.CreatedAt, &a.UpdatedAt)
	a.Permissions = parsePermissions(perms)
	return a, err
}

func (s *Store) Create(ctx context.Context, a App) (uuid.UUID, error) {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	perms := []byte(`{"viewer": true, "analyst": true}`)
	if a.Permissions != nil {
		perms, _ = json.Marshal(a.Permissions)
	}
	_, err := s.pg.Exec(ctx, `
		INSERT INTO apps (id, org_id, workspace_id, name, description, icon, status, theme, cover_url, permissions_json, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
	`, a.ID, a.OrgID, a.WorkspaceID, a.Name, a.Description, a.Icon, a.Status, a.Theme, a.CoverURL, perms, a.CreatedBy)
	return a.ID, err
}

func (s *Store) Update(ctx context.Context, orgID, wsID, id uuid.UUID, a App) error {
	perms := []byte(`{"viewer": true, "analyst": true}`)
	if a.Permissions != nil {
		perms, _ = json.Marshal(a.Permissions)
	}
	ct, err := s.pg.Exec(ctx, `
		UPDATE apps SET name=$1, description=$2, icon=$3, status=$4, theme=$5, cover_url=$6, permissions_json=$7, updated_at=now()
		WHERE id=$8 AND org_id=$9 AND workspace_id=$10
	`, a.Name, a.Description, a.Icon, a.Status, a.Theme, a.CoverURL, perms, id, orgID, wsID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("not found")
	}
	return nil
}

func (s *Store) Delete(ctx context.Context, orgID, wsID, id uuid.UUID) error {
	_, err := s.pg.Exec(ctx, `DELETE FROM apps WHERE id=$1 AND org_id=$2 AND workspace_id=$3`, id, orgID, wsID)
	return err
}

func (s *Store) SetPublicToken(ctx context.Context, orgID, wsID, id uuid.UUID, token string) error {
	ct, err := s.pg.Exec(ctx, `
		UPDATE apps SET public_token=$1, status='published', updated_at=now()
		WHERE id=$2 AND org_id=$3 AND workspace_id=$4
	`, token, id, orgID, wsID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("not found")
	}
	return nil
}

func (s *Store) GetByPublicToken(ctx context.Context, token string) (App, error) {
	var a App
	var perms []byte
	err := s.pg.QueryRow(ctx, `
		SELECT id, org_id, workspace_id, name, description, icon, status, public_token, theme, cover_url, permissions_json, created_by, created_at, updated_at
		FROM apps WHERE public_token=$1 AND status='published'
	`, token).Scan(&a.ID, &a.OrgID, &a.WorkspaceID, &a.Name, &a.Description, &a.Icon, &a.Status, &a.PublicToken, &a.Theme, &a.CoverURL, &perms, &a.CreatedBy, &a.CreatedAt, &a.UpdatedAt)
	a.Permissions = parsePermissions(perms)
	return a, err
}

func (s *Store) assertDashboardsInWorkspace(ctx context.Context, orgID, wsID uuid.UUID, dashboards []DashboardRef) error {
	if len(dashboards) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(dashboards))
	found := make(map[uuid.UUID]struct{}, len(dashboards))
	for i, d := range dashboards {
		ids[i] = d.ID
		var exists bool
		if err := s.pg.QueryRow(ctx, `
			SELECT EXISTS(SELECT 1 FROM dashboards WHERE id=$1 AND org_id=$2 AND workspace_id=$3)
		`, d.ID, orgID, wsID).Scan(&exists); err != nil {
			return err
		}
		if exists {
			found[d.ID] = struct{}{}
		}
	}
	return RejectMissingIDs(ids, found)
}

func (s *Store) assertReportsInWorkspace(ctx context.Context, orgID, wsID uuid.UUID, reports []ReportRef) error {
	if len(reports) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(reports))
	found := make(map[uuid.UUID]struct{}, len(reports))
	for i, r := range reports {
		ids[i] = r.ID
		var exists bool
		if err := s.pg.QueryRow(ctx, `
			SELECT EXISTS(SELECT 1 FROM reports WHERE id=$1 AND org_id=$2 AND workspace_id=$3)
		`, r.ID, orgID, wsID).Scan(&exists); err != nil {
			return err
		}
		if exists {
			found[r.ID] = struct{}{}
		}
	}
	return RejectMissingIDs(ids, found)
}

// SetDashboards replaces app dashboard refs after verifying each ID belongs to org+ws.
func (s *Store) SetDashboards(ctx context.Context, orgID, wsID, appID uuid.UUID, dashboards []DashboardRef) error {
	if err := s.assertDashboardsInWorkspace(ctx, orgID, wsID, dashboards); err != nil {
		return err
	}
	_, err := s.pg.Exec(ctx, `DELETE FROM app_dashboards WHERE app_id=$1`, appID)
	if err != nil {
		return err
	}
	for i, d := range dashboards {
		_, err := s.pg.Exec(ctx, `
			INSERT INTO app_dashboards (app_id, dashboard_id, dashboard_order, section)
			SELECT $1, d.id, $3, $4
			FROM dashboards d
			WHERE d.id=$2 AND d.org_id=$5 AND d.workspace_id=$6
			ON CONFLICT (app_id, dashboard_id) DO UPDATE SET dashboard_order=$3, section=$4
		`, appID, d.ID, i, d.Section, orgID, wsID)
		if err != nil {
			return err
		}
	}
	return nil
}

// Dashboards returns only dashboards that match the app's tenant (org+ws).
func (s *Store) Dashboards(ctx context.Context, orgID, wsID, appID uuid.UUID) ([]DashboardRef, error) {
	rows, err := s.pg.Query(ctx, `
		SELECT d.id, d.name, ad.dashboard_order, ad.section
		FROM app_dashboards ad
		JOIN dashboards d ON d.id = ad.dashboard_id AND d.org_id=$2 AND d.workspace_id=$3
		WHERE ad.app_id=$1
		ORDER BY ad.dashboard_order
	`, appID, orgID, wsID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DashboardRef
	for rows.Next() {
		var r DashboardRef
		if err := rows.Scan(&r.ID, &r.Name, &r.Order, &r.Section); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// SetReports replaces app report refs after verifying each ID belongs to org+ws.
func (s *Store) SetReports(ctx context.Context, orgID, wsID, appID uuid.UUID, reports []ReportRef) error {
	if err := s.assertReportsInWorkspace(ctx, orgID, wsID, reports); err != nil {
		return err
	}
	_, err := s.pg.Exec(ctx, `DELETE FROM app_reports WHERE app_id=$1`, appID)
	if err != nil {
		return err
	}
	for i, r := range reports {
		_, err := s.pg.Exec(ctx, `
			INSERT INTO app_reports (app_id, report_id, report_order, section)
			SELECT $1, r.id, $3, $4
			FROM reports r
			WHERE r.id=$2 AND r.org_id=$5 AND r.workspace_id=$6
			ON CONFLICT (app_id, report_id) DO UPDATE SET report_order=$3, section=$4
		`, appID, r.ID, i, r.Section, orgID, wsID)
		if err != nil {
			return err
		}
	}
	return nil
}

// Reports returns only reports that match the app's tenant (org+ws).
func (s *Store) Reports(ctx context.Context, orgID, wsID, appID uuid.UUID) ([]ReportRef, error) {
	rows, err := s.pg.Query(ctx, `
		SELECT r.id, r.name, ar.report_order, ar.section
		FROM app_reports ar
		JOIN reports r ON r.id = ar.report_id AND r.org_id=$2 AND r.workspace_id=$3
		WHERE ar.app_id=$1
		ORDER BY ar.report_order
	`, appID, orgID, wsID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ReportRef
	for rows.Next() {
		var r ReportRef
		if err := rows.Scan(&r.ID, &r.Name, &r.Order, &r.Section); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func parsePermissions(b []byte) *Permissions {
	if len(b) == 0 {
		return &Permissions{Viewer: true, Analyst: true}
	}
	var p Permissions
	if err := json.Unmarshal(b, &p); err != nil {
		return &Permissions{Viewer: true, Analyst: true}
	}
	return &p
}
