package net

import (
	"encoding/binary"
	"fmt"
	"google.golang.org/protobuf/proto"
	"ysj/jotools/chantypes"
	"ysj/jotools/dencode"
	"ysj/jotools/filelog"
)

const MaxLength = 4
const ProtoLen = 2
const WSIncLen = 2

func parseData(msg interface{}, protoid uint16) []byte {
	var data []byte
	if msg != nil {
		data, _ = dencode.Marshal(msg)
		filelog.Debug("server send:", string(data))
	}

	var totlelen = MaxLength + ProtoLen + uint32(len(data))
	var wdata = make([]byte, 0, totlelen)

	wdata = binary.BigEndian.AppendUint32(wdata, totlelen)
	wdata = binary.BigEndian.AppendUint16(wdata, protoid)
	if len(data) > 0 {
		wdata = append(wdata, data...)
	}
	//filelog.Debug("parseData len:", totlelen, " protoid:", protoid, " data:", string(data))
	return wdata
}

func parseWsData(msg interface{}, protoid uint16) []byte {
	var data []byte
	if msg != nil {
		data, _ = dencode.MarshalProto(msg.(proto.Message))
		filelog.Debug("server send:", msg)
	}

	var totlelen = ProtoLen + uint32(len(data))
	var wdata = make([]byte, 0, totlelen)

	wdata = binary.BigEndian.AppendUint16(wdata, protoid)
	if len(data) > 0 {
		wdata = append(wdata, data...)
	}
	//filelog.Debug("parseData len:", totlelen, " protoid:", protoid, " data:", string(data))
	return wdata
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
	var wdata = nc.parseH(msg, protoid)
	filelog.Debug("SendMsg id：", nc.Id, " protoid:", protoid)
	chantypes.TryWriteChan(nc.SendC, wdata, true)
}

// 随机发送
func (netM *NetManger) SendRandC(msg interface{}, protoid uint16) {
	var conn *NetC
	netM.cLock.RLock()
	for _, v := range netM.conn {
		conn = v
		break
	}

	netM.cLock.RUnlock()
	if conn == nil {
		filelog.Error("无可用连接", msg, protoid)
		return
	}
	wdata := conn.parseH(msg, protoid)

	chantypes.TryWriteChan(conn.SendC, wdata, true)
	filelog.Debug("SendMsg 消息发送成功 protoid：", protoid, " wdata:", len(wdata))
}

func (cm *ConnManger) SendMsg(msg interface{}, protoid uint16) {
	var wdata = cm.NetC.parseH(msg, protoid)
	var conn = cm.NetC
	if conn == nil {
		filelog.Error("conn is nil:", string(wdata), protoid)
		return
	}
	chantypes.TryWriteChan(conn.SendC, wdata, true)
	filelog.Debug("SendMsg 消息发送成功 protoid：", protoid, " wdata:", len(wdata))
}

func (netM *NetManger) SendAllConn(msg interface{}, protoid uint16) {
	var conns []*NetC
	netM.cLock.RLock()
	for _, conn := range netM.conn {
		conns = append(conns, conn)
	}
	netM.cLock.RUnlock()
	if len(conns) == 0 {
		return
	}
	wdata := conns[0].parseH(msg, protoid)
	for _, conn := range conns {
		chantypes.TryWriteChan(conn.SendC, wdata, true)
	}
	filelog.Debug("SendAllConn protoid:", protoid)
}

// 关闭
func (netM *NetManger) Close(nid int64) {
	netM.cLock.RLock()
	conn := netM.conn[nid]
	netM.cLock.RUnlock()
	conn.Close()
}

// 获取链接
func (netM *NetManger) GetConnByCid(cid int64) *NetC {
	return netM.getConn(cid)
}

func (nc *NetC) Close() {
	if nc == nil {
		return
	}
	nc.close()
}

func (netM *NetManger) String() string {
	netM.cLock.RLock()
	defer netM.cLock.RUnlock()
	return fmt.Sprintf("conn totle:%d,is ws:%t", len(netM.conn), netM.ws)
}
