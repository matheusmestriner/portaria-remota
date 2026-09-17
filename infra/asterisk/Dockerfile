FROM ubuntu:24.04
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends asterisk asterisk-config openssl ca-certificates && rm -rf /var/lib/apt/lists/*
COPY modules.conf /etc/asterisk/modules.conf
COPY pjsip.conf extensions.conf http.conf manager.conf queues.conf voicemail.conf rtp.conf /etc/asterisk/
COPY pjsip-clients.conf.example /etc/asterisk/pjsip-clients.conf.example
COPY extensions-clients.conf.example /etc/asterisk/extensions-clients.conf.example
COPY manager-clients.conf.example /etc/asterisk/manager-clients.conf.example
RUN chown -R asterisk:asterisk /etc/asterisk /var/lib/asterisk /var/log/asterisk
USER asterisk
EXPOSE 8088 8089 5060/udp 5061/tcp 10000-10100/udp
CMD ["asterisk","-f","-vvv"]
