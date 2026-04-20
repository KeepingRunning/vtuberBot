package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/KeepingRunning/vtuberBot/fetch"
	"github.com/KeepingRunning/vtuberBot/handler"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/driver"
	"github.com/wdvxdr1123/ZeroBot/message"
)

func zerobotinit() {
	zero.OnCommand("hello").
		Handle(func(ctx *zero.Ctx) {
			ctx.Send("world")
		})
	zero.OnCommand("test").
		Handle(func(ctx *zero.Ctx) {
			u, err := fetch.GetVtuberFertility(623441612)
			if err != nil {
				ctx.Send(err.Error())
				return
			}
			b, err := json.MarshalIndent(u, "", "  ")
			if err != nil {
				ctx.Send(err.Error())
				return
			}
			info := string(b)
			if len(info) > 3500 {
				info = info[:3500] + "\n…(truncated)"
			}
			ctx.Send(info)
		})

	zero.OnCommand("profile").
		Handle(func(ctx *zero.Ctx) {
			args, _ := ctx.State["args"].(string)
			args = strings.TrimSpace(args)
			if args == "" {
				ctx.Send("Usage: /profile <B站数字UID>")
				return
			}
			uid, err := strconv.ParseInt(args, 10, 64)
			if err != nil || uid <= 0 {
				ctx.Send(fmt.Sprintf("Invalid UID: %q", args))
				return
			}
			imgPath, err := handler.HandleProfile(uid)
			if err != nil {
				ctx.Send(err.Error())
				return
			}
			pngData, err := os.ReadFile(imgPath)
			if err != nil {
				ctx.Send(fmt.Sprintf("read profile png: %v", err))
				return
			}
			ctx.SendChain(message.ImageBytes(pngData))
		})
}

func main() {
	zerobotinit()

	zero.RunAndBlock(&zero.Config{
		NickName:      []string{"bot"},
		CommandPrefix: "/",
		SuperUsers:    []int64{1599949878},
		Driver: []zero.Driver{
			// 反向 WS
			driver.NewWebSocketServer(16, "ws://127.0.0.1:8080", ""),
		},
	}, nil)
}
