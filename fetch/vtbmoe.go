package fetch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

var vtbsClient = &http.Client{Timeout: 15 * time.Second}

type vtbsGuardEntry struct {
	Mid   int64  `json:"mid"`
	Uname string `json:"uname"`
	Face  string `json:"face"`
	Level int    `json:"level"`
}

// GetNumberOfGuards 请求 vtbs.moe 舰长列表，返回当前舰长人数（响应体为 JSON 数组）。
func GetNumberOfGuards(uid int64) (int, error) {
	url := fmt.Sprintf("https://api.vtbs.moe/v1/guard/%d", uid)
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", chromeUserAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := vtbsClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("vtbs.moe guard: http %d: %s", resp.StatusCode, clip(string(body), 120))
	}
	var guards []vtbsGuardEntry
	if err := json.Unmarshal(body, &guards); err != nil {
		return 0, fmt.Errorf("vtbs.moe guard: %w", err)
	}
	return len(guards), nil
}
