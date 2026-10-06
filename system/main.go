package main

import (
	"fmt"
	"image"
	sdk "iot-smart-system/sdk"
	"log"
	"math/rand"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/inconsolata"
	"golang.org/x/image/math/fixed"
)

func main() {
	iotClient, err := sdk.NewClient(sdk.ServiceConfig{
		ID:          "go_system_sub",
		Name:        "SYSTEM STATION",
		Topic:       "orange/display/system",
		RegistryURL: "http://192.168.0.101:8080",
		BrokerURL:   "tcp://192.168.0.169:1883",
	})
	if err != nil {
		log.Fatalf("Ошибка запуска SDK: %v", err)
	}
	defer iotClient.Disconnect()

	ticker := time.NewTicker(4 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// 1. Собираем нужную нам инфу
		proc := rand.Intn(15) + 15
		mem := rand.Intn(40) + 40

		canvas := sdk.CreateBaseCanvas()

		fontDrawer := &font.Drawer{
			Dst:  canvas,
			Src:  image.NewUniform(image.White),
			Face: inconsolata.Regular8x16,
		}

		fontDrawer.Dot = fixed.P(4, 14)
		fontDrawer.DrawString(fmt.Sprintf("PROC: %d %%", proc))

		fontDrawer.Dot = fixed.P(4, 34)
		fontDrawer.DrawString(fmt.Sprintf("MEM: %d %%", mem))

		fontDrawer.Dot = fixed.P(4, 54)
		fontDrawer.DrawString(time.Now().Format("15:04:05"))

		if err := iotClient.SendFrame(canvas); err != nil {
			log.Printf("[SYSTEM] Ошибка отправки кадра: %v", err)
		} else {
			log.Println("[SYSTEM] Новый кадр успешно отправлен через SDK")
		}
	}
}
