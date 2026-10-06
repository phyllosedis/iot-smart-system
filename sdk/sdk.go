package sdk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"log"
	"net/http"
	"time"

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
	jsonData, err := json.Marshal(map[string]string{
		"id":    cfg.ID,
		"name":  cfg.Name,
		"topic": cfg.Topic,
	})
	if err != nil {
		return nil, fmt.Errorf("не удалось закодировать запрос регистрации: %w", err)
	}

	httpClient := &http.Client{Timeout: 5 * time.Second}
	resp, err := httpClient.Post(cfg.RegistryURL+string(RouteRegister), "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("[SDK WARN] Не удалось зарегистрироваться в Service Discovery: %v\n", err)
	} else {
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			log.Printf("[SDK WARN] Service Discovery вернул статус %s\n", resp.Status)
		} else {
			log.Printf("[SDK INIT] Успешная HTTP-регистрация. Статус: %s\n", resp.Status)
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

func (c *Client) SendFrame(img *image.Paletted) error {
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
	return token.Error()
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
	// Best-effort снятие с регистрации, чтобы не засорять меню на ESP32
	jsonData, err := json.Marshal(map[string]string{"id": c.cfg.ID})
	if err == nil {
		httpClient := &http.Client{Timeout: 5 * time.Second}
		resp, err := httpClient.Post(c.cfg.RegistryURL+string(RouteUnregister), "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			log.Printf("[SDK WARN] Не удалось сняться с регистрации: %v\n", err)
		} else {
			resp.Body.Close()
		}
	}
	if c.mqttClient != nil {
		c.mqttClient.Disconnect(250)
	}
}
