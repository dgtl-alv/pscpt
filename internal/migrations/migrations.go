package migrations

import "database/sql"

func Run(db *sql.DB) error {
	if _, err := db.Exec(schema); err != nil {
		return err
	}
	_, _ = db.Exec(`ALTER TABLE ps_month_snapshots ADD COLUMN updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP`)
	return nil
}

const schema = `
CREATE TABLE IF NOT EXISTS users (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  name VARCHAR(160) NOT NULL,
  email VARCHAR(190) NOT NULL UNIQUE,
  password_hash VARCHAR(255) NOT NULL,
  role VARCHAR(50) NOT NULL DEFAULT 'user',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS password_reset_tokens (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  user_id BIGINT NOT NULL,
  token_hash CHAR(64) NOT NULL UNIQUE,
  expires_at DATETIME NOT NULL,
  used_at DATETIME NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS audit_events (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  user_id BIGINT NULL,
  event_type VARCHAR(80) NOT NULL,
  event_payload JSON NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS manual_uploads (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  upload_type ENUM('individual_factor','leadership') NOT NULL,
  filename VARCHAR(255) NOT NULL,
  columns_json JSON NOT NULL,
  rows_json JSON NOT NULL,
  row_count INT NOT NULL DEFAULT 0,
  uploaded_by BIGINT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (uploaded_by) REFERENCES users(id) ON DELETE SET NULL
);
CREATE TABLE IF NOT EXISTS ps_month_rows (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  snapshot_id BIGINT NOT NULL,
  period CHAR(7) NOT NULL,
  ps_employee_id VARCHAR(80) NOT NULL,
  ps_name VARCHAR(180) NOT NULL,
  ps_email VARCHAR(190) NULL,
  aec VARCHAR(160) NULL,
  region VARCHAR(120) NULL,
  employment_status VARCHAR(40) NULL,
  individual_factor_json JSON NULL,
  capability_talenta_json JSON NULL,
  capability_moodle_json JSON NULL,
  capability_lead_json JSON NULL,
  leadership_json JSON NULL,
  sales_performance_json JSON NULL,
  controls_json JSON NULL,
  metrics_json JSON NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uniq_snapshot_ps (snapshot_id, ps_employee_id),
  KEY idx_period_ps (period, ps_employee_id),
  KEY idx_aec_region (aec, region),
  FOREIGN KEY (snapshot_id) REFERENCES ps_month_snapshots(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS ps_month_snapshots (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  period CHAR(7) NOT NULL,
  status VARCHAR(40) NOT NULL DEFAULT 'draft',
  source_summary JSON NULL,
  created_by BIGINT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uniq_period (period),
  FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL
);
`
