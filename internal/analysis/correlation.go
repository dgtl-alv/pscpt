package analysis

import (
	"database/sql"
	"encoding/json"
	"math"
	"sort"
	"strconv"
)

type Result struct {
	Feature      string  `json:"feature"`
	Target       string  `json:"target"`
	Method       string  `json:"method"`
	Correlation  float64 `json:"correlation"`
	SampleSize   int     `json:"sample_size"`
	Direction    string  `json:"direction"`
	Strength     string  `json:"strength"`
	MissingPairs int     `json:"missing_pairs"`
}

func Run(db *sql.DB, snapshotID int64, target string, features []string, method string) ([]Result, error) {
	if method == "" {
		method = "pearson"
	}
	rows, err := db.Query(`SELECT metrics_json FROM ps_month_rows WHERE snapshot_id=?`, snapshotID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	pairs := map[string][2][]float64{}
	missing := map[string]int{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		metrics := map[string]interface{}{}
		_ = json.Unmarshal(raw, &metrics)
		y, ok := number(metrics[target])
		if !ok {
			for _, f := range features {
				missing[f]++
			}
			continue
		}
		for _, f := range features {
			x, ok := number(metrics[f])
			if !ok {
				missing[f]++
				continue
			}
			p := pairs[f]
			p[0] = append(p[0], x)
			p[1] = append(p[1], y)
			pairs[f] = p
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]Result, 0, len(features))
	for _, f := range features {
		p := pairs[f]
		corr := pearson(p[0], p[1])
		if method == "spearman" {
			corr = pearson(rank(p[0]), rank(p[1]))
		}
		out = append(out, Result{Feature: f, Target: target, Method: method, Correlation: round(corr), SampleSize: len(p[0]), Direction: direction(corr), Strength: strength(corr), MissingPairs: missing[f]})
	}
	sort.Slice(out, func(i, j int) bool { return math.Abs(out[i].Correlation) > math.Abs(out[j].Correlation) })
	return out, nil
}

func number(v interface{}) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int:
		return float64(x), true
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	case string:
		if x == "" {
			return 0, false
		}
		f, err := strconv.ParseFloat(x, 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func pearson(x, y []float64) float64 {
	if len(x) != len(y) || len(x) < 3 {
		return 0
	}
	var sx, sy, sxx, syy, sxy float64
	for i := range x {
		sx += x[i]
		sy += y[i]
		sxx += x[i] * x[i]
		syy += y[i] * y[i]
		sxy += x[i] * y[i]
	}
	n := float64(len(x))
	num := n*sxy - sx*sy
	den := math.Sqrt((n*sxx - sx*sx) * (n*syy - sy*sy))
	if den == 0 {
		return 0
	}
	return num / den
}

func rank(values []float64) []float64 {
	type pair struct {
		idx int
		val float64
	}
	pairs := make([]pair, len(values))
	for i, v := range values {
		pairs[i] = pair{i, v}
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].val < pairs[j].val })
	ranks := make([]float64, len(values))
	for i := 0; i < len(pairs); {
		j := i + 1
		for j < len(pairs) && pairs[j].val == pairs[i].val {
			j++
		}
		r := (float64(i+1) + float64(j)) / 2
		for k := i; k < j; k++ {
			ranks[pairs[k].idx] = r
		}
		i = j
	}
	return ranks
}

func direction(c float64) string {
	if c > 0 {
		return "positive"
	}
	if c < 0 {
		return "negative"
	}
	return "flat"
}

func strength(c float64) string {
	a := math.Abs(c)
	switch {
	case a >= .7:
		return "strong"
	case a >= .4:
		return "moderate"
	case a >= .2:
		return "weak"
	default:
		return "very weak"
	}
}

func round(v float64) float64 { return math.Round(v*10000) / 10000 }
