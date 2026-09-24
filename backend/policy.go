package main

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

const (
	DecisionReadonly = "readonly"
	DecisionWrite    = "write"
	DecisionReject   = "reject"
)

type Verdict struct {
	Decision string
	Verb     string
	Reason   string
}

var readVerbs = map[string]bool{"SELECT": true, "SHOW": true, "DESC": true, "DESCRIBE": true, "EXPLAIN": true}
var writeVerbs = map[string]bool{"INSERT": true, "UPDATE": true, "DELETE": true, "REPLACE": true}

var intoFileRe = regexp.MustCompile(`OUTFILE|DUMPFILE`)

func rejectVerdict(reason string) Verdict {
	return Verdict{Decision: DecisionReject, Reason: reason}
}

// mask 把字符串字面量、反引号标识符、注释替换成等长空格，后续判定只看结构字符
func mask(sql string) string {
	src := []byte(sql)
	out := make([]byte, 0, len(src))
	i := 0
	for i < len(src) {
		ch := src[i]
		var next byte
		if i+1 < len(src) {
			next = src[i+1]
		}
		switch {
		case ch == '/' && next == '*':
			end := strings.Index(sql[i+2:], "*/")
			stop := len(src)
			if end != -1 {
				stop = i + 2 + end + 2
			}
			out = append(out, bytes_spaces(stop-i)...)
			i = stop
		case ch == '#' || (ch == '-' && next == '-' && (i+2 >= len(src) || sql[i+2] == ' ' || sql[i+2] == '\t' || sql[i+2] == '\n' || sql[i+2] == '\r' || sql[i+2] == '(')):
			end := strings.IndexByte(sql[i:], '\n')
			stop := len(src)
			if end != -1 {
				stop = i + end
			}
			out = append(out, bytes_spaces(stop-i)...)
			i = stop
		case ch == '\'' || ch == '"' || ch == '`':
			j := i + 1
			for j < len(src) {
				if src[j] == '\\' {
					j += 2
					continue
				}
				if src[j] == ch {
					if j+1 < len(src) && src[j+1] == ch {
						j += 2
						continue
					}
					j++
					break
				}
				j++
			}
			if j > len(src) {
				j = len(src)
			}
			out = append(out, ch)
			if j-i >= 2 {
				inner := j - i - 2
				if inner > 0 {
					out = append(out, bytes_spaces(inner)...)
				}
				out = append(out, ch)
			}
			i = j
		default:
			out = append(out, ch)
			i++
		}
	}
	return string(out)
}

func bytes_spaces(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = ' '
	}
	return b
}

type token struct {
	word  string
	depth int
}

func tokenize(masked string) []token {
	var tokens []token
	depth := 0
	var word []byte
	wordDepth := 0
	flush := func() {
		if len(word) > 0 {
			tokens = append(tokens, token{word: strings.ToUpper(string(word)), depth: wordDepth})
			word = word[:0]
		}
	}
	for i := 0; i < len(masked); i++ {
		ch := masked[i]
		isLetter := ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z' || ch == '_' || ch == '$'
		if isLetter {
			if len(word) == 0 {
				wordDepth = depth
			}
			word = append(word, ch)
			continue
		}
		if ch >= '0' && ch <= '9' || ch == '@' {
			if len(word) > 0 {
				word = append(word, ch)
			}
			continue
		}
		flush()
		if ch == '(' {
			depth++
		} else if ch == ')' {
			if depth > 0 {
				depth--
			}
		}
	}
	flush()
	return tokens
}

// Classify 判定单条 SQL 的处置，语义移植自 taf-mysql-mcp/sql-policy.js
func Classify(sql string, allowWrite bool) Verdict {
	if strings.TrimSpace(sql) == "" {
		return rejectVerdict("SQL 必须是非空字符串")
	}
	if strings.Contains(sql, "/*!") {
		return rejectVerdict("禁止 MySQL 可执行注释 /*! ... */，它无法安全地静态判定")
	}
	masked := mask(sql)
	tokens := tokenize(masked)
	if len(tokens) == 0 {
		return rejectVerdict("未识别到任何 SQL 关键字")
	}
	trimmed := strings.TrimRightFunc(masked, unicode.IsSpace)
	if strings.HasSuffix(trimmed, ";") {
		trimmed = trimmed[:len(trimmed)-1]
	}
	if strings.Contains(trimmed, ";") {
		return rejectVerdict("只接受单条 SQL（结尾分号可省略）")
	}
	verb := resolveVerb(tokens)
	if verb == "WITH" {
		return rejectVerdict("WITH 之后未找到主语句")
	}
	if verb == "SELECT" {
		for idx, t := range tokens {
			if t.word == "INTO" && idx+1 < len(tokens) && intoFileRe.MatchString(tokens[idx+1].word) {
				return rejectVerdict("禁止 SELECT ... INTO OUTFILE/DUMPFILE，它会向数据库服务器写文件")
			}
		}
	}
	if readVerbs[verb] {
		return Verdict{Decision: DecisionReadonly, Verb: verb}
	}
	if writeVerbs[verb] {
		if !allowWrite {
			return rejectVerdict(fmt.Sprintf("%s 属于写操作，当前为只读模式。确认可写后在连接的 allow_write 勾选项中放开", verb))
		}
		return Verdict{Decision: DecisionWrite, Verb: verb}
	}
	return rejectVerdict(fmt.Sprintf("%s 不在允许范围内：DDL/权限/事务/存储过程类语句一律禁止", verb))
}

func resolveVerb(tokens []token) string {
	head := tokens[0].word
	if head != "WITH" {
		return head
	}
	for _, t := range tokens[1:] {
		if t.depth == 0 && (readVerbs[t.word] || writeVerbs[t.word]) {
			return t.word
		}
	}
	return "WITH"
}
