FROM golang:1.11
LABEL description="golang, gb ad a vim working environment"
MAINTAINER mowings@turbosquid.com
RUN go get github.com/constabulary/gb/...
VOLUME /app
ENV GOPATH=/app:/app/vendor
# Sets up my working env. YMMV
ENV TERM=xterm-256color
RUN apt-get -y update && apt-get -y install git vim wget tmux locales
RUN echo "en_US.UTF-8 UTF-8" > /etc/locale.gen && locale-gen
RUN cd /root && wget -q -O - https://gist.githubusercontent.com/mowings/1832c5acea6084e84ed9051edca42c36/raw/d51fda6b46f19fca603680368fe78756a5ae9395/setup.sh | /bin/bash
RUN cd /root && wget -q https://gist.githubusercontent.com/mowings/1832c5acea6084e84ed9051edca42c36/raw/d51fda6b46f19fca603680368fe78756a5ae9395/.vimrc
RUN cd /root && wget -q https://gist.githubusercontent.com/mowings/1832c5acea6084e84ed9051edca42c36/raw/d51fda6b46f19fca603680368fe78756a5ae9395/.tmux.conf
RUN echo >> /root/.vimrc && echo "set t_ut=" >> /root/.vimrc
RUN apt-get update
RUN apt-get install  -y curl sudo
WORKDIR /app
# Wait forever by default. We attach to a running instance of this container to use it
CMD exec /bin/bash -c "trap : TERM INT; sleep infinity & wait"
