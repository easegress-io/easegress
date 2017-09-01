#!/usr/bin/env python3
# -*- encoding: utf-8 -*-

'''
Description:
    This script aims to translate a input text into Chinese.
    Currently we use [MyMemory Translator](http://mymemory.translated.net/doc/spec.php) as the translate service.
    It is free, anonymous usage is limited to 1000 words/day.

Input: text needed to be translated
Output: translated text:
'''


import sys
from translate import Translator

query_text = sys.stdin.read()
translator = Translator(to_lang="zh")

if __name__ == '__main__':
    try:
        if query_text != None and len(query_text) != 0:
            text = translator.translate(query_text)
            print(text, end='')
    except Exception as e:
        err_msg = 'python script translate failed: {}'.format(e)
        print(err_msg, file=sys.stderr)
        sys.exit(1)
