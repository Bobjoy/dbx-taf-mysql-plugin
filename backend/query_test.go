package main

import (
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeClient struct {
	result string
	iRet   int32
	errMsg string
	err    error
	delay  time.Duration
}

func (f *fakeClient) Select(sql string) (int32, string, string, error) {
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	return f.iRet, f.result, f.errMsg, f.err
}

func TestExecuteSelectOrderedRows(t *testing.T) {
	c := &fakeClient{result: `[{"id":1,"title":"a"},{"id":2,"title":"b"}]`}
	out, err := Execute(c, "select id, title from t", 200, 30000)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(out.Columns, ",") != "id,title" {
		t.Fatalf("columns = %v, want id,title (order preserved)", out.Columns)
	}
	if out.Total != 2 || len(out.Rows) != 2 {
		t.Fatalf("total=%d rows=%d", out.Total, len(out.Rows))
	}
	if out.Rows[0][0].(float64) != 1 || out.Rows[0][1].(string) != "a" {
		t.Fatalf("row0 = %v", out.Rows[0])
	}
	if out.Truncated {
		t.Fatal("should not be truncated")
	}
}

func TestExecuteHeterogeneousRowsUnionColumns(t *testing.T) {
	c := &fakeClient{result: `[{"a":1},{"b":2}]`}
	out, err := Execute(c, "select ...", 200, 30000)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(out.Columns, ",") != "a,b" {
		t.Fatalf("columns = %v", out.Columns)
	}
	if len(out.Rows[0]) != 2 || out.Rows[0][1] != nil {
		t.Fatalf("row0 = %v, want [1 <nil>]", out.Rows[0])
	}
}

func TestExecuteEmptyResult(t *testing.T) {
	c := &fakeClient{result: `[]`}
	out, err := Execute(c, "select 1 where 0", 200, 30000)
	if err != nil {
		t.Fatal(err)
	}
	if out.Total != 0 || len(out.Rows) != 0 {
		t.Fatalf("out = %+v", out)
	}
}

func TestExecuteTruncation(t *testing.T) {
	c := &fakeClient{result: `[{"n":1},{"n":2},{"n":3},{"n":4},{"n":5}]`}
	out, err := Execute(c, "select n from t", 3, 30000)
	if err != nil {
		t.Fatal(err)
	}
	if out.Total != 5 || len(out.Rows) != 3 || !out.Truncated {
		t.Fatalf("out = %+v", out)
	}
}

func TestExecuteGatewayFalse(t *testing.T) {
	c := &fakeClient{result: `false`}
	_, err := Execute(c, "SHOW TABLES", 200, 30000)
	if err == nil || !strings.Contains(err.Error(), "information_schema") {
		t.Fatalf("err = %v, want gateway hint", err)
	}
}

func TestExecuteNonArrayResult(t *testing.T) {
	c := &fakeClient{result: `3`}
	out, err := Execute(c, "update t set a=1", 200, 30000)
	if err != nil {
		t.Fatal(err)
	}
	if out.Note != "执行完成，返回值：3" {
		t.Fatalf("note = %q", out.Note)
	}
}

func TestExecuteIRetError(t *testing.T) {
	c := &fakeClient{iRet: -1, errMsg: "syntax error near foo"}
	_, err := Execute(c, "select foo", 200, 30000)
	if err == nil || !strings.Contains(err.Error(), "iRet=-1") || !strings.Contains(err.Error(), "syntax error") {
		t.Fatalf("err = %v", err)
	}
}

func TestExecuteTransportError(t *testing.T) {
	c := &fakeClient{err: errors.New("connection refused")}
	_, err := Execute(c, "select 1", 200, 30000)
	if err == nil || !strings.Contains(err.Error(), "TAF 调用失败") {
		t.Fatalf("err = %v", err)
	}
}

func TestExecuteTimeout(t *testing.T) {
	c := &fakeClient{result: `[{"a":1}]`, delay: 300 * time.Millisecond}
	start := time.Now()
	_, err := Execute(c, "select 1", 200, 50)
	if err == nil || !strings.Contains(err.Error(), "查询超时") {
		t.Fatalf("err = %v", err)
	}
	if time.Since(start) > 250*time.Millisecond {
		t.Fatal("should return at timeout, not wait for client")
	}
}
