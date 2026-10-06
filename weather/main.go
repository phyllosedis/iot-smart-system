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
		ID:          "go_weather_sub",
		Name:        "WEATHER STATION",
		Topic:       "orange/display/weather",
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
		temp := rand.Intn(15) + 15
		humi := rand.Intn(40) + 40

		canvas := sdk.CreateBaseCanvas()

		fontDrawer := &font.Drawer{
			Dst:  canvas,
			Src:  image.NewUniform(image.White),
			Face: inconsolata.Regular8x16,
		}

		fontDrawer.Dot = fixed.P(4, 14)
		fontDrawer.DrawString(fmt.Sprintf("TEMP: %d C", temp))

		fontDrawer.Dot = fixed.P(4, 34)
		fontDrawer.DrawString(fmt.Sprintf("HUMI: %d %%", humi))

		fontDrawer.Dot = fixed.P(4, 54)
		fontDrawer.DrawString(time.Now().Format("15:04:05"))

		if err := iotClient.SendFrame(canvas); err != nil {
			log.Printf("[WEATHER] Ошибка отправки кадра: %v", err)
		} else {
			log.Println("[WEATHER] Новый кадр успешно отправлен через SDK")
		}
	}
}
