// TAF 线协议适配：tafgo 客户端用 BasePacket 布局（iVersion1/cPacketType2/iMessageType3/
// iRequestId4/iRet5/sServantName6/sFuncName7/sBuffer8/iTimeout9/sResultDesc10/context11/status12），
// 比开源 tarsgo 默认的 TARS RequestPacket 整体后移一位，直接发会石沉大海。
// 这里以 tarsgo 的 model.Protocol 注入 TAF 布局的组包/解包。
package ETG

import (
	"encoding/binary"

	"github.com/TarsCloud/TarsGo/tars/protocol"
	"github.com/TarsCloud/TarsGo/tars/protocol/codec"
	"github.com/TarsCloud/TarsGo/tars/protocol/res/requestf"
)

type tafProtocol struct{}

func (p tafProtocol) RequestPack(req *requestf.RequestPacket) ([]byte, error) {
	os := codec.NewBuffer()
	if err := os.WriteSliceInt8(make([]int8, 4)); err != nil {
		return nil, err
	}
	if err := os.WriteInt16(req.IVersion, 1); err != nil {
		return nil, err
	}
	if err := os.WriteInt8(req.CPacketType, 2); err != nil {
		return nil, err
	}
	if err := os.WriteInt32(0, 3); err != nil { // iMessageType
		return nil, err
	}
	if err := os.WriteInt32(req.IRequestId, 4); err != nil {
		return nil, err
	}
	if err := os.WriteInt32(0, 5); err != nil { // iRet 占位
		return nil, err
	}
	if err := os.WriteString(req.SServantName, 6); err != nil {
		return nil, err
	}
	if err := os.WriteString(req.SFuncName, 7); err != nil {
		return nil, err
	}
	if err := os.WriteHead(codec.SimpleList, 8); err != nil {
		return nil, err
	}
	if err := os.WriteHead(codec.BYTE, 0); err != nil {
		return nil, err
	}
	if err := os.WriteInt32(int32(len(req.SBuffer)), 0); err != nil {
		return nil, err
	}
	if err := os.WriteSliceInt8(req.SBuffer); err != nil {
		return nil, err
	}
	if err := os.WriteInt32(req.ITimeout, 9); err != nil {
		return nil, err
	}
	if err := os.WriteString("", 10); err != nil { // sResultDesc 占位，与 tafgo 字节一致
		return nil, err
	}
	if err := writeTafMap(os, 11, req.Context); err != nil {
		return nil, err
	}
	if err := writeTafMap(os, 12, req.Status); err != nil {
		return nil, err
	}

	bs := os.ToBytes()
	binary.BigEndian.PutUint32(bs, uint32(len(bs)))
	return bs, nil
}

func writeTafMap(os *codec.Buffer, tag byte, m map[string]string) error {
	if err := os.WriteHead(codec.MAP, tag); err != nil {
		return err
	}
	if err := os.WriteInt32(int32(len(m)), 0); err != nil {
		return err
	}
	for k, v := range m {
		if err := os.WriteString(k, 0); err != nil {
			return err
		}
		if err := os.WriteString(v, 1); err != nil {
			return err
		}
	}
	return nil
}

func (p tafProtocol) ResponseUnpack(pkg []byte) (*requestf.ResponsePacket, error) {
	rsp := &requestf.ResponsePacket{}
	is := codec.NewReader(pkg[4:])
	if err := is.ReadInt16(&rsp.IVersion, 1, false); err != nil {
		return nil, err
	}
	if err := is.ReadInt8(&rsp.CPacketType, 2, false); err != nil {
		return nil, err
	}
	if err := is.ReadInt32(&rsp.IMessageType, 3, false); err != nil {
		return nil, err
	}
	if err := is.ReadInt32(&rsp.IRequestId, 4, false); err != nil {
		return nil, err
	}
	if err := is.ReadInt32(&rsp.IRet, 5, false); err != nil {
		return nil, err
	}
	var servant, function string
	if err := is.ReadString(&servant, 6, false); err != nil {
		return nil, err
	}
	if err := is.ReadString(&function, 7, false); err != nil {
		return nil, err
	}
	// sBuffer: SimpleList 字段头 + BYTE 元素类型头 + int32 长度 + 裸数据
	if have, _, err := is.SkipToNoCheck(8, false); err != nil {
		return nil, err
	} else if have {
		if _, _, err := is.SkipToNoCheck(0, false); err != nil {
			return nil, err
		}
		var bufLen int32
		if err := is.ReadInt32(&bufLen, 0, false); err != nil {
			return nil, err
		}
		if err := is.ReadSliceInt8(&rsp.SBuffer, bufLen, false); err != nil {
			return nil, err
		}
	}
	var timeout int32
	if err := is.ReadInt32(&timeout, 9, false); err != nil {
		return nil, err
	}
	if err := is.ReadString(&rsp.SResultDesc, 10, false); err != nil {
		return nil, err
	}
	return rsp, nil
}

func (p tafProtocol) ParsePackage(rev []byte) (int, int) {
	return protocol.TarsRequest(rev)
}
