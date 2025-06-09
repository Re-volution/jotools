package redis

import (
	"fmt"
	"github.com/go-redis/redis"
	"strconv"
	"strings"
	"time"
	"ysj/jotools/filelog"
)

var rdb *redis.Client

var addr_url = "localhost:6379"

// InitRedisClient ip:port#password
func InitRedisClient(addr string) error {
	data := strings.Split(addr, "#")

	addr_url = addr
	password := ""
	if len(data) > 1 {
		password = data[1]
	}
	rdb = redis.NewClient(&redis.Options{
		Addr:        data[0],
		Password:    password,
		DB:          0,
		ReadTimeout: time.Second * 5},
	)
	// ping一下，看能不能ping通
	_, err := rdb.Ping().Result()

	if err != nil {
		return err
	}
	fmt.Println("redis连接成功")
	return nil
}
func check() {
	_, err := rdb.Ping().Result()
	if err != nil {
		InitRedisClient(addr_url)
	}
}

func GetInt(dbname string) (int, error) {
	n, e := rdb.Get(dbname).Int()
	if e != nil && e.Error() != "redis: nil" {
		check()
		return n, e
	}
	return n, nil
}

func GetInt64(dbname string) (int64, error) {
	n, e := rdb.Get(dbname).Int64()
	if e != nil && e.Error() != "redis: nil" {
		check()
		return n, e
	}
	return n, nil
}

func GetString(dbname string) (string, error) {
	n, e := rdb.Get(dbname).Result()
	if e != nil && e.Error() != "redis: nil" {
		check()
		return n, e
	}
	return n, nil
}

func GetStringExist(dbname string) (string, bool, error) {
	n, e := rdb.Get(dbname).Result()
	if e != nil {
		if e.Error() == "redis: nil" {
			return "", false, nil
		}
		check()
		return n, false, e
	}
	return n, true, nil
}

func HGetInt(dbname string, k string) (int, error) {
	n, e := rdb.HGet(dbname, k).Int()
	if e != nil && e.Error() != "redis: nil" {
		check()
		return n, e
	}
	return n, nil
}

func HGetInt64(dbname string, k string) (int64, error) {
	n, e := rdb.HGet(dbname, k).Int64()
	if e != nil && e.Error() != "redis: nil" {
		check()
		return n, e
	}
	return n, nil
}
func HGetFloat(dbname string, k string) (float64, error) {
	n, e := rdb.HGet(dbname, k).Float64()
	if e != nil && e.Error() != "redis: nil" {
		check()
		return n, e
	}
	return n, nil
}

func HGetString(dbname string, k string) (string, error) {
	n, e := rdb.HGet(dbname, k).Result()
	if e != nil && e.Error() != "redis: nil" {
		check()
		return n, e
	}
	return n, nil
}

func HDel(dbname string, k ...string) error {
	e := rdb.HDel(dbname, k...).Err()
	if e != nil && e.Error() != "redis: nil" {
		check()
		return e
	}
	return nil
}

func HMGet(dbname string, fields ...string) ([]interface{}, error) {
	n, e := rdb.HMGet(dbname, fields...).Result()
	if e != nil && e.Error() != "redis: nil" {
		check()
		return n, e
	}
	return n, nil
}
func HGetAllSS(dbname string) (map[string]string, error) {
	n, e := rdb.HGetAll(dbname).Result()
	if e != nil && e.Error() != "redis: nil" {
		check()
		n, e = rdb.HGetAll(dbname).Result()
		if e != nil && e.Error() != "redis: nil" {
			return nil, e
		}
	}

	return n, nil
}

func HGetAllIS(dbname string) (map[int]string, error) {
	n, e := rdb.HGetAll(dbname).Result()
	if e != nil && e.Error() != "redis: nil" {
		check()
		n, e = rdb.HGetAll(dbname).Result()
		if e != nil && e.Error() != "redis: nil" {
			return nil, e
		}
	}
	var rt = make(map[int]string)
	for k, v := range n {
		key, _ := strconv.Atoi(k)
		rt[key] = v
	}

	return rt, nil
}

func HGetAllIntInt(dbname string) (map[int]int, error) {
	n, e := rdb.HGetAll(dbname).Result()
	if e != nil && e.Error() != "redis: nil" {
		check()
		n, e = rdb.HGetAll(dbname).Result()
		if e != nil && e.Error() != "redis: nil" {
			return nil, e
		}
	}
	var rt = make(map[int]int)
	for k, v := range n {
		key, _ := strconv.Atoi(k)
		val, _ := strconv.Atoi(v)
		rt[key] = val
	}
	return rt, nil
}

func HGetAllIntFloat(dbname string) (map[int]float64, error) {
	n, e := rdb.HGetAll(dbname).Result()
	if e != nil && e.Error() != "redis: nil" {
		check()
		n, e = rdb.HGetAll(dbname).Result()
		if e != nil && e.Error() != "redis: nil" {
			return nil, e
		}
	}
	var rt = make(map[int]float64)
	for k, v := range n {
		key, _ := strconv.Atoi(k)
		val, _ := strconv.ParseFloat(v, 64)
		rt[key] = val
	}
	return rt, nil
}

