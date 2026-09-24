package main

import (
	"strings"
	"testing"
)

func TestMetaSQLBuilders(t *testing.T) {
	tables, err := metaSQL("tables", "")
	if err != nil || !strings.Contains(tables, "table_type = 'BASE TABLE'") || !strings.Contains(tables, "table_schema = database()") {
		t.Fatalf("tables sql = %q err = %v", tables, err)
	}
	views, err := metaSQL("views", "")
	if err != nil || !strings.Contains(views, "table_type = 'VIEW'") {
		t.Fatalf("views sql = %q err = %v", views, err)
	}
	cols, err := metaSQL("columns", "article")
	if err != nil || !strings.Contains(cols, "information_schema.columns") || !strings.Contains(cols, "table_name = 'article'") || !strings.Contains(cols, "ordinal_position") {
		t.Fatalf("columns sql = %q err = %v", cols, err)
	}
	idx, err := metaSQL("indexes", "article")
	if err != nil || !strings.Contains(idx, "information_schema.statistics") || !strings.Contains(idx, "table_name = 'article'") {
		t.Fatalf("indexes sql = %q err = %v", idx, err)
	}
}

func TestMetaSQLEscapesTableName(t *testing.T) {
	sql, err := metaSQL("columns", "a'b")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "'a''b'") {
		t.Fatalf("quote not escaped: %q", sql)
	}
	if strings.Contains(sql, "';") || strings.Contains(sql, "--") {
		t.Fatalf("escaped name should not break out of literal: %q", sql)
	}
}

func TestMetaSQLRejectsUnknownKind(t *testing.T) {
	if _, err := metaSQL("drop", ""); err == nil {
		t.Fatal("unknown kind should error")
	}
	for _, kind := range []string{"columns", "indexes"} {
		if _, err := metaSQL(kind, ""); err == nil {
			t.Fatalf("%s without table should error", kind)
		}
	}
}

func TestMetaShapesNodes(t *testing.T) {
	client := &fakeClient{result: `[{"TABLE_NAME":"a"},{"TABLE_NAME":"b"}]`}
	nodes, err := Meta(client, "tables", "", 30000)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 2 || nodes[0].Name != "a" || nodes[1].Name != "b" {
		t.Fatalf("nodes = %+v", nodes)
	}

	colClient := &fakeClient{result: `[{"column_name":"id","column_type":"int(11)","column_key":"PRI","is_nullable":"NO","column_comment":"主键"}]`}
	cols, err := Meta(colClient, "columns", "t", 30000)
	if err != nil {
		t.Fatal(err)
	}
	if len(cols) != 1 || cols[0].Name != "id" {
		t.Fatalf("cols = %+v", cols)
	}
	for _, want := range []string{"int(11)", "PRI", "主键"} {
		if !strings.Contains(cols[0].Detail, want) {
			t.Fatalf("col detail %q missing %q", cols[0].Detail, want)
		}
	}

	idxClient := &fakeClient{result: `[{"index_name":"PRIMARY","non_unique":0,"cols":"id"},{"index_name":"idx_a","non_unique":1,"cols":"a,b"}]`}
	indexes, err := Meta(idxClient, "indexes", "t", 30000)
	if err != nil {
		t.Fatal(err)
	}
	if len(indexes) != 2 {
		t.Fatalf("indexes = %+v", indexes)
	}
	if !strings.Contains(indexes[0].Detail, "unique") || !strings.Contains(indexes[0].Detail, "(id)") {
		t.Fatalf("primary detail = %q", indexes[0].Detail)
	}
	if strings.Contains(indexes[1].Detail, "unique ") || !strings.Contains(indexes[1].Detail, "(a,b)") {
		t.Fatalf("secondary detail = %q", indexes[1].Detail)
	}
}

func TestMetaHandlerRoute(t *testing.T) {
	p := newTestPlugin(t)
	p.newClient = func(servant string) (TafClient, error) {
		return &fakeClient{result: `[{"TABLE_NAME":"media_list"}]`}, nil
	}
	conn := `{"connection":{"id":"c1","external_config":{"servant":"s"}}}`
	if _, err := handle(t, p, "connection/connect", conn); err != nil {
		t.Fatal(err)
	}
	res, err := handle(t, p, "taf-mysql/meta", `{"connectionId":"c1","kind":"tables"}`)
	if err != nil {
		t.Fatalf("meta: %v", err)
	}
	nodes := res.([]MetaNode)
	if len(nodes) != 1 || nodes[0].Name != "media_list" {
		t.Fatalf("res = %+v", res)
	}
	if _, err := handle(t, p, "taf-mysql/meta", `{"connectionId":"c1","kind":"nope"}`); err == nil {
		t.Fatal("bad kind should fail")
	}
	if _, err := handle(t, p, "taf-mysql/meta", `{"connectionId":"ghost","kind":"tables"}`); err == nil {
		t.Fatal("unknown connection should fail")
	}
}
