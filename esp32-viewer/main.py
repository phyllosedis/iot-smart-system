import machine
import time
import network
import urequests
from umqtt import MQTTClient
import ssd1306

# 1. Настройки сети, подключения и URL
WIFI_SSID = "имя сети"
WIFI_PASS = "пароль сети"
MQTT_SERVER = "айпи сервера с mqtt"
TOPICS_URL = "url с топиками"

# 2. Подключаем Wi-Fi
wlan = network.WLAN(network.STA_IF)
wlan.active(True)
wlan.connect(WIFI_SSID, WIFI_PASS)

print("Подключаемся к Wi-Fi...", end="")
while not wlan.isconnected():
    time.sleep(0.5)
    print(".", end="")
print("\nWi-Fi подключен! IP:", wlan.ifconfig()[0])

# 3. Настройка I2C и экрана SSD1306
i2c = machine.I2C(0, scl=machine.Pin(22), sda=machine.Pin(21))

# Настройка джойстика (Аналоговая ось Y и кнопка выбора)
joy_y = machine.ADC(machine.Pin(34))    # Ось Y джойстика (используй пины 32-39, они поддерживают ADC)
joy_y.atten(machine.ADC.ATTN_11DB)     # Настройка диапазона до 3.3В (0-4095)

btn_select = machine.Pin(5, machine.Pin.IN, machine.Pin.PULL_UP) # Кнопка нажатия на джойстик (SW)

try:
    oled = ssd1306.SSD1306_I2C(128, 64, i2c, addr=0x3C)
    oled.fill(0)
    oled.show()
    print("Дисплей SSD1306 успешно инициализирован")
except Exception as e:
    print("Ошибка инициализации дисплея. Проверь контакты SCL/SDA!", e)
    time.sleep(5)
    machine.reset()

# Глобальные переменные состояния
topics_list = []
current_topic = None
in_menu = True

def load_topics():
    """Загрузка списка топиков по HTTP"""
    global topics_list
    print("Загружаем топики с сервера...")
    try:
        res = urequests.get(TOPICS_URL)
        try:
            if res.status_code != 200:
                raise ValueError("HTTP статус: %s" % res.status_code)
            data = res.json()
        finally:
            res.close()
        # Фильтруем битые записи без name/topic
        data = [t for t in data if isinstance(t, dict) and t.get("name") and t.get("topic")]
        if not data:
            raise ValueError("сервер вернул пустой список топиков")
        topics_list = data
        print("Топики успешно загружены:", topics_list)
    except Exception as e:
        print("Ошибка загрузки топиков по HTTP:", e)
        topics_list = [{"name": "Ошибка HTTP! Заглушка", "topic": "orange/display/weather"}]

def mqtt_callback(topic, msg):
    """Обработчик входящей графики из MQTT брокера"""
    global in_menu, current_topic

    if in_menu:
        return

    if current_topic and topic == current_topic.encode('utf-8'):
        if len(msg) == 1024:
            try:
                oled.buffer[:] = msg
                oled.show()
            except Exception as e:
                print("Ошибка отрисовки буфера:", e)

def draw_menu(selected_index):
    """Отрисовка списка топиков на экране"""
    oled.fill(0)
    oled.text("== ВЫБОР ТОПИКА ==", 0, 0, 1)
    if not topics_list:
        oled.text("Нет топиков", 0, 20, 1)
        oled.text("Жми SELECT", 0, 32, 1)
        oled.show()
        return
    selected_index = selected_index % len(topics_list)
    start_y = 16
    for idx, item in enumerate(topics_list):
        if idx >= 5:
            break
        name = item.get("name", "Unknown")[:16]
        if idx == selected_index:
            oled.text("> " + name, 0, start_y + (idx * 9), 1)
        else:
            oled.text("  " + name, 0, start_y + (idx * 9), 1)

    oled.show()


