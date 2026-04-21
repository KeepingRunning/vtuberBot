package fetch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	fertilityAPIBase = "https://workers.vrp.moe/laplace/fertility"
	chromeUserAgent  = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36"

	// 周期阶段（判定顺序：魔法期 → 排卵日 → 易孕期 → 即将魔法期 → 安全期）
	PhaseMenstrual    = "魔法期"
	PhaseOvulationDay = "排卵日"
	PhaseFertile      = "易孕期"
	PhasePreMenstrual = "即将魔法期"
	PhaseSafe         = "安全期"
)

// VtuberResponse 对应 workers.vrp.moe laplace/fertility/{uid} 返回的 JSON。
type VtuberResponse struct {
	UID                   int64                `json:"uid"`
	User                  VtuberUser           `json:"user"`
	Status                string               `json:"status"`
	DayInCycle            int                  `json:"dayInCycle"`
	OvulationDay          int                  `json:"ovulationDay"`
	NextPeriod            string               `json:"nextPeriod"`
	EffectiveCycleLength  int                  `json:"effectiveCycleLength"`
	EffectivePeriodLength int                  `json:"effectivePeriodLength"`
	DataPoints            int                  `json:"dataPoints"`
	UserPreferences       VtuberPreferences    `json:"userPreferences"`
	History               []VtuberPeriodRecord `json:"history"`
}

type VtuberUser struct {
	UID           int64           `json:"-"` // 来自 VtuberResponse 根字段，由 GetVtuberFertility 填入
	Username      string          `json:"username"`
	Avatar        string          `json:"avatar"`
	Room          int64           `json:"room"`
	LiveStatus    int             `json:"liveStatus"`
	LiveFansCount int             `json:"liveFansCount"`
	GuildInfo     VtuberGuildInfo `json:"guildInfo"`
	MCNInfo       json.RawMessage `json:"mcnInfo"`
	// 以下由 GetVtuberFertility 根据周期公式写入，不参与 JSON 反序列化。
	Period         string `json:"period,omitempty"`
	CurrentDay     int    `json:"currentDay,omitempty"`
	NumberOfGuards int    `json:"numberOfGuards,omitempty"`
	Follower       int    `json:"followers,omitempty"`
}

type VtuberGuildInfo struct {
	Current *VtuberGuildRecord  `json:"current,omitempty"`
	History []VtuberGuildRecord `json:"history"`
}

type VtuberGuildRecord struct {
	Name      string `json:"name"`
	UpdatedAt int64  `json:"updatedAt"`
}

type VtuberPreferences struct {
	CycleLength  int `json:"cycleLength"`
	PeriodLength int `json:"periodLength"`
}

type VtuberPeriodRecord struct {
	Source      string `json:"source"`
	PeriodStart int64  `json:"periodStart"`
	SubmittedAt int64  `json:"submittedAt"`
}

var laplaceClient = &http.Client{Timeout: 30 * time.Second}

func setFertilityRequestHeaders(req *http.Request) {
	req.Header.Set("User-Agent", chromeUserAgent)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,zh-CN;q=0.8,zh;q=0.7")
}

// currentCycleDay 周期内展示日 D_curr = ((dayInCycle-1) mod effectiveCycleLength) + 1
func currentCycleDay(dayInCycle, effectiveCycleLength int) (int, error) {
	if effectiveCycleLength <= 0 {
		return 0, fmt.Errorf("effectiveCycleLength must be > 0, got %d", effectiveCycleLength)
	}
	if dayInCycle <= 0 {
		return 0, fmt.Errorf("dayInCycle must be > 0, got %d", dayInCycle)
	}
	return (dayInCycle-1)%effectiveCycleLength + 1, nil
}

// classifyPhase 按优先级判定当前阶段（与 D_curr、经期长、排卵日、周期长相关）。
func classifyPhase(dCurr, effectiveCycleLength, effectivePeriodLength, ovulationDay int) string {
	// 1. 魔法期：1 <= D_curr <= effectivePeriodLength
	if effectivePeriodLength > 0 && dCurr >= 1 && dCurr <= effectivePeriodLength {
		return PhaseMenstrual
	}
	// 2. 排卵日：D_curr == ovulationDay
	if ovulationDay > 0 && dCurr == ovulationDay {
		return PhaseOvulationDay
	}
	// 3. 易孕期：(ovulationDay-5) <= D_curr <= (ovulationDay+4)，排卵日已在上一分支排除
	if ovulationDay > 0 {
		low := ovulationDay - 5
		high := ovulationDay + 4
		if dCurr >= low && dCurr <= high {
			return PhaseFertile
		}
	}
	// 4. 即将魔法期：D_curr > (effectiveCycleLength - 3)（周期最后约 3 天）
	if effectiveCycleLength > 3 && dCurr > effectiveCycleLength-3 {
		return PhasePreMenstrual
	}
	return PhaseSafe
}

// GetVtuberFertility 请求 fertility 接口，解析 JSON，计算 CurrentDay（周期内第几天）与 Period（阶段），写入 User 后返回。
func GetVtuberFertility(uid int64) (VtuberUser, error) {
	var empty VtuberUser
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	url := fmt.Sprintf("%s/%d", fertilityAPIBase, uid)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return empty, err
	}
	setFertilityRequestHeaders(req)
	resp, err := laplaceClient.Do(req)
	if err != nil {
		return empty, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return empty, err
	}
	s := string(body)
	if resp.StatusCode != http.StatusOK {
		return empty, fmt.Errorf("laplace fertility: http %d: %s", resp.StatusCode, clip(s, 200))
	}
	if looksLikeCloudflareChallenge(s) {
		return empty, fmt.Errorf("laplace fertility: blocked by cloudflare challenge (non-json html)")
	}
	var vr VtuberResponse
	if err := json.Unmarshal(body, &vr); err != nil {
		return empty, fmt.Errorf("laplace fertility: invalid json: %w", err)
	}
	dCurr, err := currentCycleDay(vr.DayInCycle, vr.EffectiveCycleLength)
	if err != nil {
		return empty, fmt.Errorf("laplace fertility: %w", err)
	}
	phase := classifyPhase(dCurr, vr.EffectiveCycleLength, vr.EffectivePeriodLength, vr.OvulationDay)
	user := vr.User
	user.UID = vr.UID
	user.CurrentDay = dCurr
	user.Period = phase
	user.NumberOfGuards, err = GetNumberOfGuards(uid)
	if err != nil {
		return empty, fmt.Errorf("Failed to get number of guards: %w", err)
	}
	user.Follower, err = GetVtuberFans(uid)
	if err != nil {
		return empty, fmt.Errorf("Failed to get followers: %w", err)
	}
	return user, nil
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func looksLikeCloudflareChallenge(s string) bool {
	ls := strings.ToLower(s)
	return strings.HasPrefix(strings.TrimSpace(s), "<!DOCTYPE") &&
		(strings.Contains(ls, "just a moment") || strings.Contains(ls, "cf-chl") || strings.Contains(ls, "cloudflare"))
}
