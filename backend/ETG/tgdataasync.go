// Package ETG 是 TgDataAsync 的客户端协议桩，基于开源 tarsgo 手写。
// 线上 jce2go 生成码依赖内网 tafgo 独有 API（JceStructBase/precision/wup），
// tarsgo v1.4.6 无对应物；且本插件只用 select 一个接口，故只保留手写最小实现。
package ETG

import (
	"context"
	"fmt"

	"github.com/TarsCloud/TarsGo/tars/model"
	"github.com/TarsCloud/TarsGo/tars/protocol/codec"
	"github.com/TarsCloud/TarsGo/tars/protocol/res/requestf"
	"github.com/TarsCloud/TarsGo/tars/util/tools"
)

type SelectReq struct {
	Sql    string
	Param  string
	Option string
}

type SelectRsp struct {
	IRet   int32
	Result string
	Error  string
}

type TgDataAsync struct {
	servant model.Servant
}

// SetServant 实现 tars.ProxyPrx，由 Communicator.StringToProxy 注入；
// 同时挂上 TAF 线协议（tarsgo 默认按 TARS 布局发包，TAF 网关不认）。
func (c *TgDataAsync) SetServant(s model.Servant) {
	s.TarsSetProtocol(tafProtocol{})
	c.servant = s
}

// SetTimeout 覆盖调用超时（毫秒）。
func (c *TgDataAsync) SetTimeout(ms int) {
	if c.servant != nil {
		c.servant.TarsSetTimeout(ms)
	}
}

// Select 调用网关 select 接口。DML 也走这里（Option 里带 skipSqlCheck）。
func (c *TgDataAsync) Select(req *SelectReq, rsp *SelectRsp) (int32, error) {
	if c.servant == nil {
		return 0, fmt.Errorf("代理未绑定 servant，请先 StringToProxy")
	}

	out := codec.NewBuffer()
	out.WriteHead(codec.StructBegin, 1)
	if err := out.WriteString(req.Sql, 0); err != nil {
		return 0, err
	}
	if err := out.WriteString(req.Param, 1); err != nil {
		return 0, err
	}
	if err := out.WriteString(req.Option, 2); err != nil {
		return 0, err
	}
	out.WriteHead(codec.StructEnd, 0)
	// 出参占位（tag 2 零值结构），与生成码行为保持一致
	out.WriteHead(codec.StructBegin, 2)
	out.WriteInt32(0, 0)
	out.WriteString("", 1)
	out.WriteString("", 2)
	out.WriteHead(codec.StructEnd, 0)

	resp := new(requestf.ResponsePacket)
	if err := c.servant.TarsInvoke(context.Background(), 0, "select", out.ToBytes(), nil, nil, resp); err != nil {
		return 0, err
	}
	if resp.IRet != 0 {
		return 0, fmt.Errorf("TARS 包级错误 iRet=%d：%s", resp.IRet, resp.SResultDesc)
	}

	in := codec.NewReader(tools.Int8ToByte(resp.SBuffer))
	var ret int32
	if err := in.ReadInt32(&ret, 0, true); err != nil {
		return 0, fmt.Errorf("解析返回值失败：%w", err)
	}
	found, err := in.SkipTo(codec.StructBegin, 2, false)
	if err != nil {
		return ret, err
	}
	if found {
		if err := in.ReadInt32(&rsp.IRet, 0, false); err != nil {
			return ret, err
		}
		if err := in.ReadString(&rsp.Result, 1, false); err != nil {
			return ret, err
		}
		if err := in.ReadString(&rsp.Error, 2, false); err != nil {
			return ret, err
		}
		if err := in.SkipToStructEnd(); err != nil {
			return ret, err
		}
	}
	return ret, nil
}