def wait_for_button_release():
    """Ждем отпускания кнопки, чтобы убрать автоповтор при удержании"""
    while btn_select.value() == 0:
        time.sleep(0.05)

# Загружаем структуру меню
load_topics()

# 5. Инициализация и коннект к Mosquitto
client = MQTTClient("esp32-display-cl", MQTT_SERVER, port=1883)
client.set_callback(mqtt_callback)

try:
    client.connect()
    print("Успешное подключение к Mosquitto!")
except Exception as e:
    print("Не удалось подключиться к MQTT брокеру:", e)
    time.sleep(5)
    machine.reset()

menu_index = 0
draw_menu(menu_index)

last_joy_time = 0
JOY_DELAY_MS = 300  # Задержка между шагами перемещения джойстика

while True:
    try:
        now = time.ticks_ms()

        if in_menu:
            # Дергаем MQTT-стек и в меню, иначе брокер отвалит по keepalive,
            # пока пользователь листает список
            try:
                client.check_msg()
            except Exception as e:
                print("Ошибка MQTT в меню:", e)

            if not topics_list:
                # Нечего листать — пробуем перезагрузить список раз в ~5 сек
                if time.ticks_diff(now, last_joy_time) > 5000:
                    last_joy_time = now
                    load_topics()
                    draw_menu(menu_index)
                # Возврат в режим просмотра невозможен, но SELECT обновляет список
                if btn_select.value() == 0:
                    wait_for_button_release()
                    last_joy_time = now
                    load_topics()
                    draw_menu(menu_index)
                time.sleep(0.02)
                continue

            menu_index = menu_index % len(topics_list)
            # Читаем положение джойстика по оси Y
            y_val = joy_y.read()

            # Если отклонили джойстик ВВЕРХ (значение близко к 0)
            if y_val < 1000 and time.ticks_diff(now, last_joy_time) > JOY_DELAY_MS:
                last_joy_time = now
                menu_index = (menu_index - 1) % len(topics_list) # Листаем вверх
                draw_menu(menu_index)

            # Если отклонили джойстик ВНИЗ (значение близко к 4095)
            elif y_val > 3000 and time.ticks_diff(now, last_joy_time) > JOY_DELAY_MS:
                last_joy_time = now
                menu_index = (menu_index + 1) % len(topics_list) # Листаем вниз
                draw_menu(menu_index)

            # Нажатие кнопки джойстика (Клик для выбора)
            if btn_select.value() == 0 and time.ticks_diff(now, last_joy_time) > JOY_DELAY_MS:
                last_joy_time = now

                if current_topic:
                    try:
                        client.unsubscribe(current_topic)
                    except:
                        pass

                chosen = topics_list[menu_index].get("topic")
                if not chosen:
                    print("У выбранного пункта нет topic, выбор отменен")
                    wait_for_button_release()
                    continue
                current_topic = chosen
                print("Выбран топик подписки:", current_topic)

                oled.fill(0)
                oled.text("Subscribing...", 0, 20, 1)
                oled.show()

                try:
                    client.subscribe(current_topic)
                except Exception as e:
                    print("Ошибка подписки на топик:", e)
                    oled.fill(0)
                    oled.text("Sub failed!", 0, 20, 1)
                    oled.show()
                    time.sleep(1)
                    draw_menu(menu_index)
                    wait_for_button_release()
                    continue
                in_menu = False
                print("Вышли из меню. Ждем кадры...")
                wait_for_button_release()

        else:
            client.check_msg()

            # Возврат в меню по нажатию кнопки джойстика во время показа графики
            if btn_select.value() == 0 and time.ticks_diff(now, last_joy_time) > JOY_DELAY_MS:
                last_joy_time = now
                in_menu = True
                draw_menu(menu_index)
                wait_for_button_release()

        time.sleep(0.02)

    except Exception as e:
        print("Сбой в рабочем цикле, уходим в ребут...", e)
        time.sleep(2)
        machine.reset()

