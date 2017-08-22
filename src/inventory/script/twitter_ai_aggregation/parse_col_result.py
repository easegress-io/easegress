#!/usr/bin/env python3
# -*- encoding: utf-8 -*-

'''
Description:
    This script aims to parse colorized result from Algorithma's colorize API
    https://algorithmia.com/algorithms/deeplearning/ColorfulImageColorization

Input: response body
    {
      "result":{
        "output":"data://.algo/deeplearning/ColorfulImageColorization/temp/DHuyYr-V0AAN6x2.png"},
        "metadata":{"content_type":"json","duration":17.489276079}
    }

Output: url suffix
        ".algo/deeplearning/ColorfulImageColorization/temp/lincoln.png",
'''

import sys
import json

if __name__ == '__main__':
    try:
        response = json.loads(sys.stdin.read())
        url_suffix = response.get('result').get('output')
        print(url_suffix[7:], end='')
    except Exception as e:
        err_msg = 'python script parse colorized result failed: {}'.format(e)
        print(err_msg, file=sys.stderr)
        sys.exit(1)
