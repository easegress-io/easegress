#!/usr/bin/env python3
# -*- encoding: utf-8 -*-


'''
Description:
    This script aims to parse tweet img url field from one tweet
Input: one tweet in json format
    {
        "text": "tweet text which could be empty",
        "image_url": "tweet image url"
        "user_name": "the tweet user name",
        "created_at": "Fri Aug 25 16:17:35 2017"
    }

Output: speech filename in text format
    news@08-25-03:37
'''

import calendar
from datetime import datetime, timedelta
import json
import string
import sys


def utc_to_local(utc_dt):
    # get integer timestamp to avoid precision lost
    timestamp = calendar.timegm(utc_dt.timetuple())
    local_dt = datetime.fromtimestamp(timestamp)
    return local_dt.replace(microsecond=utc_dt.microsecond)


if __name__ == '__main__':
    try:
        tweet = json.loads(sys.stdin.read())
        created_at = tweet.get('created_at')
        dt = datetime.strptime(created_at, '%c')
        dt = utc_to_local(dt)

        filename = 'NEWS@'+ dt.strftime('%b%d-%H:%M')
        print(filename, end='')
    except Exception as e:
        err_msg = 'python script parse tweet text failed: {}'.format(e)
        print(err_msg, file=sys.stderr)
        sys.exit(1)
