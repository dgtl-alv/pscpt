package psmonth

import (
	"database/sql"
	"encoding/json"
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
	Metrics    map[string]interface{} `json:"metrics"`
}

func CreateSnapshot(db *sql.DB, period string, userID int64) (int64, error) {
	res, err := db.Exec(`INSERT INTO ps_month_snapshots (period, status, source_summary, created_by) VALUES (?, 'draft', JSON_OBJECT('manual_uploads', true), ?) ON DUPLICATE KEY UPDATE updated_at=NOW()`, period, userID)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	if id == 0 {
		err = db.QueryRow(`SELECT id FROM ps_month_snapshots WHERE period=?`, period).Scan(&id)
	}
	return id, err
}

func UpsertRow(db *sql.DB, row Row) error {
	payload, err := json.Marshal(row.Metrics)
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO ps_month_rows (snapshot_id, period, ps_employee_id, ps_name, ps_email, aec, region, employment_status, metrics_json)
VALUES (?,?,?,?,?,?,?,?,?)
ON DUPLICATE KEY UPDATE ps_name=VALUES(ps_name), ps_email=VALUES(ps_email), aec=VALUES(aec), region=VALUES(region), employment_status=VALUES(employment_status), metrics_json=VALUES(metrics_json), updated_at=NOW()`,
		row.SnapshotID, row.Period, row.PSEmployee, row.PSName, row.PSEmail, row.AEC, row.Region, row.Status, payload)
	return err
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
