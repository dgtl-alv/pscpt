package psmonth

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Snapshot struct {
	ID        int64     `json:"id"`
	Period    string    `json:"period"`
	Status    string    `json:"status"`
	RowCount  int       `json:"row_count"`
	CreatedAt time.Time `json:"created_at"`
}

type Row struct {
	ID         int64                  `json:"id"`
	SnapshotID int64                  `json:"snapshot_id"`
	Period     string                 `json:"period"`
	PSEmployee string                 `json:"ps_employee_id"`
	PSName     string                 `json:"ps_name"`
	PSEmail    string                 `json:"ps_email"`
	AEC        string                 `json:"aec"`
	Region     string                 `json:"region"`
	Status     string                 `json:"status"`
	Factor     map[string]interface{} `json:"individual_factor,omitempty"`
	Capability map[string]interface{} `json:"individual_capability,omitempty"`
	Sales      map[string]interface{} `json:"sales_performance,omitempty"`
	Metrics    map[string]interface{} `json:"metrics"`
}

type BuildSummary struct {
	SnapshotID int64          `json:"snapshot_id"`
	Period     string         `json:"period"`
	RowCount   int            `json:"row_count"`
	Sources    map[string]int `json:"sources"`
	Skipped    int            `json:"skipped_rows"`
}

func CreateSnapshot(db *sql.DB, period string, userID int64) (BuildSummary, error) {
	res, err := db.Exec(`INSERT INTO ps_month_snapshots (period, status, source_summary, created_by) VALUES (?, 'draft', JSON_OBJECT('manual_uploads', true), ?) ON DUPLICATE KEY UPDATE updated_at=NOW()`, period, userID)
	if err != nil {
		return BuildSummary{}, err
	}
	id, _ := res.LastInsertId()
	if id == 0 {
		err = db.QueryRow(`SELECT id FROM ps_month_snapshots WHERE period=?`, period).Scan(&id)
	}
	if err != nil {
		return BuildSummary{}, err
	}
	summary, err := BuildManualMaster(db, id, period)
	if err != nil {
		return BuildSummary{}, err
	}
	payload, _ := json.Marshal(summary)
	_, _ = db.Exec(`UPDATE ps_month_snapshots SET source_summary=?, status='ready', updated_at=NOW() WHERE id=?`, payload, id)
	return summary, nil
}

func UpsertRow(db *sql.DB, row Row) error {
	metrics, err := json.Marshal(row.Metrics)
	if err != nil {
		return err
	}
	factor, _ := json.Marshal(row.Factor)
	capability, _ := json.Marshal(row.Capability)
	sales, _ := json.Marshal(row.Sales)
	_, err = db.Exec(`INSERT INTO ps_month_rows (snapshot_id, period, ps_employee_id, ps_name, ps_email, aec, region, employment_status, individual_factor_json, individual_capability_json, sales_performance_json, metrics_json)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?)
ON DUPLICATE KEY UPDATE ps_name=VALUES(ps_name), ps_email=VALUES(ps_email), aec=VALUES(aec), region=VALUES(region), employment_status=VALUES(employment_status), individual_factor_json=VALUES(individual_factor_json), individual_capability_json=VALUES(individual_capability_json), sales_performance_json=VALUES(sales_performance_json), metrics_json=VALUES(metrics_json), updated_at=NOW()`,
		row.SnapshotID, row.Period, row.PSEmployee, row.PSName, row.PSEmail, row.AEC, row.Region, row.Status, nullJSON(factor), nullJSON(capability), nullJSON(sales), metrics)
	return err
}

func BuildManualMaster(db *sql.DB, snapshotID int64, period string) (BuildSummary, error) {
	uploads, err := latestManualUploads(db)
	if err != nil {
		return BuildSummary{}, err
	}
	items := map[string]*Row{}
	summary := BuildSummary{SnapshotID: snapshotID, Period: period, Sources: map[string]int{}}
	for kind, rawRows := range uploads {
		for _, raw := range rawRows {
			if !rowMatchesPeriod(raw, period) {
				continue
			}
			key := rowKey(raw)
			if key == "" {
				summary.Skipped++
				continue
			}
			item := items[key]
			if item == nil {
				item = &Row{SnapshotID: snapshotID, Period: period, Metrics: map[string]interface{}{}}
				items[key] = item
			}
			applyIdentity(item, raw, key)
			switch kind {
			case "individual_factor":
				item.Factor = raw
			case "individual_capability":
				item.Capability = raw
			case "sales_performance":
				item.Sales = raw
			}
			for k, v := range raw {
				if k == "period" {
					continue
				}
				item.Metrics[k] = normalizeMetric(v)
			}
			summary.Sources[kind]++
		}
	}
	if _, err := db.Exec(`DELETE FROM ps_month_rows WHERE snapshot_id=?`, snapshotID); err != nil {
		return BuildSummary{}, err
	}
	for _, item := range items {
		if item.PSEmployee == "" {
			item.PSEmployee = rowKey(item.Metrics)
		}
		if item.PSName == "" {
			item.PSName = item.PSEmployee
		}
		if item.Metrics == nil {
			item.Metrics = map[string]interface{}{}
		}
		if err := UpsertRow(db, *item); err != nil {
			return BuildSummary{}, err
		}
		summary.RowCount++
	}
	return summary, nil
}

