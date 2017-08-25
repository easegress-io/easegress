#!/usr/bin/env python3
# -*- encoding: utf-8 -*-


import os
import itchat
from http.server import BaseHTTPRequestHandler, HTTPServer
from pyqrcode import QRCode

QR_FILE = 'qr.txt'

def writeQrToFile(uuid, status, qrcode):
    qrCode = QRCode('https://login.weixin.qq.com/l/' + uuid)

    with open(QR_FILE, 'wt') as f:
        print_qr(qrCode.text(1), f)

def print_qr(qrText, f, white='MM', black='  ', blockCount=1):
    white *= abs(blockCount)
    qr = qrText.replace('0', white).replace('1', black)
    f.write(qr)

itchat.auto_login(hotReload=True, qrCallback=writeQrToFile)

def sendout_to_wechat(filename, content):
    itchat.get_chatrooms(update=True, contactOnly=False)
    with open(filename, 'wb') as f:
        f.write(content)
    itchat.search_chatrooms(name='Breaking News')[0].send('@fil@'+filename)
    os.remove(filename)


class WeChatTTPServer(BaseHTTPRequestHandler):
  def do_POST(self):
        print(self.path)

        prefix = '/upload_speech/'
        if len(self.path) <= len(prefix) or self.path[:len(prefix)] != prefix:
            self.send_error(404)
            return

        filename = self.path[len(prefix):]

        content_len = int(self.headers.get('Content-Length'))
        content = self.rfile.read(content_len)

        sendout_to_wechat(filename+'.mp3', content)

        return


def run():
  server_address = ('', 8000)
  httpd = HTTPServer(server_address, WeChatTTPServer)
  httpd.serve_forever()


if __name__ == '__main__':
    run()
