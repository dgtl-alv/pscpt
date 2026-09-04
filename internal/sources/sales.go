package sources

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"pscpt/internal/config"
	"pscpt/internal/uploads"
)

type SalesRow struct {
	PSEmployeeID string         `json:"ps_employee_id,omitempty"`
	PSName       string         `json:"ps_name,omitempty"`
	PSEmail      string         `json:"ps_email,omitempty"`
	Salesperson  string         `json:"salesperson_name,omitempty"`
	OrderRef     string         `json:"order_ref,omitempty"`
	OrderState   string         `json:"order_state,omitempty"`
	OrderDate    *time.Time     `json:"order_date,omitempty"`
	BikeModel    string         `json:"bike_model,omitempty"`
	Quantity     float64        `json:"quantity,omitempty"`
	AmountTotal  float64        `json:"amount_total,omitempty"`
	Raw          map[string]any `json:"raw"`
}

type RunSummary struct {
	ID      int64  `json:"id"`
	Source  string `json:"source_type"`
	Mode    string `json:"mode"`
	Period  string `json:"period"`
	Status  string `json:"status"`
	File    string `json:"filename,omitempty"`
	Rows    int    `json:"row_count"`
	Created string `json:"created_at"`
}

func SaveSalesUpload(db *sql.DB, period string, file multipart.File, header *multipart.FileHeader, userID int64) (int64, []string, []SalesRow, error) {
	if err := validatePeriod(period); err != nil {
		return 0, nil, nil, err
	}
	parsed, err := uploads.Parse(file, header)
	if err != nil {
		return 0, nil, nil, err
	}
	rows := make([]SalesRow, 0, parsed.Count)
	for _, raw := range parsed.Rows {
		rows = append(rows, salesRowFromUpload(raw))
	}
	runID, err := saveSalesRun(db, "sales_performance", "upload_file", period, filepath.Base(header.Filename), userID, rows, map[string]any{"columns": parsed.Columns})
	if err != nil {
		return 0, nil, nil, err
	}
	return runID, parsed.Columns, rows, nil
}

func SyncOdooSalesOrders(ctx context.Context, db *sql.DB, cfg config.EmicaConfig, period string, userID int64) (int64, []SalesRow, error) {
	if err := validatePeriod(period); err != nil {
		return 0, nil, err
	}
	client, err := newOdooClient(cfg)
	if err != nil {
		return 0, nil, err
	}
	uid, err := client.authenticate(ctx)
	if err != nil {
		return 0, nil, err
	}
	start, end, _ := periodRange(period)
	orderFields := []string{"name", "state", "date_order", "user_id", "amount_total"}
	domain := []any{[]any{"date_order", ">=", start.Format("2006-01-02 15:04:05")}, []any{"date_order", "<", end.Format("2006-01-02 15:04:05")}}
	orders, err := client.searchRead(ctx, uid, "sale.order", domain, orderFields, 0)
	if err != nil {
		return 0, nil, err
	}
	orderIDs := make([]any, 0, len(orders))
	orderByID := map[int64]map[string]any{}
	for _, order := range orders {
		id := int64(number(order["id"]))
		if id > 0 {
			orderIDs = append(orderIDs, id)
			orderByID[id] = order
		}
	}
	rows := []SalesRow{}
	if len(orderIDs) > 0 {
		lineFields := []string{"order_id", "product_id", "product_uom_qty", "price_total"}
		lines, err := client.searchRead(ctx, uid, "sale.order.line", []any{[]any{"order_id", "in", orderIDs}}, lineFields, 0)
		if err != nil {
			return 0, nil, err
		}
		for _, line := range lines {
			orderID, orderRef := many2one(line["order_id"])
			order := orderByID[orderID]
			_, productName := many2one(line["product_id"])
			_, salesperson := many2one(order["user_id"])
			rows = append(rows, SalesRow{
				Salesperson: salesperson,
				OrderRef:    firstNonEmpty(orderRef, text(order["name"])),
				OrderState:  text(order["state"]),
				OrderDate:   parseTime(text(order["date_order"])),
				BikeModel:   productName,
				Quantity:    number(line["product_uom_qty"]),
				AmountTotal: number(line["price_total"]),
				Raw:         map[string]any{"order": order, "line": line},
			})
		}
	}
	runID, err := saveSalesRun(db, "sales_performance", "sync_api", period, "", userID, rows, map[string]any{"api": "odoo_execute_kw_search_read", "models": []string{"sale.order", "sale.order.line"}})
	if err != nil {
		return 0, nil, err
	}
	return runID, rows, nil
}

