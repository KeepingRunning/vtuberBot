package handler

import (
	"fmt"
	"github.com/KeepingRunning/vtuberBot/render"
	"github.com/KeepingRunning/vtuberBot/fetch"
)

// HandleProfile 拉取 fertility 接口并返回 VtuberUser 的 JSON 文本（便于直接发送或再交给 render）。
func HandleProfile(uid int64) (string, error) {
	user, err := fetch.GetVtuberFertility(uid)
	if err != nil {
		return "", fmt.Errorf("Failed to get profile: %w", err)
	}
	img, err := render.RenderProfile(user)
	if err != nil {
		return "", fmt.Errorf("Failed to render profile: %w", err)
	}
	return img, nil
}


