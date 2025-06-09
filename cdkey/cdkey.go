package cdkey

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"ysj/jotools/filelog"
)

const (
	cdkeyLen       = 30 //cdkey总长度
	sigHeadNeedLen = 2  //sha1签名取前后的长度
	sigEndNeedLen  = 4  //sha1签名取前后的长度
	sha1SigLen     = 40 //sha1生成签名的总长度
	infoLen        = 18 //数据段实际长度    还有6字节被base64填充
)

var cdkeySalt = "wwwwwwww" //盐

func SetSalt(salt string) {
	cdkeySalt = salt
}

// ParseCDKey 解析cdk,返回相关数据
func ParseCDKey(key string) (ty, uid, itemId, num int, timestamp int64, errCode int32) {
	if len(key) != cdkeyLen {
		filelog.Error("cdkey length error:", key)
		errCode = 12
		return
	}
	// sig: [0:4]和[4:] 构成签名信息
	sig := key[:sigHeadNeedLen] + key[cdkeyLen-sigEndNeedLen:]
	// infos: [4:32-4] 由base64生成的数据组成
	infos := key[sigHeadNeedLen : cdkeyLen-sigEndNeedLen]
	h := sha1.New()
	_, err := io.WriteString(h, infos)
	if err != nil {
		errCode = 13
		filelog.Error("WriteString err:", err)
		return
	}
	_, err = io.WriteString(h, cdkeySalt)
	if err != nil {
		errCode = 14
		filelog.Error("WriteString err:", err)
		return
	}
	cSha1key := fmt.Sprintf("%x", h.Sum(nil))
	if cSha1key[:sigHeadNeedLen]+cSha1key[sha1SigLen-sigEndNeedLen:] != sig {
		errCode = 15
		return
	}
	// wKey:  长度为 18,其中 [0:3] 保留,  [3]==ty, [4:8]==uid, [9:13]==itemId,[13:15]==num [15:19]==timestamp
	wKey, err := base64.StdEncoding.DecodeString(infos)
	if err != nil {
		errCode = 16
		filelog.Error("DecodeString infos err:", err)
		return
	}

	var start, end = 0, 3
	_, _, _ = wKey[0], wKey[1], wKey[2] //留空的数据段
	start, end = end, end+1
	ty = int(wKey[start])
	start, end = end, end+4
	uid = int(binary.LittleEndian.Uint32(wKey[start:end]))
	start, end = end, end+4
	itemId = int(binary.LittleEndian.Uint32(wKey[start:end]))
	start, end = end, end+2
	num = int(binary.LittleEndian.Uint16(wKey[start:end]))
	start, end = end, end+4
	timestamp = int64(binary.LittleEndian.Uint32(wKey[start:end]))
	return
}

// CreateCDKEY @param: uidInterval 唯一id的开始和结束区间
// @param data 字段自行填充,长度最大为3
func CreateCDKEY(uidInterval [2]uint32, ty uint8, itemId int32, num int16, timestamp uint32, data ...byte) []string {
	var wRetKey []string
	for uid := uidInterval[0]; uid < uidInterval[1]; uid++ {
		// wKey:  长度为 18,其中 [0:3] 保留,  [3]==ty, [4:8]==uid, [9:13]==itemId,[13:15]==num [15:19]==timestamp
		var wKey = make([]byte, infoLen)
		var start, end = 0, 3
		for k, v := range data {
			if k >= 3 {
				break
			}
			wKey[k] = v //留空
		}

		start, end = end, end+1
		wKey[start] = ty
		start, end = end, end+4
		binary.LittleEndian.PutUint32(wKey[start:end], uid)
		start, end = end, end+4
		binary.LittleEndian.PutUint32(wKey[start:end], uint32(itemId))
		start, end = end, end+2
		binary.LittleEndian.PutUint16(wKey[start:end], uint16(num))
		start, end = end, end+4
		binary.LittleEndian.PutUint32(wKey[start:end], uint32(timestamp))

		// infos: [4:32-4] 由base64生成的数据组成
		infos := base64.StdEncoding.EncodeToString(wKey)
		h := sha1.New()
		io.WriteString(h, infos)
		io.WriteString(h, cdkeySalt)

		// sig: [0:4]和[4:] 构成签名信息
		sig := fmt.Sprintf("%x", h.Sum(nil))

		//fmt.Printf("uid:%d,ty:%d,itemId:%d,num:%d,cSha1key:%s,le:%d\r\n", uid, ty, itemId, num, sig, len(sig))
		wRetKey = append(wRetKey, sig[:sigHeadNeedLen]+infos+sig[sha1SigLen-sigEndNeedLen:])
	}

	return wRetKey
}
