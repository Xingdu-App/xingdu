import socket, threading, time

def serve(host, prefix):
    with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as sock:
        sock.bind((host, 18082))
        while True:
            body, peer = sock.recvfrom(65535)
            sock.sendto(prefix + body, peer)
for host, prefix in [('93.184.216.34', b'PUBLIC:'), ('93.184.216.35', b'PUBLIC:'), ('127.0.0.1', b'PRIVATE:')]:
    threading.Thread(target=serve, args=(host, prefix), daemon=True).start()
while True: time.sleep(60)
