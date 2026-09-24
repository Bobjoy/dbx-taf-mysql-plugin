package main

import (
	"encoding/json"
	"log"
	"regexp"
	"strconv"
	"sync"

	"dbxtaf/plugin/ETG"

	tars "github.com/TarsCloud/TarsGo/tars"
	dbxpluginsdk "github.com/t8y2/dbx/plugins/sdk/go/dbx-plugin-sdk"
)

const (
	pluginID      = "com.bao.taf-mysql"
	pluginVersion = "0.1.0"
	maxRows       = 200
	timeoutMs     = 30000
)

type connectionState struct {
	client     TafClient
	allowWrite bool
}

type plugin struct {
	mutex     sync.Mutex
	clients   map[string]*connectionState
	newClient func(servant string) (TafClient, error)
}

type tafClient struct{ proxy *ETG.TgDataAsync }

func (c *tafClient) Select(sql string) (int32, string, string, error) {
	req := &ETG.SelectReq{Sql: sql, Param: "[]", Option: `{"skipSqlCheck":true}`}
	rsp := &ETG.SelectRsp{}
	if _, err := c.proxy.Select(req, rsp); err != nil {
		return 0, "", "", err
	}
	return rsp.IRet, rsp.Result, rsp.Error, nil
}

func newPlugin() *plugin {
	communicator := tars.NewCommunicator()
	return &plugin{
		clients: map[string]*connectionState{},
		newClient: func(servant string) (TafClient, error) {
			proxy := new(ETG.TgDataAsync)
			communicator.StringToProxy(servant, proxy)
			proxy.SetTimeout(servantTimeout(servant))
			return &tafClient{proxy: proxy}, nil
		},
	}
}

var servantTimeoutRe = regexp.MustCompile(`-t\s+(\d+)`)

// servant 串里 -t 参数优先，缺省 60s（与 tafgo 行为一致）
func servantTimeout(servant string) int {
	if m := servantTimeoutRe.FindStringSubmatch(servant); m != nil {
		if ms, err := strconv.Atoi(m[1]); err == nil && ms > 0 {
			return ms
		}
	}
	return 60000
}

type connPayload struct {
	Connection struct {
		ID             string `json:"id"`
		ExternalConfig struct {
			Servant    string `json:"servant"`
			AllowWrite bool   `json:"allow_write"`
		} `json:"external_config"`
	} `json:"connection"`
}

type queryPayload struct {
	ConnectionID string `json:"connectionId"`
	SQL          string `json:"sql"`
}

func (p *plugin) Handle(
	_ dbxpluginsdk.RequestContext,
	method string,
	params json.RawMessage,
	_ *dbxpluginsdk.Emitter,
) (any, *dbxpluginsdk.PluginError) {
	switch method {
	case "connection/test":
		return p.testConnection(params)
	case "connection/connect":
		return p.connect(params)
	case "connection/disconnect":
		return p.disconnect(params)
	case "taf-mysql/query":
		return p.query(params)
	case "taf-mysql/meta":
		return p.meta(params)
	default:
		return nil, dbxpluginsdk.MethodNotFound(method)
	}
}

func parseConn(params json.RawMessage) (*connPayload, *dbxpluginsdk.PluginError) {
	var payload connPayload
	if err := json.Unmarshal(params, &payload); err != nil {
		return nil, dbxpluginsdk.NewError(-32602, "Invalid request parameters")
	}
	return &payload, nil
}

func (p *plugin) testConnection(params json.RawMessage) (any, *dbxpluginsdk.PluginError) {
	payload, parseErr := parseConn(params)
	if parseErr != nil {
		return nil, parseErr
	}
	servant := payload.Connection.ExternalConfig.Servant
	if servant == "" {
		return map[string]any{"success": false, "message": "servant 未填写"}, nil
	}
	client, err := p.newClient(servant)
	if err != nil {
		return map[string]any{"success": false, "message": err.Error()}, nil
	}
	if _, execErr := Execute(client, "select 1", maxRows, timeoutMs); execErr != nil {
		return map[string]any{"success": false, "message": execErr.Error()}, nil
	}
	return map[string]any{"success": true, "message": "TAF 连接可用"}, nil
}

func (p *plugin) connect(params json.RawMessage) (any, *dbxpluginsdk.PluginError) {
	payload, parseErr := parseConn(params)
	if parseErr != nil {
		return nil, parseErr
	}
	config := payload.Connection.ExternalConfig
	if config.Servant == "" {
		return nil, dbxpluginsdk.NewError(-32602, "servant 未填写")
	}
	client, err := p.newClient(config.Servant)
	if err != nil {
		return nil, dbxpluginsdk.NewError(-32000, err.Error())
	}
	p.mutex.Lock()
	p.clients[payload.Connection.ID] = &connectionState{client: client, allowWrite: config.AllowWrite}
	p.mutex.Unlock()
	return map[string]any{"success": true}, nil
}

func (p *plugin) disconnect(params json.RawMessage) (any, *dbxpluginsdk.PluginError) {
	payload, parseErr := parseConn(params)
	if parseErr != nil {
		return nil, parseErr
	}
	p.mutex.Lock()
	delete(p.clients, payload.Connection.ID)
	p.mutex.Unlock()
	return map[string]any{"success": true}, nil
}

func (p *plugin) query(params json.RawMessage) (any, *dbxpluginsdk.PluginError) {
	var payload queryPayload
	if err := json.Unmarshal(params, &payload); err != nil {
		return nil, dbxpluginsdk.NewError(-32602, "Invalid request parameters")
	}
	p.mutex.Lock()
	state, ok := p.clients[payload.ConnectionID]
	p.mutex.Unlock()
	if !ok {
		return nil, dbxpluginsdk.NewError(-32000, "连接未建立，请先在 dbx 中连接该 TAF MySQL 连接")
	}
	verdict := Classify(payload.SQL, state.allowWrite)
	if verdict.Decision == DecisionReject {
		return nil, dbxpluginsdk.NewError(-32000, "已拒绝："+verdict.Reason)
	}
	out, err := Execute(state.client, payload.SQL, maxRows, timeoutMs)
	if err != nil {
		return nil, dbxpluginsdk.NewError(-32000, err.Error())
	}
	return out, nil
}

func (p *plugin) meta(params json.RawMessage) (any, *dbxpluginsdk.PluginError) {
	var payload struct {
		ConnectionID string `json:"connectionId"`
		Kind         string `json:"kind"`
		Table        string `json:"table"`
	}
	if err := json.Unmarshal(params, &payload); err != nil {
		return nil, dbxpluginsdk.NewError(-32602, "Invalid request parameters")
	}
	p.mutex.Lock()
	state, ok := p.clients[payload.ConnectionID]
	p.mutex.Unlock()
	if !ok {
		return nil, dbxpluginsdk.NewError(-32000, "连接未建立，请先在 dbx 中连接该 TAF MySQL 连接")
	}
	nodes, err := Meta(state.client, payload.Kind, payload.Table, timeoutMs)
	if err != nil {
		return nil, dbxpluginsdk.NewError(-32000, err.Error())
	}
	return nodes, nil
}

func main() {
	metadata := dbxpluginsdk.Metadata{
		ID:           pluginID,
		Version:      pluginVersion,
		Capabilities: []string{"connections"},
	}
	server := dbxpluginsdk.NewServer(metadata, newPlugin())
	if err := server.Serve(); err != nil {
		log.Fatal(err)
	}
}
