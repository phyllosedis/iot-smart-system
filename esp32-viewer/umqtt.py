import usocket as socket
import ustruct as struct

class MQTTException(Exception):
    pass

class MQTTClient:
    def __init__(self, client_id, server, port=0, user=None, password=None, keepalive=0, ssl=False, ssl_params={}):
        if port == 0:
            port = 8883 if ssl else 1883
        self.client_id = client_id
        self.sock = None
        self.server = server
        self.port = port
        self.ssl = ssl
        self.ssl_params = ssl_params
        self.pid = 0
        self.cb = None
        self.user = user
        self.pswd = password
        self.keepalive = keepalive

    def _send_str(self, s):
        if isinstance(s, str):
            s = s.encode("utf-8")
        self.sock.write(struct.pack("!H", len(s)))
        self.sock.write(s)

    def _recv_len(self):
        n = 0
        sh = 0
        while 1:
            res = self.sock.read(1)
            if res is None or res == b"":
                raise MQTTException("Unexpected EOF")
            b = res[0]
            n |= (b & 0x7F) << sh
            if not b & 0x80:
                return n
            sh += 7

    def set_callback(self, cb):
        self.cb = cb

    def connect(self, clean_session=True):
        self.sock = socket.socket()
        addr_info = socket.getaddrinfo(self.server, self.port)
        actual_addr = addr_info[0][-1]

        self.sock.connect(actual_addr)
        if self.ssl:
            import ussl
            self.sock = ussl.wrap_socket(self.sock, **self.ssl_params)

        cid = self.client_id
        if isinstance(cid, str):
            cid = cid.encode("utf-8")

        msg_len = 10 + 2 + len(cid)

        connect_flags = 0
        if clean_session:
            connect_flags |= 0x02

        if self.user is not None:
            user_bytes = self.user.encode("utf-8") if isinstance(self.user, str) else self.user
            msg_len += 2 + len(user_bytes)
            connect_flags |= 0x80
            if self.pswd is not None:
                pswd_bytes = self.pswd.encode("utf-8") if isinstance(self.pswd, str) else self.pswd
                msg_len += 2 + len(pswd_bytes)
                connect_flags |= 0x40

        pkt = bytearray()
        pkt.append(0x10)
        pkt.append(msg_len)
        pkt.extend(b"\x00\x04MQTT\x04")
        pkt.append(connect_flags)
        pkt.append(self.keepalive >> 8)
        pkt.append(self.keepalive & 0xFF)

        self.sock.write(pkt)
        self._send_str(cid)

        if self.user is not None:
            self._send_str(user_bytes)
            if self.pswd is not None:
                self._send_str(pswd_bytes)

        resp = self.sock.read(4)
        if resp is None or len(resp) < 4:
            raise MQTTException("Bad CONNACK")
        if resp[3] != 0:
            raise MQTTException(resp[3])
        return resp[2] & 1

    def disconnect(self):
        try:
            self.sock.write(b"\xe0\0")
        except:
            pass
        self.sock.close()
        self.sock = None

    def subscribe(self, topic, qos=0):
        assert self.cb is not None
        if isinstance(topic, str):
            topic = topic.encode("utf-8")

        self.pid += 1

        pkt = bytearray()
        pkt.append(0x82)
        msg_len = 2 + 2 + len(topic) + 1
        pkt.append(msg_len)

        pkt.append(self.pid >> 8)
        pkt.append(self.pid & 0xFF)

        self.sock.write(pkt)
        self._send_str(topic)
        self.sock.write(struct.pack("B", qos))

        resp = self.sock.read(5)
        if resp is None or len(resp) < 5:
            raise MQTTException("Bad SUBACK")

    def wait_msg(self):
        res = self.sock.read(1)
        if res is None or res == b"":
            return None
        if res == b"\xd0":
            self.sock.read(1)
            return None
        op = res[0]
        if op & 0xF0 != 0x30:
            return op

        sz = self._recv_len()

        # Корректно читаем 2 байта длины топика
        topic_len_buf = self.sock.read(2)
        if len(topic_len_buf) < 2:
            return None
        topic_len = (topic_len_buf[0] << 8) | topic_len_buf[1]

        topic = self.sock.read(topic_len)
        sz -= 2 + topic_len

        if op & 6:
            self.sock.read(2)
            sz -= 2

        # Заставляем выкачать весь буфер картинки из сокета, даже если он идет кусками
        msg = bytearray()
        while len(msg) < sz:
            chunk = self.sock.read(sz - len(msg))
            if not chunk:
                break
            msg.extend(chunk)

        self.cb(topic, msg)

    def check_msg(self):
        self.sock.setblocking(False)
        try:
            return self.wait_msg()
        except OSError:
            return None

