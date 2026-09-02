package mljobs

import (
	"context"
	"encoding/json"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type Result struct {
	Actual       []float64 `json:"actual"`
	Forecast     []float64 `json:"forecast"`
	Low          []float64 `json:"low"`
	High         []float64 `json:"high"`
	Anomalies    []any     `json:"anomalies,omitempty"`
	Assumptions  []string  `json:"assumptions"`
	Error        string    `json:"error,omitempty"`
	FromWorker   bool      `json:"from_worker"`
}

func Forecast(ctx context.Context, rdb *redis.Client, series []float64, horizon int) Result {
	if horizon <= 0 {
		horizon = 12
	}
	if rdb != nil {
		id := uuid.NewString()
		key := "thedobra:ml:result:" + id
		job, _ := json.Marshal(map[string]any{"id": id, "kind": "forecast", "series": series, "horizon": horizon, "result_key": key})
		_ = rdb.LPush(ctx, "thedobra:ml:jobs", job).Err()
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			raw, err := rdb.Get(ctx, key).Bytes()
			if err == nil && len(raw) > 0 {
				var out Result
				if json.Unmarshal(raw, &out) == nil {
					out.FromWorker = true
					if len(out.Assumptions) == 0 {
						out.Assumptions = assumptions()
					}
					return out
				}
			}
			time.Sleep(80 * time.Millisecond)
		}
	}
	return localForecast(series, horizon)
}

func localForecast(series []float64, horizon int) Result {
	if len(series) < 3 {
		return Result{Error: "histórico insuficiente para prever", Assumptions: assumptions()}
	}
	n := float64(len(series))
	var sx, sy, sxx, sxy float64
	for i, y := range series {
		x := float64(i)
		sx += x
		sy += y
		sxx += x * x
		sxy += x * y
	}
	den := n*sxx - sx*sx
	if den == 0 {
		den = 1
	}
	slope := (n*sxy - sx*sy) / den
	intercept := (sy - slope*sx) / n
	var ss float64
	for i, y := range series {
		p := intercept + slope*float64(i)
		d := y - p
		ss += d * d
	}
	sigma := math.Sqrt(ss / n)
	pred := make([]float64, horizon)
	lo := make([]float64, horizon)
	hi := make([]float64, horizon)
	for i := 0; i < horizon; i++ {
		p := intercept + slope*(n+float64(i))
		pred[i] = p
		lo[i] = p - 1.96*sigma
		hi[i] = p + 1.96*sigma
	}
	return Result{
		Actual: series, Forecast: pred, Low: lo, High: hi,
		Assumptions: assumptions(), FromWorker: false,
	}
}

func assumptions() []string {
	return []string{
		"Tendência linear ajustada por mínimos quadrados",
		"Intervalo de 95% assume resíduos homocedásticos",
		"Não é uma projecção inventada pela IA generativa",
	}
}
