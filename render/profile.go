package render

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/KeepingRunning/vtuberBot/fetch"
	"github.com/chromedp/chromedp"
)

//go:embed template/profile.html
var profileTemplateFS embed.FS

func wrapHTML(fragment string) string {
	return `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8"/>
<meta name="viewport" content="width=device-width, initial-scale=1"/>
<script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="m-0">` + fragment + `
</body>
</html>`
}

func periodEmoji(period string) string {
	switch period {
	case fetch.PhaseMenstrual:
		return "🩸"
	case fetch.PhaseOvulationDay:
		return "🥚"
	case fetch.PhaseFertile:
		return "🌸"
	case fetch.PhasePreMenstrual:
		return "⏳"
	case fetch.PhaseSafe:
		return "💚"
	default:
		return "💚"
	}
}

func toFileURL(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	abs = filepath.ToSlash(abs)
	if strings.HasPrefix(abs, "/") {
		return "file://" + abs, nil
	}
	// Windows: C:/...
	if len(abs) >= 2 && abs[1] == ':' {
		return "file:///" + abs, nil
	}
	return "file://" + abs, nil
}

// RenderProfile 用 profile.html 渲染卡片，经无头 Chrome + Tailwind CDN 截图为 PNG。
// 文件写在当前工作目录下的 tmp/（需进程 cwd 合理，例如在项目根执行 go run）。
// 返回生成 PNG 的路径；发送私聊图片请读文件后用 message.ImageBytes，不要用裸路径 message.Image（协议端按 URL 解析 file 字段）。
func RenderProfile(user fetch.VtuberUser) (string, error) {
	// ParseFS 会把文件内容挂到「文件名 basename」模板上（此处为 profile.html），
	// 若 New("profile") 与之一致，根模板 profile 会为空，Execute 会报 incomplete or empty template。
	tmpl, err := template.New("profile.html").Funcs(template.FuncMap{
		"periodEmoji": periodEmoji,
	}).ParseFS(profileTemplateFS, "template/profile.html")
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}
	var body bytes.Buffer
	if err := tmpl.Execute(&body, user); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	full := wrapHTML(body.String())

	tmpDir := filepath.Join(".", "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir tmp: %w", err)
	}

	htmlF, err := os.CreateTemp(tmpDir, "vtuber-profile-*.html")
	if err != nil {
		return "", err
	}
	htmlPath := htmlF.Name()
	if _, err := htmlF.WriteString(full); err != nil {
		_ = htmlF.Close()
		_ = os.Remove(htmlPath)
		return "", err
	}
	if err := htmlF.Close(); err != nil {
		_ = os.Remove(htmlPath)
		return "", err
	}
	defer func() { _ = os.Remove(htmlPath) }()

	pngF, err := os.CreateTemp(tmpDir, "vtuber-profile-*.png")
	if err != nil {
		return "", err
	}
	pngPath := pngF.Name()
	_ = pngF.Close()

	navURL, err := toFileURL(htmlPath)
	if err != nil {
		_ = os.Remove(pngPath)
		return "", err
	}

	parent, cancelAll := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelAll()

	allocOpts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(parent, allocOpts...)
	defer cancelAlloc()
	taskCtx, cancelTask := chromedp.NewContext(allocCtx)
	defer cancelTask()

	var pic []byte
	if err := chromedp.Run(taskCtx,
		// 不设 EmulateScale 时 DPR=1，截图 CSS 像素 1:1，聊天里放大很糊；2× 约等于 Retina。
		chromedp.EmulateViewport(448, 384, chromedp.EmulateScale(3)),
		chromedp.Navigate(navURL),
		chromedp.Sleep(2*time.Second),
		chromedp.Screenshot("body", &pic, chromedp.ByQuery),
	); err != nil {
		_ = os.Remove(pngPath)
		return "", fmt.Errorf("chromedp screenshot: %w", err)
	}

	if err := os.WriteFile(pngPath, pic, 0o644); err != nil {
		_ = os.Remove(pngPath)
		return "", err
	}
	absPNG, err := filepath.Abs(pngPath)
	if err != nil {
		_ = os.Remove(pngPath)
		return "", err
	}
	return absPNG, nil
}
