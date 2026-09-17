module weather

go 1.27.1

require (
	github.com/eclipse/paho.mqtt.golang v1.5.1 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	golang.org/x/net v0.44.0 // indirect
	golang.org/x/sync v0.17.0 // indirect
)

require (
	golang.org/x/image v0.46.0
	iot-smart-system/sdk v1.0.0
)

replace iot-smart-system/sdk => ../sdk
