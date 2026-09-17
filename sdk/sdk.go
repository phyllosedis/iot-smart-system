package sdk

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/draw"
	"log"
	"net/http"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type Route string

type ServiceConfig struct {
	ID          string // Уникальный ID
	Name        string // Имя для меню ESP32
	Topic       string // MQTT топик
	RegistryURL string // URL сервера регистрации
	BrokerURL   string // URL Mosquitto
}

type Client struct {
	cfg        ServiceConfig
	mqttClient mqtt.Client
}

var (
	RouteRegister   Route = "/register"
	RouteUnregister Route = "/unregister"
	RouteTopics     Route = "/topics"
)

func NewClient(cfg ServiceConfig) (*Client, error) {
	regReq := map[string]string{
		"id":    cfg.ID,
		"name":  cfg.Name,
		"topic": cfg.Topic,
	}
	jsonData, err := json.Marshal(regReq)
	if err == nil {
		resp, err := http.Post(cfg.RegistryURL+string(RouteRegister), "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			log.Printf("[SDK WARN] Не удалось зарегистрироваться в Service Discovery: %v\n", err)
		} else {
			log.Printf("[SDK INIT] Успешная HTTP-регистрация. Статус: %s\n", resp.Status)
			resp.Body.Close()
		}
	}

	opts := mqtt.NewClientOptions().AddBroker(cfg.BrokerURL).SetClientID("sdk-" + cfg.ID)
	opts.SetAutoReconnect(true)
	mqttClient := mqtt.NewClient(opts)

	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}
	log.Println("[SDK MQTT] Успешно подключено к брокеру Mosquitto")

	return &Client{
		cfg:        cfg,
		mqttClient: mqttClient,
	}, nil
}

func (c *Client) SendFrame(img *image.Paletted) {
	buffer := make([]byte, 1024)
	for page := 0; page < 8; page++ {
		for x := 0; x < 128; x++ {
			var b byte = 0
			for bit := 0; bit < 8; bit++ {
				y := page*8 + bit
				if img.ColorIndexAt(x, y) == 1 {
					b |= (1 << bit)
				}
			}
			buffer[page*128+x] = b
		}
	}

	token := c.mqttClient.Publish(c.cfg.Topic, 1, false, buffer)
	token.Wait()
}

func CreateBaseCanvas() *image.Paletted {
	width, height := 128, 64
	img := image.NewPaletted(image.Rect(0, 0, width, height), color.Palette{
		color.RGBA{0, 0, 0, 255},
		color.RGBA{255, 255, 255, 255},
	})
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{0, 0, 0, 255}}, image.Point{}, draw.Src)
	return img
}

func (c *Client) Disconnect() {
	c.mqttClient.Disconnect(250)
}
