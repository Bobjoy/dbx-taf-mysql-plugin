package main

import (
	"encoding/json"
	"strings"
	"testing"

	dbxpluginsdk "github.com/t8y2/dbx/plugins/sdk/go/dbx-plugin-sdk"
)

func newTestPlugin(t *testing.T) *plugin {
	t.Helper()
	p := newPlugin()
	p.newClient = func(servant string) (TafClient, error) {
		return &fakeClient{result: `[{"ok":1}]`}, nil
	}
	return p
}

func handle(t *testing.T, p *plugin, method, params string) (any, *dbxpluginsdk.PluginError) {
	t.Helper()
	result, err := p.Handle(dbxpluginsdk.RequestContext{}, method, json.RawMessage(params), nil)
	return result, err
}

func TestConnectionLifecycle(t *testing.T) {
	p := newTestPlugin(t)
	params := `{"connection":{"id":"c1","external_config":{"servant":"APP.X@tcp -h 1.2.3.4 -p 1 -t 60000","allow_write":true}}}`

	if _, err := handle(t, p, "connection/connect", params); err != nil {
		t.Fatalf("connect: %v", err)
	}
	res, err := handle(t, p, "taf-mysql/query", `{"connectionId":"c1","sql":"select 1"}`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	out := res.(QueryOutput)
	if len(out.Rows) != 1 || out.Columns[0] != "ok" {
		t.Fatalf("query out = %+v", out)
	}
	if _, err := handle(t, p, "connection/disconnect", params); err != nil {
		t.Fatalf("disconnect: %v", err)
	}
	if _, err := handle(t, p, "taf-mysql/query", `{"connectionId":"c1","sql":"select 1"}`); err == nil {
		t.Fatal("query after disconnect should fail")
	}
}

func TestTestConnectionUsesSelect1(t *testing.T) {
	p := newTestPlugin(t)
	var gotSql string
	p.newClient = func(servant string) (TafClient, error) {
		return recorderClient{onSelect: func(sql string) { gotSql = sql }}, nil
	}
	res, err := handle(t, p, "connection/test", `{"connection":{"id":"t1","external_config":{"servant":"APP.X@tcp -h 1 -p 1 -t 1"}}}`)
	if err != nil {
		t.Fatalf("test: %+v", err)
	}
	m := res.(map[string]any)
	if m["success"] != true {
		t.Fatalf("test result = %v", m)
	}
	if gotSql != "select 1" {
		t.Fatalf("test issued %q, want select 1", gotSql)
	}
}

func TestQueryPolicyEnforced(t *testing.T) {
	p := newTestPlugin(t)
	p.newClient = func(servant string) (TafClient, error) {
		return &fakeClient{result: `false`}, nil
	}
	conn := `{"connection":{"id":"ro","external_config":{"servant":"s"}}}`
	if _, err := handle(t, p, "connection/connect", conn); err != nil {
		t.Fatal(err)
	}
	// 默认只读：DML 拒绝
	_, err := handle(t, p, "taf-mysql/query", `{"connectionId":"ro","sql":"DELETE FROM t"}`)
	if err == nil || !strings.Contains(err.Message, "只读") {
		t.Fatalf("err = %+v, want readonly reject", err)
	}
	if _, err := handle(t, p, "connection/disconnect", conn); err != nil {
		t.Fatal(err)
	}
	// 开 allowWrite 后 DML 放行到客户端
	connW := `{"connection":{"id":"rw","external_config":{"servant":"s","allow_write":true}}}`
	if _, err := handle(t, p, "connection/connect", connW); err != nil {
		t.Fatal(err)
	}
	_, err = handle(t, p, "taf-mysql/query", `{"connectionId":"rw","sql":"DELETE FROM t"}`)
	if err == nil || !strings.Contains(err.Message, "information_schema") {
		t.Fatalf("err = %+v, want gateway hint (policy passed)", err)
	}
}

func TestMissingServantRejected(t *testing.T) {
	p := newTestPlugin(t)
	if _, err := handle(t, p, "connection/connect", `{"connection":{"id":"x","external_config":{}}}`); err == nil {
		t.Fatal("connect without servant should fail")
	}
}

func TestUnknownMethod(t *testing.T) {
	p := newTestPlugin(t)
	if _, err := handle(t, p, "taf-mysql/nope", `{}`); err == nil {
		t.Fatal("unknown method should fail")
	}
}

type recorderClient struct{ onSelect func(string) }

func (r recorderClient) Select(sql string) (int32, string, string, error) {
	r.onSelect(sql)
	return 0, `[{"ok":1}]`, "", nil
}