func ListRuns(db *sql.DB) ([]RunSummary, error) {
	rows, err := db.Query(`SELECT id, source_type, mode, period, status, COALESCE(filename,''), row_count, created_at FROM source_runs ORDER BY id DESC LIMIT 80`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RunSummary{}
	for rows.Next() {
		var item RunSummary
		var created time.Time
		if err := rows.Scan(&item.ID, &item.Source, &item.Mode, &item.Period, &item.Status, &item.File, &item.Rows, &created); err != nil {
			return nil, err
		}
		item.Created = created.Format(time.RFC3339)
		out = append(out, item)
	}
	return out, rows.Err()
}

func saveSalesRun(db *sql.DB, source, mode, period, filename string, userID int64, rows []SalesRow, summary map[string]any) (int64, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	summaryJSON, _ := json.Marshal(summary)
	res, err := tx.Exec(`INSERT INTO source_runs (source_type, mode, period, filename, row_count, summary_json, created_by) VALUES (?,?,?,?,?,?,?)`, source, mode, period, nullEmpty(filename), len(rows), summaryJSON, userID)
	if err != nil {
		return 0, err
	}
	runID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	stmt, err := tx.Prepare(`INSERT INTO sales_performance_rows (run_id, period, ps_employee_id, ps_name, ps_email, salesperson_name, order_ref, order_state, order_date, bike_model, quantity, amount_total, raw_json) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	for _, row := range rows {
		raw, _ := json.Marshal(row.Raw)
		_, err := stmt.Exec(runID, period, nullEmpty(row.PSEmployeeID), nullEmpty(row.PSName), nullEmpty(row.PSEmail), nullEmpty(row.Salesperson), nullEmpty(row.OrderRef), nullEmpty(row.OrderState), row.OrderDate, nullEmpty(row.BikeModel), nullFloat(row.Quantity), nullFloat(row.AmountTotal), raw)
		if err != nil {
			return 0, err
		}
	}
	return runID, tx.Commit()
}

type odooClient struct {
	baseURL string
	db      string
	user    string
	secret  string
	http    *http.Client
}

func newOdooClient(cfg config.EmicaConfig) (*odooClient, error) {
	if cfg.BaseURL == "" || cfg.DB == "" || cfg.Username == "" {
		return nil, fmt.Errorf("EMICA_BASE_URL, EMICA_ODOO_DB, and EMICA_ODOO_USERNAME are required")
	}
	secret := firstNonEmpty(cfg.APIKey, cfg.Password)
	if secret == "" {
		return nil, fmt.Errorf("EMICA_ODOO_API_KEY or EMICA_ODOO_PASSWORD is required")
	}
	timeout, err := time.ParseDuration(cfg.Timeout)
	if err != nil {
		timeout = 45 * time.Second
	}
	return &odooClient{baseURL: strings.TrimRight(cfg.BaseURL, "/"), db: cfg.DB, user: cfg.Username, secret: secret, http: &http.Client{Timeout: timeout}}, nil
}

func (c *odooClient) authenticate(ctx context.Context) (int64, error) {
	var uid int64
	err := c.call(ctx, "common", "authenticate", []any{c.db, c.user, c.secret, map[string]any{}}, &uid)
	if err != nil {
		return 0, err
	}
	if uid == 0 {
		return 0, fmt.Errorf("odoo authentication failed")
	}
	return uid, nil
}

func (c *odooClient) searchRead(ctx context.Context, uid int64, model string, domain []any, fields []string, limit int) ([]map[string]any, error) {
	kwargs := map[string]any{"fields": fields}
	if limit > 0 {
		kwargs["limit"] = limit
	}
	var out []map[string]any
	err := c.call(ctx, "object", "execute_kw", []any{c.db, uid, c.secret, model, "search_read", []any{domain}, kwargs}, &out)
	return out, err
}

func (c *odooClient) call(ctx context.Context, service, method string, args []any, result any) error {
	endpoint, err := url.JoinPath(c.baseURL, "jsonrpc")
	if err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": "call", "params": map[string]any{"service": service, "method": method, "args": args}, "id": time.Now().UnixNano()})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return fmt.Errorf("odoo http %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	var envelope struct {
		Result json.RawMessage `json:"result"`
		Error  any             `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return err
	}
	if envelope.Error != nil {
		raw, _ := json.Marshal(envelope.Error)
		return fmt.Errorf("odoo rpc error: %s", raw)
	}
	return json.Unmarshal(envelope.Result, result)
}

func salesRowFromUpload(raw map[string]any) SalesRow {
	return SalesRow{
		PSEmployeeID: firstRaw(raw, "ps_employee_id", "employee_id", "nik", "id_karyawan"),
		PSName:       firstRaw(raw, "ps_name", "name", "nama", "product_specialist"),
		PSEmail:      firstRaw(raw, "ps_email", "email"),
		Salesperson:  firstRaw(raw, "salesperson_name", "salesperson", "sales_person"),
		OrderRef:     firstRaw(raw, "order_ref", "order", "so_number", "name"),
		OrderState:   firstRaw(raw, "order_state", "state", "status"),
		OrderDate:    parseTime(firstRaw(raw, "order_date", "date_order", "tanggal")),
		BikeModel:    firstRaw(raw, "bike_model", "model", "product", "product_name"),
		Quantity:     parseFloat(firstRaw(raw, "quantity", "qty", "product_uom_qty")),
		AmountTotal:  parseFloat(firstRaw(raw, "amount_total", "amount", "price_total", "total")),
		Raw:          raw,
	}
}

func validatePeriod(period string) error {
	if _, _, err := periodRange(period); err != nil {
		return fmt.Errorf("period must use YYYY-MM")
	}
	return nil
}

func periodRange(period string) (time.Time, time.Time, error) {
	start, err := time.Parse("2006-01", period)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return start, start.AddDate(0, 1, 0), nil
}

func firstRaw(raw map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := raw[key]; ok && strings.TrimSpace(fmt.Sprint(v)) != "" {
			return strings.TrimSpace(fmt.Sprint(v))
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func text(v any) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func number(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case json.Number:
		f, _ := x.Float64()
		return f
	case string:
		return parseFloat(x)
	default:
		return 0
	}
}

func parseFloat(v string) float64 {
	clean := strings.ReplaceAll(strings.TrimSpace(v), ",", "")
	f, _ := strconv.ParseFloat(clean, 64)
	return f
}

func parseTime(v string) *time.Time {
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339, "2006-01-02", "02/01/2006", "1/2/2006"} {
		if t, err := time.Parse(layout, strings.TrimSpace(v)); err == nil {
			return &t
		}
	}
	return nil
}

func many2one(v any) (int64, string) {
	items, ok := v.([]any)
	if !ok || len(items) == 0 {
		return 0, text(v)
	}
	return int64(number(items[0])), text(items[min(1, len(items)-1)])
}

func nullEmpty(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}

func nullFloat(v float64) any {
	if v == 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		return nil
	}
	return v
}
