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

import sys
import base64
import json
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


store_path = './.tweet_since_id'
def get_since_id(user_id):
    try:
        with open(store_path) as f:
            user_since_ids = json.loads(f.read())
            return user_since_ids.get(user_id)
    except:
        return None

def set_since_id(user_id, since_id):
    with open(store_path, 'w') as f:
        try:
            user_since_ids = json.loads(f.read())
        except:
            user_since_ids = {}
        user_since_ids[user_id] = since_id
        f.write(json.dumps(user_since_ids))


def wait_new_tweet(user_ids):
    stream = Stream(auth, MyListener())
    stream.filter(follow=user_ids)


def get_existed_tweet(user_since_ids):
    for user_id, since_id in user_since_ids.items():
        statuses = api.user_timeline(user_id=user_id, since_id=since_id)
        if statuses is None or len(statuses) == 0:
            continue

        # NOTICE: Here it doesn't try to find the exactly nearest since_id status.
        return statuses[-1]

    return None


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


    print(json.dumps(d))
    set_since_id(status.user.id_str, status.id)
    sys.exit(0)


def quick_test():
    mega_ease = api.get_user(screen_name='mega_ease')
    user_since_ids = {
            mega_ease.id_str: 897843917890043904
    }
    status = get_existed_tweet(user_since_ids)
    send_out_status(status)

if __name__ == '__main__':
    try:
        mega_ease = api.get_user(screen_name='mega_ease')
        cnnbrk = api.get_user(screen_name='cnnbrk')
        users = [mega_ease, cnnbrk]

        user_since_ids = {}
        user_ids = []
        for user in users:
            user_ids.append(user.id_str)
            since_id = get_since_id(user.id_str)
            if since_id is not None:
                user_since_ids[user.id_str] = since_id

        if len(user_since_ids) == 0:
            wait_new_tweet(user_ids)
        else:
            status = get_existed_tweet(user_since_ids)
            if status == None:
                wait_new_tweet(user_ids)
            else:
                send_out_status(status)
    except Exception as e:
        err_msg = 'python script handles tweet input failed: {}'.format(e)
        print(err_msg, file=sys.stderr)
        sys.exit(1)
