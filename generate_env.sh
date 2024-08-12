#!/usr/bin/env bash

if [ -e '.env' ]; then
    echo "Found existing .env file. Delete and re-run to regenerate!"
    exit 0
fi

touch .env
chmod 0600 .env

echo "JWT_SECRET=`openssl rand -base64 64 | tr -d '\n'`" >> .env
