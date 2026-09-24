package main

import (
	"regexp"
	"strings"
	"testing"
)

func assertDecision(t *testing.T, sql string, allowWrite bool, want, verbPattern string) {
	t.Helper()
	got := Classify(sql, allowWrite)
	if got.Decision != want {
		t.Fatalf("Classify(%q, allowWrite=%v) = %q (%s), want %q", sql, allowWrite, got.Decision, got.Reason, want)
	}
	if verbPattern != "" && !regexp.MustCompile(verbPattern).MatchString(got.Verb) {
		t.Fatalf("verb for %q = %q, want match %q", sql, got.Verb, verbPattern)
	}
}

func TestReadonlyStatements(t *testing.T) {
	for _, sql := range []string{
		"select 1", "SELECT * FROM t_order WHERE id=1", "  \n\t SELECT 1",
		"sElEcT 1", "SHOW TABLES", "show create table t_order", "desc t_order",
		"DESCRIBE t_order", "EXPLAIN SELECT 1 FROM dual",
	} {
		assertDecision(t, sql, false, DecisionReadonly, "")
	}
}

func TestSemicolonAndCommentsAreNotMultiStatement(t *testing.T) {
	readonly := []string{
		"SELECT 1;", "SELECT 1 ;",
		"SELECT * FROM t -- 注掉的东西; 分号不算",
		"SELECT * FROM t # 井号注释; 不算",
		"/* 头部注释; 不算 */ SELECT 1",
		"SELECT /* 中间; 注释 */ 1",
		"SELECT 1 /* 尾部 */ ;",
	}
	for _, sql := range readonly {
		assertDecision(t, sql, false, DecisionReadonly, "")
	}
	assertDecision(t, "SELECT 1 ;;", false, DecisionReject, "")
}

func TestLiteralsDoNotTriggerDetection(t *testing.T) {
	for _, sql := range []string{
		"SELECT ';' AS x", "SELECT 'a;b' AS x", `SELECT "insert into t" AS x`,
		"SELECT 'it\\'s; tricky' AS x", "SELECT `delete` FROM t",
		"SELECT 'drop table x' FROM dual",
	} {
		assertDecision(t, sql, false, DecisionReadonly, "")
	}
}

func TestMultiStatementRejected(t *testing.T) {
	for _, sql := range []string{
		"SELECT 1; DROP TABLE t", "SELECT 1; DELETE FROM t",
		"SELECT 'a;b' AS x; SELECT 2", "SHOW TABLES; SELECT 1",
	} {
		assertDecision(t, sql, false, DecisionReject, "")
	}
}

func TestDDLAlwaysRejected(t *testing.T) {
	ddl := []string{
		"DROP TABLE t", "TRUNCATE t_user", "ALTER TABLE t ADD COLUMN x int",
		"CREATE TABLE t (id int)", "RENAME TABLE a TO b", "GRANT ALL ON db TO u",
	}
	for _, sql := range ddl {
		assertDecision(t, sql, false, DecisionReject, "")
		assertDecision(t, sql, true, DecisionReject, "")
	}
	// DELETE 是对照组：开写开关后应放行
	assertDecision(t, "DELETE FROM t", false, DecisionReject, "")
	assertDecision(t, "DELETE FROM t", true, DecisionWrite, "DELETE")
}

func TestDMLRequiresAllowWrite(t *testing.T) {
	for _, sql := range []string{
		"UPDATE t SET a=1 WHERE id=2", "DELETE FROM t WHERE id=1",
		"INSERT INTO t VALUES (1)", "REPLACE INTO t VALUES (1)",
	} {
		got := Classify(sql, false)
		if got.Decision != DecisionReject {
			t.Fatalf("no-write %q = %q, want reject", sql, got.Decision)
		}
		if !strings.Contains(got.Reason, "allow_write") {
			t.Fatalf("reject reason %q should mention allow_write", got.Reason)
		}
		assertDecision(t, sql, true, DecisionWrite, "")
	}
}

func TestWithPrefixResolvesRealVerb(t *testing.T) {
	assertDecision(t, "WITH x AS (SELECT 1 AS id) SELECT * FROM x", false, DecisionReadonly, "")
	assertDecision(t, "WITH x AS (SELECT id FROM t) , y AS (SELECT 1) SELECT * FROM x, y", false, DecisionReadonly, "")
	dml := "WITH x AS (SELECT id FROM t) DELETE FROM t WHERE id IN (SELECT id FROM x)"
	assertDecision(t, dml, false, DecisionReject, "")
	assertDecision(t, dml, true, DecisionWrite, "DELETE")
	assertDecision(t, "WITH x AS (SELECT id FROM t) UPDATE t SET a=1 WHERE id IN (SELECT id FROM x)", true, DecisionWrite, "UPDATE")
}

func TestSelectIntoFileRejected(t *testing.T) {
	assertDecision(t, "SELECT * FROM t INTO OUTFILE '/tmp/x.csv'", false, DecisionReject, "")
	assertDecision(t, "SELECT * FROM t INTO DUMPFILE '/tmp/x.bin'", false, DecisionReject, "")
	assertDecision(t, "SELECT outfile_name FROM t", false, DecisionReadonly, "")
}

func TestExecutableCommentRejected(t *testing.T) {
	for _, sql := range []string{"/*! SELECT 1 */", "SELECT 1 /*! FROM t */", "SELECT '/*! x */' FROM dual"} {
		assertDecision(t, sql, false, DecisionReject, "")
	}
}

func TestLockTxnProcRejected(t *testing.T) {
	for _, sql := range []string{
		"LOCK TABLES t READ", "START TRANSACTION", "CALL some_proc()",
		"SET @a = 1", "USE my_db", `LOAD DATA INFILE "/tmp/x" INTO TABLE t`,
	} {
		assertDecision(t, sql, false, DecisionReject, "")
	}
}

func TestEmptyInputRejected(t *testing.T) {
	for _, sql := range []string{"", "   ", "   ;  "} {
		assertDecision(t, sql, false, DecisionReject, "")
	}
}

func TestTrailingJunkStillReadonly(t *testing.T) {
	assertDecision(t, "  SELECT order_id AS x FROM T WHERE a='b;c' ; -- tail\n", false, DecisionReadonly, "")
	assertDecision(t, "SELECT 1\n\n/* x */\n-- y\n", false, DecisionReadonly, "")
}
