#!/bin/bash
mkdir -p /var/loghog/$LOGHOG_CONTAINER_ID
CONFIG_FILE=/var/loghog/$LOGHOG_CONTAINER_ID/config.json
LOG_FILE=/var/loghog/$LOGHOG_CONTAINER_ID/logstash-agent.log
cat > $CONFIG_FILE <<EOM
{
  "network": {
    "servers": [ "${LOGSTASH_HOST}" ],
    "timeout": 15,
    "ssl certificate": "/app/logstash-forwarder.crt",
    "ssl ca": "/app/logstash-forwarder.crt",
    "ssl key": "/app/logstash-forwarder.key",
    "tls host": "${LOGSTASH_TLS_HOST}"
  },
  "files": [
    {
      "paths": [
        "-"
      ],
      "fields": { 
	  	"type": "${LOGSTASH_TYPE}", 
		 "forwarder_tag": "service",
		 "public_hostname": "${LOGSTASH_PUBLIC_HOSTNAME}",
		 "host": "${LOGHOG_HOSTNAME}"
		}
	}
  ]
}
EOM
while true
do
	echo "Starting forwarder for $LOGHOG_CONTAINER_ID"
	/app/logstash-forwarder --config $CONFIG_FILE  &>> $LOG_FILE
	echo "Forwarder for $LOGHOG_CONTAINER_ID exited unexpectedly"
	sleep 5
done

