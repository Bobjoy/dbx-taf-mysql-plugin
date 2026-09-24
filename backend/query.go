package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type TafClient interface {
	Select(sql string) (int32, string, string, error)
}

type QueryOutput struct {
	Columns   []string `json:"columns"`
	Rows      [][]any  `json:"rows"`
	Total     int      `json:"total"`
	Truncated bool     `json:"truncated"`
	ElapsedMs int64    `json:"elapsedMs"`
	Note      string   `json:"note,omitempty"`
}

type selectOutcome struct {
	iRet   int32
	result string
	errMsg string
	err    error
}

func Execute(client TafClient, sql string, maxRows int, timeoutMs int64) (QueryOutput, error) {
	start := time.Now()
	done := make(chan selectOutcome, 1)
	go func() {
		iRet, result, errMsg, err := client.Select(sql)
		done <- selectOutcome{iRet, result, errMsg, err}
	}()

	var outcome selectOutcome
	select {
	case outcome = <-done:
	case <-time.After(time.Duration(timeoutMs) * time.Millisecond):
		return QueryOutput{}, fmt.Errorf("查询超时（%dms 未返回）。多半是 servant 地址或对象名不对，或当前网络到不了该环境", timeoutMs)
	}

	if outcome.err != nil {
		return QueryOutput{}, fmt.Errorf("TAF 调用失败：%v", outcome.err)
	}
	if outcome.iRet != 0 {
		return QueryOutput{}, fmt.Errorf("查询失败 iRet=%d|%s", outcome.iRet, outcome.errMsg)
	}

	var probe any
	if err := json.Unmarshal([]byte(outcome.result), &probe); err != nil {
		return QueryOutput{Note: fmt.Sprintf("执行完成，未返回结果集。原始返回：%s", outcome.result), ElapsedMs: ms(time.Since(start))}, nil
	}
	// 网关执行不了这条 SQL 时返回字面量 false
	if probe == false {
		return QueryOutput{}, fmt.Errorf("数据访问服务没有返回结果集：这条 SQL 网关执行不了。查表清单用 select table_name from information_schema.tables where table_schema = database()，查字段用 information_schema.columns")
	}
	if _, isArray := probe.([]any); !isArray {
		return QueryOutput{Note: fmt.Sprintf("执行完成，返回值：%s", outcome.result), ElapsedMs: ms(time.Since(start))}, nil
	}

	columns, rows, err := parseRows(outcome.result)
	if err != nil {
		return QueryOutput{}, fmt.Errorf("结果解析失败：%v", err)
	}
	out := QueryOutput{Columns: columns, Total: len(rows), ElapsedMs: ms(time.Since(start))}
	for idx, row := range rows {
		if idx >= maxRows {
			out.Truncated = true
			break
		}
		out.Rows = append(out.Rows, row)
	}
	return out, nil
}

func ms(d time.Duration) int64 { return int64(d / time.Millisecond) }

// parseRows 按 JSON 原文顺序解析列名（map 会丢键序，故用 token 流）
func parseRows(raw string) ([]string, [][]any, error) {
	dec := json.NewDecoder(strings.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		return nil, nil, err
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '[' {
		return nil, nil, fmt.Errorf("expect JSON array")
	}
	colIndex := map[string]int{}
	var columns []string
	var rows [][]any

	for dec.More() {
		var element json.RawMessage
		if err := dec.Decode(&element); err != nil {
			return nil, nil, err
		}
		row := make([]any, 0, len(columns)+1)
		if len(element) > 0 && element[0] == '{' {
			rowEd := json.NewDecoder(strings.NewReader(string(element)))
			if _, err := rowEd.Token(); err != nil {
				return nil, nil, err
			}
			for rowEd.More() {
				keyTok, err := rowEd.Token()
				if err != nil {
					return nil, nil, err
				}
				key, _ := keyTok.(string)
				var value any
				if err := rowEd.Decode(&value); err != nil {
					return nil, nil, err
				}
				idx, seen := colIndex[key]
				if !seen {
					idx = len(columns)
					colIndex[key] = idx
					columns = append(columns, key)
				}
				for len(row) <= idx {
					row = append(row, nil)
				}
				row[idx] = value
			}
		} else {
			var value any
			if err := json.Unmarshal(element, &value); err != nil {
				return nil, nil, err
			}
			idx, seen := colIndex["value"]
			if !seen {
				idx = len(columns)
				colIndex["value"] = idx
				columns = append(columns, "value")
			}
			for len(row) <= idx {
				row = append(row, nil)
			}
			row[idx] = value
		}
		rows = append(rows, row)
	}
	width := len(columns)
	for i := range rows {
		for len(rows[i]) < width {
			rows[i] = append(rows[i], nil)
		}
	}
	return columns, rows, nil
}
