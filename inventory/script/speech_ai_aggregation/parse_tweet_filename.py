#!/usr/bin/env python3
# -*- encoding: utf-8 -*-


'''
Description:
    This script aims to parse tweet img url field from one tweet
Input: one tweet in json format
    {
        "text": "tweet text which could be empty",
        "image_url": "tweet image url",
        "user_name": "the tweet user name",
        "created_at": "Fri Aug 25 16:17:35 2017"
    }
Output: speech filename in text format
    news@08-25-03:37
'''

import calendar
from datetime import datetime
from dateutil import tz
import json
import string
import sys

def utc_to_china(utc_dt):
    from_zone = tz.gettz('UTC')
    to_zone = tz.gettz('Asia/Chongqing')

    utc_dt = utc_dt.replace(tzinfo=from_zone)
    return utc_dt.astimezone(to_zone)


if __name__ == '__main__':
    try:
        tweet = json.loads(sys.stdin.read())
        created_at = tweet.get('created_at')
        dt = datetime.strptime(created_at, '%c')
        dt = utc_to_china(dt)

        filename = 'NEWS@'+ dt.strftime('%b%d-%H:%M')
        print(filename, end='')
    except Exception as e:
        err_msg = 'python script parse tweet text failed: {}'.format(e)
        print(err_msg, file=sys.stderr)
        sys.exit(1)