func HGetAllValS(dbname string) ([]string, error) {
	n, e := rdb.HVals(dbname).Result()
	if e != nil && e.Error() != "redis: nil" {
		check()
		n, e = rdb.HVals(dbname).Result()
		if e != nil && e.Error() != "redis: nil" {
			return nil, e
		}
	}

	return n, nil
}

func HMGetInt(dbname string, fields ...string) (map[string]int, error) {
	n, e := HMGet(dbname, fields...)
	if e != nil {
		return nil, e
	}
	var res = make(map[string]int)
	for k, v := range n {
		if v == nil {
			continue
		}
		str, _ := v.(string)
		number, err := strconv.Atoi(str)
		if err != nil {
			filelog.Error("string to int errorCode,dbname:", dbname, " k:", k, " v:", v)
			continue
		}
		res[fields[k]] = number
	}

	return res, e
}

func HMGetInt32(dbname string, fields ...string) (map[string]int32, error) {
	n, e := HMGet(dbname, fields...)
	if e != nil {
		return nil, e
	}
	var res = make(map[string]int32)
	for k, v := range n {
		if v == nil {
			continue
		}
		str, _ := v.(string)
		number, err := strconv.Atoi(str)
		if err != nil {
			filelog.Error("string to int errorCode,dbname:", dbname, " k:", k, " v:", v)
			continue
		}
		res[fields[k]] = int32(number)
	}

	return res, e
}

func HMGetInt64(dbname string, fields ...string) (map[string]int64, error) {
	n, e := HMGet(dbname, fields...)
	if e != nil {
		return nil, e
	}
	var res = make(map[string]int64)
	for k, v := range n {
		if v == nil {
			continue
		}
		str, _ := v.(string)
		number, err := strconv.Atoi(str)
		if err != nil {
			filelog.Error("string to int errorCode,dbname:", dbname, " k:", k, " v:", v)
			continue
		}
		res[fields[k]] = int64(number)
	}

	return res, e
}

func HMGetString(dbname string, fields ...string) (map[string]string, error) {
	n, e := HMGet(dbname, fields...)
	if e != nil {
		return nil, e
	}
	var res = make(map[string]string)
	for k, v := range n {
		res[fields[k]], _ = v.(string)
	}

	return res, e
}

func HKeys(dbname string) ([]string, error) {
	data, e := rdb.HKeys(dbname).Result()
	if e != nil && e.Error() != "redis: nil" {
		check()
		return data, e
	}
	return data, e
}

func HKeysToInt(dbname string) ([]int, error) {
	var ret []int
	data, e := HKeys(dbname)
	for _, v := range data {
		n, e := strconv.Atoi(v)
		if e != nil {
			return nil, e
		}
		ret = append(ret, n)
	}
	return ret, e
}

func Set(key string, value interface{}, expiration time.Duration) {
	if err := rdb.Set(key, value, expiration).Err(); err != nil {
		check()
		err = rdb.Set(key, value, expiration).Err()
		if err != nil {
			fmt.Println("redis Set errorCode:", err, key, value)
		}
	}
}

func HSet(key string, filed string, value interface{}) error {
	err := rdb.HSet(key, filed, value).Err()
	if err != nil {
		check()
		err = rdb.HSet(key, filed, value).Err()
		if err != nil {
			fmt.Println("redis HSet errorCode:", err, key, filed, value)
		}
	}
	return err
}

func HSetNX(key string, filed string, value interface{}) (bool, error) {
	bo, err := rdb.HSetNX(key, filed, value).Result()
	if err != nil {
		check()
		bo, err = rdb.HSetNX(key, filed, value).Result()
		if err != nil {
			fmt.Println("redis HSetNx errorCode:", err, key, filed, value)
		}
	}
	return bo, err
}

func HMSet(key string, v map[string]interface{}) {
	if err := rdb.HMSet(key, v).Err(); err != nil {
		check()
		err = rdb.HMSet(key, v).Err()
		if err != nil {
			fmt.Println("redis HMSet errorCode:", err, key, v)
		}
	}
}

func KeysStrins(key string) ([]string, error) {
	rt, err := rdb.Keys(key).Result()
	if err != nil {
		check()
		rt, err = rdb.Keys(key).Result()
		if err != nil {
			fmt.Println("redis Keys errorCode:", err, key)
		}
	}
	return rt, err
}

func Del(key string) {
	if err := rdb.Del(key).Err(); err != nil {
		check()
		err = rdb.Del(key).Err()
		if err != nil {
			fmt.Println("redis Del errorCode:", err, key)
		}
	}
}

func Inc(k string, i int64) (int64, error) {
	num, err := rdb.IncrBy(k, i).Result()
	if err != nil {
		check()
		err = rdb.IncrBy(k, i).Err()
		if err != nil {
			fmt.Println("redis IncrBy errorCode:", err, k, i)
			return 0, err
		}

	}
	return num, nil
}

