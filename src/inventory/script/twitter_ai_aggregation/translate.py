#!/usr/bin/env python3
# -*- encoding: utf-8 -*-

'''
Description:
    This script aims to translate a input text into Chinese.
    Currently we use Youdao as the translate service.

Input: text needed to be translated
Output: translated text:
'''


import sys
import hashlib
import urllib.request
import json
from urllib.parse import urlencode, quote_plus
from pprint import pprint

query_text = sys.stdin.read()
src_lang = 'auto'
dst_lang = 'zh-CHS'
app_key = '019c4cc9f450f060'
app_secret = 'b5JkHPoTHbDOZqo3X3GsnZ9ExdecSfkr'
salt='hex'
sign_str = app_key+query_text+salt+app_secret
sign = hashlib.md5(sign_str.encode()).hexdigest()

url_args = {
        'q': query_text,
        'from': src_lang,
        'to': dst_lang,
        'appKey': app_key,
        'salt': salt,
        'sign': sign,
}

url_args_str = urlencode(url_args)
url = 'https://openapi.youdao.com/api?'+url_args_str

def translate():
    with urllib.request.urlopen(url) as response:
        data = json.loads(response.read().decode())
        print(data['translation'][0], end='')

if __name__ == '__main__':
    try:
        if len(query_text) != 0:
            translate()
    except Exception as e:
        err_msg = 'python script handles tweet input failed: {}'.format(e)
        print(e, file=sys.stderr)
        sys.exit(1)
