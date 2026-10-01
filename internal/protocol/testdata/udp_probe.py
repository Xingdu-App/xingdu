"""SOCKS5 UDP acceptance using a single association across destinations."""
import socket, struct, sys

def read(sock, size):
    out = b''
    while len(out) < size:
        chunk = sock.recv(size-len(out))
        if not chunk: raise RuntimeError('SOCKS control connection closed')
        out += chunk
    return out

with socket.create_connection(('127.0.0.1', 21900), timeout=5) as control:
    control.sendall(b'\x05\x01\x00')
    assert read(control, 2) == b'\x05\x00'
    control.sendall(b'\x05\x03\x00\x01' + b'\x00'*6)
    head = read(control, 4)
    assert head[:3] == b'\x05\x00\x00', 'UDP associate rejected'
    family = socket.AF_INET if head[3] == 1 else socket.AF_INET6
    assert head[3] in (1, 4)
    host = socket.inet_ntop(family, read(control, 4 if family == socket.AF_INET else 16))
    port = struct.unpack('!H', read(control, 2))[0]
    if host in ('0.0.0.0', '::'): host = '127.0.0.1' if family == socket.AF_INET else '::1'
    with socket.socket(family, socket.SOCK_DGRAM) as udp:
        udp.settimeout(2)
        for target, expected in [('93.184.216.34', True), ('127.0.0.1', False), ('::ffff:127.0.0.1', False), ('93.184.216.35', True), ('93.184.216.34', True)]:
            payload = b'xingdu-probe-' + target.encode()
            address = (b'\x04' + socket.inet_pton(socket.AF_INET6, target)) if ':' in target else (b'\x01' + socket.inet_aton(target))
            message = b'\x00\x00\x00' + address + struct.pack('!H', 18082) + payload
            udp.sendto(message, (host, port))
            try:
                response, _ = udp.recvfrom(65535)
                replied = response.endswith(b'PUBLIC:' + payload)
                assert b'PRIVATE:' not in response, 'private UDP destination reached'
            except socket.timeout:
                replied = False
            assert replied == expected, 'UDP destination routing mismatch: ' + target
print('UDP pass')