func HInc(k, f string, i int64) (int64, error) {
	num, err := rdb.HIncrBy(k, f, i).Result()
	if err != nil {
		check()
		err = rdb.HIncrBy(k, f, i).Err()
		if err != nil {
			fmt.Println("redis HInc errorCode:", err, k, f, i)
			return 0, err
		}

	}
	return num, nil
}
func HIncFloat(k, f string, i float64) (float64, error) {
	num, err := rdb.HIncrByFloat(k, f, i).Result()
	if err != nil {
		check()
		err = rdb.HIncrByFloat(k, f, i).Err()
		if err != nil {
			fmt.Println("redis HIncFloat errorCode:", err, k, f, i)
			return 0, err
		}

	}
	return num, nil
}

func SMEMBERS(name string) []string {
	data, err := rdb.SMembers(name).Result()
	if err != nil {
		check()
		data, err = rdb.SMembers(name).Result()
		if err != nil {
			fmt.Println("redis SMembers errorCode:", err, name)
			return nil
		}
	}

	return data
}

func SAdd(name, k string, ot time.Duration) bool {
	n, err := rdb.SAdd(name, k).Result()
	if err != nil {
		check()
		n, err = rdb.SAdd(name, k).Result()
		if err != nil {
			fmt.Println("redis SAdd errorCode:", err, name, k)
			return true
		}
	}
	if ot != 0 {
		Expire(name, ot)
	}

	return n == 1
}

func ZAdd(key string, openid string, score int64) (int64, error) {
	var t = redis.Z{
		Score:  float64(score),
		Member: openid,
	}
	jf, err := rdb.ZAdd(key, t).Result()
	if err != nil {
		check()
		jf, err = rdb.ZAdd(key, t).Result()
		if err != nil {
			fmt.Println("redis ZAdd errorCode:", err, key, float64(score), openid)
			return 0, err
		}
	}
	return int64(jf), err
}

func ZAdds(key string, t ...redis.Z) error {
	_, err := rdb.ZAdd(key, t...).Result()
	if err != nil {
		check()
		_, err = rdb.ZAdd(key, t...).Result()
		if err != nil {
			fmt.Println("redis ZAdd errorCode:", err, key, t)
			return err
		}
	}
	return err
}

func ZINCRBY(key string, openid string, score int64) (float64, error) {
	jf, err := rdb.ZIncrBy(key, float64(score), openid).Result()
	if err != nil {
		check()
		jf, err = rdb.ZIncrBy(key, float64(score), openid).Result()
		if err != nil {
			fmt.Println("redis ZIncrBy errorCode:", err, key, float64(score), openid)
			return 0, err
		}
	}
	return jf, nil
}

func ZRevRank(key string, openid string) (int64, error) {
	mc, err := rdb.ZRevRank(key, openid).Result()
	if err != nil {
		if err.Error() == "redis: nil" {
			return -1, nil
		}
		check()
		mc, err = rdb.ZRevRank(key, openid).Result()
		if err != nil {
			fmt.Println("redis ZRevRank errorCode:", err, key, openid)
			return -1, err
		}
	}
	return mc, nil
}

func ZScore(key string, openid string) (float64, error) {
	mc, err := rdb.ZScore(key, openid).Result()
	if err != nil && err.Error() != "redis: nil" {
		check()
		mc, err = rdb.ZScore(key, openid).Result()
		if err != nil {
			fmt.Println("redis ZRevRank errorCode:", err, key, openid)
			return 0, err
		}
	}
	return mc, nil
}

func ZRevRange(key string, start, stop int64) ([]redis.Z, error) {
	rankdata, err := rdb.ZRevRangeWithScores(key, start, stop).Result()
	if err != nil && err.Error() != "redis: nil" {
		check()
		rankdata, err = rdb.ZRevRangeWithScores(key, start, stop).Result()
		if err != nil && err.Error() != "redis: nil" {
			fmt.Println("redis ZRevRange errorCode:", err, key, start, stop)
			return nil, err
		}
	}
	return rankdata, nil
}

func ReName(key, newName string) {
	rdb.Rename(key, newName)
}

func Expire(key string, t time.Duration) error {
	if err := rdb.Expire(key, t).Err(); err != nil {
		check()
		err = rdb.Expire(key, t).Err()
		if err != nil {
			fmt.Println("redis Expire errorCode:", err, key)
		}
		return err
	}
	return nil
}

func Subscribe(ch ...string) *redis.PubSub {
	return rdb.Subscribe(ch...)
}

func Publish(ch string, msg interface{}) {
	err := rdb.Publish(ch, msg).Err()
	if err != nil {
		filelog.Error("Publish err:", err, " ch:", ch, " msg:", msg)
	}
}

func GetZ(k interface{}, v float64) redis.Z {
	return redis.Z{v, k}
}

// 若存在则返回 0
func SetNx(key string, value interface{}) bool {
	b, _ := rdb.SetNX(key, value, 0).Result()
	return b
}

// 若存在则返回 0
func SetNx_Ex(key string, value interface{}, ex time.Duration) bool {
	b, _ := rdb.SetNX(key, value, 0).Result()
	return b
}

func RPush(key string, value interface{}) error {
	return rdb.RPush(key, value).Err()
}

func RPop(key string) string {
	return rdb.RPop(key).Val()
}
