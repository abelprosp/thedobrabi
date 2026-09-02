package ingest

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
)

func (e *Engine) LoadDemo(ctx context.Context, orgID, wsID, userID uuid.UUID, rows int) (Result, error) {
	if rows <= 0 {
		rows = 25000
	}
	if rows > 200000 {
		rows = 200000
	}
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"order_date", "region", "product", "channel", "seller", "customer", "segment", "quantity", "unit_price", "cost", "revenue", "profit"})
	regions := []string{"North", "South", "East", "West", "Midwest"}
	products := []string{"Atlas CRM", "Pulse Analytics", "Nimbus Cloud", "Forge ERP", "Helix Support", "Orbit Mail"}
	channels := []string{"Direct", "Partner", "Self-serve", "Marketplace"}
	sellers := []string{"Ana Costa", "Bruno Lima", "Carla Souza", "Diego Alves", "Eva Martins"}
	segments := []string{"Enterprise", "Mid-market", "SMB"}
	rng := rand.New(rand.NewSource(42))
	start := time.Now().UTC().AddDate(0, 0, -400)
	for i := 0; i < rows; i++ {
		d := start.AddDate(0, 0, rng.Intn(400))
		mult := 1.0
		if d.Month() == time.Now().UTC().Month() {
			mult = 0.82
		}
		if int(d.Month()) == int(time.Now().UTC().Month())-1 || (time.Now().UTC().Month() == 1 && d.Month() == 12) {
			mult = 0.92
		}
		product := products[rng.Intn(len(products))]
		if product == "Forge ERP" && d.After(time.Now().UTC().AddDate(0, -3, 0)) {
			mult *= 0.79
		}
		region := regions[rng.Intn(len(regions))]
		if region == "South" && d.Month() >= 8 {
			mult *= 0.82
		}
		qty := 1 + rng.Intn(12)
		price := []float64{49, 99, 149, 299, 799, 1499}[rng.Intn(6)]
		if product == "Atlas CRM" {
			price = 299
		}
		rev := float64(qty) * price * mult
		cost := rev * (0.42 + rng.Float64()*0.18)
		profit := rev - cost
		cust := fmt.Sprintf("C%04d", rng.Intn(1800)+1)
		_ = w.Write([]string{
			d.Format("2006-01-02"),
			region, product,
			channels[rng.Intn(len(channels))],
			sellers[rng.Intn(len(sellers))],
			cust,
			segments[rng.Intn(len(segments))],
			fmt.Sprintf("%d", qty),
			fmt.Sprintf("%.2f", price),
			fmt.Sprintf("%.2f", cost),
			fmt.Sprintf("%.2f", rev),
			fmt.Sprintf("%.2f", profit),
		})
	}
	w.Flush()
	return e.IngestFile(ctx, orgID, wsID, userID, "Sales demo", "sales_demo.csv", bytes.NewReader(buf.Bytes()))
}
