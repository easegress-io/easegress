#!/usr/bin/env python3
# -*- encoding: utf-8 -*-

'''
Description:
    This script aims to crawl a tweet and retrieve pure text and image from it.
    The currrent hard code screen_name is mega_ease whose twitter web address is
    https://twitter.com/mega_ease.

Input: None
Output: str in json format:
    {
        "text": "tweet text which could be empty",
        "image": "tweet image encoded in base64 which could be empty"
    }
'''

import sys
import base64
import json
import urllib.request
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


class MyListener(StreamListener):
    def on_status(self, status):
        send_out_status(status)

    def on_error(self, status_code):
        return True


# TODO: EaseGateway should set workdir
store_path = './.tweet_since_id'
def get_since_id():
    try:
        with open(store_path) as f:
            since_id = int(f.read())
            return since_id
    except:
        return None

def set_since_id(since_id):
    with open(store_path, 'w') as f:
        f.write(str(since_id))


def wait_new_tweet(user_id):
    stream = Stream(auth, MyListener())
    stream.filter(follow=[user_id])


def get_existed_tweet(user_id, since_id):
    statuses = api.user_timeline(user_id=user_id, since_id=since_id)
    if len(statuses) == 0:
        return None

    # NOTICE: Here it doesn't try to find the exactly nearest since_id status.
    return statuses[-1]


def send_out_status(status):
    d = {'text': '', 'image': ''}
    d['text'] = status.text

    image_url = None
    entities = status.entities
    if entities is not None:
        media = entities.get('media')
        if media is not None and len(media) > 0:
            image_url = media[0].get('media_url_https')

    if image_url is not None:
        response = urllib.request.urlopen(image_url)
        image = response.read()
        d['image'] = base64.b64encode(image).decode()

    print(json.dumps(d))
    set_since_id(status.id)
    sys.exit(0)


def for_test():
    user = api.get_user(screen_name='mega_ease')
    status = get_existed_tweet(user.id_str, 897843965738663936)
    send_out_status(status)

if __name__ == '__main__':
    try:
        user = api.get_user(screen_name='mega_ease')
        since_id = get_since_id()
        if since_id is None:
            wait_new_tweet(user.id_str)
        else:
            status = get_existed_tweet(user.id_str, since_id)
            if status == None:
                wait_new_tweet(user.id_str)
            else:
                send_out_status(status)
    except Exception as e:
        print(e, file=sys.stderr)
        sys.exit(1)