func latestManualUploads(db *sql.DB) (map[string][]map[string]interface{}, error) {
	rows, err := db.Query(`SELECT m.upload_type, m.rows_json
FROM manual_uploads m
JOIN (
  SELECT upload_type, MAX(id) id
  FROM manual_uploads
  WHERE upload_type IN ('individual_factor','individual_capability','sales_performance')
  GROUP BY upload_type
) latest ON latest.id=m.id
ORDER BY m.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]map[string]interface{}{}
	for rows.Next() {
		var kind string
		var payload []byte
		if err := rows.Scan(&kind, &payload); err != nil {
			return nil, err
		}
		var raw []map[string]interface{}
		if err := json.Unmarshal(payload, &raw); err != nil {
			return nil, fmt.Errorf("parse %s upload rows: %w", kind, err)
		}
		out[kind] = raw
	}
	return out, rows.Err()
}

func rowMatchesPeriod(row map[string]interface{}, period string) bool {
	value := strings.TrimSpace(fmt.Sprint(row["period"]))
	return value == "" || value == period
}

func rowKey(row map[string]interface{}) string {
	for _, key := range []string{"ps_employee_id", "employee_id", "nik", "id_karyawan", "ps_email", "email", "ps_name", "name", "nama"} {
		value := strings.ToLower(strings.TrimSpace(fmt.Sprint(row[key])))
		if value != "" && value != "<nil>" {
			return value
		}
	}
	return ""
}

func applyIdentity(row *Row, raw map[string]interface{}, fallback string) {
	row.PSEmployee = first(row.PSEmployee, raw, "ps_employee_id", "employee_id", "nik", "id_karyawan")
	if row.PSEmployee == "" {
		row.PSEmployee = fallback
	}
	row.PSName = first(row.PSName, raw, "ps_name", "name", "nama", "product_specialist")
	row.PSEmail = first(row.PSEmail, raw, "ps_email", "email")
	row.AEC = first(row.AEC, raw, "aec")
	row.Region = first(row.Region, raw, "region")
	row.Status = first(row.Status, raw, "employment_status", "status")
}

func first(current string, row map[string]interface{}, keys ...string) string {
	if strings.TrimSpace(current) != "" {
		return current
	}
	for _, key := range keys {
		value := strings.TrimSpace(fmt.Sprint(row[key]))
		if value != "" && value != "<nil>" {
			return value
		}
	}
	return ""
}

func normalizeMetric(value interface{}) interface{} {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" {
		return nil
	}
	clean := strings.ReplaceAll(text, "%", "")
	clean = strings.ReplaceAll(clean, ",", "")
	if n, err := strconv.ParseFloat(clean, 64); err == nil {
		return n
	}
	return text
}

func nullJSON(raw []byte) interface{} {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	return raw
}

func ListSnapshots(db *sql.DB) ([]Snapshot, error) {
	rows, err := db.Query(`SELECT s.id, s.period, s.status, COUNT(r.id) row_count, s.created_at FROM ps_month_snapshots s LEFT JOIN ps_month_rows r ON r.snapshot_id=s.id GROUP BY s.id ORDER BY s.period DESC LIMIT 36`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Snapshot{}
	for rows.Next() {
		var item Snapshot
		if err := rows.Scan(&item.ID, &item.Period, &item.Status, &item.RowCount, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func ListRows(db *sql.DB, snapshotID int64) ([]Row, error) {
	rows, err := db.Query(`SELECT id, snapshot_id, period, ps_employee_id, ps_name, ps_email, aec, region, employment_status, metrics_json FROM ps_month_rows WHERE snapshot_id=? ORDER BY ps_name LIMIT 500`, snapshotID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Row{}
	for rows.Next() {
		var item Row
		var payload []byte
		if err := rows.Scan(&item.ID, &item.SnapshotID, &item.Period, &item.PSEmployee, &item.PSName, &item.PSEmail, &item.AEC, &item.Region, &item.Status, &payload); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(payload, &item.Metrics)
		items = append(items, item)
	}
	return items, rows.Err()
}
