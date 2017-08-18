#!/usr/bin/env python3
# -*- encoding: utf-8 -*-

'''
Description:
    This script aims to retrieve text and image and post it.
    The currrent hard code screen_name is `7kilo_` whose twitter web address is
    https://twitter.com/7kilo_.

Input: str in json format:
    {
        "text": "tweet text which could be empty",
        "image": "tweet image encoded in base64 which could be empty"
    }
Output: None
'''

import sys
import base64
import json
import tempfile
import imghdr
from pprint import pprint

import tweepy
from tweepy import Stream
from tweepy import OAuthHandler
from tweepy.streaming import StreamListener

consumer_key='CxxA37fDXWlO4nTf57qu3QmI8'
consumer_secret='ADW3q82AMPZ0cREUFFsQIQ2sPFnCaCnZZYjXqfDUcrlITaUoqB'
access_token_key='888426634268745728-eO45HlfeiuETbGai6Cz9PbH1JhaYsBz'
access_token_secret='tLjtUSEleShoiu0RPrKlNunnXp4ijH3ufmfjb3lyKoZMd'

auth = OAuthHandler(consumer_key, consumer_secret)
auth.set_access_token(access_token_key, access_token_secret)

api = tweepy.API(auth)


def read_json():
    d = json.loads(sys.stdin.read())
    return d.get('text'), d.get('image')


def post_tweet(text, image):
    if text is not None and len(text) > 140:
        raise Exception("text length is greater than 140 characters")

    if image is None:
        api.update_status(status=text)
    else:
        img = base64.b64decode(image)
        fp = tempfile.NamedTemporaryFile()
        fp.write(img)
        img_type = imghdr.what(fp.name)
        if img_type is None:
            fp.close()
            raise Exception("bad photo format")

        fp.seek(0)
        api.update_with_media(filename='x.'+img_type, status=text, file=fp)
        fp.close()


if __name__ == '__main__':
    try:
        text, image = read_json()
        if text is None and image is None:
            raise Exception('text/image are all empty in input json')
        post_tweet(text, image)
    except Exception as e:
        print(e, file=sys.stderr)
        sys.exit(1)

