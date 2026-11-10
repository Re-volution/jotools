package jtime

import (
	"github.com/Re-volution/ltime"
	"time"
)

const Day = 24 * 60 * 60

// 次日hour时间戳
func NextDayTimestampByHour(hour int) int64 {
	t := getCurrZeroT()
	return t.Add(time.Duration(24+hour) * time.Hour).Unix()
}

// 当前时区当天零点
func getCurrZeroT() time.Time {
	now := ltime.GetNow()
	y, m, d := now.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, now.Location())
}

// 某日零点时间戳
func GetNDayZeroHourTimestamp(n int) int64 {
	t := getCurrZeroT()
	return t.AddDate(0, 0, n).Unix()
}

// 获取hour小时后的时间戳
func GetNHourTimestamp(hour int) int64 {
	now := ltime.GetNow()
	return now.Add(time.Duration(hour) * time.Hour).Unix()
}

// 获取下个整小时的时间戳
func GetNextHourTimestamp() int64 {
	now := ltime.GetNow()
	t := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())
	return t.Add(time.Hour).Unix()
}

// 获取N天之后的时间戳
func GetNDayTimestamp(day int) int64 {
	now := ltime.GetNow()
	return now.AddDate(0, 0, day).Unix()
}

// 下周一凌晨时间戳。当天是周一时，返回再过 7 天的周一。
func GetNWeekZeroHourTimestamp() int64 {
	now := ltime.GetNow()
	y, m, d := now.Date()
	zero := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return zero.AddDate(0, 0, 8-weekday).Unix()
}

// 下个月一号凌晨时间戳
func GetNMonthZeroHourTimestamp() int64 {
	now := ltime.GetNow()
	year, month, _ := now.AddDate(0, 1, 0).Date()
	t := time.Date(year, month, 1, 0, 0, 0, 0, now.Location())
	return t.Unix()
}

// 取当周 周N的日期
func GetCurrWeekNDay(n int) (string, int) {
	now := ltime.GetNow()
	we := GetCurrWeek()
	day := n - int(we)
	now = now.AddDate(0, 0, day)
	return now.Format("2006-01-02"), now.Day()
}

// 取当周 周N的日期
func GetCurrWeek() int {
	now := ltime.GetNow()
	week := now.Weekday()
	if week == 0 {
		return 7
	} else {
		return int(now.Weekday())
	}
}
