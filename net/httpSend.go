package net

import (
	"io"
	"net"
	"net/http"
	"strings"
	"time"
	"ysj/jotools/filelog"
)

func HttpDo(url string, reader *strings.Reader, header map[string]string) ([]byte, error) {
	client := &http.Client{
		Transport: &http.Transport{
			Dial: func(netw, addr string) (net.Conn, error) {
				c, err := net.DialTimeout(netw, addr, time.Second*3) //设置建立连接超时
				if err != nil {
					return nil, err
				}
				c.SetDeadline(time.Now().Add(5 * time.Second)) //设置发送接收数据超时
				return c, nil
			},
		},
	}

	req, err := http.NewRequest("POST", url, reader)
	if err != nil {
		return nil, err
	}

	for k, v := range header {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	filelog.Log("HttpDo body:", string(body))
	filelog.Log("HttpDo header:", resp.Header)
	filelog.Log("HttpDo send header:", req.Header)
	return body, nil
}

func HttpDoGet(url string, reader map[string]string, header map[string]string) ([]byte, error) {
	client := &http.Client{
		Transport: &http.Transport{
			Dial: func(netw, addr string) (net.Conn, error) {
				c, err := net.DialTimeout(netw, addr, time.Second*3) //设置建立连接超时
				if err != nil {
					return nil, err
				}
				c.SetDeadline(time.Now().Add(5 * time.Second)) //设置发送接收数据超时
				return c, nil
			},
		},
	}
	if len(reader) != 0 {
		url += "?"
		for k, v := range reader {
			url += (k + "=" + v + "&")
		}
		url = url[:len(url)-1]
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	for k, v := range header {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	filelog.Log(string(body))
	return body, nil
}
