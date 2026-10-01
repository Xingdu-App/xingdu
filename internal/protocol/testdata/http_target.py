import http.server, socketserver
class Server(http.server.ThreadingHTTPServer):
    def server_bind(self):
        socketserver.TCPServer.server_bind(self)
        self.server_name, self.server_port = self.server_address
class Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200)
        self.end_headers()
        self.wfile.write(b'xingdu-matrix-ok')
    def log_message(self, *args): pass
Server(('0.0.0.0', 18081), Handler).serve_forever()
