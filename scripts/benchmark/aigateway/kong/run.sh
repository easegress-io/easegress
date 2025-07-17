# pull images
docker pull kong/kong-gateway:3.11.0.0


# run container
docker run --name benchmark-kong \
    -e "KONG_DATABASE=off" \
    -e "KONG_PROXY_ACCESS_LOG=/dev/stdout" \
    -e "KONG_ADMIN_ACCESS_LOG=/dev/stdout" \
    -e "KONG_PROXY_ERROR_LOG=/dev/stderr" \
    -e "KONG_ADMIN_ERROR_LOG=/dev/stderr" \
    -e "KONG_ADMIN_LISTEN=0.0.0.0:8001" \
    -p 8000:8000 \
    -p 8001:8001 \
    kong/kong-gateway:3.11.0.0
# If everything went well, and if you created your container with the default ports, 
# Kong Gateway should be listening on your host's 8000 (Proxy⁠), 8443 (Proxy SSL⁠), 
# 8001 (Admin API⁠) and 8444 (Admin API SSL⁠) ports.


# clean container
docker rm benchmark-kong


# create proxy
curl -i -X POST http://localhost:8001/config \
  -F "config=@kong.config.yaml"


# test proxy
curl -X POST "http://localhost:8000/chat" \
    -H "Accept: application/json" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer xxx" \
    --json '{
    "model": "gpt-4",
    "messages": [
        {
        "role": "user",
        "content": "Say this is a test!"
        }
    ]
    }' -v 


# hey
hey -n 2000 -c 50 -m POST \
   -H "Content-Type: application/json" \
   -d '{
    "model": "gpt-4",
    "messages": [
        {
        "role": "user",
        "content": "Say this is a test!"
        }
    ]
    }' \
   "http://localhost:8000/chat"