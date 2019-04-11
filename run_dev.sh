#!/bin/bash
docker-compose up -d
docker-compose exec  loghog /bin/bash
sudo chown -R $USER:$USER .
