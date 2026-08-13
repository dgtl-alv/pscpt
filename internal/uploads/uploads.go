package uploads

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"
)

type ParsedFile struct {
	Columns []string                 `json:"columns"`
	Rows    []map[string]interface{} `json:"rows"`
	Count   int                      `json:"count"`
}

func Parse(file multipart.File, header *multipart.FileHeader) (ParsedFile, error) {
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".xlsx") {
		return ParsedFile{}, fmt.Errorf("only .xlsx supported for now")
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return ParsedFile{}, err
	}
	book, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return ParsedFile{}, err
	}
	defer book.Close()
	sheet := book.GetSheetName(0)
	rows, err := book.GetRows(sheet)
	if err != nil {
		return ParsedFile{}, err
	}
	if len(rows) < 2 {
		return ParsedFile{}, fmt.Errorf("xlsx needs header and at least one data row")
	}
	columns := normalizeHeaders(rows[0])
	out := make([]map[string]interface{}, 0, len(rows)-1)
	for _, row := range rows[1:] {
		item := map[string]interface{}{}
		empty := true
		for i, col := range columns {
			value := ""
			if i < len(row) {
				value = strings.TrimSpace(row[i])
			}
			if value != "" {
				empty = false
			}
			item[col] = value
		}
		if !empty {
			out = append(out, item)
		}
	}
	return ParsedFile{Columns: columns, Rows: out, Count: len(out)}, nil
}

func Save(db *sql.DB, kind, filename string, userID int64, parsed ParsedFile) (int64, error) {
	payload, err := json.Marshal(parsed.Rows)
	if err != nil {
		return 0, err
	}
	cols, err := json.Marshal(parsed.Columns)
	if err != nil {
		return 0, err
	}
	res, err := db.Exec(`INSERT INTO manual_uploads (upload_type, filename, columns_json, rows_json, row_count, uploaded_by) VALUES (?,?,?,?,?,?)`, kind, filepath.Base(filename), cols, payload, parsed.Count, userID)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func normalizeHeaders(headers []string) []string {
	seen := map[string]int{}
	out := make([]string, len(headers))
	for i, h := range headers {
		key := strings.ToLower(strings.TrimSpace(h))
		key = strings.NewReplacer(" ", "_", "-", "_", "/", "_", ".", "_").Replace(key)
		if key == "" {
			key = fmt.Sprintf("column_%d", i+1)
		}
		seen[key]++
		if seen[key] > 1 {
			key = fmt.Sprintf("%s_%d", key, seen[key])
		}
		out[i] = key
	}
	return out
}
