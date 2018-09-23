#!/usr/bin/env python3
# -*- encoding: utf-8 -*-

'''
Description:
    This script aims to read one chinese text and synthesize it into
    a section of speech.

Input: str in chinese language.
Output: speech in binary mp3 format.

'''

import random
import sys

from aip import AipSpeech


app_id = '9991806'
app_key = 'oDTkybkMMb8fxfZFKufGIWD4'
secret_key = 'm6uMS5wtzA05LXrz5U4AyGG4fLCENtMv'

api = AipSpeech(app_id, app_key, secret_key)

def synthesize(text):
    timbres = [0, 1, 3, 4]
    ops = {'per': timbres[random.randint(0, len(timbres) - 1)]}

    result = api.synthesis(text, 'zh', 1, ops)

    if not isinstance(result, dict):
        sys.stdout.buffer.write(result)
        sys.exit(0)
    else:
        raise Exception('server error return code {}'.format(result.get('err_no')))


if __name__ == '__main__':
    try:
        text = sys.stdin.read()
        synthesize(text)
    except Exception as e:
        err_msg = 'python script handles text input failed: {}'.format(e)
        print(e, file=sys.stderr)
        sys.exit(1)
