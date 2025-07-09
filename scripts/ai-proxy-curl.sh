#!/bin/bash

# After creating ai-gateway-server and ai-gateway-pipeline,
# we can use this kind of example to test the AI proxy server.

curl http://127.0.0.1:8080/v1/chat/completions -H "Content-Type: application/json" -d '{
    "model": "gpt-4o",
    "messages": [
      {"role": "user", "content": "Hello, who are you?"}
    ],
    "max_tokens": 100
  }'
