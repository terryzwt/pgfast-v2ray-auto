FROM golang:1.17-alpine
MAINTAINER zterry <zterry@qq.com>
ENV V2RAY_VERSION "4.31.0"
ADD supervisord.conf /etc/supervisord.conf
ADD pgfast /go/

RUN apk add --no-cache git supervisor && \
    wget -O ./v2ray.zip https://github.com/v2fly/v2ray-core/releases/download/v${V2RAY_VERSION}/v2ray-linux-64.zip > /dev/null 2>&1 && \
    unzip v2ray.zip && chmod +x v2ray v2ctl && \
    mkdir -p /usr/local/share/v2ray/ /etc/v2ray && \
    mv v2ray v2ctl /usr/bin/ && \
    mv geosite.dat geoip.dat /usr/local/share/v2ray/ && \
    mv config.json /etc/v2ray/config.json  && \
    rm v2ray.zip vpoint_vmess_freedom.json vpoint_socks_vmess.json

EXPOSE 4080
EXPOSE 5080

CMD ["/usr/bin/supervisord", "-c", "/etc/supervisord.conf"]
