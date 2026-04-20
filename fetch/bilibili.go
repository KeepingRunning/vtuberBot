package fetch

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// BiliResponse 定义 API 返回的结构体
type BiliResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	User    User   `json:"data"`
}

type User struct {
	Mid       int `json:"mid"`
	Following int `json:"following"`
	Follower  int `json:"follower"`
	Black     int `json:"black"`
}

func GetVtuberFans(uid int64) (int, error) {
	url := fmt.Sprintf("https://api.bilibili.com/x/relation/stat?vmid=%d", uid)

	// 1. 创建请求实例
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("创建请求失败: %v", err)
	}

	// 2. 注入 User-Agent，防止被 B 站拦截
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	// 3. 使用带超时的 Client 发起请求
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("获取b站粉丝数失败: %v", err)
	}
	defer resp.Body.Close()

	// 4. 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("读取响应内容失败: %v", err)
	}

	// 5. 解析 JSON
	var res BiliResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return 0, fmt.Errorf("解析 JSON 失败: %v", err)
	}

	// 6. 校验业务逻辑状态码
	if res.Code != 0 {
		return 0, fmt.Errorf("B站接口返回错误: %s (code:%d)", res.Message, res.Code)
	}

	return res.User.Follower, nil
}