"""Public-network RustDesk OSS smoke test; uses no desktop session or credentials."""
import base64
import socket
import sys
import time
import uuid
from contextlib import closing

HOST = "89.116.220.139"


def varint(value):
    result = bytearray()
    while value > 127:
        result.append((value & 127) | 128)
        value >>= 7
    return bytes(result + bytes([value]))


def field(number, value):
    return varint(number << 3 | 2) + varint(len(value)) + value


def frame(payload):
    size = len(payload)
    width = next(n for n in range(1, 5) if size < 1 << (8 * n - 2))
    return ((size << 2) | (width - 1)).to_bytes(width, "little") + payload


def exact(stream, size):
    result = b""
    while len(result) < size:
        block = stream.recv(size - len(result))
        assert block, "connection closed before full response"
        result += block
    return result


def read_frame(stream):
    header = exact(stream, 1)
    header += exact(stream, header[0] % 4)
    size = int.from_bytes(header, "little") >> 2
    assert size <= 4096, "unexpected oversized response"
    return exact(stream, size)


def connect(port):
    return socket.create_connection((HOST, port), timeout=8)


def main():
    key = sys.argv[1].encode("ascii")
    assert len(base64.b64decode(key, validate=True)) == 32, "invalid public key"
    assert field(20, b"") == b"\xa2\x01\x00"
    assert frame(b"x" * 64)[:2] == b"\x01\x01"

    for port in (21115, 21116):
        with closing(connect(port)) as stream:
            stream.sendall(frame(field(20, b"")))
            response = read_frame(stream)
            assert response.startswith(b"\xaa\x01"), (port, response.hex())
        print(f"TCP {port}: NAT protocol response OK")

    with closing(socket.socket(socket.AF_INET, socket.SOCK_DGRAM)) as udp:
        udp.settimeout(8)
        udp.connect((HOST, 21116))
        udp.send(field(6, field(1, ("probe-" + uuid.uuid4().hex).encode())))
        response = udp.recv(4096)
        assert response == field(7, b"\x10\x01"), response.hex()
    print("UDP 21116: registration response OK (no device key registered)")

    with closing(connect(21117)) as stream:
        request = field(18, field(2, uuid.uuid4().hex.encode()) + field(6, b"invalid-probe"))
        stream.sendall(frame(request))
        try:
            assert stream.recv(1) == b"", "relay accepted invalid key"
        except ConnectionResetError:
            pass
    print("Relay: invalid key rejected")

    request = frame(field(18, field(2, uuid.uuid4().hex.encode()) + field(6, key)))
    with closing(connect(21117)) as first, closing(connect(21117)) as second:
        first.sendall(request)
        # The first peer must enter the relay's waiting map before the second arrives.
        time.sleep(0.5)
        second.sendall(request)
        time.sleep(0.3)
        for sender, receiver in ((first, second), (second, first)):
            payload = b"rustdesk-deployment-check:" + uuid.uuid4().bytes
            sender.sendall(payload)
            assert exact(receiver, len(payload)) == payload, "relay payload mismatch"
    print("TCP 21117: authenticated bidirectional relay OK")


if __name__ == "__main__":
    main()
