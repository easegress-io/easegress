# pull images
docker pull megaease/easegress:latest


# run container
docker run --add-host=host.docker.internal:host-gateway \
    --name benchmark-easegress \
    -v `pwd`/config.yaml:/opt/easegress/config.yaml \
    -p 18080:8080 \
    megaease/easegress:latest


# clean up
docker rm benchmark-easegress


# create proxy
docker exec -it benchmark-easegress /opt/easegress/bin/egctl apply -f /opt/easegress/config.yaml


# test proxy
curl http://127.0.0.1:18080/v1/chat/completions -X POST \
    -H "Content-Type: application/json" \
    -d '{"model": "gpt", "stream": false}' | jq .


# hey
hey -n 1000 -c 50 -m POST \
   -H "Content-Type: application/json" \
   -d '{"model": "gpt", "stream": false}' \
   http://127.0.0.1:18080/v1/chat/completions