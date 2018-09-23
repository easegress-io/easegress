#!/usr/bin/env python3
# -*- encoding: utf-8 -*-

'''
Description:
    This script aims to crawl a tweet and retrieve pure text and image from it.
    The currrent hard code screen_name are mega_ease and CNN Breaking News.
    https://twitter.com/mega_ease.
    https://twitter.com/cnnbrk

Input: None
Output: str in json format:
    {
        "text": "tweet text which could be empty",
        "image_url": "tweet image url",
        "user_name": "the_tweet_user_name",
        "created_at": "Fri Aug 25 16:17:35 2017"
    }
'''

import base64
import json
import sys
import urllib.request

import tweepy
from tweepy import Stream
from tweepy import OAuthHandler
from tweepy.streaming import StreamListener

consumer_key='lkGINFcD7UbR1SO911TJxUAUS'
consumer_secret='c2SsylCrt06EaIMR1ULP517CwumYjQkzlX5NFWhdMPAoVjZwHK'
access_token_key='888426634268745728-rmkfSKDVKqwZLceUeNrZXQKIMp6UaUr'
access_token_secret='ND0C6OiKehgsG91bwzGzPtBpHzfCuhNeyQRbPhJwDchcd'

auth = OAuthHandler(consumer_key, consumer_secret)
auth.set_access_token(access_token_key, access_token_secret)

api = tweepy.API(auth)


class MyListener(StreamListener):
    def on_connect(self):
        print('on_connect')
        return True

    def on_disconnect(self, notice):
        print('on_disconnect:', notice)
        return False

    def on_status(self, status):
        send_out_status(status)
        return True

    def on_error(self, status_code):
        print('on_error:', status_code)
        return True


def wait_new_tweet(user_ids):
    stream = Stream(auth, MyListener())
    stream.filter(follow=user_ids)


def send_out_status(status):
    d = {
            'text': '',
            'image_url': '',
            'user_name': '',
            'created_at': '',
    }

    if hasattr(status, 'text'):
        d['text'] = status.text

    if hasattr(status, 'entities'):
        entities = status.entities
        if entities is not None:
            media = entities.get('media')
            if media is not None and len(media) > 0:
                image_url = media[0].get('media_url_https')
                if image_url is not None:
                    d['image_url'] = image_url

    if hasattr(status, 'user'):
        user = status.user
        if hasattr(user, 'name'):
            d['user_name'] = user.name

    if hasattr(status, 'created_at'):
        d['created_at'] = status.created_at.strftime('%c')


    data = json.dumps(d).encode()
    headers = { 'Content-Type': 'application/json' }
    req = urllib.request.Request(url='http://127.0.0.1:10080/upload_image_tweet',
            method='POST', data=data, headers=headers)
    print('sending req:', req)
    try:
        with urllib.request.urlopen(req) as f:
            pass
        print('received response status code:', f.status)
    except Exception as e:
        print('urlopen failed:', e)


def main():
    try:
        mega_ease = api.get_user(screen_name='mega_ease')
        cnnbrk = api.get_user(screen_name='cnnbrk')
        users = [mega_ease, cnnbrk]

        wait_new_tweet([user.id_str for user in users])
    except Exception as e:
        err_msg = 'python script handles tweet input failed: {}'.format(e)
        print(err_msg, file=sys.stderr)
        sys.exit(1)

if __name__ == '__main__':
    main()
