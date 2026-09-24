package main

import (
	"fmt"
	"strings"
)

type MetaNode struct {
	Name   string `json:"name"`
	Detail string `json:"detail,omitempty"`
}

const metaMaxRows = 5000

func quoteSQL(s string) string { return strings.ReplaceAll(s, "'", "''") }

func metaSQL(kind, table string) (string, error) {
	switch kind {
	case "tables":
		return "select table_name from information_schema.tables where table_schema = database() and table_type = 'BASE TABLE' order by table_name", nil
	case "views":
		return "select table_name from information_schema.tables where table_schema = database() and table_type = 'VIEW' order by table_name", nil
	case "columns":
		if table == "" {
			return "", fmt.Errorf("columns 需要 table 参数")
		}
		return fmt.Sprintf("select column_name, column_type, column_key, is_nullable, column_comment from information_schema.columns where table_schema = database() and table_name = '%s' order by ordinal_position", quoteSQL(table)), nil
	case "indexes":
		if table == "" {
			return "", fmt.Errorf("indexes 需要 table 参数")
		}
		return fmt.Sprintf("select index_name, non_unique, group_concat(column_name order by seq_in_index) as cols from information_schema.statistics where table_schema = database() and table_name = '%s' group by index_name, non_unique order by index_name", quoteSQL(table)), nil
	}
	return "", fmt.Errorf("未知元数据类型: %s", kind)
}

func cell(columns []string, row []any, name string) (any, bool) {
	for i, c := range columns {
		if strings.EqualFold(c, name) && i < len(row) {
			return row[i], true
		}
	}
	return nil, false
}

func text(rows [][]any, columns []string, idx int, name string) string {
	if v, ok := cell(columns, rows[idx], name); ok && v != nil {
		if s, isStr := v.(string); isStr {
			return s
		}
		return fmt.Sprint(v)
	}
	return ""
}

func Meta(client TafClient, kind, table string, timeoutMs int64) ([]MetaNode, error) {
	sql, err := metaSQL(kind, table)
	if err != nil {
		return nil, err
	}
	out, err := Execute(client, sql, metaMaxRows, timeoutMs)
	if err != nil {
		return nil, err
	}
	nodes := make([]MetaNode, 0, len(out.Rows))
	for i := range out.Rows {
		switch kind {
		case "tables", "views":
			nodes = append(nodes, MetaNode{Name: text(out.Rows, out.Columns, i, "table_name")})
		case "columns":
			var parts []string
			if t := text(out.Rows, out.Columns, i, "column_type"); t != "" {
				parts = append(parts, t)
			}
			if k := text(out.Rows, out.Columns, i, "column_key"); k != "" {
				parts = append(parts, k)
			}
			if text(out.Rows, out.Columns, i, "is_nullable") == "YES" {
				parts = append(parts, "NULL")
			}
			if c := text(out.Rows, out.Columns, i, "column_comment"); c != "" {
				parts = append(parts, c)
			}
			nodes = append(nodes, MetaNode{Name: text(out.Rows, out.Columns, i, "column_name"), Detail: strings.Join(parts, " ")})
		case "indexes":
			unique := ""
			if v, ok := cell(out.Columns, out.Rows[i], "non_unique"); ok {
				if f, isNum := v.(float64); isNum && f == 0 {
					unique = "unique "
				}
			}
			cols := text(out.Rows, out.Columns, i, "cols")
			nodes = append(nodes, MetaNode{
				Name:   text(out.Rows, out.Columns, i, "index_name"),
				Detail: unique + "(" + cols + ")",
			})
		}
	}
	return nodes, nil
}
