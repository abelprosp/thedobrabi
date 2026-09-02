package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"time"
)

func main() {
	rows := flag.Int("rows", 1000000, "number of rows")
	dataset := flag.String("dataset", "sales", "dataset kind: sales")
	out := flag.String("out", "sales.csv", "output csv path")
	flag.Parse()

	f, err := os.Create(*out)
	if err != nil {
		fatal(err)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()

	switch *dataset {
	case "sales":
		writeSales(w, *rows)
	default:
		fatal(fmt.Errorf("unknown dataset %s", *dataset))
	}
	fmt.Printf("wrote %d rows to %s\n", *rows, *out)
}

func writeSales(w *csv.Writer, n int) {
	_ = w.Write([]string{"order_date", "region", "product", "channel", "seller", "customer", "segment", "quantity", "unit_price", "cost", "revenue", "profit"})
	regions := []string{"North", "South", "East", "West", "Midwest"}
	products := []string{"Atlas CRM", "Pulse Analytics", "Nimbus Cloud", "Forge ERP", "Helix Support", "Orbit Mail"}
	channels := []string{"Direct", "Partner", "Self-serve", "Marketplace"}
	sellers := []string{"Ana Costa", "Bruno Lima", "Carla Souza", "Diego Alves", "Eva Martins"}
	segments := []string{"Enterprise", "Mid-market", "SMB"}
	rng := rand.New(rand.NewSource(1))
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		d := start.AddDate(0, 0, rng.Intn(900))
		qty := 1 + rng.Intn(12)
		price := []float64{49, 99, 149, 299, 799, 1499}[rng.Intn(6)]
		rev := float64(qty) * price
		cost := rev * (0.4 + rng.Float64()*0.2)
		_ = w.Write([]string{
			d.Format("2006-01-02"),
			regions[rng.Intn(len(regions))],
			products[rng.Intn(len(products))],
			channels[rng.Intn(len(channels))],
			sellers[rng.Intn(len(sellers))],
			fmt.Sprintf("C%05d", rng.Intn(50000)+1),
			segments[rng.Intn(len(segments))],
			fmt.Sprintf("%d", qty),
			fmt.Sprintf("%.2f", price),
			fmt.Sprintf("%.2f", cost),
			fmt.Sprintf("%.2f", rev),
			fmt.Sprintf("%.2f", rev-cost),
		})
		if i > 0 && i%1_000_000 == 0 {
			w.Flush()
			fmt.Fprintf(os.Stderr, "… %d\n", i)
		}
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
