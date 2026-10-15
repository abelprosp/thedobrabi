package manage

import (
	"time"

	"github.com/thedobra/thedobra/services/api/internal/intelligence"
)

type Discovery struct {
	ID          string         `json:"id"`
	DatasetID   string         `json:"dataset_id,omitempty"`
	Kind        string         `json:"kind"`
	Title       string         `json:"title"`
	Body        string         `json:"body"`
	NextStep    string         `json:"next_step"`
	ImpactScore float64        `json:"impact_score"`
	Evidence    map[string]any `json:"evidence,omitempty"`
	Fingerprint string         `json:"fingerprint"`
	Feedback    string         `json:"feedback"`
	Watching    bool           `json:"watching"`
	CreatedAt   time.Time      `json:"created_at"`
	LastSeenAt  time.Time      `json:"last_seen_at"`
}

type Action struct {
	ID             string         `json:"id"`
	DiscoveryID    string         `json:"discovery_id,omitempty"`
	DatasetID      string         `json:"dataset_id,omitempty"`
	OwnerID        string         `json:"owner_id,omitempty"`
	OwnerName      string         `json:"owner_name"`
	Title          string         `json:"title"`
	Body           string         `json:"body"`
	Measure        string         `json:"measure"`
	ExpectedResult string         `json:"expected_result"`
	Baseline       map[string]any `json:"baseline,omitempty"`
	Status         string         `json:"status"`
	DueAt          string         `json:"due_at,omitempty"`
	ReviewAt       string         `json:"review_at,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DueSoon        bool           `json:"due_soon,omitempty"`
	ReviewDue      bool           `json:"review_due,omitempty"`
	LatestReview   *ActionReview  `json:"latest_review,omitempty"`
}

type ActionReview struct {
	ID                 string    `json:"id"`
	ActionID           string    `json:"action_id"`
	ExecutedNote       string    `json:"executed_note"`
	IndicatorValue     *float64  `json:"indicator_value,omitempty"`
	IndicatorDeltaPct  *float64  `json:"indicator_delta_pct,omitempty"`
	IndicatorNarrative string    `json:"indicator_narrative"`
	EffectNarrative    string    `json:"effect_narrative"`
	CreatedAt          time.Time `json:"created_at"`
}

type CreateActionInput struct {
	DiscoveryID    string `json:"discovery_id"`
	DatasetID      string `json:"dataset_id"`
	OwnerID        string `json:"owner_id"`
	OwnerName      string `json:"owner_name"`
	Title          string `json:"title"`
	Body           string `json:"body"`
	Measure        string `json:"measure"`
	ExpectedResult string `json:"expected_result"`
	DueAt          string `json:"due_at"`
	ReviewAt       string `json:"review_at"`
}

type ReviewActionInput struct {
	ExecutedNote string `json:"executed_note"`
}

type Memory struct {
	ID         string    `json:"id"`
	DatasetID  string    `json:"dataset_id,omitempty"`
	Note       string    `json:"note"`
	ChartKey   string    `json:"chart_key,omitempty"`
	OccurredOn string    `json:"occurred_on"`
	CreatedAt  time.Time `json:"created_at"`
}

type WeeklyReview struct {
	ID           string         `json:"id"`
	Period       string         `json:"period"`
	Agenda       map[string]any `json:"agenda"`
	ShareToken   string         `json:"share_token,omitempty"`
	ShareURL     string         `json:"share_url,omitempty"`
	NextReviewAt string         `json:"next_review_at,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
}

type PastDecision struct {
	Period    string `json:"period"`
	Title     string `json:"title"`
	OwnerName string `json:"owner_name,omitempty"`
	When      string `json:"when"`
}

type Home struct {
	Datasets         int                 `json:"datasets"`
	Brief            *intelligence.Brief `json:"brief"`
	Cadence          string              `json:"cadence"`
	CadenceMode      string              `json:"cadence_mode"`
	SuggestedCadence string              `json:"suggested_cadence"`
	DataUpdatedAt    *time.Time          `json:"data_updated_at,omitempty"`
	WhatChanged      []Discovery         `json:"what_changed"`
	Attention        []Discovery         `json:"attention"`
	ActionNow        *Discovery          `json:"action_now,omitempty"`
	Quiet            bool                `json:"quiet"`
	QuietMessage     string              `json:"quiet_message,omitempty"`
	OpenActions      int                 `json:"open_actions"`
	DueReviews       []Action            `json:"due_reviews"`
	WeekReviewReady  bool                `json:"week_review_ready"`
	Memories         []Memory            `json:"memories"`
	PastDecisions    []PastDecision      `json:"past_decisions"`
	WatchMeasures    []string            `json:"watch_measures"`
}

type GoalPace struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	MetricName    string  `json:"metric_name"`
	DatasetID     string  `json:"dataset_id"`
	TargetValue   float64 `json:"target_value"`
	CurrentValue  float64 `json:"current_value"`
	Remaining     float64 `json:"remaining"`
	Pct           float64 `json:"pct"`
	Period        string  `json:"period"`
	PeriodStart   string  `json:"period_start"`
	PeriodEnd     string  `json:"period_end"`
	RemainingDays int     `json:"remaining_business_days"`
	DailyPace     float64 `json:"daily_pace"`
	Reached       bool    `json:"reached"`
	Currency      bool    `json:"currency"`
	Status        string  `json:"status"`
	Narrative     string  `json:"narrative"`
	Hypothesis    string  `json:"hypothesis,omitempty"`
	OwnerID       string  `json:"owner_id,omitempty"`
}
