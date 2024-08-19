#!/usr/bin/env bash

if [ -e '.env' ]; then
    echo "Found existing .env file. Delete and re-run to regenerate!"
    exit 0
fi

touch .env
chmod 0600 .env

echo "JWT_SECRET=`openssl rand -base64 64 | tr -d '\n'`" >> .env
# this is a publicly accessible key. We would keep it secret if running in a production environment
echo "POLKA_API_KEY=f271c81ff7084ee5b99a5091b42d486e" >> .env
