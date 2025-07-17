# pull images
docker pull bitnami/etcd:3.4.9
docker pull apache/apisix:3.13.0-debian


# create docker containers
docker run -it --name benchmark-apisix-etcd \
-p 5379:2379 \
-p 5380:2380 \
--env ALLOW_NONE_AUTHENTICATION=yes bitnami/etcd:3.4.9

docker run --name benchmark-apisix \
 -v `pwd`/apisix.config.yaml:/usr/local/apisix/conf/config.yaml \
 -p 9080:9080 \
 -p 9180:9180 \
 apache/apisix:3.13.0-debian


# clean up docker containers
docker rm benchmark-apisix-etcd
docker rm benchmark-apisix


# create proxy
curl "http://127.0.0.1:9180/apisix/admin/routes/1" -X PUT \
  -H "X-API-KEY: iamadmin" \
  -d @llm-proxy.json


# list proxy
curl "http://127.0.0.1:9180/apisix/admin/routes" \
  -H "X-API-KEY: iamadmin" | jq .


# delete proxy by id
curl -X DELETE "http://127.0.0.1:9180/apisix/admin/routes/{id}" \
  -H "X-API-KEY: iamadmin" 


# test proxy
curl "http://127.0.0.1:9080/llm" -X POST \
  -H "Content-Type: application/json" \
  -H "Host: api.openai.com" \
  -d '{
    "messages": [
      { "role": "system", "content": "You are a mathematician" },
      { "role": "user", "content": "What is 1+1?" }
    ]
  }' | jq . 


# hey
hey -n 2000 -c 50 -m POST \
   -H "Content-Type: application/json" \
   -d '{
    "messages": [
      { "role": "system", "content": "You are a mathematician" },
      { "role": "user", "content": "What is 1+1?" }
    ]
  }' \
   http://127.0.0.1:9080/llm
