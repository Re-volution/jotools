package net

import (
	"encoding/binary"
	"github.com/Re-volution/jotools/chantypes"
	"github.com/Re-volution/jotools/dencode"
	"github.com/Re-volution/jotools/filelog"
	"google.golang.org/protobuf/proto"
)

const MaxLength = 4
const ProtoLen = 2
const maxPayload = 200000

func parseData(msg interface{}, protoid uint16) []byte {
	data, ok := marshalBody(msg, false)
	if !ok {
		return nil
	}
	if len(data) > maxPayload {
		filelog.Error("send data too big:", len(data))
		return nil
	}

	totlelen := uint32(MaxLength + ProtoLen + len(data))
	wdata := make([]byte, 0, totlelen)
	wdata = binary.BigEndian.AppendUint32(wdata, totlelen)
	wdata = binary.BigEndian.AppendUint16(wdata, protoid)
	if len(data) > 0 {
		wdata = append(wdata, data...)
	}
	return wdata
}

func parseWsData(msg interface{}, protoid uint16) []byte {
	data, ok := marshalBody(msg, true)
	if !ok {
		return nil
	}
	if len(data) > maxPayload {
		filelog.Error("send data too big:", len(data))
		return nil
	}

	wdata := make([]byte, 0, ProtoLen+len(data))
	wdata = binary.BigEndian.AppendUint16(wdata, protoid)
	if len(data) > 0 {
		wdata = append(wdata, data...)
	}
	return wdata
}

func marshalBody(msg interface{}, ws bool) ([]byte, bool) {
	if msg == nil {
		return nil, true
	}
	if ws {
		pm, ok := msg.(proto.Message)
		if !ok {
			filelog.Error("ws msg is not proto.Message")
			return nil, false
		}
		data, err := dencode.MarshalProto(pm)
		if err != nil {
			filelog.Error("marshal proto err:", err)
			return nil, false
		}
		filelog.Debug("server send:", msg)
		return data, true
	}
	data, err := dencode.Marshal(msg)
	if err != nil {
		filelog.Error("marshal err:", err)
		return nil, false
	}
	filelog.Debug("server send:", string(data))
	return data, true
}

// ws发送给指定玩家
func (netM *NetManger) WsSendById(msg proto.Message, id int64, protoid uint16) {
	conn := netM.GetConnByCid(id)
	if conn == nil {
		filelog.Warring("not found this nid:", id)
		return
	}
	conn.SendMsg(msg, protoid)
}

// ws发送给指定玩家组
func (netM *NetManger) WsSendByIds(msg proto.Message, ids []int64, protoid uint16) {
	var wdata []byte
	for _, id := range ids {
		conn := netM.GetConnByCid(id)
		if conn == nil {
			filelog.Warring("not found this nid:", id)
			continue
		}
		if len(wdata) == 0 {
			wdata = conn.parseH(msg, protoid)
			if len(wdata) == 0 {
				return
			}
		}
		chantypes.TryWriteChan(conn.SendC, wdata, true)
	}
	filelog.Debug("WsSendById ids：", ids, " protoid:", protoid, " len:", len(wdata))
}

// tcp发送给指定玩家
func (netM *NetManger) SendById(msg interface{}, id int64, protoid uint16) {
	conn := netM.GetConnByCid(id)
	if conn == nil {
		filelog.Warring("not found this nid:", id)
		return
	}

	conn.SendMsg(msg, protoid)
}

func (nc *NetC) SendMsg(msg interface{}, protoid uint16) {
	if nc == nil || nc.parseH == nil {
		filelog.Error("send on nil conn, protoid:", protoid)
		return
	}
	if nc.off.Load() {
		return
	}
	wdata := nc.parseH(msg, protoid)
	if len(wdata) == 0 {
		return
	}
	filelog.Debug("SendMsg id：", nc.Id, " protoid:", protoid)
	chantypes.TryWriteChan(nc.SendC, wdata, true)
}

// 随机发送
func (netM *NetManger) SendRandC(msg interface{}, protoid uint16) {
	var conn *NetC
	netM.conn.Range(func(key, value interface{}) bool {
		conn = value.(*NetC)
		return false
	})
	if conn == nil {
		filelog.Error("无可用连接", msg, protoid)
		return
	}
	wdata := conn.parseH(msg, protoid)
	if len(wdata) == 0 {
		return
	}
	chantypes.TryWriteChan(conn.SendC, wdata, true)
	filelog.Debug("SendMsg 消息发送成功 protoid：", protoid, " wdata:", len(wdata))
}

func (cm *ConnManger) SendMsg(msg interface{}, protoid uint16) {
	if cm == nil {
		filelog.Error("conn is nil:", protoid)
		return
	}
	conn := cm.current()
	if conn == nil {
		filelog.Error("conn is nil:", protoid)
		return
	}
	conn.SendMsg(msg, protoid)
}

func (netM *NetManger) SendAllConn(msg interface{}, protoid uint16) {
	var conns []*NetC
	netM.conn.Range(func(key, value interface{}) bool {
		conns = append(conns, value.(*NetC))
		return true
	})
	if len(conns) == 0 {
		return
	}
	wdata := conns[0].parseH(msg, protoid)
	if len(wdata) == 0 {
		return
	}
	for _, conn := range conns {
		chantypes.TryWriteChan(conn.SendC, wdata, true)
	}
	filelog.Debug("SendAllConn protoid:", protoid)
}

// 关闭
func (netM *NetManger) Close(nid int64) {
	conn := netM.getConn(nid)
	if conn != nil {
		conn.Close()
	}
}

// 获取链接
func (netM *NetManger) GetConnByCid(cid int64) *NetC {
	return netM.getConn(cid)
}
