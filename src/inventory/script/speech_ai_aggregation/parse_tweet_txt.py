#!/usr/bin/env python3
# -*- encoding: utf-8 -*-

'''
Description:
    This script aims to parse tweet img url field from one tweet
Input: one tweet in json format
    {
        "text": "tweet text which could be empty",
        "image_url": "tweet image url"
    }

Output: text in text format
        "We cracked the GFW!"
'''

import sys
import json

if __name__ == '__main__':
    try:
        tweet = json.loads(sys.stdin.read())
        print(tweet.get('text'), end='')
    except Exception as e:
        err_msg = 'python script parse tweet text failed: {}'.format(e)
        print(err_msg, file=sys.stderr)
        sys.exit(1)
